package libtable

import (
	"io"

	"github.com/aoiflux/libtable/partition"
)

type TableType = partition.TableType

const (
	TypeUnknown TableType = partition.TypeUnknown
	TypeMBR     TableType = partition.TypeMBR
	TypeGPT     TableType = partition.TypeGPT
	TypeBSD     TableType = partition.TypeBSD
	TypeSun     TableType = partition.TypeSun
	TypeMac     TableType = partition.TypeMac
)

type PartFlag = partition.PartFlag

const (
	PartFlagAlloc   PartFlag = partition.PartFlagAlloc
	PartFlagUnalloc PartFlag = partition.PartFlagUnalloc
	PartFlagMeta    PartFlag = partition.PartFlagMeta
)

type Partition = partition.Partition
type Table = partition.Table
type Options = partition.Options
type Parser = partition.Parser

var (
	ErrUnknownTable = partition.ErrUnknownTable
	ErrInvalidTable = partition.ErrInvalidTable
)

func New() *Parser {
	return partition.New()
}

func Parse(r io.ReaderAt, sizeBytes uint64, opts Options) (*Table, error) {
	return partition.Parse(r, sizeBytes, opts)
}
