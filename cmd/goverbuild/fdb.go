package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/I-Am-Dench/goverbuild/database/fdb"
)

type RowWriter interface {
	Row(fdb.Row)
	Flush() error
}

type FdbTable struct {
	*tabwriter.Writer

	Columns []*fdb.Column
}

func NewFdbTable(w io.Writer, columns []*fdb.Column, withColumnTypes bool) *FdbTable {
	tab := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, column := range columns {
		if withColumnTypes {
			fmt.Fprintf(tab, "%s[%s]\t", column.Name, column.Variant)
		} else {
			fmt.Fprintf(tab, "%s\t", column.Name)
		}
	}
	io.WriteString(tab, "\n")

	return &FdbTable{tab, columns}
}

func (tab *FdbTable) Row(row fdb.Row) {
	for i := 0; i < len(tab.Columns); i++ {
		entry, err := row.Column(i)
		if err != nil {
			log.Fatal(err)
		}

		switch entry.Variant() {
		case fdb.VariantNull:
			io.WriteString(tab, "[null]")
		case fdb.VariantI32:
			fmt.Fprintf(tab, "%d", entry.Int32())
		case fdb.VariantU32:
			fmt.Fprintf(tab, "%d", entry.Uint32())
		case fdb.VariantReal:
			fmt.Fprintf(tab, "%g", entry.Float32())
		case fdb.VariantNVarChar, fdb.VariantText:
			s, err := entry.String()
			if err != nil {
				log.Fatal(err)
			}
			io.WriteString(tab, s)
		case fdb.VariantBool:
			fmt.Fprintf(tab, "%t", entry.Bool())
		case fdb.VariantI64:
			v, err := entry.Int64()
			if err != nil {
				log.Fatal(err)
			}
			fmt.Fprintf(tab, "%d", v)
		case fdb.VariantU64:
			v, err := entry.Uint64()
			if err != nil {
				log.Fatal(err)
			}
			fmt.Fprintf(tab, "%d", v)
		default:
			fmt.Fprintf(tab, "?unknown: %s?", entry.Variant())
		}
		io.WriteString(tab, "\t")
	}
	io.WriteString(tab, "\n")
}

type FdbCsv struct {
	*csv.Writer

	Columns []*fdb.Column
}

func NewFdbCsv(w io.Writer, columns []*fdb.Column, withHeader bool) *FdbCsv {
	c := csv.NewWriter(w)

	if withHeader {
		record := make([]string, len(columns))
		for i, column := range columns {
			record[i] = column.Name
		}
		c.Write(record)
	}

	return &FdbCsv{c, columns}
}

func (c *FdbCsv) Row(row fdb.Row) {
	record := make([]string, len(c.Columns))

	for i := 0; i < len(c.Columns); i++ {
		entry, err := row.Column(i)
		if err != nil {
			log.Fatal(err)
		}

		switch entry.Variant() {
		case fdb.VariantNull:
			record[i] = ""
		case fdb.VariantI32:
			record[i] = strconv.FormatInt(int64(entry.Int32()), 10)
		case fdb.VariantU32:
			record[i] = strconv.FormatUint(uint64(entry.Uint32()), 10)
		case fdb.VariantReal:
			record[i] = strconv.FormatFloat(float64(entry.Float32()), 'g', -1, 32)
		case fdb.VariantNVarChar, fdb.VariantText:
			s, err := entry.String()
			if err != nil {
				log.Fatal(err)
			}
			record[i] = s
		case fdb.VariantBool:
			if entry.Bool() {
				record[i] = "true"
			} else {
				record[i] = "false"
			}
		case fdb.VariantI64:
			v, err := entry.Int64()
			if err != nil {
				log.Fatal(err)
			}
			record[i] = strconv.FormatInt(v, 10)
		case fdb.VariantU64:
			v, err := entry.Uint64()
			if err != nil {
				log.Fatal(err)
			}
			record[i] = strconv.FormatUint(v, 10)
		default:
			record[i] = fmt.Sprintf("?unknown: %s?", entry.Variant())
		}
	}

	c.Write(record)
}

func (c *FdbCsv) Flush() error {
	c.Writer.Flush()
	return nil
}

func fdbDump(args []string) {
	flagset := flag.NewFlagSet("fdb:dump", flag.ExitOnError)
	withColumnTypes := flagset.Bool("coltypes", false, "Show type information next to column name.")
	asCsv := flagset.Bool("csv", false, "Write table info as a csv.")
	csvHeader := flagset.Bool("csvheader", false, "Write column names as the first row of csv data.")
	flagset.Parse(args)

	path := GetArgFilename(flagset, 0)

	if flagset.NArg() < 2 {
		log.Fatalf("missing table name")
	}

	tableName := flagset.Args()[1]

	db, err := fdb.OpenReader(path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", path)
	}

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	table, ok := db.FindTable(tableName)
	if !ok {
		log.Fatalf("table does not exist: %s", tableName)
	}

	var w RowWriter
	if *asCsv {
		w = NewFdbCsv(os.Stdout, table.Columns, *csvHeader)
	} else {
		w = NewFdbTable(os.Stdout, table.Columns, *withColumnTypes)
	}

	for row, err := range table.Rows() {
		if err != nil {
			log.Fatal(err)
		}

		w.Row(row)
	}

	if !*asCsv {
		fmt.Fprintf(os.Stdout, "%s\n%s\n", table.Name, strings.Repeat("=", len(table.Name)))
	}
	w.Flush()
}

func fdbTables(args []string) {
	flagset := flag.NewFlagSet("fdb:tables", flag.ExitOnError)
	flagset.Parse(args)

	path := GetArgFilename(flagset, 0)

	db, err := fdb.OpenReader(path)
	if errors.Is(err, os.ErrNotExist) {
		log.Fatalf("file does not exist: %s", path)
	}

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	for _, table := range db.Tables() {
		fmt.Println(table.Name)
	}
}

var FdbCommands = CommandList{
	"tables": fdbTables,
	"dump":   fdbDump,
}

func doFdb(args []string) {
	if len(args) < 1 {
		FdbCommands.Usage()
	}

	command, ok := FdbCommands[args[0]]
	if !ok {
		FdbCommands.Usage()
	}

	command(args[1:])
}
