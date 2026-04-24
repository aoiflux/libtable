# libtable

libtable is a pure Go library for parsing partition tables from disk images.

It is designed to be easy to embed in tools, scripts, and forensic workflows.

## Features

- Pure Go (no CGO dependency)
- Root import API: `github.com/aoiflux/libtable`
- Partition table support:
  - MBR (DOS, including extended partitions)
  - GPT (including primary and backup header handling)
  - BSD disklabel
  - Sun VTOC
  - Mac partition map
- Autodetect mode and explicit type parsing
- Rich partition metadata:
  - LBAs, byte offsets, lengths
  - allocation/meta flags
  - GPT GUID and attributes when available
- Safety checks for malformed input:
  - bounds checks
  - GPT CRC validation
  - malformed chain handling for extended MBR entries

## Install

```bash
go get github.com/aoiflux/libtable
```

## Quick Start

```go
package main

import (
	"fmt"
	"os"

	libtable "github.com/aoiflux/libtable"
)

func main() {
	f, err := os.Open("disk.img")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		panic(err)
	}

	table, err := libtable.Parse(f, uint64(st.Size()), libtable.Options{
		Type: libtable.TypeUnknown,
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Table: %s (block size: %d)\n", table.Type, table.BlockSize)
	for _, p := range table.Partitions {
		fmt.Printf("[%d] start=%d len=%d flags=%d desc=%s\n", p.Index, p.StartLBA, p.LengthLBA, p.Flags, p.TypeName)
	}
}
```

## Parse Options

`libtable.Options` fields:

- `Type`: choose a specific parser, or `TypeUnknown` for autodetect
- `Offset`: byte offset where partition parsing should start
- `BlockSize`: sector size used by parser (defaults to 512 when zero)

Available types:

- `libtable.TypeUnknown`
- `libtable.TypeMBR`
- `libtable.TypeGPT`
- `libtable.TypeBSD`
- `libtable.TypeSun`
- `libtable.TypeMac`

## Included Example Programs

### 1) Pretty console report

```bash
go run ./examples/inspect -image ./disk.img
```

Optional flags:

- `-type auto|mbr|gpt|bsd|sun|mac`
- `-offset <bytes>`
- `-sector-size <bytes>`
- `-allocated-only`

### 2) Minimal integration

```bash
go run ./examples/basic ./disk.img
```

### 3) JSON output for scripts and pipelines

```bash
go run ./examples/json -image ./disk.img -pretty=true
```

## Example Output (inspect)

```text
LIBTABLE PARTITION REPORT
========================================================================
Image      : ./disk.img
Size       : 8589934592 bytes (8.00 GiB)
Table Type : gpt
Block Size : 512 bytes
Offset     : 0 bytes
Backup GPT : false
Entries    : 10

Idx  Start LBA  End LBA  Length LBA  Start Byte   Size      Flags         Description
0    0          0        1           0            512 B     META          Safety Table
1    1          1        1           512          512 B     META          GPT Header
3    2048       1050623  1048576     1048576      512.00 MiB ALLOC       EFI System
```

## Data Model Highlights

Each parsed `Partition` can include:

- `StartLBA`, `LengthLBA`
- `TypeName`, `Name`
- `Flags` (`PartFlagAlloc`, `PartFlagUnalloc`, `PartFlagMeta`)
- `GUIDType`, `GUIDUnique`, and `Attributes` for GPT entries

## Best Practices

- Always pass the full image size to `Parse` for correct bounds checks.
- Use `TypeUnknown` unless you explicitly know the table type.
- For offset-based parsing (embedded images), set `Options.Offset`.
- For non-standard sector layouts, set `Options.BlockSize`.

## Development

Run tests:

```bash
go test ./...
```
