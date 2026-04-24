package partition

import (
	"fmt"
	"sort"
)

const bsdMagic = 0x82564557

func parseBSD(img *imageReader) (*Table, error) {
	sector, err := img.readLBA(1)
	if err != nil {
		return nil, wrapInvalid(err, "bsd: read disklabel")
	}
	if len(sector) < 512 {
		return nil, fmt.Errorf("%w: bsd sector too small", ErrInvalidTable)
	}
	if le32(sector[0:4]) != bsdMagic || le32(sector[132:136]) != bsdMagic {
		return nil, fmt.Errorf("%w: bsd magic mismatch", ErrInvalidTable)
	}

	numParts := le16(sector[138:140])
	if numParts > 16 {
		numParts = 16
	}
	t := &Table{Type: TypeBSD, BlockSize: img.blockSize, Offset: img.offset}
	t.Partitions = append(t.Partitions, Partition{
		StartLBA:    1,
		LengthLBA:   1,
		TypeName:    "Partition Table",
		Flags:       PartFlagMeta,
		TableNumber: -1,
		SlotNumber:  -1,
	})

	max := img.maxLBA()
	base := 148
	for i := uint16(0); i < numParts; i++ {
		off := base + int(i)*16
		size := uint64(le32(sector[off : off+4]))
		start := uint64(le32(sector[off+4 : off+8]))
		fstype := sector[off+12]
		if size == 0 {
			continue
		}
		if i < 2 && start > max {
			return nil, fmt.Errorf("%w: bsd start out of image bounds", ErrInvalidTable)
		}
		t.Partitions = append(t.Partitions, Partition{
			StartLBA:    start,
			LengthLBA:   size,
			TypeCode:    uint64(fstype),
			TypeName:    bsdTypeName(fstype),
			Flags:       PartFlagAlloc,
			TableNumber: -1,
			SlotNumber:  int8(i),
		})
	}

	sort.Slice(t.Partitions, func(i, j int) bool { return t.Partitions[i].StartLBA < t.Partitions[j].StartLBA })
	addUnallocated(t, img.maxLBA())
	return t, nil
}

func bsdTypeName(t byte) string {
	switch t {
	case 0:
		return "Unused (0x00)"
	case 1:
		return "Swap (0x01)"
	case 2:
		return "Version 6 (0x02)"
	case 3:
		return "Version 7 (0x03)"
	case 4:
		return "System V (0x04)"
	case 5:
		return "4.1BSD (0x05)"
	case 6:
		return "Eighth Edition (0x06)"
	case 7:
		return "4.2BSD (0x07)"
	case 8:
		return "MSDOS (0x08)"
	case 9:
		return "4.4LFS (0x09)"
	case 10:
		return "Unknown (0x0A)"
	case 11:
		return "HPFS (0x0B)"
	case 12:
		return "ISO9660 (0x0C)"
	case 13:
		return "Boot (0x0D)"
	case 14:
		return "Vinum (0x0E)"
	default:
		return fmt.Sprintf("Unknown Type (0x%02x)", t)
	}
}
