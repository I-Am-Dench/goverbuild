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
	"github.com/I-Am-Dench/goverbuild/archive/manifest"
)

type Extractor struct {
	Verbose          bool
	IgnoreErrors     bool
	RemoveMismatches bool

	InstallPath string

	Archive archive.Archive
}

func (e *Extractor) log(format string, a ...any) {
	if e.Verbose {
		fmt.Printf("goverbuild: "+format, a...)
	}
}

func (e *Extractor) logFatal(format string, a ...any) {
	if e.Verbose || e.IgnoreErrors {
		log.Printf(format, a...)
	}

	if !e.IgnoreErrors {
		os.Exit(1)
	}
}

func (e *Extractor) Extract(path string) {
	if filepath.Ext(path) == ".pk" {
		return
	}

	pack, _, err := e.Archive.FindPack(path)
	if err != nil {
		if errors.Is(err, archive.ErrNotCataloged) {
			e.log("extractor: %s: %v\n", path, err)
		} else {
			e.logFatal("extractor: %s: %v", path, err)
		}
		return
	}

	record, ok := pack.Search(path)
	if !ok {
		e.log("extractor: %s: not packed\n", path)
		return
	}

	outputPath := strings.ToLower(filepath.Join(e.InstallPath, path))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		e.logFatal("extractor: %s: %v", path, err)
		return
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		e.logFatal("extractor: %s: %v", path, err)
		return
	}
	defer outputFile.Close()

	section, err := record.Section()
	if err != nil {
		e.logFatal("extractor: %s: %v", path, err)
		return
	}

	hash := md5.New()
	section = io.TeeReader(section, hash)

	if _, err := io.Copy(outputFile, section); err != nil {
		e.logFatal("extractor: %s: %v", path, err)
		return
	}

	if actual := hash.Sum(nil); !bytes.Equal(actual, record.UncompressedChecksum) {
		if !e.RemoveMismatches {
			e.logFatal("extractor: %s: hashes do not match: expected %x but got %x", path, record.UncompressedChecksum, actual)
		} else {
			outputFile.Close()
			os.Remove(outputPath)
			e.logFatal("extractor: %s: hashes do not match: expected %x but got %x: removing %s", path, record.UncompressedChecksum, actual, outputPath)
		}
	}

	e.log("extractor: success: extracted %s\n", path)
}

func doExtract(args []string) {
	flagset := flag.NewFlagSet("extract", flag.ExitOnError)
	verbose := flagset.Bool("v", false, "Verbose.")
	ignoreErrors := flagset.Bool("ie", false, "Ignores errors. Otherwise, the extractor logs an error and exists with an error code.")
	manifestPath := flagset.String("manifest", "versions\\trunk.txt", "(.txt) The primary manifest file.")
	catalogPath := flagset.String("catalog", "versions\\primary.pki", "(.pki) The primary catalog file.")
	installPath := flagset.String("output", ".", "The directory to extract the client in to.")
	removeMismatches := flagset.Bool("removemismatches", false, "Remove files with mismatched md5 hashes.")
	flagset.Parse(args)

	archive, err := archive.Open(*installPath, *catalogPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("catalog file does not exist: %s", *catalogPath)
	}

	if err != nil {
		log.Fatalf("catalog: %v", err)
	}

	extractor := Extractor{
		Verbose:          *verbose,
		IgnoreErrors:     *ignoreErrors,
		RemoveMismatches: *removeMismatches,
		InstallPath:      *installPath,
		Archive:          archive,
	}
	defer func() {
		if err := extractor.Archive.Close(); err != nil {
			log.Print(err)
		}
	}()

	if flagset.NArg() >= 1 {
		extractor.Extract(flagset.Arg(0))
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
