package partition

import (
	"encoding/binary"
	"fmt"
	"io"
)

type imageReader struct {
	r             io.ReaderAt
	sizeBytes     uint64
	sizeKnown     bool
	offset        uint64
	blockSize     uint32
	gptDisableCRC bool
}

func newImageReader(r io.ReaderAt, sizeBytes uint64, opts Options) *imageReader {
	opts = opts.withDefaults()
	return &imageReader{
		r:             r,
		sizeBytes:     sizeBytes,
		sizeKnown:     sizeBytes > 0,
		offset:        opts.Offset,
		blockSize:     opts.BlockSize,
		gptDisableCRC: opts.GPTDisableCRC,
	}
}

func (ir *imageReader) readAt(buf []byte, absOff uint64) error {
	_, err := ir.r.ReadAt(buf, int64(absOff))
	if err != nil {
		return err
	}
	return nil
}

func (ir *imageReader) readLBA(lba uint64) ([]byte, error) {
	buf := make([]byte, ir.blockSize)
	abs := ir.offset + lba*uint64(ir.blockSize)
	if ir.sizeKnown && abs+uint64(len(buf)) > ir.sizeBytes {
		return nil, io.EOF
	}
	if err := ir.readAt(buf, abs); err != nil {
		return nil, err
	}
	return buf, nil
}

func (ir *imageReader) readLBAN(lba uint64, count uint64) ([]byte, error) {
	total := count * uint64(ir.blockSize)
	buf := make([]byte, total)
	abs := ir.offset + lba*uint64(ir.blockSize)
	if ir.sizeKnown && abs+uint64(len(buf)) > ir.sizeBytes {
		return nil, io.EOF
	}
	if err := ir.readAt(buf, abs); err != nil {
		return nil, err
	}
	return buf, nil
}

func (ir *imageReader) maxLBA() uint64 {
	if !ir.sizeKnown || ir.blockSize == 0 || ir.sizeBytes <= ir.offset {
		return 0
	}
	return (ir.sizeBytes - ir.offset) / uint64(ir.blockSize)
}

func (ir *imageReader) hasKnownSize() bool {
	return ir.sizeKnown
}

func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func le32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }
func le64(b []byte) uint64 { return binary.LittleEndian.Uint64(b) }
func be16(b []byte) uint16 { return binary.BigEndian.Uint16(b) }
func be32(b []byte) uint32 { return binary.BigEndian.Uint32(b) }

func utf16leToString(b []byte) string {
	runes := make([]rune, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		r := rune(le16(b[i : i+2]))
		if r == 0 {
			break
		}
		runes = append(runes, r)
	}
	return string(runes)
}

func trimASCIIZ(b []byte) string {
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	for n > 0 && (b[n-1] == ' ' || b[n-1] == '\t') {
		n--
	}
	return string(b[:n])
}

func guidFromGPTBytes(b []byte) string {
	if len(b) != 16 {
		return ""
	}
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		le32(b[0:4]), le16(b[4:6]), le16(b[6:8]),
		b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15],
	)
}
