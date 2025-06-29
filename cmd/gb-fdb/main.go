package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

type Converter interface {
	WriteFdb(io.WriteSeeker) error
}

var DriverName string

const Usage = `Usage:
	gb-fdb toFdb [options] <database DSN> [output file]
	gb-fdb fromFdb [options] <input file> <database DSN>`

func usage(flagset *flag.FlagSet) func() {
	return func() {
		fmt.Println(Usage)
		fmt.Println("\nOptions:")
		flagset.PrintDefaults()
	}
}

var Converters = map[string]func(db *sql.DB) Converter{
	"sqlite3": NewSqlite,
}

func GetConverter(driverName, dsn string) (Converter, error) {
	converter, ok := Converters[driverName]
	if !ok {
		return nil, fmt.Errorf("unknown converter: %s", driverName)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("get converter: %v", err)
	}

	return converter(db), nil
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("gb-fdb: ")

	flagset := flag.NewFlagSet("gb-fdb", flag.ExitOnError)
	flagset.StringVar(&DriverName, "driver", "sqlite3", "Supported drivers: sqlite3")
	flagset.Usage = usage(flagset)

	if len(os.Args) < 2 {
		flagset.Usage()
		return
	}

	flagset.Parse(os.Args[2:])

	switch subcommand := os.Args[1]; subcommand {
	case "toFdb":
		input := flagset.Arg(0)
		if len(input) == 0 {
			log.Fatal("missing database DSN")
		}

		output := flagset.Arg(1)
		if len(output) == 0 {
			output = "cdclient.fdb"
		}

		converter, err := GetConverter(DriverName, input)
		if err != nil {
			log.Fatal(err)
		}

		file, err := os.Create(output)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		if err := converter.WriteFdb(file); err != nil {
			log.Fatal(err)
		}
	case "fromFdb":
	default:
		log.Fatalf("unknown subcommand: %s", subcommand)
	}
}
