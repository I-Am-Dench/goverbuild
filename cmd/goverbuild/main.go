package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/I-Am-Dench/goverbuild/archive/catalog"
	"github.com/I-Am-Dench/goverbuild/archive/manifest"
	"github.com/I-Am-Dench/goverbuild/archive/pack"
)

func packShow(args []string) {
	flagset := flag.NewFlagSet("pack:show", flag.ExitOnError)
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
		fmt.Fprintf(tab, "%d\t%d\t%d\t%d\t%x\t%d\t%x\t%t\n", record.Crc, record.CrcLower, record.CrcUpper, record.OriginalSize, record.OriginalHash, record.CompressedSize, record.CompressedHash, record.IsCompressed)
	}
	tab.Flush()
}

func packDump(args []string) {
	flagset := flag.NewFlagSet("pack:dump", flag.ExitOnError)
	dir := flagset.String("dir", ".", "Sets the output directory for pack record dumps. The directory will be created if it does not already exist.")
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

func packSearchAndShow(pack *pack.Pack, path string) {
	record, ok := pack.Search(path)
	if !ok {
		fmt.Printf("failed to find \"%s\"\n", path)
		return
	}

	fmt.Println()
	tab := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tab, "crc_index\tcrc_lower\tcrc_upper\toriginal_size\toriginal_hash\tcompressed_size\tcompressed_hash\tis_compressed")
	fmt.Fprintf(tab, "%d\t%d\t%d\t%d\t%x\t%d\t%x\t%t\n", record.Crc, record.CrcLower, record.CrcUpper, record.OriginalSize, record.OriginalHash, record.CompressedSize, record.CompressedHash, record.IsCompressed)
	tab.Flush()
	fmt.Println()
}

func packSearch(args []string) {
	flagset := flag.NewFlagSet("pack:search", flag.ExitOnError)
	find := flagset.String("find", "", "Specifies the path to search for in the pack. Including this option causes the program to exit after receiving a result.")
	flagset.Parse(args)

	if flagset.NArg() < 1 {
		log.Fatal("expected filepath")
	}

	packPath := flagset.Args()[0]

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

		packSearchAndShow(pack, strings.TrimSpace(find))
	}
}

func doPack(args []string) {
	if len(args) < 1 {
		log.Fatal("expected subcommand: 'show', 'dump', or 'search'")
	}

	switch args[0] {
	case "show":
		packShow(args[1:])
	case "dump":
		packDump(args[1:])
	case "search":
		packSearch(args[1:])
	default:
		log.Fatal("expected subcommand: 'show', 'dump', or 'search'")
	}
}

func catalogShow(args []string) {
	flagset := flag.NewFlagSet("catalog:show", flag.ExitOnError)
	skip := flagset.Int("skip", 0, "Sets at which file index to start displaying.")
	limit := flagset.Int("limit", -1, "Sets the maximum amount of files that should be displayed. If the limit is < 0, all files will be shown.")
	flagset.Parse(args)

	if flagset.NArg() < 1 {
		log.Fatal("expected filename")
	}

	path := flagset.Args()[0]

	catalog, err := catalog.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", path)
	}

	if err != nil {
		log.Fatal(err)
	}

	files := catalog.Files
	if *skip < 0 {
		log.Fatal("starting index must be >= 0")
	}

	if *skip >= len(files) {
		log.Fatalf("index out of bound: accessing index %d with length %d", *skip, len(files))
	}

	endIndex := len(files)
	if *limit >= 0 {
		endIndex = *skip + *limit
	}

	tab := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tab, "crc_index\tcrc_lower\tcrc_upper\tpack_name\tis_compressed")

	for i := *skip; i < endIndex; i++ {
		file := files[i]
		fmt.Fprintf(tab, "%d\t%d\t%d\t%s\t%t\n", file.Crc, file.CrcLower, file.CrcUpper, file.Name, file.IsCompressed)
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
	tab := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tab, "crc_index\tcrc_lower\tcrc_upper\tpack_name\tis_compressed")
	fmt.Fprintf(tab, "%d\t%d\t%d\t%s\t%t\n", file.Crc, file.CrcLower, file.CrcUpper, file.Name, file.IsCompressed)
	tab.Flush()
	fmt.Println()
}

func catalogSearch(args []string) {
	flagset := flag.NewFlagSet("catalog:search", flag.ExitOnError)
	find := flagset.String("find", "", "Specifies the path to search for in the catalog. Including this option causes the program to exit after receiving a result.")
	flagset.Parse(args)

	if flagset.NArg() < 1 {
		log.Fatal("expected filepath")
	}

	catalogPath := flagset.Args()[0]

	catalog, err := catalog.Open(catalogPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", catalogPath)
	}

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Loaded %d entries from \"%s\"\n", len(catalog.Files), catalogPath)
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

		catalogSearchAndShow(catalog, strings.TrimSpace(find))
	}
}

func doCatalog(args []string) {
	if len(args) < 1 {
		log.Fatal("expected subcommand: 'show' or 'search'")
	}

	switch args[0] {
	case "show":
		catalogShow(args[1:])
	case "search":
		catalogSearch(args[1:])
	default:
		log.Fatal("expected subcommand: 'show' or 'search'")
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

func doExtract(args []string) {
	flagset := flag.NewFlagSet("extract", flag.ExitOnError)
	verbose := flagset.Bool("v", false, "Verbose.")
	ignoreErrors := flagset.Bool("ie", false, "Ignores errors. Otherwise, the extractor logs an error and exits with an error code.")
	manifestPath := flagset.String("manifest", "trunk.txt", "(.txt) The primary manifest file.")
	catalogPath := flagset.String("catalog", "primary.pki", "(.pki) The primary catalog file.")
	rel := flagset.String("rel", "", "Used when searching for pack (.pk) files. Instead of using the full path (i.e. client\\res\\pack\\...), extract will instead use a path relative to the rel path. If rel is client\\res, extract will get packs from .\\pack")
	client := flagset.String("output", ".", "The directory to extract the client in to.")
	flagset.Parse(args)

	catalog, err := catalog.Open(*catalogPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("catalog file does not exist: %s", *catalogPath)
	}

	if err != nil {
		log.Fatalf("catalog: %v", err)
	}

	extractor := Extractor{
		Verbose:      *verbose,
		IgnoreErrors: *ignoreErrors,
		Catalog:      catalog,
		Rel:          *rel,
		Client:       *client,
	}

	if flagset.NArg() >= 1 {
		extractor.Extract(flagset.Args()[0])
		return
	}

	manifest, err := manifest.Open(*manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("manifest file does not exist: %s", *manifestPath)
	}

	if err != nil {
		log.Fatalf("manifest: %v", err)
	}

	fmt.Printf("(%s) Extracting client resources...\n", manifest.Name)
	for _, file := range manifest.Files {
		extractor.Extract(file.Name())
	}
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("goverbuild: ")

	if len(os.Args) < 2 {
		log.Fatal("expected subcommand: 'pack', 'catalog', or 'manifest'")
	}

	switch os.Args[1] {
	case "pack":
		doPack(os.Args[2:])
	case "catalog":
		doCatalog(os.Args[2:])
	case "manifest":
		doManifest(os.Args[2:])
	case "extract":
		doExtract(os.Args[2:])
	default:
		log.Fatal("expected subcommand: 'pack', 'catalog', or 'manifest'")
	}
}
