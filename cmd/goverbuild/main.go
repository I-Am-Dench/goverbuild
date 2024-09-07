package main

import (
	"log"
	"os"
)

var Commands = CommandList{
	"pack":     doPack,
	"catalog":  doCatalog,
	"manifest": doManifest,
	"extract":  doExtract,
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("goverbuild: ")

	if len(os.Args) < 2 {
		Commands.Usage()
	}

	command, ok := Commands[os.Args[1]]
	if !ok {
		Commands.Usage()
	}

	command(os.Args[2:])
}
