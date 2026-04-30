package partition

import (
	"fmt"
	"sort"
)

const mbrMaxExtendedDepth = 128

var mbrTypeNames = map[byte]string{
	0x00: "Empty",
	0x01: "DOS FAT12",
	0x05: "DOS Extended",
	0x06: "DOS FAT16",
	0x07: "NTFS / exFAT",
	0x0B: "Win95 FAT32",
	0x0C: "Win95 FAT32 LBA",
	0x0E: "Win95 FAT16 LBA",
	0x0F: "Win95 Extended",
	0x82: "Linux Swap / Solaris x86",
	0x83: "Linux",
	0x85: "Linux Extended",
	0x8E: "Linux LVM",
	0xA5: "BSD",
	0xA8: "Mac OS X",
	0xAB: "Mac OS X Boot",
	0xAF: "Mac OS X HFS",
	0xEE: "GPT Safety Partition",
	0xEF: "EFI System",
	0xFD: "Linux RAID",
}

func parseMBR(img *imageReader, testMode bool) (*Table, error) {
	sector, err := img.readLBA(0)
	if err != nil {
		return nil, wrapInvalid(err, "mbr: read primary sector")
	}
	if len(sector) < 512 {
		return nil, fmt.Errorf("%w: mbr sector too small", ErrInvalidTable)
	}
	if le16(sector[510:512]) != 0xAA55 {
		return nil, fmt.Errorf("%w: mbr signature missing", ErrInvalidTable)
	}

	if testMode {
		oem := string(sector[3:11])
		if hasPrefix(oem, "MSDOS") || hasPrefix(oem, "MSWIN") || hasPrefix(oem, "NTFS") || hasPrefix(oem, "FAT") {
			return nil, fmt.Errorf("%w: mbr test rejected likely filesystem boot sector", ErrInvalidTable)
		}
	}

	t := &Table{Type: TypeMBR, BlockSize: img.blockSize, Offset: img.offset}
	t.Partitions = append(t.Partitions, Partition{
		StartLBA:    0,
		LengthLBA:   1,
		TypeName:    "Primary Table (#0)",
		Flags:       PartFlagMeta,
		TableNumber: -1,
		SlotNumber:  -1,
	})

	added := false
	maxAddr := img.maxLBA()
	hasBounds := img.hasKnownSize()
	for i := 0; i < 4; i++ {
		off := 446 + i*16
		ptype := sector[off+4]
		start := uint64(le32(sector[off+8 : off+12]))
		size := uint64(le32(sector[off+12 : off+16]))
		if size == 0 {
			continue
		}
		if hasBounds && i < 2 && start > maxAddr {
			return nil, fmt.Errorf("%w: mbr partition start out of bounds", ErrInvalidTable)
		}
		added = true
		if isExtendedType(ptype) {
			t.Partitions = append(t.Partitions, Partition{
				StartLBA:    start,
				LengthLBA:   size,
				TypeCode:    uint64(ptype),
				TypeName:    mbrTypeName(ptype),
				Flags:       PartFlagMeta,
				TableNumber: 0,
				SlotNumber:  int8(i),
			})
			if err := parseExtendedChain(img, t, start, start, 1); err != nil {
				// Follow TSK behavior: keep going even if one EBR chain is malformed.
			}
			continue
		}
		t.Partitions = append(t.Partitions, Partition{
			StartLBA:    start,
			LengthLBA:   size,
			TypeCode:    uint64(ptype),
			TypeName:    mbrTypeName(ptype),
			Flags:       PartFlagAlloc,
			TableNumber: 0,
			SlotNumber:  int8(i),
		})
	}
	if !added {
		return nil, fmt.Errorf("%w: mbr has no valid entries", ErrInvalidTable)
	}

	sort.Slice(t.Partitions, func(i, j int) bool { return t.Partitions[i].StartLBA < t.Partitions[j].StartLBA })
	if hasBounds {
		addUnallocated(t, maxAddr)
	}
	return t, nil
}

func parseExtendedChain(img *imageReader, t *Table, sectCur, sectBase uint64, table int) error {
	if table > mbrMaxExtendedDepth {
		return fmt.Errorf("%w: extended partition chain too deep", ErrInvalidTable)
	}
	maxAddr := img.maxLBA()
	hasBounds := img.hasKnownSize()
	if hasBounds && (sectCur >= maxAddr || sectBase >= maxAddr) {
		return fmt.Errorf("%w: extended partition table outside image bounds", ErrInvalidTable)
	}

	sector, err := img.readLBA(sectCur)
	if err != nil {
		return err
	}
	if le16(sector[510:512]) != 0xAA55 {
		return fmt.Errorf("%w: invalid extended table signature", ErrInvalidTable)
	}

	t.Partitions = append(t.Partitions, Partition{
		StartLBA:    sectCur,
		LengthLBA:   1,
		TypeName:    fmt.Sprintf("Extended Table (#%d)", table),
		Flags:       PartFlagMeta,
		TableNumber: int8(table),
		SlotNumber:  -1,
	})

	for i := 0; i < 4; i++ {
		off := 446 + i*16
		ptype := sector[off+4]
		start := uint64(le32(sector[off+8 : off+12]))
		size := uint64(le32(sector[off+12 : off+16]))
		if start == 0 || size == 0 {
			continue
		}
		if isExtendedType(ptype) {
			next := sectBase + start
			if hasBounds && next >= maxAddr {
				return fmt.Errorf("%w: extended partition pointer outside image bounds", ErrInvalidTable)
			}
			for _, p := range t.Partitions {
				if p.StartLBA == next && p.Flags&PartFlagMeta != 0 {
					return fmt.Errorf("%w: loop in extended chain", ErrInvalidTable)
				}
			}
			t.Partitions = append(t.Partitions, Partition{
				StartLBA:    next,
				LengthLBA:   size,
				TypeCode:    uint64(ptype),
				TypeName:    mbrTypeName(ptype),
				Flags:       PartFlagMeta,
				TableNumber: int8(table),
				SlotNumber:  int8(i),
			})
			if err := parseExtendedChain(img, t, next, sectBase, table+1); err != nil {
				return err
			}
			continue
		}
		logicalStart := sectCur + start
		if hasBounds && logicalStart >= maxAddr {
			return fmt.Errorf("%w: logical partition start outside image bounds", ErrInvalidTable)
		}
		t.Partitions = append(t.Partitions, Partition{
			StartLBA:    logicalStart,
			LengthLBA:   size,
			TypeCode:    uint64(ptype),
			TypeName:    mbrTypeName(ptype),
			Flags:       PartFlagAlloc,
			TableNumber: int8(table),
			SlotNumber:  int8(i),
		})
	}
	return nil
}

func mbrTypeName(pt byte) string {
	if s, ok := mbrTypeNames[pt]; ok {
		return fmt.Sprintf("%s (0x%02x)", s, pt)
	}
	return fmt.Sprintf("Unknown Type (0x%02x)", pt)
}

func isExtendedType(pt byte) bool {
	return pt == 0x05 || pt == 0x0F || pt == 0x85
}

func hasPrefix(s, p string) bool {
	if len(s) < len(p) {
		return false
	}
	for i := 0; i < len(p); i++ {
		if s[i] != p[i] {
			return false
		}
	}
	return true
}
