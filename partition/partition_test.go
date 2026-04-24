package partition

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func TestParseMBR(t *testing.T) {
	disk := buildMBRDisk()

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeMBR})
	if err != nil {
		t.Fatalf("parse mbr failed: %v", err)
	}
	if tbl.Type != TypeMBR {
		t.Fatalf("type mismatch: got %s", tbl.Type)
	}
	found := false
	for _, p := range tbl.Partitions {
		if p.Flags&PartFlagAlloc != 0 && p.StartLBA == 2048 && p.LengthLBA == 4096 {
			found = true
		}
	}
	if !found {
		t.Fatalf("allocated MBR partition not found")
	}
}

func TestParseMBRRejectsMalformedInputs(t *testing.T) {
	tests := []struct {
		name string
		disk func() []byte
	}{
		{
			name: "missing signature",
			disk: func() []byte {
				disk := buildMBRDisk()
				binary.LittleEndian.PutUint16(disk[510:512], 0)
				return disk
			},
		},
		{
			name: "out of bounds primary start",
			disk: func() []byte {
				disk := buildMBRDisk()
				entry := disk[446 : 446+16]
				binary.LittleEndian.PutUint32(entry[8:12], 0xFFFFFFFE)
				return disk
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectParseFails(t, tc.disk(), Options{Type: TypeMBR})
		})
	}
}

func TestParseMBRMalformedExtendedChainDoesNotFailWholeParse(t *testing.T) {
	disk := buildMBRWithBrokenExtendedChainDisk()

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeMBR})
	if err != nil {
		t.Fatalf("expected parser to recover from malformed extended chain, got error: %v", err)
	}
	if tbl.Type != TypeMBR {
		t.Fatalf("type mismatch: got %s", tbl.Type)
	}
}

func TestParseGPT(t *testing.T) {
	disk := buildGPTDisk()

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeGPT})
	if err != nil {
		t.Fatalf("parse gpt failed: %v", err)
	}
	if tbl.Type != TypeGPT {
		t.Fatalf("type mismatch: got %s", tbl.Type)
	}
	found := false
	for _, p := range tbl.Partitions {
		if p.Flags&PartFlagAlloc != 0 && p.StartLBA == 2048 && p.LengthLBA == 2048 {
			if p.TypeName != "rootfs" {
				t.Fatalf("unexpected gpt name: %q", p.TypeName)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("allocated GPT partition not found")
	}
}

func TestParseGPTWithBlockSizeFallback(t *testing.T) {
	disk := buildGPTDiskWithBlockSize(4096)

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeGPT})
	if err != nil {
		t.Fatalf("parse gpt with fallback failed: %v", err)
	}
	if tbl.Type != TypeGPT {
		t.Fatalf("type mismatch: got %s", tbl.Type)
	}
	if tbl.BlockSize != 4096 {
		t.Fatalf("expected detected block size 4096, got %d", tbl.BlockSize)
	}
}

func TestParseAutoDetectGPT(t *testing.T) {
	disk := buildGPTDisk()

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{})
	if err != nil {
		t.Fatalf("autodetect failed: %v", err)
	}
	if tbl.Type != TypeGPT {
		t.Fatalf("expected gpt from autodetect, got %s", tbl.Type)
	}
}

func TestParseAutoDetectMBR(t *testing.T) {
	disk := buildMBRDisk()

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{})
	if err != nil {
		t.Fatalf("autodetect failed: %v", err)
	}
	if tbl.Type != TypeMBR {
		t.Fatalf("expected mbr from autodetect, got %s", tbl.Type)
	}
}

func TestParseBSD(t *testing.T) {
	disk := buildBSDDisk()

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeBSD})
	if err != nil {
		t.Fatalf("parse bsd failed: %v", err)
	}
	if tbl.Type != TypeBSD {
		t.Fatalf("type mismatch: got %s", tbl.Type)
	}
	found := false
	for _, p := range tbl.Partitions {
		if p.Flags&PartFlagAlloc != 0 && p.StartLBA == 2048 && p.LengthLBA == 1024 {
			found = true
		}
	}
	if !found {
		t.Fatalf("allocated BSD partition not found")
	}
}

