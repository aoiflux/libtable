package partition

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"sort"
)

const (
	gptSig            = "EFI PART"
	gptProtectiveType = 0xEE
)

func parseGPT(img *imageReader) (*Table, error) {
	candidateSizes := []uint32{img.blockSize, 512, 1024, 2048, 4096, 8192}
	seen := make(map[uint32]struct{}, len(candidateSizes))
	var lastErr error

	for _, bs := range candidateSizes {
		if bs == 0 {
			continue
		}
		if _, ok := seen[bs]; ok {
			continue
		}
		seen[bs] = struct{}{}

		alt := *img
		alt.blockSize = bs

		t, err := parseGPTWithBlockSize(&alt)
		if err == nil {
			return t, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("%w: gpt parse failed", ErrInvalidTable)
	}
	return nil, lastErr
}

func parseGPTWithBlockSize(img *imageReader) (*Table, error) {
	t, err := parseGPTPrimary(img)
	if err == nil {
		return t, nil
	}
	return parseGPTSecondary(img)
}

func parseGPTPrimary(img *imageReader) (*Table, error) {
	mbr, err := img.readLBA(0)
	if err != nil {
		return nil, wrapInvalid(err, "gpt: read protective mbr")
	}
	if le16(mbr[510:512]) != 0xAA55 {
		return nil, fmt.Errorf("%w: gpt protective mbr magic missing", ErrInvalidTable)
	}
	if mbr[446+4] != gptProtectiveType {
		return nil, fmt.Errorf("%w: protective mbr partition type is not 0xEE", ErrInvalidTable)
	}

	return parseGPTAt(img, 1, false)
}

func parseGPTSecondary(img *imageReader) (*Table, error) {
	max := img.maxLBA()
	if max < 2 {
		return nil, fmt.Errorf("%w: image too small for backup gpt", ErrInvalidTable)
	}
	return parseGPTAt(img, max-1, true)
}

func parseGPTAt(img *imageReader, headerLBA uint64, isBackup bool) (*Table, error) {
	hdr, err := img.readLBA(headerLBA)
	if err != nil {
		return nil, wrapInvalid(err, "gpt: read header")
	}
	if !bytes.Equal(hdr[0:8], []byte(gptSig)) {
		return nil, fmt.Errorf("%w: gpt header signature missing", ErrInvalidTable)
	}

	headerSize := le32(hdr[12:16])
	if headerSize < 92 || uint64(headerSize) > uint64(img.blockSize) {
		return nil, fmt.Errorf("%w: gpt header size invalid", ErrInvalidTable)
	}

	if !img.gptDisableCRC {
		storedHeaderCRC := le32(hdr[16:20])
		headerForCRC := make([]byte, headerSize)
		copy(headerForCRC, hdr[:headerSize])
		for i := 16; i < 20 && i < len(headerForCRC); i++ {
			headerForCRC[i] = 0
		}
		computedHeaderCRC := crc32.ChecksumIEEE(headerForCRC)
		if computedHeaderCRC != storedHeaderCRC {
			return nil, fmt.Errorf("%w: gpt header crc mismatch", ErrInvalidTable)
		}
	}

	tableStart := le64(hdr[72:80])
	entryCount := le32(hdr[80:84])
	entrySize := le32(hdr[84:88])
	storedTableCRC := le32(hdr[88:92])
	if entrySize < 128 {
		return nil, fmt.Errorf("%w: gpt entry size too small", ErrInvalidTable)
	}
	if entryCount > 8192 {
		entryCount = 8192
	}
	totalEntryBytes := uint64(entryCount) * uint64(entrySize)
	tableBlocks := (totalEntryBytes + uint64(img.blockSize) - 1) / uint64(img.blockSize)

	if tableStart >= img.maxLBA() {
		return nil, fmt.Errorf("%w: gpt table start outside image", ErrInvalidTable)
	}
	if tableStart+tableBlocks > img.maxLBA() {
		return nil, fmt.Errorf("%w: gpt table extends past image bounds", ErrInvalidTable)
	}

	if totalEntryBytes > 0 {
		tableRaw, err := img.readLBAN(tableStart, tableBlocks)
		if err != nil {
			return nil, wrapInvalid(err, "gpt: read partition table for crc")
		}
		if !img.gptDisableCRC {
			computedTableCRC := crc32.ChecksumIEEE(tableRaw[:totalEntryBytes])
			if computedTableCRC != storedTableCRC {
				return nil, fmt.Errorf("%w: gpt entry table crc mismatch", ErrInvalidTable)
			}
		}
	}

	t := &Table{Type: TypeGPT, BlockSize: img.blockSize, Offset: img.offset, IsBackup: isBackup}
	if !isBackup {
		t.Partitions = append(t.Partitions, Partition{
			StartLBA:    0,
			LengthLBA:   1,
			TypeName:    "Safety Table",
			Flags:       PartFlagMeta,
			TableNumber: -1,
			SlotNumber:  -1,
		})
	}
	t.Partitions = append(t.Partitions, Partition{
		StartLBA:    headerLBA,
		LengthLBA:   (uint64(headerSize) + uint64(img.blockSize) - 1) / uint64(img.blockSize),
		TypeName:    "GPT Header",
		Flags:       PartFlagMeta,
		TableNumber: -1,
		SlotNumber:  -1,
	})
	t.Partitions = append(t.Partitions, Partition{
		StartLBA:    tableStart,
		LengthLBA:   tableBlocks,
		TypeName:    "Partition Table",
		Flags:       PartFlagMeta,
		TableNumber: -1,
		SlotNumber:  -1,
	})

	entriesPerBlock := uint32(img.blockSize) / entrySize
	if entriesPerBlock == 0 {
		return nil, fmt.Errorf("%w: gpt entry size larger than sector size", ErrInvalidTable)
	}

	var i uint32
	for block := uint64(0); i < entryCount; block++ {
		buf, err := img.readLBA(tableStart + block)
		if err != nil {
			return nil, wrapInvalid(err, "gpt: read entry block")
		}
		for off := uint32(0); off+entrySize <= uint32(len(buf)) && i < entryCount; off += entrySize {
			entry := buf[off : off+entrySize]
			start := le64(entry[32:40])
			if start == 0 {
				i++
				continue
			}
			end := le64(entry[40:48])
			if end < start {
				i++
				continue
			}
			name := utf16leToString(entry[56 : 56+72])
			t.Partitions = append(t.Partitions, Partition{
				StartLBA:    start,
				LengthLBA:   end - start + 1,
				TypeName:    name,
				Flags:       PartFlagAlloc,
				TableNumber: -1,
				SlotNumber:  int8(i),
				Attributes:  le64(entry[48:56]),
				GUIDType:    guidFromGPTBytes(entry[0:16]),
				GUIDUnique:  guidFromGPTBytes(entry[16:32]),
			})
			i++
		}
	}

	sort.Slice(t.Partitions, func(i, j int) bool { return t.Partitions[i].StartLBA < t.Partitions[j].StartLBA })
	addUnallocated(t, img.maxLBA())
	return t, nil
}
