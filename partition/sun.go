package partition

import (
	"fmt"
	"sort"
)

const (
	sunMagic  = 0xDABE
	sunSanity = 0x600DDEEE
)

func parseSun(img *imageReader) (*Table, error) {
	if t, err := parseSunSparcAt(img, 0); err == nil {
		return t, nil
	}
	if t, err := parseSunI386At(img, 0); err == nil {
		return t, nil
	}
	return parseSunI386At(img, 1)
}

func parseSunSparcAt(img *imageReader, lba uint64) (*Table, error) {
	buf, err := img.readLBA(lba)
	if err != nil {
		return nil, err
	}
	if be16(buf[508:510]) != sunMagic || be32(buf[188:192]) != sunSanity {
		return nil, fmt.Errorf("%w: sun sparc magic/sanity mismatch", ErrInvalidTable)
	}
	numParts := int(be16(buf[140:142]))
	if numParts > 8 {
		numParts = 8
	}
	secPerTr := uint64(be16(buf[436:438]))
	numHeads := uint64(be16(buf[434:436]))
	cylConv := secPerTr * numHeads

	t := &Table{Type: TypeSun, BlockSize: img.blockSize, Offset: img.offset}
	max := img.maxLBA()
	for i := 0; i < numParts; i++ {
		metaOff := 142 + i*4
		layoutOff := 444 + i*8
		ptype := be16(buf[metaOff : metaOff+2])
		startCyl := uint64(be32(buf[layoutOff : layoutOff+4]))
		size := uint64(be32(buf[layoutOff+4 : layoutOff+8]))
		if size == 0 {
			continue
		}
		start := cylConv * startCyl
		if i < 2 && start > max {
			return nil, fmt.Errorf("%w: sun sparc start out of bounds", ErrInvalidTable)
		}
		flags := PartFlagAlloc
		if ptype == 5 && start == 0 {
			flags = PartFlagMeta
		}
		t.Partitions = append(t.Partitions, Partition{
			StartLBA:    start,
			LengthLBA:   size,
			TypeCode:    uint64(ptype),
			TypeName:    sunTypeName(ptype),
			Flags:       flags,
			TableNumber: -1,
			SlotNumber:  int8(i),
		})
	}
	if len(t.Partitions) == 0 {
		return nil, fmt.Errorf("%w: sun sparc no partitions", ErrInvalidTable)
	}
	sort.Slice(t.Partitions, func(i, j int) bool { return t.Partitions[i].StartLBA < t.Partitions[j].StartLBA })
	addUnallocated(t, img.maxLBA())
	return t, nil
}

func parseSunI386At(img *imageReader, lba uint64) (*Table, error) {
	buf, err := img.readLBA(lba)
	if err != nil {
		return nil, err
	}
	if le16(buf[508:510]) != sunMagic || le32(buf[12:16]) != sunSanity {
		return nil, fmt.Errorf("%w: sun i386 magic/sanity mismatch", ErrInvalidTable)
	}
	numParts := int(le16(buf[30:32]))
	if numParts > 16 {
		numParts = 16
	}
	t := &Table{Type: TypeSun, BlockSize: img.blockSize, Offset: img.offset}
	max := img.maxLBA()
	for i := 0; i < numParts; i++ {
		off := 72 + i*12
		ptype := le16(buf[off : off+2])
		start := uint64(le32(buf[off+4 : off+8]))
		size := uint64(le32(buf[off+8 : off+12]))
		if size == 0 {
			continue
		}
		if i < 2 && start > max {
			return nil, fmt.Errorf("%w: sun i386 start out of bounds", ErrInvalidTable)
		}
		flags := PartFlagAlloc
		if ptype == 5 && start == 0 {
			flags = PartFlagMeta
		}
		t.Partitions = append(t.Partitions, Partition{
			StartLBA:    start,
			LengthLBA:   size,
			TypeCode:    uint64(ptype),
			TypeName:    sunTypeName(ptype),
			Flags:       flags,
			TableNumber: -1,
			SlotNumber:  int8(i),
		})
	}
	if len(t.Partitions) == 0 {
		return nil, fmt.Errorf("%w: sun i386 no partitions", ErrInvalidTable)
	}
	sort.Slice(t.Partitions, func(i, j int) bool { return t.Partitions[i].StartLBA < t.Partitions[j].StartLBA })
	addUnallocated(t, img.maxLBA())
	return t, nil
}

func sunTypeName(t uint16) string {
	switch t {
	case 0:
		return "Unassigned (0x00)"
	case 1:
		return "boot (0x01)"
	case 2:
		return "/ (0x02)"
	case 3:
		return "swap (0x03)"
	case 4:
		return "/usr/ (0x04)"
	case 5:
		return "backup (0x05)"
	case 6:
		return "stand (0x06)"
	case 7:
		return "/var/ (0x07)"
	case 8:
		return "/home/ (0x08)"
	case 9:
		return "alt sector (0x09)"
	case 10:
		return "cachefs (0x0A)"
	default:
		return fmt.Sprintf("Unknown Type (0x%04x)", t)
	}
}
