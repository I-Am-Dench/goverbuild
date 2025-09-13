package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/cache"
	"github.com/I-Am-Dench/goverbuild/archive/manifest"
)

func cacheCheck(args []string) {
	flagset := flag.NewFlagSet("cache:check", flag.ExitOnError)
	flagset.BoolVar(&VerboseFlag, "v", false, "Enable verbose logging.")
	root := flagset.String("root", ".", "Root directory for quick check resources.")
	flagset.Parse(args)

	cacheFileName := flagset.Arg(0)
	if len(cacheFileName) == 0 {
		cacheFileName = filepath.Join("versions", "quickcheck.txt")
	}

	cacheFile, err := cache.ReadFile(cacheFileName)
	if errors.Is(err, os.ErrNotExist) {
		Error.Fatalf("cache file does not exist: %s", cacheFileName)
	}

	if err != nil {
		Error.Fatal(err)
	}
	defer func() {
		if err := cache.WriteFile(cacheFileName, cacheFile); err != nil {
			Error.Println(err)
		}
	}()

	for qc := range cacheFile.All() {
		stat, err := os.Stat(filepath.Join(*root, qc.Path()))
		if err != nil {
			Error.Printf("%s: %v", qc.Path(), err)
			break
		}

		if err := qc.Check(stat, archive.Info{
			UncompressedSize:     uint32(qc.Size()),
			UncompressedChecksum: qc.Checksum(),
		}); err != nil {
			Error.Printf("%s: %v", qc.Path(), err)
		} else {
			Verbose.Printf("%s: entry matches!", qc.Path())
		}
	}
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
	flagset.BoolVar(&VerboseFlag, "v", false, "Enable verbose logging.")
	root := flagset.String("root", ".", "Root directory for quick check resources.")
	flagset.Parse(args)

	cacheFileName := flagset.Arg(0)
	if len(cacheFileName) == 0 {
		cacheFileName = filepath.Join("versions", "quickcheck.txt")
	}

	manifestFileName := flagset.Arg(1)
	if len(manifestFileName) == 0 {
		manifestFileName = filepath.Join("versions", "trunk.txt")
	}

	cacheFile, err := cache.ReadFile(cacheFileName)
	if errors.Is(err, os.ErrNotExist) {
		Error.Fatalf("cache file does not exist: %s", cacheFileName)
	}

	if err != nil {
		Error.Fatal(err)
	}
	defer func() {
		if err := cache.WriteFile(cacheFileName, cacheFile); err != nil {
			Error.Print(err)
		}
	}()

	manifestFile, err := manifest.ReadFile(manifestFileName)
	if errors.Is(err, os.ErrNotExist) {
		Error.Fatalf("manifest file does not exist: %s", manifestFileName)
	}

	if err != nil {
		Error.Fatal(err)
	}

	for _, entry := range manifestFile.Entries {
		stat, err := os.Stat(filepath.Join(*root, entry.Path))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}

		if err != nil {
			Info.Printf("%s: %v", entry.Path, err)
			continue
		}

		qc, ok := cacheFile.Load(entry.Path)
		if ok {
			if err := qc.Check(stat, entry.Info); err == nil {
				Verbose.Printf("%s: entry matches!", entry.Path)
				continue
			}
		}

		if err := verifyEntry(filepath.Join(*root, entry.Path), entry); err != nil {
			Verbose.Printf("%s: %v", entry.Path, err)
			continue
		}

		cacheFile.Store(entry.Path, stat, entry.Info)
		Verbose.Printf("%s: added!", entry.Path)
	}
}

var CacheCommands = CommandList{
	"check":  cacheCheck,
	"update": cacheUpdate,
}

func doCache(args []string) {
	SetLogPrefix("goverbuild(cache): ")

	if len(args) < 1 {
		CacheCommands.Usage()
	}

	command, ok := CacheCommands[args[0]]
	if !ok {
		CacheCommands.Usage()
	}

	command(args[1:])
}
