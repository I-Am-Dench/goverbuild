package main

import (
	"bytes"
	"crypto/md5"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/manifest"
)

type Extractor struct {
	IgnoreErrors     bool
	RemoveMismatches bool

	InstallPath string

	Archive archive.Archive
}

func (e *Extractor) LogFatal(format string, a ...any) {
	if e.IgnoreErrors {
		Verbose.Printf(format, a...)
	} else {
		Error.Fatalf(format, a...)
	}
}

func (e *Extractor) Extract(path string) {
	if filepath.Ext(path) == ".pk" {
		return
	}

	record, err := e.Archive.Load(path)
	if err != nil {
		if errors.Is(err, archive.ErrNotCataloged) || errors.Is(err, archive.ErrNotPacked) {
			Verbose.Printf("%s: %v", path, err)
		} else {
			e.LogFatal("%s: %v", path, err)
		}
		return
	}

	outputName := strings.ToLower(filepath.Join(e.InstallPath, path))
	if err := os.MkdirAll(filepath.Dir(outputName), 0755); err != nil {
		e.LogFatal("%s: %v", path, err)
		return
	}

	outputFile, err := os.Create(outputName)
	if err != nil {
		e.LogFatal("%s: %v", path, err)
		return
	}
	defer outputFile.Close()

	section, err := record.Section()
	if err != nil {
		e.LogFatal("%s: %v", path, err)
		return
	}

	hash := md5.New()
	section = io.TeeReader(section, hash)

	if _, err := io.Copy(outputFile, section); err != nil {
		e.LogFatal("%s: %v", path, err)
		return
	}

	if actual := hash.Sum(nil); !bytes.Equal(actual, record.UncompressedChecksum) {
		if !e.RemoveMismatches {
			e.LogFatal("%s: hashes do not match: expected %x but got %x", path, record.UncompressedChecksum, actual)
		} else {
			outputFile.Close()
			os.Remove(outputName)
			e.LogFatal("%s: hashes do not match: expected %x but got %x; removing %s", path, record.UncompressedChecksum, actual, outputName)
		}
	}

	Verbose.Printf("success: extracted %s", path)
}

func doExtract(args []string) {
	SetLogPrefix("goverbuild(extract): ")

	flagset := flag.NewFlagSet("extract", flag.ExitOnError)
	flagset.BoolVar(&VerboseFlag, "v", false, "Enable verbose logging.")
	ignoreErrors := flagset.Bool("ignoreErrors", false, "Ignore errors. Otherwise, the extractor logs an error and exists with an error code.")
	manifestName := flagset.String("manifest", filepath.Join("versions", "trunk.txt"), "(.txt) The primary manifest file.")
	catalogName := flagset.String("catalog", filepath.Join("versions", "primary.pki"), "(.pki) The primary catalog file.")
	installPath := flagset.String("install", ".", "The directory to extract the client in to.")
	removeMismatches := flagset.Bool("removeMismatches", false, "Remove files with mismatched md5 hashes.")
	flagset.Parse(args)

	archive, err := archive.Open(*installPath, *catalogName)
	if errors.Is(err, os.ErrNotExist) {
		Error.Fatalf("catalog file does not exist: %s", *catalogName)
	}

	if err != nil {
		Error.Fatalf("catalog: %v", err)
	}

	extractor := Extractor{
		IgnoreErrors:     *ignoreErrors,
		RemoveMismatches: *removeMismatches,
		InstallPath:      *installPath,
		Archive:          archive,
	}
	defer func() {
		if err := extractor.Archive.Close(); err != nil {
			Error.Print(err)
		}
	}()

	if name := flagset.Arg(0); len(name) > 0 {
		extractor.Extract(name)
		return
	}

	manifest, err := manifest.ReadFile(*manifestName)
	if errors.Is(err, os.ErrNotExist) {
		Error.Fatalf("manifest file does not exist: %s", *manifestName)
	}

	if err != nil {
		Error.Fatalf("manifest: %v", err)
	}

	Info.Printf("(%s) Extracting client resources...", manifest.Name)
	for _, entry := range manifest.Entries {
		extractor.Extract(entry.Path)
	}
}
