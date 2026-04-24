package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	libtable "github.com/aoiflux/libtable"
)

func main() {
	var imagePath string
	var tableType string
	var offset uint64
	var sectorSize uint
	var allocatedOnly bool

	flag.StringVar(&imagePath, "image", "", "Path to disk image file")
	flag.StringVar(&tableType, "type", "auto", "Partition table type: auto|mbr|gpt|bsd|sun|mac")
	flag.Uint64Var(&offset, "offset", 0, "Byte offset to begin parsing")
	flag.UintVar(&sectorSize, "sector-size", 512, "Sector size used while parsing")
	flag.BoolVar(&allocatedOnly, "allocated-only", false, "Only print allocated partitions")
	flag.Parse()

	if imagePath == "" {
		fmt.Fprintf(os.Stderr, "Usage: go run ./examples/inspect -image <disk.img> [-type auto|mbr|gpt|bsd|sun|mac] [-offset N] [-sector-size N] [-allocated-only]\n")
		os.Exit(2)
	}

	typeOpt, err := parseType(tableType)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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

	opts := libtable.Options{
		Type:      typeOpt,
		Offset:    offset,
		BlockSize: uint32(sectorSize),
	}

	tbl, err := libtable.Parse(f, uint64(st.Size()), opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse partition table: %v\n", err)
		os.Exit(1)
	}

	printSummary(imagePath, st.Size(), tbl)
	fmt.Println()
	printPartitions(tbl, allocatedOnly)
}

func parseType(v string) (libtable.TableType, error) {
	s := strings.ToLower(strings.TrimSpace(v))
	switch s {
	case "", "auto", "detect":
		return libtable.TypeUnknown, nil
	case "mbr", "dos":
		return libtable.TypeMBR, nil
	case "gpt":
		return libtable.TypeGPT, nil
	case "bsd":
		return libtable.TypeBSD, nil
	case "sun":
		return libtable.TypeSun, nil
	case "mac":
		return libtable.TypeMac, nil
	default:
		return libtable.TypeUnknown, fmt.Errorf("unsupported -type value %q", v)
	}
}

func printSummary(imagePath string, imageSize int64, tbl *libtable.Table) {
	fmt.Println("LIBTABLE PARTITION REPORT")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("Image      : %s\n", imagePath)
	fmt.Printf("Size       : %d bytes (%s)\n", imageSize, humanBytes(uint64(imageSize)))
	fmt.Printf("Table Type : %s\n", tbl.Type)
	fmt.Printf("Block Size : %d bytes\n", tbl.BlockSize)
	fmt.Printf("Offset     : %d bytes\n", tbl.Offset)
	fmt.Printf("Backup GPT : %v\n", tbl.IsBackup)
	fmt.Printf("Entries    : %d\n", len(tbl.Partitions))
}

func printPartitions(tbl *libtable.Table, allocatedOnly bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Idx\tStart LBA\tEnd LBA\tLength LBA\tStart Byte\tSize\tFlags\tDescription\tGUID Type\tGUID Unique")

	for _, p := range tbl.Partitions {
		if allocatedOnly && p.Flags&libtable.PartFlagAlloc == 0 {
			continue
		}

		desc := strings.TrimSpace(p.TypeName)
		if p.Name != "" {
			desc = p.Name
		}
		if desc == "" {
			desc = "-"
		}

		startByte := p.StartLBA * uint64(tbl.BlockSize)
		endLBA := p.StartLBA
		if p.LengthLBA > 0 {
			endLBA = p.StartLBA + p.LengthLBA - 1
		}

		guidType := p.GUIDType
		if guidType == "" {
			guidType = "-"
		}
		guidUnique := p.GUIDUnique
		if guidUnique == "" {
			guidUnique = "-"
		}

		fmt.Fprintf(
			w,
			"%d\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t%s\n",
			p.Index,
			p.StartLBA,
			endLBA,
			p.LengthLBA,
			startByte,
			humanBytes(p.LengthLBA*uint64(tbl.BlockSize)),
			formatFlags(p.Flags),
			desc,
			guidType,
			guidUnique,
		)
	}

	_ = w.Flush()
}

func formatFlags(flags libtable.PartFlag) string {
	var parts []string
	if flags&libtable.PartFlagAlloc != 0 {
		parts = append(parts, "ALLOC")
	}
	if flags&libtable.PartFlagUnalloc != 0 {
		parts = append(parts, "UNALLOC")
	}
	if flags&libtable.PartFlagMeta != 0 {
		parts = append(parts, "META")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "|")
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	d, exp := uint64(unit), 0
	for v := n / unit; v >= unit && exp < 6; v /= unit {
		d *= unit
		exp++
	}
	prefixes := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB"}
	if exp >= len(prefixes) {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.2f %s", float64(n)/float64(d), prefixes[exp])
}
