package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/I-Am-Dench/goverbuild/archive"
)

type PackRecordTable struct {
	*tabwriter.Writer
}

func NewPackRecordTable() *PackRecordTable {
	tab := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tab, "crc\tindex_lower\tindex_upper\toriginal_size\toriginal_hash\tcompressed_size\tcompressed_hash\tdata_pointer\tis_compressed")
	return &PackRecordTable{tab}
}

func (t *PackRecordTable) Record(record *archive.PackRecord) *PackRecordTable {
	fmt.Fprintf(t, "%d\t%d\t%d\t%d\t%x\t%d\t%x\t%d\t%t\n", record.Crc, record.LowerIndex, record.UpperIndex, record.UncompressedSize, record.UncompressedChecksum, record.CompressedSize, record.CompressedChecksum, record.DataPointer(), record.IsCompressed)
	return t
}

func openPack(path string) *archive.Pack {
	pack, err := archive.OpenPack(path)
	if errors.Is(err, os.ErrNotExist) {
		Error.Fatalf("pack does not exist: %s", path)
	}

	if err != nil {
		Error.Fatal(err)
	}

	return pack
}

func packShow(args []string) {
	flagset := flag.NewFlagSet("pack:show", flag.ExitOnError)
	skip := flagset.Int("skip", 0, "How many records to skip before displaying.")
	limit := flagset.Int("limit", -1, "The maximum amount of records that should be displayed. If the limit is < 0, all records will be shown.")
	flagset.Parse(args)

	packFileName := flagset.Arg(0)
	if len(packFileName) == 0 {
		Error.Fatal("input name not provided")
	}

	pack := openPack(packFileName)
	defer pack.Close()

	tab := NewPackRecordTable()
	for _, record := range SkipLimitSlice(*skip, *limit, pack.Records()) {
		tab.Record(record)
	}
	tab.Flush()
}

func packDump(args []string) {
	flagset := flag.NewFlagSet("pack:dump", flag.ExitOnError)
	dir := flagset.String("dir", ".", "The output directory for pack record dumps. The directory will be created if it does not already exist.")
	skip := flagset.Int("skip", 0, "How many records to skip before dumping.")
	limit := flagset.Int("limit", -1, "The maximum amount of records that should be dumped. If the limit is < 0, all records will be dumped.")
	flagset.Parse(args)

	packFileName := flagset.Arg(0)
	if len(packFileName) == 0 {
		Error.Fatal("input name not provided")
	}

	pack := openPack(packFileName)
	defer pack.Close()

	if err := os.MkdirAll(*dir, 0755); err != nil {
		Error.Fatal(err)
	}

	packName := filepath.Base(packFileName)
	for i, record := range SkipLimitSlice(*skip, *limit, pack.Records()) {
		dumpPath := filepath.Join(*dir, fmt.Sprint(packName, ".", i, ".dump"))

		file, err := os.Create(dumpPath)
		if err != nil {
			Error.Print(err)
			continue
		}

		section, hash, err := record.SectionWithHash()
		if err != nil {
			Error.Print(err)
			file.Close()
			continue
		}

		if n, err := io.Copy(file, section); err != nil {
			Error.Print(err)
		} else {
			Info.Printf("Dumped %s (%d bytes); Calculated hash: %x", dumpPath, n, hash.Sum(nil))
		}

		file.Close()
	}
}

func packSearchAndShow(pack *archive.Pack, path string) {
	record, ok := pack.Search(path)
	if !ok {
		fmt.Printf("failed to find \"%s\"\n", path)
		return
	}

	fmt.Println()
	NewPackRecordTable().Record(record).Flush()
	fmt.Println()
}

func packSearch(args []string) {
	flagset := flag.NewFlagSet("pack:search", flag.ExitOnError)
	find := flagset.String("find", "", "Specifies the path the search for within the pack. Including this option causes the program to exit after receiving a result.")
	flagset.Parse(args)

	packFileName := flagset.Arg(0)
	if len(packFileName) == 0 {
		Error.Fatal("input name not provided")
	}

	pack := openPack(packFileName)
	defer pack.Close()

	fmt.Printf("Loaded %d records from \"%s\"\n", len(pack.Records()), packFileName)
	if len(*find) > 0 {
		packSearchAndShow(pack, *find)
		return
	}

	for {
		fmt.Print("Search (leave blank to exit): ")
		find := Scanln()

		if len(strings.TrimSpace(find)) == 0 {
			return
		}

		packSearchAndShow(pack, find)
	}
}

func packExtract(args []string) {
	flagset := flag.NewFlagSet("pack:extract", flag.ExitOnError)
	output := flagset.String("o", "", "Specified the output path of the extracted resource.")
	flagset.Parse(args)

	packFileName := flagset.Arg(0)
	if len(packFileName) == 0 {
		Error.Fatal("pack path not provided")
	}

	findFileName := flagset.Arg(1)
	if len(findFileName) == 0 {
		Error.Fatal("resource path not provided")
	}

	pack := openPack(packFileName)
	defer pack.Close()

	record, ok := pack.Search(findFileName)
	if !ok {
		Error.Printf("failed to find \"%s\"", findFileName)
		os.Exit(3)
	}

	outputName := GetOutputName(*output, findFileName)

	file, err := os.Create(outputName)
	if err != nil {
		Error.Fatal(err)
	}
	defer file.Close()

	section, hash, err := record.SectionWithHash()
	if err != nil {
		Error.Fatal(err)
	}

	if _, err := io.Copy(file, section); err != nil {
		Error.Fatal(err)
	}

	if actual := hash.Sum(nil); !bytes.Equal(actual, record.UncompressedChecksum) {
		Error.Printf("warning: md5 hashes do not match expected %x but got %x", record.UncompressedChecksum, actual)
	}

	Info.Printf("extracted \"%s\" to \"%s\"", findFileName, outputName)
}

var PackCommands = CommandList{
	"show":    packShow,
	"dump":    packDump,
	"search":  packSearch,
	"extract": packExtract,
}

func doPack(args []string) {
	SetLogPrefix("goverbuild(pack): ")

	if len(args) < 1 {
		PackCommands.Usage()
	}

	command, ok := PackCommands[args[0]]
	if !ok {
		PackCommands.Usage()
	}

	command(args[1:])
}
