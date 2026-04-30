package libtable

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestRootParseAndNew(t *testing.T) {
	disk := makeSimpleMBRDisk(t)

	parser := New()
	if parser == nil {
		t.Fatal("New returned nil parser")
	}

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeMBR})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if tbl.Type != TypeMBR {
		t.Fatalf("expected table type mbr, got %s", tbl.Type)
	}
}

func TestParseOptionalOffset(t *testing.T) {
	base := makeSimpleMBRDisk(t)
	prefix := make([]byte, 4096)
	disk := append(prefix, base...)

	if _, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeMBR}); err == nil {
		t.Fatal("expected parse at image start to fail for prefixed image")
	}

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeMBR}, uint64(len(prefix)))
	if err != nil {
		t.Fatalf("Parse with optional offset failed: %v", err)
	}
	if tbl.Type != TypeMBR {
		t.Fatalf("expected table type mbr, got %s", tbl.Type)
	}
	if tbl.Offset != uint64(len(prefix)) {
		t.Fatalf("expected table offset %d, got %d", len(prefix), tbl.Offset)
	}
}

func TestParseOptionalOffsetOverridesOptions(t *testing.T) {
	base := makeSimpleMBRDisk(t)
	prefix := make([]byte, 2048)
	disk := append(prefix, base...)

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeMBR, Offset: 1}, uint64(len(prefix)))
	if err != nil {
		t.Fatalf("Parse with optional offset override failed: %v", err)
	}
	if tbl.Offset != uint64(len(prefix)) {
		t.Fatalf("expected optional offset %d to override options offset, got %d", len(prefix), tbl.Offset)
	}
}

func TestParseWithoutKnownSize(t *testing.T) {
	disk := makeSimpleMBRDisk(t)

	tbl, err := Parse(bytes.NewReader(disk), 0, Options{Type: TypeMBR})
	if err != nil {
		t.Fatalf("Parse with unknown size failed: %v", err)
	}
	if tbl.Type != TypeMBR {
		t.Fatalf("expected table type mbr, got %s", tbl.Type)
	}
}

func TestParseWithoutKnownSizeWithOffset(t *testing.T) {
	base := makeSimpleMBRDisk(t)
	prefix := make([]byte, 1024)
	disk := append(prefix, base...)

	tbl, err := Parse(bytes.NewReader(disk), 0, Options{Type: TypeMBR}, uint64(len(prefix)))
	if err != nil {
		t.Fatalf("Parse with unknown size and offset failed: %v", err)
	}
	if tbl.Offset != uint64(len(prefix)) {
		t.Fatalf("expected table offset %d, got %d", len(prefix), tbl.Offset)
	}
}

func TestParseUnknownSizeConvenience(t *testing.T) {
	disk := makeSimpleMBRDisk(t)

	tbl, err := ParseUnknownSize(bytes.NewReader(disk), Options{Type: TypeMBR})
	if err != nil {
		t.Fatalf("ParseUnknownSize failed: %v", err)
	}
	if tbl.Type != TypeMBR {
		t.Fatalf("expected table type mbr, got %s", tbl.Type)
	}
}

func TestParseUnknownSizeConvenienceWithOffset(t *testing.T) {
	base := makeSimpleMBRDisk(t)
	prefix := make([]byte, 1536)
	disk := append(prefix, base...)

	tbl, err := ParseUnknownSize(bytes.NewReader(disk), Options{Type: TypeMBR}, uint64(len(prefix)))
	if err != nil {
		t.Fatalf("ParseUnknownSize with offset failed: %v", err)
	}
	if tbl.Offset != uint64(len(prefix)) {
		t.Fatalf("expected table offset %d, got %d", len(prefix), tbl.Offset)
	}
}

func makeSimpleMBRDisk(t *testing.T) []byte {
	t.Helper()
	disk := make([]byte, 8*1024*1024)
	mbr := disk[:512]
	binary.LittleEndian.PutUint16(mbr[510:512], 0xAA55)
	entry := mbr[446 : 446+16]
	entry[4] = 0x83
	binary.LittleEndian.PutUint32(entry[8:12], 2048)
	binary.LittleEndian.PutUint32(entry[12:16], 4096)
	return disk
}
