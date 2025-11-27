package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/I-Am-Dench/goverbuild/archive/manifest"
)

func doManifest(args []string) {
	SetLogPrefix("goverbuild(manifest): ")

	flagset := flag.NewFlagSet("manifest", flag.ExitOnError)
	version := flagset.Bool("version", false, "Display only version info")
	flagset.Parse(args)

	fileName := flagset.Arg(0)
	if len(fileName) == 0 {
		Error.Fatal("input name not provided")
	}

	manifestFile, err := manifest.ReadFile(fileName)
	if errors.Is(err, os.ErrNotExist) {
		Error.Fatalf("file does not exist: %s", fileName)
	}

	if err != nil {
		Error.Fatal(err)
	}

	if len(manifestFile.Name) > 0 {
		fmt.Printf("Version: %d (%s)\n", manifestFile.Version, manifestFile.Name)
	} else {
		fmt.Printf("Version: %d\n", manifestFile.Version)
	}

	if *version {
		return
	}

	entries := manifestFile.Entries()

	Info.Printf("Found %d files:", len(entries))
	for _, entry := range entries {
		fmt.Printf("%s => uncompressedSize=%d; uncompressedChecksum=%x; compressedSize=%d; compressedChecksum=%x\n", entry.Path, entry.UncompressedSize, entry.UncompressedChecksum, entry.CompressedSize, entry.CompressedChecksum)
	}
}
