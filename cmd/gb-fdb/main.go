package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/I-Am-Dench/goverbuild/database/fdb"
	_ "github.com/mattn/go-sqlite3"
)

var (
	VerboseFlag  bool
	ExcludeTable string
)

type verboseWriter struct{}

func (v verboseWriter) Write(b []byte) (int, error) {
	if VerboseFlag {
		return os.Stdout.Write(b)
	} else {
		return len(b), nil
	}
}

var (
	Error   = log.New(os.Stderr, "gd-fdb: ", 0)
	Verbose = log.New(verboseWriter{}, "gd-fdb: ", 0)
)

type Exclude struct {
	All     bool
	Columns []string
}

type Converter interface {
	WriteFdb(w io.WriteSeeker, exclude map[string]*Exclude) error
	ReadFdb(*fdb.Reader) error
	GetExcludeTable(tableName string) (map[string]*Exclude, error)
}

var DriverName string

const Usage = `Usage:
	gb-fdb toFdb [options] <database DSN> [output file]
	gb-fdb fromFdb [options] <database DSN> [input file]`

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
	flagset := flag.NewFlagSet("gb-fdb", flag.ExitOnError)
	flagset.BoolVar(&VerboseFlag, "v", false, "Enable verbose logging.")
	flagset.StringVar(&ExcludeTable, "excludeTable", "", "The name of the table that indicates which columns to exclude when converting to FDB. The game originally used the DBExclude table. See: https://docs.lu-dev.net/en/latest/database/DBExclude.html")
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
			Error.Fatal("missing database DSN")
		}

		output := flagset.Arg(1)
		if len(output) == 0 {
			output = "cdclient.fdb"
		}

		converter, err := GetConverter(DriverName, input)
		if err != nil {
			Error.Fatal(err)
		}

		excludes := make(map[string]*Exclude)
		if len(ExcludeTable) > 0 {
			e, err := converter.GetExcludeTable(ExcludeTable)
			if err != nil {
				Error.Fatal(err)
			}
			excludes = e
		}

		Verbose.Printf("Converting %s (%s) to %s", DriverName, input, output)

		file, err := os.Create(output)
		if err != nil {
			Error.Fatal(err)
		}
		defer file.Close()

		if err := converter.WriteFdb(file, excludes); err != nil {
			Error.Fatal(err)
		}
	case "fromFdb":
		output := flagset.Arg(0)
		if len(output) == 0 {
			Error.Fatal("missing database DSN")
		}

		input := flagset.Arg(1)
		if len(input) == 0 {
			input = "cdclient.fdb"
		}

		converter, err := GetConverter(DriverName, output)
		if err != nil {
			Error.Fatal(err)
		}

		Verbose.Printf("Converting %s to %s (%s)", input, DriverName, output)

		r, err := fdb.OpenReader(input)
		if err != nil {
			Error.Fatal(err)
		}

		if err := converter.ReadFdb(r); err != nil {
			Error.Fatal(err)
		}
	default:
		Error.Fatalf("unknown subcommand: %s", subcommand)
	}
}
