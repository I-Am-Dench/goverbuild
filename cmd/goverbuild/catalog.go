package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/I-Am-Dench/goverbuild/archive"
)

type CatalogRecordTable struct {
	*tabwriter.Writer
}

func NewCatalogRecordTable() *CatalogRecordTable {
	tab := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tab, "crc\tcrc_lower\tcrc_upper\tpack_name\tis_compressed")
	return &CatalogRecordTable{tab}
}

func (t *CatalogRecordTable) Record(record *archive.CatalogRecord) *CatalogRecordTable {
	fmt.Fprintf(t, "%d\t%d\t%d\t%s\t%t\n", record.Crc, record.LowerIndex, record.UpperIndex, record.PackName, record.IsCompressed)
	return t
}

func openCatalog(path string) *archive.Catalog {
	catalog, err := archive.OpenCatalog(path)
	if errors.Is(err, os.ErrNotExist) {
		Error.Fatalf("catalog does not exist: %s", path)
	}

	if err != nil {
		Error.Fatal(err)
	}

	return catalog
}

func catalogShow(args []string) {
	flagset := flag.NewFlagSet("catalog:show", flag.ExitOnError)
	skip := flagset.Int("skip", 0, "How many records to skip before displaying.")
	limit := flagset.Int("limit", -1, "The maximum amount of records that should be displayed. If the limit is < 0, all records will be shown.")
	flagset.Parse(args)

	catalogFileName := flagset.Arg(0)
	if len(catalogFileName) == 0 {
		Error.Fatal("input name not provided")
	}

	catalog := openCatalog(catalogFileName)
	defer catalog.Close()

	tab := NewCatalogRecordTable()
	for _, record := range SkipLimitSlice(*skip, *limit, catalog.Records()) {
		tab.Record(record)
	}
	tab.Flush()
}

func catalogSearchAndShow(catalog *archive.Catalog, path string) {
	record, ok := catalog.Search(path)
	if !ok {
		fmt.Printf("failed to find \"%s\"\n", path)
		return
	}

	fmt.Println()
	NewCatalogRecordTable().Record(record).Flush()
	fmt.Println()
}

func catalogSearch(args []string) {
	flagset := flag.NewFlagSet("catalog:search", flag.ExitOnError)
	find := flagset.String("find", "", "Specifies the path the search for within the catalog. Including this option causes the program to exit after receiving a result.")
	flagset.Parse(args)

	catalogFileName := flagset.Arg(0)
	if len(catalogFileName) == 0 {
		Error.Fatal("input name not provided")
	}

	catalog := openCatalog(catalogFileName)
	defer catalog.Close()

	fmt.Printf("Loaded %d entries from \"%s\"\n", len(catalog.Records()), catalogFileName)
	if len(*find) > 0 {
		catalogSearchAndShow(catalog, *find)
		return
	}

	for {
		fmt.Print("Search (leave blank to exit): ")
		find := Scanln()

		if len(strings.TrimSpace(find)) == 0 {
			return
		}

		catalogSearchAndShow(catalog, find)
	}
}

var CatalogCommands = CommandList{
	"show":   catalogShow,
	"search": catalogSearch,
}

func doCatalog(args []string) {
	SetLogPrefix("goverbuild(catalog): ")

	if len(args) < 1 {
		CatalogCommands.Usage()
	}

	command, ok := CatalogCommands[args[0]]
	if !ok {
		CatalogCommands.Usage()
	}

	command(args[1:])
}
