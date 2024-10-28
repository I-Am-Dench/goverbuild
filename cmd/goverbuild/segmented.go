package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/I-Am-Dench/goverbuild/compress/segmented"
)

func segmentedCompress(args []string) {
	flagset := flag.NewFlagSet("segmented:compress", flag.ExitOnError)
	output := flagset.String("o", "", "Sets where to output the file.")
	flagset.Parse(args)

	inputPath := GetArgFilename(flagset, 0)

	outputPath := filepath.Base(inputPath)
	if len(*output) > 0 {
		outputPath = *output
	}

	if stat, err := os.Stat(outputPath); err == nil && stat.IsDir() {
		outputPath = filepath.Join(*output, filepath.Base(inputPath))
	}
	outputPath += ".sd0"

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		log.Fatal(err)
	}

	inputFile, err := os.Open(inputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer inputFile.Close()

	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer outputFile.Close()

	compressor := segmented.NewDataWriter(outputFile)

	if _, err := io.Copy(compressor, inputFile); err != nil {
		log.Fatal(err)
	}

	if err := compressor.Close(); err != nil {
		log.Fatal(err)
	}
}

func segmentedDecompress(args []string) {
	flagset := flag.NewFlagSet("segmented:compress", flag.ExitOnError)
	output := flagset.String("o", "", "Sets where to output the file.")
	flagset.Parse(args)

	inputPath := GetArgFilename(flagset, 0)

	outputPath := strings.TrimSuffix(filepath.Base(inputPath), ".sd0")
	if len(*output) > 0 {
		outputPath = *output
	}

	if stat, err := os.Stat(outputPath); err == nil && stat.IsDir() {
		outputPath = filepath.Join(*output, strings.TrimSuffix(filepath.Base(inputPath), ".sd0"))
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		log.Fatal(err)
	}

	inputFile, err := os.Open(inputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer inputFile.Close()

	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer outputFile.Close()

	decompressor, err := segmented.NewDataReader(inputFile)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := io.Copy(outputFile, decompressor); err != nil {
		log.Fatal(err)
	}
}

var SegmentedCommands = CommandList{
	"compress":   segmentedCompress,
	"decompress": segmentedDecompress,
}

func doSegmented(args []string) {
	if len(args) < 1 {
		SegmentedCommands.Usage()
	}

	command, ok := SegmentedCommands[args[0]]
	if !ok {
		SegmentedCommands.Usage()
	}

	command(args[1:])
}