func TestParseBSDRejectsMalformedInputs(t *testing.T) {
	tests := []struct {
		name string
		disk func() []byte
	}{
		{
			name: "bad magic",
			disk: func() []byte {
				disk := buildBSDDisk()
				binary.LittleEndian.PutUint32(disk[512:516], 0)
				return disk
			},
		},
		{
			name: "out of bounds start",
			disk: func() []byte {
				disk := buildBSDDisk()
				entry := disk[512+148 : 512+148+16]
				binary.LittleEndian.PutUint32(entry[4:8], 0xFFFFFFFE)
				return disk
			},
		},
		{
			name: "truncated image",
			disk: func() []byte {
				return make([]byte, 512)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectParseFails(t, tc.disk(), Options{Type: TypeBSD})
		})
	}
}

func TestParseSunI386(t *testing.T) {
	disk := buildSunI386Disk()

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeSun})
	if err != nil {
		t.Fatalf("parse sun failed: %v", err)
	}
	if tbl.Type != TypeSun {
		t.Fatalf("type mismatch: got %s", tbl.Type)
	}
	found := false
	for _, p := range tbl.Partitions {
		if p.Flags&PartFlagAlloc != 0 && p.StartLBA == 4096 && p.LengthLBA == 2048 {
			found = true
		}
	}
	if !found {
		t.Fatalf("allocated SUN partition not found")
	}
}

func TestParseSunRejectsMalformedInputs(t *testing.T) {
	tests := []struct {
		name string
		disk func() []byte
	}{
		{
			name: "bad magic",
			disk: func() []byte {
				disk := buildSunI386Disk()
				binary.LittleEndian.PutUint16(disk[508:510], 0)
				return disk
			},
		},
		{
			name: "out of bounds start",
			disk: func() []byte {
				disk := buildSunI386Disk()
				part := disk[72 : 72+12]
				binary.LittleEndian.PutUint32(part[4:8], 0xFFFFFFFE)
				return disk
			},
		},
		{
			name: "truncated image",
			disk: func() []byte {
				return make([]byte, 256)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectParseFails(t, tc.disk(), Options{Type: TypeSun})
		})
	}
}

func TestParseMac(t *testing.T) {
	disk := buildMacDisk()

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeMac})
	if err != nil {
		t.Fatalf("parse mac failed: %v", err)
	}
	if tbl.Type != TypeMac {
		t.Fatalf("type mismatch: got %s", tbl.Type)
	}
	found := false
	for _, p := range tbl.Partitions {
		if p.Flags&PartFlagAlloc != 0 && p.StartLBA == 4096 && p.LengthLBA == 2048 {
			found = true
		}
	}
	if !found {
		t.Fatalf("allocated MAC partition not found")
	}
}

func TestParseMacRejectsMalformedInputs(t *testing.T) {
	tests := []struct {
		name string
		disk func() []byte
	}{
		{
			name: "bad magic",
			disk: func() []byte {
				disk := buildMacDisk()
				binary.BigEndian.PutUint16(disk[512:514], 0)
				return disk
			},
		},
		{
			name: "out of bounds start",
			disk: func() []byte {
				disk := buildMacDisk()
				entry := disk[512:1024]
				binary.BigEndian.PutUint32(entry[8:12], 0xFFFFFFFE)
				return disk
			},
		},
		{
			name: "truncated image",
			disk: func() []byte {
				return make([]byte, 512)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectParseFails(t, tc.disk(), Options{Type: TypeMac})
		})
	}
}

