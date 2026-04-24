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
