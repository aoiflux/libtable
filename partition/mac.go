package partition

import (
	"fmt"
	"sort"
)

const macMagic = 0x504D

func parseMac(img *imageReader) (*Table, error) {
	t, err := parseMacWithBlock(img)
	if err == nil {
		return t, nil
	}
	// TSK-style fallback between 512 and 4096 sector sizes.
	if img.blockSize == 512 || img.blockSize == 4096 {
		alt := *img
		if img.blockSize == 512 {
			alt.blockSize = 4096
		} else {
			alt.blockSize = 512
		}
		return parseMacWithBlock(&alt)
	}
	return nil, err
}

func parseMacWithBlock(img *imageReader) (*Table, error) {
	first, err := img.readLBA(1)
	if err != nil {
		return nil, wrapInvalid(err, "mac: read first map entry")
	}
	if be16(first[0:2]) != macMagic {
		return nil, fmt.Errorf("%w: mac magic mismatch", ErrInvalidTable)
	}
	count := be32(first[4:8])
	if count == 0 {
		return nil, fmt.Errorf("%w: mac map size is zero", ErrInvalidTable)
	}

	t := &Table{Type: TypeMac, BlockSize: img.blockSize, Offset: img.offset}
	max := img.maxLBA()
	hasBounds := img.hasKnownSize()
	for i := uint32(0); i < count; i++ {
		buf, err := img.readLBA(1 + uint64(i))
		if err != nil {
			return nil, wrapInvalid(err, "mac: read map entry")
		}
		if be16(buf[0:2]) != macMagic {
			return nil, fmt.Errorf("%w: mac entry magic mismatch", ErrInvalidTable)
		}
		start := uint64(be32(buf[8:12]))
		length := uint64(be32(buf[12:16]))
		status := be32(buf[80:84])
		if length == 0 {
			continue
		}
		if hasBounds && i < 2 && start > max {
			return nil, fmt.Errorf("%w: mac start out of image bounds", ErrInvalidTable)
		}
		flags := PartFlagAlloc
		if status == 0 {
			flags = PartFlagUnalloc
		}
		t.Partitions = append(t.Partitions, Partition{
			StartLBA:    start,
			LengthLBA:   length,
			TypeName:    trimASCIIZ(buf[48:80]),
			Name:        trimASCIIZ(buf[16:48]),
			Flags:       flags,
			TableNumber: -1,
			SlotNumber:  int8(i),
		})
	}
	if len(t.Partitions) == 0 {
		return nil, fmt.Errorf("%w: no mac partitions found", ErrInvalidTable)
	}
	t.Partitions = append(t.Partitions, Partition{
		StartLBA:    1,
		LengthLBA:   uint64(count),
		TypeName:    "Table",
		Flags:       PartFlagMeta,
		TableNumber: -1,
		SlotNumber:  -1,
	})
	sort.Slice(t.Partitions, func(i, j int) bool { return t.Partitions[i].StartLBA < t.Partitions[j].StartLBA })
	if hasBounds {
		addUnallocated(t, max)
	}
	return t, nil
}
