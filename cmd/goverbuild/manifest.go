package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/I-Am-Dench/goverbuild/archive/manifest"
)

func doManifest(args []string) {
	flagset := flag.NewFlagSet("manifest", flag.ExitOnError)
	version := flagset.Bool("version", false, "Display only version info")
	flagset.Parse(args)

	path := GetArgFilename(flagset, 0)

	manifest, err := manifest.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", path)
	}

	if err != nil {
		log.Fatal(err)
	}

	if len(manifest.Name) > 0 {
		fmt.Printf("Version: %d (%s)\n", manifest.Version, manifest.Name)
	} else {
		fmt.Printf("Version: %d\n", manifest.Version)
	}

	if *version {
		os.Exit(0)
	}

	fmt.Printf("Found %d files:\n", len(manifest.Entries))
	for _, entry := range manifest.Entries {
		fmt.Print("\t", entry, "\n")
	}
}
