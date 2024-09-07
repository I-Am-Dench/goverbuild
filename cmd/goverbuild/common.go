package main

import (
	"flag"
	"log"
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

func GetArgFilename(flagset *flag.FlagSet, i int, message ...string) string {
	m := "no filename provided"
	if len(message) > 0 {
		m = message[0]
	}

	if flagset.NArg() < i+1 {
		log.Fatal(m)
	}

	return flagset.Args()[i]
}

func SkipLimitSlice[T any](skip, limit int, list []T) []T {
	if skip < 0 {
		log.Fatal("starting index must be >= 0")
	}

	if skip >= len(list) {
		log.Fatalf("index out of bounds: accessing index %d with length %d", skip, len(list))
	}

	endIndex := len(list)
	if limit >= 0 {
		endIndex = min(skip+limit, len(list))
	}

	return list[skip:endIndex]
}
