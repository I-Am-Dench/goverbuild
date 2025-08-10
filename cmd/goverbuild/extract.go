package main

import (
	"bytes"
	"crypto/md5"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/catalog"
	"github.com/I-Am-Dench/goverbuild/archive/manifest"
)

type Extractor struct {
	Verbose          bool
	IgnoreErrors     bool
	Catalog          *catalog.Catalog
	Rel              string
	Client           string
	RemoveMismatches bool

	packs map[string]*archive.Pack
}

func (extractor *Extractor) getPack(name string) (*archive.Pack, error) {
	if extractor.packs == nil {
		extractor.packs = make(map[string]*archive.Pack)
	}

	if pack, ok := extractor.packs[name]; ok {
		return pack, nil
	}

	pack, err := archive.OpenPack(name)
	if err != nil {
		return nil, fmt.Errorf("get pack: %w", err)
	}
	extractor.packs[name] = pack

	return pack, nil
}

func (extractor *Extractor) Close() error {
	errs := []error{}
	for _, pack := range extractor.packs {
		if err := pack.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (extractor *Extractor) log(format string, a ...any) {
	if extractor.Verbose {
		fmt.Printf("goverbuild: "+format, a...)
	}
}

func (extractor *Extractor) logFatal(format string, a ...any) {
	if extractor.Verbose || !extractor.IgnoreErrors {
		log.Printf(format, a...)
	}

	if !extractor.IgnoreErrors {
		os.Exit(1)
	}
}

func (extractor *Extractor) Extract(path string) {
	if filepath.Ext(path) == ".pk" {
		return
	}

	file, ok := extractor.Catalog.Search(path)
	if !ok {
		extractor.log("extractor: %s: not cataloged\n", path)
		return
	}

	packname, err := filepath.Rel(extractor.Rel, file.PackName)
	if err != nil {
		extractor.logFatal("extractor: %s: %v", path, err)
		return
	}

	pack, err := extractor.getPack(packname)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			extractor.log("extractor: %s: pack does not exist: %s", path, packname)
		} else {
			extractor.logFatal("extractor: %s: %v", path, err)
		}
		return
	}

	record, ok := pack.Search(path)
	if !ok {
		extractor.log("extractor: %s: not packed\n", path)
		return
	}

	// client/res/forkp/effects/celebration/celebrationSentinelSword/celebrationSentinelSword4.psb

	outputPath := strings.ToLower(filepath.Join(extractor.Client, path))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		extractor.logFatal("extractor: %s: %v", path, err)
		return
	}

	out, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		extractor.logFatal("extractor: %s: %v", path, err)
		return
	}
	defer out.Close()

	section, err := record.Section()
	if err != nil {
		extractor.logFatal("extractor: %s: %v", path, err)
		return
	}

	hash := md5.New()
	section = io.TeeReader(section, hash)

	if _, err := io.Copy(out, section); err != nil {
		extractor.logFatal("extractor: %s: %v", path, err)
		return
	}

	if h := hash.Sum(nil); !bytes.Equal(h, record.UncompressedChecksum) {
		if !extractor.RemoveMismatches {
			extractor.logFatal("extractor: %s: hashes do not match: %x != %x", path, h, record.UncompressedChecksum)
		} else {
			out.Close()
			os.Remove(outputPath)
			extractor.logFatal("extractor: %s: hashes do not match: %x != %x: removing %s", path, h, record.UncompressedChecksum, outputPath)
		}
	}

	extractor.log("extractor: success: extracted %s\n", path)
}

func doExtract(args []string) {
	flagset := flag.NewFlagSet("extract", flag.ExitOnError)
	verbose := flagset.Bool("v", false, "Verbose.")
	ignoreErrors := flagset.Bool("ie", false, "Ignores errors. Otherwise, the extractor logs an error and exist with an error code.")
	manifestPath := flagset.String("manifest", "versions\\trunk.txt", "(.txt) The primary manifest file.")
	catalogPath := flagset.String("catalog", "versions\\primary.pki", "(.pki) The primary catalog file.")
	rel := flagset.String("rel", "", "Used when searching for pack (.pk) files. Instead of using the full path (i.e. client\\res\\pack\\...), extract will instead use a path relative to the rel path. If rel is client\\res, extract will get packs from .\\pack")
	client := flagset.String("output", ".", "The directory to extract the client in to.")
	removeMismatches := flagset.Bool("removemismatches", false, "Remove files with mismatched md5 hashes.")
	flagset.Parse(args)

	catalog, err := catalog.ReadFile(*catalogPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("catalog file does not exist: %s", *catalogPath)
	}

	if err != nil {
		log.Fatalf("catalog: %v", err)
	}

	extractor := Extractor{
		Verbose:          *verbose,
		IgnoreErrors:     *ignoreErrors,
		Catalog:          catalog,
		Rel:              *rel,
		Client:           *client,
		RemoveMismatches: *removeMismatches,
	}
	defer func() {
		if err := extractor.Close(); err != nil {
			log.Print(err)
		}
	}()

	if flagset.NArg() >= 1 {
		extractor.Extract(flagset.Args()[0])
		return
	}

	manifest, err := manifest.ReadFile(*manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("manifest file does not exist: %s", *manifestPath)
	}

	if err != nil {
		log.Fatalf("manifest: %v", err)
	}

	fmt.Printf("(%s) Extracting client resources...\n", manifest.Name)
	for _, entry := range manifest.Entries {
		extractor.Extract(entry.Path)
	}
}
