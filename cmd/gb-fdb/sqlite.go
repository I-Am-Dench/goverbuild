package main

import (
	"database/sql"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/I-Am-Dench/goverbuild/database/fdb"
)

type Sqlite struct {
	*sql.DB
}

func NewSqlite(db *sql.DB) Converter {
	return &Sqlite{db}
}

func (db Sqlite) toVariant(colType string) (fdb.Variant, bool) {
	switch strings.ToUpper(colType) {
	case "INTEGER", "INT", "INT32":
		return fdb.VariantI32, true
	case "REAL":
		return fdb.VariantReal, true
	case "TEXT", "TEXT4":
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

func (db Sqlite) toColType(variant fdb.Variant) (string, bool) {
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

func moveToFront(columns []*fdb.Column, i int) {
	for j := i - 1; j >= 0; j-- {
		columns[j], columns[j+1] = columns[j+1], columns[j]
	}
}

func (db Sqlite) queryTable(name string, exclude *Exclude) (*fdb.Table, error) {
	query := "SELECT name, type, pk FROM pragma_table_info(?)"
	Verbose.Print(name, ": ", query)

	rows, err := db.Query(query, name)
	if err != nil {
		return nil, err
	}

	primaryKeyIndex := -1
	i := 0

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

		if exclude != nil && slices.Contains(exclude.Columns, colName) {
			Verbose.Print(name, ": excluding column \"", colName, "\"")
			i++
			continue
		}

		table.Columns = append(table.Columns, &fdb.Column{
			Variant: variant,
			Name:    colName,
		})

		if primaryKeyIndex < 0 && isPrimaryKey {
			Verbose.Print("Found primary key \"", colName, "\"")
			primaryKeyIndex = i
		}
		i++
	}

	if primaryKeyIndex > 0 {
		moveToFront(table.Columns, primaryKeyIndex)
	}

	return table, nil
}

func (db Sqlite) collectTables(excludes map[string]*Exclude) ([]*fdb.Table, error) {
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

		exclude, ok := excludes[name]
		if ok && exclude.All {
			Verbose.Print("excluding table \"", name, "\"")
			continue
		}

		names = append(names, name)
	}

	tables := []*fdb.Table{}
	for _, name := range names {
		table, err := db.queryTable(name, excludes[name])
		if err != nil {
			return nil, fmt.Errorf("collect tables: query table: %s: %v", name, err)
		}

		tables = append(tables, table)
	}

	return tables, nil
}

func (db Sqlite) WriteFdb(w io.WriteSeeker, excludes map[string]*Exclude) error {
	tables, err := db.collectTables(excludes)
	if err != nil {
		return fmt.Errorf("sqlite3: %v", err)
	}

	byName := map[string]*fdb.Table{}
	for _, table := range tables {
		byName[table.Name] = table
	}

	builder := fdb.NewBuilder(w, tables)
	if err := builder.Flush(IterTables(db.DB, byName)); err != nil {
		return fmt.Errorf("sqlite3: %v", err)
	}

	return nil
}

func (db Sqlite) dropTables(tables []*fdb.Table) error {
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

func (db Sqlite) rowValues(row fdb.Row, values []any) error {
	for i := range values {
		v, err := row.Value(i)
		if err != nil {
			return err
		}

		values[i] = v
	}
	return nil
}

func (db Sqlite) createTable(table *fdb.Table) error {
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

func (db Sqlite) ReadFdb(r *fdb.Reader) error {
	if err := db.dropTables(r.Tables()); err != nil {
		return err
	}

	for _, table := range r.Tables() {
		if err := db.createTable(table); err != nil {
			return fmt.Errorf("create table %s: %v", table.Name, err)
		}
	}

	return nil
}

func (db Sqlite) GetExcludeTable(tableName string) (map[string]*Exclude, error) {
	excludes := make(map[string]*Exclude)

	query := fmt.Sprint("SELECT \"table\", \"column\" FROM ", tableName)
	Verbose.Println(query)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var name, column string
		if err := rows.Scan(&name, &column); err != nil {
			return nil, err
		}

		exclude, ok := excludes[name]
		if !ok {
			exclude = &Exclude{}
			excludes[name] = exclude
		}

		if column == "*" {
			exclude.All = true
		} else {
			exclude.Columns = append(exclude.Columns, column)
		}
	}

	return excludes, nil
}
