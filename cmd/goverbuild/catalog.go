package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/I-Am-Dench/goverbuild/archive/catalog"
)

type CatalogFileTable struct {
	*tabwriter.Writer
}

func NewCatalogFileTable() *CatalogFileTable {
	tab := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tab, "crc\tcrc_lower\tcrc_upper\tpack_name\tis_compressed")
	return &CatalogFileTable{tab}
}

func (tab *CatalogFileTable) File(file *catalog.File) *CatalogFileTable {
	fmt.Fprintf(tab, "%d\t%d\t%d\t%s\t%t\n", file.Crc, file.CrcLower, file.CrcUpper, file.Name, file.IsCompressed)
	return tab
}

func catalogShow(args []string) {
	flagset := flag.NewFlagSet("catalog:show", flag.ExitOnError)
	skip := flagset.Int("skip", 0, "Sets at which file index to start displaying.")
	limit := flagset.Int("limit", -1, "Sets the maximum amount of files that should be displayed. If the limit is < 0, all files will be shown.")
	flagset.Parse(args)

	path := GetArgFilename(flagset, 0)

	catalog, err := catalog.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", path)
	}

	if err != nil {
		log.Fatal(err)
	}

	tab := NewCatalogFileTable()
	for _, file := range SkipLimitSlice(*skip, *limit, catalog.Files) {
		tab.File(file)
	}
	tab.Flush()
}

func catalogSearchAndShow(catalog *catalog.Catalog, path string) {
	file, ok := catalog.Search(path)
	if !ok {
		fmt.Printf("failed to find \"%s\"\n", path)
		return
	}

	fmt.Println()
	NewCatalogFileTable().File(file).Flush()
	fmt.Println()
}

func catalogSearch(args []string) {
	flagset := flag.NewFlagSet("catalog:search", flag.ExitOnError)
	find := flagset.String("find", "", "Specifies the path to serach for in the catalog. Including this option causes the program to exit after receiving a result.")
	flagset.Parse(args)

	path := GetArgFilename(flagset, 0)

	catalog, err := catalog.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", path)
	}

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Loaded %d entries from \"%s\"\n", len(catalog.Files), path)
	if len(*find) > 0 {
		catalogSearchAndShow(catalog, *find)
		os.Exit(0)
	}

	for {
		var find string
		fmt.Print("Search (leave blank to exit): ")
		fmt.Scanln(&find)

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
	if len(args) < 1 {
		CatalogCommands.Usage()
	}

	command, ok := CatalogCommands[args[0]]
	if !ok {
		CatalogCommands.Usage()
	}

	command(args[1:])
}
