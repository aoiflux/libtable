package main

import (
	"fmt"
	"os"

	libtable "github.com/aoiflux/libtable"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./examples/basic <disk-image>")
		os.Exit(2)
	}

	path := os.Args[1]
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open image: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat image: %v\n", err)
		os.Exit(1)
	}

	tbl, err := libtable.Parse(f, uint64(st.Size()), libtable.Options{Type: libtable.TypeUnknown})
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Table type: %s\n", tbl.Type)
	fmt.Printf("Block size: %d\n", tbl.BlockSize)
	fmt.Printf("Partitions: %d\n\n", len(tbl.Partitions))

	for _, p := range tbl.Partitions {
		if p.Flags&libtable.PartFlagAlloc == 0 {
			continue
		}
		fmt.Printf("[%d] start=%d len=%d type=%s\n", p.Index, p.StartLBA, p.LengthLBA, p.TypeName)
	}
}
