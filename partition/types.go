package partition

import "fmt"

type TableType string

const (
	TypeUnknown TableType = "unknown"
	TypeMBR     TableType = "mbr"
	TypeGPT     TableType = "gpt"
	TypeBSD     TableType = "bsd"
	TypeSun     TableType = "sun"
	TypeMac     TableType = "mac"
)

type PartFlag uint8

const (
	PartFlagAlloc   PartFlag = 0x01
	PartFlagUnalloc PartFlag = 0x02
	PartFlagMeta    PartFlag = 0x04
)

type Partition struct {
	Index       int
	StartLBA    uint64
	LengthLBA   uint64
	TypeCode    uint64
	TypeName    string
	Name        string
	Flags       PartFlag
	TableNumber int8
	SlotNumber  int8
	Attributes  uint64
	GUIDType    string
	GUIDUnique  string
}

type Table struct {
	Type       TableType
	BlockSize  uint32
	Offset     uint64
	IsBackup   bool
	Partitions []Partition
}

type Options struct {
	Offset    uint64
	BlockSize uint32
	Type      TableType
}

func (o Options) withDefaults() Options {
	if o.BlockSize == 0 {
		o.BlockSize = 512
	}
	return o
}

type Parser struct{}

func New() *Parser { return &Parser{} }

var (
	ErrUnknownTable = fmt.Errorf("partition: could not detect a supported partition table")
	ErrInvalidTable = fmt.Errorf("partition: invalid partition table")
)
