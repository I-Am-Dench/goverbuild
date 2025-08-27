package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/I-Am-Dench/goverbuild/compress/segmented"
)

func segmentedCompress(args []string) {
	flagset := flag.NewFlagSet("segmented:compress", flag.ExitOnError)
	output := flagset.String("o", "", "Sets the output path. If this options is not specified, the output name is the input name suffixed with '.sd0'.")
	chunkSize := flagset.Int("chunkSize", segmented.DefaultChunkSize, "Sets the compression chunk size.")
	flagset.Parse(args)

	inputName := flagset.Arg(0)
	if len(inputName) == 0 {
		Error.Fatal("input name not provided")
	}

	outputName := GetOutputName(*output, inputName+".sd0")

	inputFile, err := os.Open(inputName)
	if err != nil {
		Error.Fatal(err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputName)
	if err != nil {
		Error.Fatal(err)
	}
	defer outputFile.Close()

	compressor := segmented.NewDataWriterSize(outputFile, *chunkSize)

	if _, err := io.Copy(compressor, inputFile); err != nil {
		Error.Fatal(err)
	}

	if err := compressor.Close(); err != nil {
		Error.Fatal(err)
	}
}

func segmentedDecompress(args []string) {
	flagset := flag.NewFlagSet("segmented:compress", flag.ExitOnError)
	output := flagset.String("o", "", "Sets the output path. If this options is not specified, the output name is the input name trimmed of the '.sd0' suffix.")
	flagset.Parse(args)

	inputName := flagset.Arg(0)
	if len(inputName) == 0 {
		Error.Fatal("input name not provided")
	}

	outputName := GetOutputName(*output, strings.TrimSuffix(filepath.Base(inputName), ".sd0"))

	inputFile, err := os.Open(inputName)
	if err != nil {
		Error.Fatal(err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputName)
	if err != nil {
		Error.Fatal(err)
	}
	defer outputFile.Close()

	decompressor, err := segmented.NewDataReader(inputFile)
	if err != nil {
		Error.Fatal(err)
	}

	if _, err := io.Copy(outputFile, decompressor); err != nil {
		Error.Fatal(err)
	}
}

var SegmentedCommands = CommandList{
	"compress":   segmentedCompress,
	"decompress": segmentedDecompress,
}

func doSegmented(args []string) {
	SetLogPrefix("goverbuild(segmented): ")

	if len(args) < 1 {
		SegmentedCommands.Usage()
	}

	command, ok := SegmentedCommands[args[0]]
	if !ok {
		SegmentedCommands.Usage()
	}

	command(args[1:])
}
