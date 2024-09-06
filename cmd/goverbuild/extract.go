package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/I-Am-Dench/goverbuild/archive/catalog"
	"github.com/I-Am-Dench/goverbuild/archive/pack"
)

type Extractor struct {
	Verbose      bool
	IgnoreErrors bool
	Catalog      *catalog.Catalog
	Rel          string
	Client       string

	packs map[string]*pack.Pack
}

func (extractor *Extractor) getPack(name string) (*pack.Pack, error) {
	if extractor.packs == nil {
		extractor.packs = make(map[string]*pack.Pack)
	}

	if pack, ok := extractor.packs[name]; ok {
		return pack, nil
	}

	pack, err := pack.Open(name)
	if err != nil {
		return nil, fmt.Errorf("get pack: %w", err)
	}
	extractor.packs[name] = pack

	return pack, nil
}

// Exits on error
func (extractor *Extractor) Extract(path string) {

	file, ok := extractor.Catalog.Search(path)
	if !ok {
		if extractor.Verbose {
			log.Printf("extractor: %s: not cataloged", path)
		}
		return
	}

	packname, err := filepath.Rel(extractor.Rel, file.Name)
	if err != nil {
		if extractor.Verbose {
			log.Printf("extractor: %s: %v", path, err)
		}

		if !extractor.IgnoreErrors {
			os.Exit(1)
		}

		return
	}

	pack, err := extractor.getPack(packname)
	if err != nil {
		if extractor.Verbose {
			log.Printf("extractor: %s: %v", path, err)
		}

		if !extractor.IgnoreErrors {
			os.Exit(1)
		}

		return
	}

	record, ok := pack.Search(path)
	if !ok {
		if extractor.Verbose {
			log.Printf("extractor: %s: no pack record", path)
		}

		return
	}

	outputPath := strings.ToLower(filepath.Join(extractor.Client, path))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		if extractor.Verbose {
			log.Printf("extractor: %s: %v", path, err)
		}

		if !extractor.IgnoreErrors {
			os.Exit(1)
		}

		return
	}

	out, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		if extractor.Verbose {
			log.Printf("extractor: %s: %v", path, err)
		}

		if !extractor.IgnoreErrors {
			os.Exit(1)
		}

		return
	}

	section, err := record.Section()
	if err != nil {
		if extractor.Verbose {
			log.Printf("extractor: %s: %v", path, err)
		}

		if !extractor.IgnoreErrors {
			os.Exit(0)
		}

		return
	}

	if _, err := io.Copy(out, section); err != nil {
		if extractor.Verbose {
			log.Printf("extractor: %s: %v", path, err)
		}

		if !extractor.IgnoreErrors {
			os.Exit(1)
		}

		return
	}

	fmt.Printf("goverbuild: extractor: success: extracted %s\n", path)
}
