package main

import (
	"database/sql"
	"fmt"
	"iter"
	"strings"

	"github.com/I-Am-Dench/goverbuild/database/fdb"
)

// type dataEntry struct {
// 	variant fdb.Variant
// 	data    any
// }

// func (e *dataEntry) Variant() fdb.Variant {
// 	return e.variant
// }

// func (e *dataEntry) RawData() uint32 {
// 	return 0
// }

// func (e *dataEntry) Int32() int32 {
// 	return e.data.(int32)
// }

// func (e *dataEntry) Uint32() uint32 {
// 	return e.data.(uint32)
// }

// func (e *dataEntry) Float32() float32 {
// 	return e.data.(float32)
// }

// func (e *dataEntry) String() (string, error) {
// 	return e.data.(string), nil
// }

// func (e *dataEntry) Bool() bool {
// 	return e.data.(bool)
// }

// func (e *dataEntry) Int64() (int64, error) {
// 	return e.data.(int64), nil
// }

// func (e *dataEntry) Uint64() (uint64, error) {
// 	return e.data.(uint64), nil
// }

// func (e *dataEntry) IsNull() bool {
// 	return e.data == nil
// }

// func (e *dataEntry) scanString(s string) error {
// 	if e.variant != fdb.VariantNVarChar && e.variant != fdb.VariantText {
// 		return fmt.Errorf("cannot scan string into %v", e.variant)
// 	}
// 	e.data = s
// 	return nil
// }

// func (e *dataEntry) scanFloat64(f float64) error {
// 	if e.variant != fdb.VariantReal {
// 		return fmt.Errorf("cannot scan float64 into %v", e.variant)
// 	}
// 	e.data = float32(f)
// 	return nil
// }

// func (e *dataEntry) scanInt64(i int64) error {
// 	if e.variant == fdb.VariantI32 {
// 		e.data = int32(i)
// 	} else if e.variant == fdb.VariantI64 {
// 		e.data = i
// 	} else if e.variant == fdb.VariantBool {
// 		if i == 0 {
// 			e.data = false
// 		} else {
// 			e.data = true
// 		}
// 	} else {
// 		return fmt.Errorf("cannot scan int64 into %v", e.variant)
// 	}
// 	return nil
// }

// func (e *dataEntry) scanUint64(i uint64) error {
// 	if e.variant == fdb.VariantU32 {
// 		e.data = uint32(i)
// 	} else if e.variant == fdb.VariantU64 {
// 		e.data = i
// 	} else {
// 		return fmt.Errorf("cannot scan uint64 into %v", e.variant)
// 	}
// 	return nil
// }

// func (e *dataEntry) Scan(value any) error {
// 	if value == nil {
// 		e.data = nil
// 		e.variant = fdb.VariantNull
// 		return nil
// 	}

// 	switch v := value.(type) {
// 	case string:
// 		return e.scanString(v)
// 	case float64:
// 		return e.scanFloat64(v)
// 	case int64:
// 		return e.scanInt64(v)
// 	case uint64:
// 		return e.scanUint64(v)
// 	default:
// 		return fmt.Errorf("cannot scan %T", v)
// 	}
// }

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
