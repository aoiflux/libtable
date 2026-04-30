package partition

import (
	"fmt"
	"io"
)

func Parse(r io.ReaderAt, sizeBytes uint64, opts Options, tableOffset ...uint64) (*Table, error) {
	return New().Parse(r, sizeBytes, opts, tableOffset...)
}

func ParseUnknownSize(r io.ReaderAt, opts Options, tableOffset ...uint64) (*Table, error) {
	return Parse(r, 0, opts, tableOffset...)
}

func (p *Parser) Parse(r io.ReaderAt, sizeBytes uint64, opts Options, tableOffset ...uint64) (*Table, error) {
	if r == nil {
		return nil, fmt.Errorf("partition: nil reader")
	}
	if len(tableOffset) > 1 {
		return nil, fmt.Errorf("partition: at most one table offset may be provided")
	}
	if len(tableOffset) == 1 {
		opts.Offset = tableOffset[0]
	}

	img := newImageReader(r, sizeBytes, opts)
	switch opts.Type {
	case TypeMBR:
		return parseMBR(img, false)
	case TypeGPT:
		return parseGPT(img)
	case TypeBSD:
		return parseBSD(img)
	case TypeSun:
		return parseSun(img)
	case TypeMac:
		return parseMac(img)
	case "", TypeUnknown:
		return autoDetect(img)
	default:
		return nil, fmt.Errorf("partition: unsupported requested type %q", opts.Type)
	}
}

func (p *Parser) ParseUnknownSize(r io.ReaderAt, opts Options, tableOffset ...uint64) (*Table, error) {
	return p.Parse(r, 0, opts, tableOffset...)
}

func autoDetect(img *imageReader) (*Table, error) {
	var selected *Table
	var selectedType TableType

	accept := func(next *Table, nextType TableType) error {
		if selected == nil {
			selected = next
			selectedType = nextType
			return nil
		}
		return fmt.Errorf("partition: multiple partition systems detected: %s or %s", nextType, selectedType)
	}

	if t, err := parseMBR(img, true); err == nil {
		selected = t
		selectedType = TypeMBR
	}

	if t, err := parseBSD(img); err == nil {
		selected = t
		selectedType = TypeBSD
	}

	if t, err := parseGPT(img); err == nil {
		if selectedType == TypeMBR && t.IsBackup {
			// Keep MBR if GPT parse only succeeded from the secondary header.
		} else {
			if selectedType == TypeMBR {
				if mbrLooksLikeProtectiveForGPT(selected) {
					selected = nil
					selectedType = TypeUnknown
				} else {
					return nil, fmt.Errorf("partition: multiple partition systems detected: gpt or %s", selectedType)
				}
			}
			if err := accept(t, TypeGPT); err != nil {
				return nil, err
			}
		}
	}

	if t, err := parseSun(img); err == nil {
		if err := accept(t, TypeSun); err != nil {
			return nil, err
		}
	}

	if t, err := parseMac(img); err == nil {
		if err := accept(t, TypeMac); err != nil {
			return nil, err
		}
	}

	if selected == nil {
		return nil, ErrUnknownTable
	}
	return selected, nil
}

func mbrLooksLikeProtectiveForGPT(t *Table) bool {
	for _, p := range t.Partitions {
		if p.Flags&PartFlagAlloc == 0 {
			continue
		}
		if p.TypeCode == 0xEE && p.StartLBA <= 63 {
			return true
		}
	}
	return false
}

func addUnallocated(t *Table, maxLBA uint64) {
	if len(t.Partitions) == 0 {
		return
	}
	parts := make([]Partition, 0, len(t.Partitions)+4)
	cur := uint64(0)
	for _, p := range t.Partitions {
		if p.Flags&PartFlagMeta != 0 {
			continue
		}
		if p.StartLBA > cur {
			parts = append(parts, Partition{
				StartLBA:    cur,
				LengthLBA:   p.StartLBA - cur,
				TypeName:    "Unallocated",
				Flags:       PartFlagUnalloc,
				TableNumber: -1,
				SlotNumber:  -1,
			})
		}
		end := p.StartLBA + p.LengthLBA
		if end > cur {
			cur = end
		}
		parts = append(parts, p)
	}
	if cur < maxLBA {
		parts = append(parts, Partition{
			StartLBA:    cur,
			LengthLBA:   maxLBA - cur,
			TypeName:    "Unallocated",
			Flags:       PartFlagUnalloc,
			TableNumber: -1,
			SlotNumber:  -1,
		})
	}
	for i := range parts {
		parts[i].Index = i
	}
	t.Partitions = parts
}

func wrapInvalid(err error, what string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %s (%v)", ErrInvalidTable, what, err)
}
