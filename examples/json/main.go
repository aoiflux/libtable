package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	libtable "github.com/aoiflux/libtable"
)

func main() {
	var imagePath string
	var pretty bool

	flag.StringVar(&imagePath, "image", "", "Path to disk image")
	flag.BoolVar(&pretty, "pretty", true, "Pretty-print JSON")
	flag.Parse()

	if imagePath == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./examples/json -image <disk-image> [-pretty=true|false]")
		os.Exit(2)
	}

	f, err := os.Open(imagePath)
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

	enc := json.NewEncoder(os.Stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(tbl); err != nil {
		fmt.Fprintf(os.Stderr, "encode json: %v\n", err)
		os.Exit(1)
	}
}
