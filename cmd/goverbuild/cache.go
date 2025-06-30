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

func cacheCheck(args []string) {
	flagset := flag.NewFlagSet("cache:check", flag.ExitOnError)
	verbose := flagset.Bool("v", false, "Verbose mode.")
	root := flagset.String("root", ".", "Root directory for quick check resources.")
	flagset.Parse(args)

	cachePath := "versions/quickcheck.txt"
	if flagset.NArg() > 0 {
		cachePath = flagset.Args()[0]
	}

	cachefile, err := cache.Open(cachePath, 128)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("cache file does not exist: %s", cachePath)
	}

	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := cachefile.Close(); err != nil {
			log.Println(err)
		}
	}()

	cachefile.ForEach(func(qc cache.QuickCheck) bool {
		stat, err := os.Stat(filepath.Join(*root, qc.Path()))
		if err != nil {
			log.Printf("%s: %v", qc.Path(), err)
			return true
		}

		info := archive.Info{
			UncompressedSize:     uint32(qc.Size()),
			UncompressedChecksum: qc.Checksum(),
		}

		if err := qc.Check(stat, info); err != nil {
			log.Printf("%s: %v", qc.Path(), err)
		} else if *verbose {
			fmt.Printf("goverbuild: %s: entry matches!\n", qc.Path())
		}

		return true
	})
}

func verifyEntry(path string, entry *manifest.Entry) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := entry.VerifyUncompressed(file); err != nil {
		return err
	}

	return nil
}

func cacheUpdate(args []string) {
	flagset := flag.NewFlagSet("cache:update", flag.ExitOnError)
	verbose := flagset.Bool("v", false, "Verbose mode.")
	root := flagset.String("root", ".", "Root directory for quick check resources.")
	flagset.Parse(args)

	cachePath := "versions/quickcheck.txt"
	if flagset.NArg() > 0 {
		cachePath = flagset.Args()[0]
	}

	manifestPath := "versions/trunk.txt"
	if flagset.NArg() > 1 {
		manifestPath = flagset.Args()[1]
	}

	cachefile, err := cache.Open(cachePath, 128)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("cache file does not exist: %s", cachePath)
	}

	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := cachefile.Close(); err != nil {
			log.Println(err)
		}
	}()

	manifestfile, err := manifest.ReadFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("manifest file does not exist: %s", manifestPath)
	}

	if err != nil {
		log.Println(err)
	}

	for _, entry := range manifestfile.Entries {
		stat, err := os.Stat(filepath.Join(*root, entry.Path))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}

		if err != nil {
			log.Printf("%s: %v", entry.Path, err)
			continue
		}

		qc, ok := cachefile.Get(entry.Path)
		if ok {
			if err := qc.Check(stat, entry.Info); err == nil {
				if *verbose {
					fmt.Printf("goverbuild: %s: entry matches!\n", entry.Path)
				}
				continue
			}
		}

		if err := verifyEntry(filepath.Join(*root, entry.Path), entry); err != nil {
			log.Printf("%s: %v", entry.Path, err)
			continue
		}

		if err := cachefile.Store(entry.Path, stat, entry.Info); err != nil {
			log.Printf("%s: %v", entry.Path, err)
		} else if *verbose {
			fmt.Printf("goverbuild: %s: added!\n", entry.Path)
		}
	}
}

var CacheCommands = CommandList{
	"check":  cacheCheck,
	"update": cacheUpdate,
}

func doCache(args []string) {
	if len(args) < 1 {
		CacheCommands.Usage()
	}

	command, ok := CacheCommands[args[0]]
	if !ok {
		CacheCommands.Usage()
	}

	command(args[1:])
}
