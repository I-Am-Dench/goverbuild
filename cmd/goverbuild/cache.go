package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/cache"
	"github.com/I-Am-Dench/goverbuild/archive/manifest"
)

func checkCache(root string, verbose bool) cache.RangeFunc {
	return func(qc cache.QuickCheck) bool {
		stat, err := os.Stat(filepath.Join(root, qc.Path()))
		if err != nil {
			log.Printf("%s: %v", qc.Path(), err)
			return true
		}

		info := archive.Info{
			UncompressedSize:     uint32(qc.Size()),
			UncompressedChecksum: qc.Hash(),
		}

		if err := qc.Check(stat, info); err != nil {
			log.Printf("%s: %v", qc.Path(), err)
		} else if verbose {
			fmt.Printf("goverbuild: %s: entry matches!\n", qc.Path())
		}

		return true
	}
}

func checkCacheWithManifest(root string, verbose bool, cachefile *cache.Cache, manifestfile *manifest.Manifest) cache.RangeFunc {
	return func(qc cache.QuickCheck) bool {
		entry, ok := manifestfile.GetEntry(qc.Path())
		if !ok {
			log.Printf("%s: entry is not tracked by manifest file", qc.Path())
			return true
		}

		stat, err := os.Stat(filepath.Join(root, qc.Path()))
		if err != nil {
			log.Printf("%s: %v", qc.Path(), err)
			return true
		}

		err = qc.Check(stat, entry.Info)
		if err == nil {
			if verbose {
				fmt.Printf("goverbuild: %s: entry matches!\n", qc.Path())
			}
			return true
		}

		log.Printf("%s: %v", qc.Path(), err)

		file, err := os.Open(filepath.Join(root, qc.Path()))
		if err != nil {
			log.Printf("%s: %v", qc.Path(), err)
			return true
		}

		if err := entry.VerifyUncompressed(file); err != nil {
			log.Printf("%s: %v: manifest mismatch...not updating", qc.Path(), err)
			return true
		}

		if err := cachefile.Store(qc.Path(), file); err != nil {
			log.Printf("%s: %v", qc.Path(), err)
		} else if verbose {
			fmt.Printf("goverbuild: %s: updated quick check entry\n", qc.Path())
		}

		return true
	}
}

func doCache(args []string) {
	flagset := flag.NewFlagSet("cache", flag.ExitOnError)
	verbose := flagset.Bool("v", false, "Verbose mode.")
	update := flagset.Bool("update", false, "Update cache on mismatches.")
	manifestPath := flagset.String("manifest", "versions/trunk.txt", "Path to the manifest file to update the cache with.")
	root := flagset.String("root", ".", "Root directory for quick check resources.")
	flagset.Parse(args)

	path := "versions/quickcheck.txt"
	if flagset.NArg() > 0 {
		path = flagset.Args()[0]
	}

	cachefile, err := cache.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", path)
	}
	defer func() {
		if err := cachefile.Close(); err != nil {
			log.Println(err)
		}
	}()

	if err != nil {
		log.Fatal(err)
	}

	var manifestfile *manifest.Manifest
	if *update {
		manifestfile, err = manifest.Open(*manifestPath)
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("manifest file does not exist: %s", *manifestPath)
		}

		if err != nil {
			log.Fatal(err)
		}
	}

	var check cache.RangeFunc
	if manifestfile == nil {
		check = checkCache(*root, *verbose)
	} else {
		check = checkCacheWithManifest(*root, *verbose, cachefile, manifestfile)
	}

	cachefile.ForEach(check)
}
