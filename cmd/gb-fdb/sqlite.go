package main

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/I-Am-Dench/goverbuild/database/fdb"
)

type Sqlite struct {
	*sql.DB
}

func NewSqlite(db *sql.DB) Converter {
	return &Sqlite{db}
}

func (db *Sqlite) toVariant(colType string) (fdb.Variant, bool) {
	switch strings.ToUpper(colType) {
	case "INT32":
		return fdb.VariantI32, true
	case "REAL":
		return fdb.VariantReal, true
	case "TEXT4":
		return fdb.VariantNVarChar, true
	case "INT_BOOL":
		return fdb.VariantBool, true
	case "INT64":
		return fdb.VariantI64, true
	case "TEXT8", "TEXT_XML", "BLOB", "BLOB_NONE":
		return fdb.VariantText, true
	default:
		return fdb.VariantNull, false
	}
}

func (db *Sqlite) toColType(variant fdb.Variant) (string, bool) {
	switch variant {
	case fdb.VariantI32:
		return "INT32", true
	case fdb.VariantReal:
		return "REAL", true
	case fdb.VariantNVarChar:
		return "TEXT4", true
	case fdb.VariantBool:
		return "INT_BOOL", true
	case fdb.VariantI64:
		return "INT64", true
	case fdb.VariantText:
		return "BLOB", true
	case fdb.VariantNull:
		return "BLOB_NONE", true
	default:
		return "", false
	}
}

func (db *Sqlite) queryTable(name string) (*fdb.Table, error) {
	query := "SELECT name, type, pk FROM pragma_table_info(?)"
	Verbose.Print(name, ": ", query)

	rows, err := db.Query(query, name)
	if err != nil {
		return nil, err
	}

	table := &fdb.Table{
		Name:    name,
		Columns: []*fdb.Column{},
	}
	for rows.Next() {
		var (
			colName      string
			colType      string
			isPrimaryKey bool
		)
		if err := rows.Scan(&colName, &colType, &isPrimaryKey); err != nil {
			return nil, fmt.Errorf("%s: %v", name, err)
		}

		variant, ok := db.toVariant(colType)
		if !ok {
			return nil, fmt.Errorf("%s: unknown column type: %s", colName, colType)
		}

		table.Columns = append(table.Columns, &fdb.Column{
			Variant: variant,
			Name:    colName,
		})
	}

	return table, nil
}

func (db *Sqlite) collectTables() ([]*fdb.Table, error) {
	query := "SELECT name FROM sqlite_schema WHERE type = 'table' AND name NOT LIKE 'sqlite_%'"
	Verbose.Println(query)

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("collect tables: %v", err)
	}

	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("collect tables: %v", err)
		}
		names = append(names, name)
	}

	tables := []*fdb.Table{}
	for _, name := range names {
		table, err := db.queryTable(name)
		if err != nil {
			return nil, fmt.Errorf("collect tables: query table: %s: %v", name, err)
		}

		tables = append(tables, table)
	}

	return tables, nil
}

func (db *Sqlite) queryRows(table *fdb.Table) (*sql.Rows, error) {
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

func (db *Sqlite) scanRow(rows *sql.Rows, columns []*fdb.Column) (fdb.Row, error) {
	row := fdb.Row{}
	for _, col := range columns {
		row = append(row, &dataEntry{
			variant: col.Variant,
		})
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

func (db *Sqlite) WriteFdb(w io.WriteSeeker) error {
	tables, err := db.collectTables()
	if err != nil {
		return fmt.Errorf("sqlite3: %v", err)
	}

	byName := map[string]*fdb.Table{}
	for _, table := range tables {
		byName[table.Name] = table
	}

	builder := fdb.NewBuilder(tables)
	if err := builder.FlushTo(w, func(tableName string) func() (row fdb.Row, err error) {
		table := byName[tableName]

		rows, err := db.queryRows(table)
		if err != nil {
			return func() (fdb.Row, error) {
				return nil, fmt.Errorf("%s: %v", table.Name, err)
			}
		}

		return func() (row fdb.Row, err error) {
			if rows.Next() {
				return db.scanRow(rows, table.Columns)
			}

			if rows.Err() != nil {
				return nil, rows.Err()
			}

			return nil, io.EOF
		}
	}); err != nil {
		return fmt.Errorf("sqlite3: %v", err)
	}

	return nil
}

func (db *Sqlite) dropTables(tables []*fdb.Table) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, table := range tables {
		query := fmt.Sprint("DROP TABLE IF EXISTS ", table.Name)
		Verbose.Print(query)

		if _, err := tx.Exec(query); err != nil {
			return fmt.Errorf("drop %s: %v", table.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (db *Sqlite) rowValues(row fdb.Row, values []any) error {
	for i := range values {
		v, err := row.Value(i)
		if err != nil {
			return err
		}

		values[i] = v
	}
	return nil
}

func (db *Sqlite) createTable(table *fdb.Table) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	placeholders := strings.Builder{}
	colNames := strings.Builder{}
	for i, col := range table.Columns {
		colType, ok := db.toColType(col.Variant)
		if !ok {
			return fmt.Errorf("%s: invalid column variant: %v", col.Name, col.Variant)
		}

		if i > 0 {
			placeholders.WriteRune(',')
			colNames.WriteRune(',')
		}

		placeholders.WriteRune('?')

		colNames.WriteRune('"')
		colNames.WriteString(col.Name)
		colNames.WriteString("\" ")
		colNames.WriteString(colType)
	}

	createQuery := fmt.Sprint("CREATE TABLE ", table.Name, " (", colNames.String(), ")")
	Verbose.Println(createQuery)

	if _, err := tx.Exec(createQuery); err != nil {
		return err
	}

	rows, err := table.HashTable().Rows()
	if err != nil {
		return err
	}

	insertQuery := fmt.Sprint("INSERT INTO ", table.Name, " VALUES (", placeholders.String(), ")")
	Verbose.Println(insertQuery)

	values := make([]any, len(table.Columns))
	for rows.Next() {
		if err := db.rowValues(rows.Row(), values); err != nil {
			return err
		}

		if _, err := tx.Exec(insertQuery, values...); err != nil {
			return err
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (db *Sqlite) WriteDb(f *fdb.DB) error {
	if err := db.dropTables(f.Tables()); err != nil {
		return err
	}

	for _, table := range f.Tables() {
		if err := db.createTable(table); err != nil {
			return fmt.Errorf("create table %s: %v", table.Name, err)
		}
	}

	return nil
}