func TestParseGPTRejectsMalformedInputs(t *testing.T) {
	tests := []struct {
		name string
		disk func() []byte
	}{
		{
			name: "bad header crc",
			disk: func() []byte {
				disk := buildGPTDisk()
				bs := 512
				binary.LittleEndian.PutUint32(disk[bs+16:bs+20], 0)
				return disk
			},
		},
		{
			name: "bad entry table crc",
			disk: func() []byte {
				disk := buildGPTDisk()
				bs := 512
				entry := disk[2*bs : 3*bs]
				entry[56] ^= 0x01
				return disk
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectParseFails(t, tc.disk(), Options{Type: TypeGPT})
		})
	}
}

func TestParseGPTDisableCRCAcceptsBadCRC(t *testing.T) {
	disk := buildGPTDisk()
	bs := 512
	binary.LittleEndian.PutUint32(disk[bs+16:bs+20], 0)
	binary.LittleEndian.PutUint32(disk[bs+88:bs+92], 0)

	tbl, err := Parse(bytes.NewReader(disk), uint64(len(disk)), Options{Type: TypeGPT, GPTDisableCRC: true})
	if err != nil {
		t.Fatalf("expected parse success when CRC is disabled, got: %v", err)
	}
	if tbl.Type != TypeGPT {
		t.Fatalf("type mismatch: got %s", tbl.Type)
	}
}

func expectParseFails(t *testing.T, disk []byte, opts Options) {
	t.Helper()
	_, err := Parse(bytes.NewReader(disk), uint64(len(disk)), opts)
	if err == nil {
		t.Fatalf("expected parse failure")
	}
}

func buildMBRDisk() []byte {
	disk := make([]byte, 8*1024*1024)
	mbr := disk[:512]
	binary.LittleEndian.PutUint16(mbr[510:512], 0xAA55)
	entry := mbr[446 : 446+16]
	entry[4] = 0x83
	binary.LittleEndian.PutUint32(entry[8:12], 2048)
	binary.LittleEndian.PutUint32(entry[12:16], 4096)
	return disk
}

func buildMBRWithBrokenExtendedChainDisk() []byte {
	disk := make([]byte, 8*1024*1024)

	mbr := disk[:512]
	binary.LittleEndian.PutUint16(mbr[510:512], 0xAA55)
	prim := mbr[446 : 446+16]
	prim[4] = 0x05
	binary.LittleEndian.PutUint32(prim[8:12], 1)
	binary.LittleEndian.PutUint32(prim[12:16], 2048)

	ebr1 := disk[512 : 2*512]
	binary.LittleEndian.PutUint16(ebr1[510:512], 0xAA55)
	link1 := ebr1[446+16 : 446+32]
	link1[4] = 0x05
	binary.LittleEndian.PutUint32(link1[8:12], 1)
	binary.LittleEndian.PutUint32(link1[12:16], 1024)

	ebr2 := disk[2*512 : 3*512]
	binary.LittleEndian.PutUint16(ebr2[510:512], 0xAA55)
	link2 := ebr2[446+16 : 446+32]
	link2[4] = 0x05
	binary.LittleEndian.PutUint32(link2[8:12], 1)
	binary.LittleEndian.PutUint32(link2[12:16], 1024)

	return disk
}

func buildGPTDisk() []byte {
	return buildGPTDiskWithBlockSize(512)
}

