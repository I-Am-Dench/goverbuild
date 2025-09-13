package main

import (
	"database/sql"
	"fmt"
	"iter"
	"strings"

	"github.com/I-Am-Dench/goverbuild/database/fdb"
)

func queryRows(db *sql.DB, table *fdb.Table) (*sql.Rows, error) {
	colNames := strings.Builder{}
	for i, col := range table.Columns {
		if i > 0 {
			colNames.WriteRune(',')
		}
		colNames.WriteRune('"')
		colNames.WriteString(col.Name)
		colNames.WriteRune('"')
	}

	query := fmt.Sprint("SELECT ", colNames.String(), " FROM ", table.Name)
	Verbose.Println(query)
	return db.Query(query)
}

func scanRow(rows *sql.Rows, columns []*fdb.Column) (fdb.Row, error) {
	row := fdb.Row{}
	for _, col := range columns {
		row = append(row, fdb.NewEntry(col.Variant))
	}

	entries := make([]any, len(row))
	for i := range row {
		entries[i] = row[i]
	}

	if err := rows.Scan(entries...); err != nil {
		return nil, err
	}

	return row, nil
}

func iterRows(db *sql.DB, table *fdb.Table) iter.Seq2[fdb.Row, error] {
	return func(yield func(fdb.Row, error) bool) {
		rows, err := queryRows(db, table)
		if err != nil {
			yield(nil, err)
			return
		}

		for rows.Next() {
			row, err := scanRow(rows, table.Columns)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(row, nil) {
				return
			}
		}

		if err := rows.Err(); err != nil {
			yield(nil, err)
		}
	}
}

func IterTables(db *sql.DB, tables map[string]*fdb.Table) fdb.RowsFunc {
	return func(tableName string) iter.Seq2[fdb.Row, error] {
		return iterRows(db, tables[tableName])
	}
}
