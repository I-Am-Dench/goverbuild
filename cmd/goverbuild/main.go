package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/I-Am-Dench/goverbuild/archive/manifest"
	"github.com/I-Am-Dench/goverbuild/archive/pack"
)

func pkShow(args []string) {
	flagset := flag.NewFlagSet("pk:show", flag.ExitOnError)
	skip := flagset.Int("skip", 0, "Sets at which record index to start displaying.")
	limit := flagset.Int("limit", -1, "Sets the maximum amount of records that should be displayed. If the limit is < 0, all records will be shown.")
	flagset.Parse(args)

	if flagset.NArg() < 1 {
		log.Fatal("expected filename")
	}

	path := flagset.Args()[0]

	pack, err := pack.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", path)
	}
	defer pack.Close()

	if err != nil {
		log.Fatal(err)
	}

	records := pack.Records()
	if *skip < 0 {
		log.Fatal("starting index must be >= 0")
	}

	if *skip >= len(records) {
		log.Fatalf("index out of bound: accessing index %d with length %d", *skip, len(records))
	}

	endIndex := len(records)
	if *limit >= 0 {
		endIndex = *skip + *limit
	}

	tab := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tab, "crc_index\tcrc_lower\tcrc_upper\toriginal_size\toriginal_hash\tcompressed_size\tcompressed_hash\tis_compressed")

	for i := 0; i < endIndex; i++ {
		record := records[i]
		fmt.Fprintf(tab, "%d\t%d\t%d\t%d\t%x\t%d\t%x\t%t\n", record.CrcIndex, record.CrcLower, record.CrcUpper, record.OriginalSize, record.OriginalHash, record.CompressedSize, record.CompressedHash, record.IsCompressed)
	}
	tab.Flush()
}

func pkDump(args []string) {
	flagset := flag.NewFlagSet("pk:dump", flag.ExitOnError)
	dir := flagset.String("dir", ".", "Sets the output directory for pk record dumps. The directory will be created if it does not already exist.")
	skip := flagset.Int("skip", 0, "Sets which record index to start dumping from.")
	limit := flagset.Int("limit", -1, "Sets the maximum amount of records that should be dumped. If the limit is < 0, all records will be dumped.")
	flagset.Parse(args)

	if flagset.NArg() < 1 {
		log.Fatal("expected filename")
	}

	path := flagset.Args()[0]

	pack, err := pack.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", path)
	}
	defer pack.Close()

	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(*dir, 0755); err != nil {
		log.Fatal(err)
	}

	records := pack.Records()
	if *skip < 0 {
		log.Fatal("starting index must be >= 0")
	}

	if *skip >= len(records) {
		log.Fatalf("index out of bound: accessing index %d with length %d", *skip, len(records))
	}

	endIndex := len(records)
	if *limit >= 0 {
		endIndex = *skip + *limit
	}

	packname := filepath.Base(path)
	for i := *skip; i < endIndex; i++ {
		dumppath := filepath.Join(*dir, fmt.Sprint(packname, ".", i, ".dump"))

		file, err := os.OpenFile(dumppath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
		if err != nil {
			log.Printf("dump: %v", err)
			continue
		}

		record := records[i]
		if n, err := io.Copy(file, record.Section()); err != nil {
			log.Printf("dump: %v", err)
		} else {
			fmt.Printf("Dumped %s (%d bytes)\n", dumppath, n)
		}

		file.Close()
	}
}

func doPk(args []string) {
	if len(args) < 1 {
		log.Fatal("expected subcommand: 'show', 'dump', or 'extract'")
	}

	switch args[0] {
	case "show":
		pkShow(args[1:])
	case "dump":
		pkDump(args[1:])
	case "extract":
	default:
		log.Fatal("expected subcommand: 'show', 'dump', or 'extract'")
	}
}

func doManifest(args []string) {
	if len(args) < 1 {
		log.Fatal("expected filename")
	}

	manifest, err := manifest.Open(args[0])
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", args[0])
	}

	if err != nil {
		log.Fatal(err)
	}

	if len(manifest.Name) > 0 {
		fmt.Printf("Version: %d (%s)\n", manifest.Version, manifest.Name)
	} else {
		fmt.Printf("Version: %d\n", manifest.Version)
	}

	fmt.Printf("Found %d files:\n", len(manifest.Files))
	for _, file := range manifest.Files {
		fmt.Print("\t", file, "\n")
	}
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("goverbuild: ")

	if len(os.Args) < 2 {
		log.Fatal("expected subcommand: 'pk' or 'manifest'")
	}

	switch os.Args[1] {
	case "pk":
		doPk(os.Args[2:])
	case "manifest":
		doManifest(os.Args[2:])
	default:
		log.Fatal("expected subcommand: 'pk' or 'manifest'")
	}
}
