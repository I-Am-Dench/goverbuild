package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/I-Am-Dench/goverbuild/archive/pack"
)

type PackRecordTable struct {
	*tabwriter.Writer
}

func NewPackRecordTable() *PackRecordTable {
	tab := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tab, "crc\tindex_lower\tindex_upper\toriginal_size\toriginal_hash\tcompressed_size\tcompressed_hash\tdata_pointer\tis_compressed")
	return &PackRecordTable{tab}
}

func (tab *PackRecordTable) Record(record *pack.Record) *PackRecordTable {
	fmt.Fprintf(tab, "%d\t%d\t%d\t%d\t%x\t%d\t%x\t%d\t%t\n", record.Crc, record.LowerIndex, record.UpperIndex, record.UncompressedSize, record.UncompressedChecksum, record.CompressedSize, record.CompressedChecksum, record.DataPointer(), record.IsCompressed)
	return tab
}

func packShow(args []string) {
	flagset := flag.NewFlagSet("pack:show", flag.ExitOnError)
	skip := flagset.Int("skip", 0, "Sets at which record index to start displaying.")
	limit := flagset.Int("limit", -1, "Sets the maximum amount of records that should be displayed. If the limit is < 0, all records will be shown")
	flagset.Parse(args)

	path := GetArgFilename(flagset, 0)

	pack, err := pack.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", path)
	}
	defer pack.Close()

	if err != nil {
		log.Fatal(err)
	}

	tab := NewPackRecordTable()
	for _, record := range SkipLimitSlice(*skip, *limit, pack.Records()) {
		tab.Record(record)
	}
	tab.Flush()
}

func packDump(args []string) {
	flagset := flag.NewFlagSet("pack:dump", flag.ExitOnError)
	dir := flagset.String("dir", ".", "Sets the output directory for pack record dumps. The directory will be created if it does not already exist.")
	skip := flagset.Int("skip", 0, "Sets which record index to start dumping from.")
	limit := flagset.Int("limit", -1, "Sets the maximum amount of records that should be dumped. If the limit is < 0, all records will be dumped.")
	flagset.Parse(args)

	path := GetArgFilename(flagset, 0)

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

	packname := filepath.Base(path)
	for i, record := range SkipLimitSlice(*skip, *limit, pack.Records()) {
		dumppath := filepath.Join(*dir, fmt.Sprint(packname, ".", i, ".dump"))

		file, err := os.OpenFile(dumppath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
		if err != nil {
			log.Printf("dump: %v", err)
			continue
		}

		section, hash, err := record.Section()
		if err != nil {
			log.Printf("dump: %v", err)
			file.Close()
			continue
		}

		if n, err := io.Copy(file, section); err != nil {
			log.Printf("dump: %v", err)
		} else {
			fmt.Printf("Dumped %s (%d bytes); Calculated hash: %x\n", dumppath, n, hash.Sum(nil))
		}

		file.Close()
	}
}

func packSearchAndShow(pack *pack.Pack, path string) {
	record, ok := pack.Search(path)
	if !ok {
		fmt.Printf("faild to find \"%s\"\n", path)
		return
	}

	fmt.Println()
	NewPackRecordTable().Record(record).Flush()
	fmt.Println()
}

func packSearch(args []string) {
	flagset := flag.NewFlagSet("pack:search", flag.ExitOnError)
	find := flagset.String("find", "", "Specifies the path to search for within the pack. Including this option causes the program to exit after receiving a result.")
	flagset.Parse(args)

	packPath := GetArgFilename(flagset, 0)

	pack, err := pack.Open(packPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", packPath)
	}

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Loaded %d records from \"%s\"\n", len(pack.Records()), packPath)
	if len(*find) > 0 {
		packSearchAndShow(pack, *find)
		os.Exit(0)
	}

	for {
		var find string
		fmt.Print("Search (leave blank to exit): ")
		fmt.Scanln(&find)

		if len(strings.TrimSpace(find)) == 0 {
			return
		}

		packSearchAndShow(pack, find)
	}
}

func packExtract(args []string) {
	flagset := flag.NewFlagSet("pack:extract", flag.ExitOnError)
	output := flagset.String("output", "", "Specifies the output path of the extracted resource.")
	flagset.Parse(args)

	packPath := GetArgFilename(flagset, 0, "no pack path provided")
	findPath := GetArgFilename(flagset, 1, "no resource path provided")

	pack, err := pack.Open(packPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", packPath)
	}
	defer pack.Close()

	if err != nil {
		log.Fatal(err)
	}

	record, ok := pack.Search(findPath)
	if !ok {
		fmt.Printf("failed to find \"%s\"\n", findPath)
		os.Exit(3)
	}

	outputPath := filepath.Base(findPath)
	if len(*output) > 0 {
		outputPath = *output
	}

	if stat, err := os.Stat(outputPath); err == nil && stat.IsDir() {
		outputPath = filepath.Join(*output, filepath.Base(findPath))
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		log.Fatal(err)
	}

	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	section, hash, err := record.Section()
	if err != nil {
		log.Fatal(err)
	}

	if _, err := io.Copy(file, section); err != nil {
		log.Fatal(err)
	}

	if h := hash.Sum(nil); !bytes.Equal(h, record.UncompressedChecksum) {
		log.Printf("warning: md5 hashes do not match: %x != %x", h, record.UncompressedChecksum)
	}

	fmt.Printf("extracted \"%s\" to \"%s\"\n", findPath, outputPath)
}

var PackCommands = CommandList{
	"show":    packShow,
	"dump":    packDump,
	"search":  packSearch,
	"extract": packExtract,
}

func doPack(args []string) {
	if len(args) < 1 {
		PackCommands.Usage()
	}

	command, ok := PackCommands[args[0]]
	if !ok {
		PackCommands.Usage()
	}

	command(args[1:])
}
