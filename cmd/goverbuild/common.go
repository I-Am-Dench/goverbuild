package main

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type CommandList map[string]func(args []string)

func (list *CommandList) Usage() {
	keys := []string{}
	for key := range *list {
		keys = append(keys, key)
	}

	log.Fatalf("expected subcommand: {%s}", strings.Join(keys, "|"))
}

func GetOutputName(output, defaultName string) string {
	if len(output) == 0 {
		output = defaultName
	}

	if stat, err := os.Stat(output); err == nil && stat.IsDir() {
		output = filepath.Join(output, filepath.Base(defaultName))
	}

	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		Error.Fatal(err)
	}

	return output
}

func SkipLimitSlice[T any](skip, limit int, list []T) []T {
	if skip < 0 {
		Error.Fatal("starting index must be >= 0")
	}

	if skip >= len(list) {
		Error.Fatalf("index out of bounds: accessing index %d with length %d", skip, len(list))
	}

	endIndex := len(list)
	if limit >= 0 {
		endIndex = min(skip+limit, len(list))
	}

	return list[skip:endIndex]
}

var scanReader = bufio.NewReader(os.Stdin)

func Scanln() string {
	s, _ := scanReader.ReadString('\n')
	return s
}
