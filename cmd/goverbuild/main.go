package main

import (
	"log"
	"os"
)

var VerboseFlag bool

type verboseWriter struct{}

func (v *verboseWriter) Write(b []byte) (int, error) {
	if VerboseFlag {
		return os.Stdout.Write(b)
	} else {
		return len(b), nil
	}
}

var (
	Info    = log.New(os.Stdout, "goverbuild: ", 0)
	Error   = log.New(os.Stderr, "goverbuild: ", 0)
	Verbose = log.New(&verboseWriter{}, "goverbuild: ", 0)
)

func SetLogPrefix(prefix string) {
	Info.SetPrefix(prefix)
	Error.SetPrefix(prefix)
	Verbose.SetPrefix(prefix)
}

var Commands = CommandList{
	"pack":      doPack,
	"catalog":   doCatalog,
	"manifest":  doManifest,
	"extract":   doExtract,
	"cache":     doCache,
	"segmented": doSegmented,
	"fdb":       doFdb,
}

func main() {

	if len(os.Args) < 2 {
		Commands.Usage()
	}

	command, ok := Commands[os.Args[1]]
	if !ok {
		Commands.Usage()
	}

	command(os.Args[2:])
}