func buildGPTDiskWithBlockSize(bs int) []byte {
	disk := make([]byte, 8*1024*1024)

	mbr := disk[:bs]
	binary.LittleEndian.PutUint16(mbr[510:512], 0xAA55)
	mbr[446+4] = 0xEE
	binary.LittleEndian.PutUint32(mbr[446+8:446+12], 1)
	binary.LittleEndian.PutUint32(mbr[446+12:446+16], 0xFFFFFFFF)

	h := disk[bs : 2*bs]
	copy(h[0:8], []byte("EFI PART"))
	binary.LittleEndian.PutUint32(h[8:12], 0x00010000)
	binary.LittleEndian.PutUint32(h[12:16], 92)
	binary.LittleEndian.PutUint64(h[24:32], 1)
	binary.LittleEndian.PutUint64(h[32:40], uint64(len(disk)/bs-1))
	binary.LittleEndian.PutUint64(h[40:48], 34)
	binary.LittleEndian.PutUint64(h[48:56], uint64(len(disk)/bs-34))
	binary.LittleEndian.PutUint64(h[72:80], 2)
	binary.LittleEndian.PutUint32(h[80:84], 128)
	binary.LittleEndian.PutUint32(h[84:88], 128)

	e := disk[2*bs : 3*bs]
	for i := 0; i < 16; i++ {
		e[i] = byte(i + 1)
	}
	for i := 0; i < 16; i++ {
		e[16+i] = byte(0x10 + i)
	}
	binary.LittleEndian.PutUint64(e[32:40], 2048)
	binary.LittleEndian.PutUint64(e[40:48], 4095)
	binary.LittleEndian.PutUint64(e[48:56], 0)
	name := []rune("rootfs")
	for i, r := range name {
		binary.LittleEndian.PutUint16(e[56+i*2:58+i*2], uint16(r))
	}

	setGPTCRCs(disk, bs)

	return disk
}

func setGPTCRCs(disk []byte, bs int) {
	header := disk[bs : 2*bs]
	entryCount := binary.LittleEndian.Uint32(header[80:84])
	entrySize := binary.LittleEndian.Uint32(header[84:88])
	tableStartLBA := binary.LittleEndian.Uint64(header[72:80])

	tableBytes := uint64(entryCount) * uint64(entrySize)
	tableOffset := int(tableStartLBA * uint64(bs))
	tableCRC := crc32.ChecksumIEEE(disk[tableOffset : tableOffset+int(tableBytes)])
	binary.LittleEndian.PutUint32(header[88:92], tableCRC)

	headerSize := binary.LittleEndian.Uint32(header[12:16])
	headerCRCInput := make([]byte, headerSize)
	copy(headerCRCInput, header[:headerSize])
	binary.LittleEndian.PutUint32(headerCRCInput[16:20], 0)
	headerCRC := crc32.ChecksumIEEE(headerCRCInput)
	binary.LittleEndian.PutUint32(header[16:20], headerCRC)
}

func buildBSDDisk() []byte {
	disk := make([]byte, 8*1024*1024)
	bs := 512

	label := disk[bs : 2*bs]
	binary.LittleEndian.PutUint32(label[0:4], 0x82564557)
	binary.LittleEndian.PutUint32(label[132:136], 0x82564557)
	binary.LittleEndian.PutUint16(label[138:140], 16)

	entry := label[148 : 148+16]
	binary.LittleEndian.PutUint32(entry[0:4], 1024)
	binary.LittleEndian.PutUint32(entry[4:8], 2048)
	entry[12] = 7

	return disk
}

func buildSunI386Disk() []byte {
	disk := make([]byte, 8*1024*1024)
	label := disk[:512]

	binary.LittleEndian.PutUint32(label[12:16], 0x600DDEEE)
	binary.LittleEndian.PutUint16(label[30:32], 16)
	binary.LittleEndian.PutUint16(label[508:510], 0xDABE)

	part := label[72 : 72+12]
	binary.LittleEndian.PutUint16(part[0:2], 2)
	binary.LittleEndian.PutUint32(part[4:8], 4096)
	binary.LittleEndian.PutUint32(part[8:12], 2048)

	return disk
}

func buildMacDisk() []byte {
	disk := make([]byte, 8*1024*1024)
	bs := 512
	entry := disk[bs : 2*bs]

	binary.BigEndian.PutUint16(entry[0:2], 0x504D)
	binary.BigEndian.PutUint32(entry[4:8], 1)
	binary.BigEndian.PutUint32(entry[8:12], 4096)
	binary.BigEndian.PutUint32(entry[12:16], 2048)
	copy(entry[16:48], []byte("MacPartition"))
	copy(entry[48:80], []byte("Apple_HFS"))
	binary.BigEndian.PutUint32(entry[80:84], 1)

	return disk
}
