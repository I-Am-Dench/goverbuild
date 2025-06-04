package fdb

import (
	"encoding/binary"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/I-Am-Dench/goverbuild/database/fdb/internal/deferredwriter"
)

type writer = deferredwriter.Writer

type RowsFunc func(tableName string) func() (row Row, err error)

type Builder struct {
	tables []*Table
}

func NewBuilder(tables []*Table) *Builder {
	slices.SortFunc(tables, func(a, b *Table) int {
		return strings.Compare(a.Name, b.Name)
	})

	return &Builder{
		tables: tables,
	}
}

func (b *Builder) writeDescription(w *writer, table *Table) (err error) {
	defer func() {
		if err == nil {
			err = w.Flush()
		}
	}()

	if err := w.PutUint32(uint32(len(table.Columns))); err != nil {
		return fmt.Errorf("num columns: %v", err)
	}

	if err := w.DeferString(table.Name); err != nil {
		return fmt.Errorf("table name: %v", err)
	}

	if err := w.Array(len(table.Columns), func(w *writer, i int) error {
		column := table.Columns[i]
		if err := w.PutUint32(uint32(column.Variant)); err != nil {
			return fmt.Errorf("variant: %v", err)
		}

		if err := w.DeferString(column.Name); err != nil {
			return fmt.Errorf("column name: %v", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("columns: %v", err)
	}

	return nil
}

func (b *Builder) collectBuckets(table *Table, f RowsFunc) ([][]Row, error) {
	if len(table.Columns) == 0 {
		return [][]Row{}, nil
	}

	rowsById := make(map[uint32][]Row)

	iter := f(table.Name)

	row, err := iter()
	for ; err == nil; row, err = iter() {
		if len(table.Columns) != len(row) {
			return nil, fmt.Errorf("%s: mismatched columns: expected %d columns but got %d", table.Name, len(table.Columns), len(row))
		}

		key, err := row.Id()
		if err != nil {
			return nil, err
		}

		rows := rowsById[uint32(key)]
		rowsById[uint32(key)] = append(rows, row)
	}

	if err != io.EOF {
		return nil, err
	}

	buckets := make([][]Row, bitCeil(len(rowsById)))
	for id, rows := range rowsById {
		index := id % uint32(len(buckets))
		buckets[index] = append(buckets[index], rows...)
	}

	return buckets, nil
}

func (b *Builder) writeEntry(w *writer, entry Entry) error {
	if err := w.PutUint32(uint32(entry.Variant())); err != nil {
		return fmt.Errorf("variant: %v", entry.Variant())
	}

	switch entry.Variant() {
	case NullVariant:
		if err := w.PutUint32(0); err != nil {
			return fmt.Errorf("null: %v", err)
		}
	case I32Variant:
		if err := w.PutInt32(entry.Int32()); err != nil {
			return fmt.Errorf("int32: %v", err)
		}
	case U32Variant:
		if err := w.PutUint32(entry.Uint32()); err != nil {
			return fmt.Errorf("uint32: %v", err)
		}
	case RealVariant:
		if err := w.PutFloat32(entry.Float32()); err != nil {
			return fmt.Errorf("float32: %v", err)
		}
	case NVarCharVariant, TextVariant:
		s, err := entry.String()
		if err != nil {
			return fmt.Errorf("string: %v", err)
		}

		if err := w.DeferString(s); err != nil {
			return fmt.Errorf("string: %v", err)
		}
	case BoolVariant:
		if err := w.PutBool(entry.Bool()); err != nil {
			return fmt.Errorf("bool: %v", err)
		}
	case I64Variant:
		i, err := entry.Int64()
		if err != nil {
			return fmt.Errorf("int64: %v", err)
		}

		if err := w.DeferInt64(i); err != nil {
			return fmt.Errorf("int64: %v", err)
		}
	case U64Variant:
		i, err := entry.Uint64()
		if err != nil {
			return fmt.Errorf("uint64: %v", err)
		}

		if err := w.DeferUint64(i); err != nil {
			return fmt.Errorf("uint64: %v", err)
		}
	default:
		return fmt.Errorf("unknown variant: %v", entry.Variant())
	}
	return nil
}

func (b *Builder) writeRow(w *writer, row Row) (err error) {
	defer func() {
		if err == nil {
			err = w.Flush()
		}
	}()

	if err := w.PutUint32(uint32(len(row))); err != nil {
		return fmt.Errorf("row: num columns: %v", err)
	}

	if err := w.Array(len(row), func(w *deferredwriter.Writer, i int) error {
		entry, err := row.Column(i)
		if err != nil {
			return fmt.Errorf("row: %d: %v", i, err)
		}

		if err := b.writeEntry(w, entry); err != nil {
			return fmt.Errorf("row: %d: %v", i, err)
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (b *Builder) writeBucket(w *writer, bucket []Row) error {
	return w.DeferredArray(2, func(w *deferredwriter.Writer, i int) (hasData bool, err error) {
		if i == 0 {
			return true, b.writeRow(w, bucket[0])
		}

		if len(bucket) > 1 {
			return true, b.writeBucket(w, bucket[1:])
		}
		return false, nil // No more data in the list
	}, false, 0xff)
}

func (b *Builder) writeBuckets(buckets [][]Row) deferredwriter.DeferredArrayFunc {
	return func(w *deferredwriter.Writer, i int) (hasData bool, err error) {
		bucket := buckets[i]
		if len(bucket) == 0 {
			return false, nil
		}

		if err := b.writeBucket(w, bucket); err != nil {
			return false, fmt.Errorf("bucket: %v", err)
		}

		return true, nil
	}
}

func (b *Builder) writeRows(w *writer, table *Table, rows RowsFunc) error {
	buckets, err := b.collectBuckets(table, rows)
	if err != nil {
		return fmt.Errorf("buckets: %v", err)
	}

	if err := w.PutUint32(uint32(len(buckets))); err != nil {
		return fmt.Errorf("buckets: %v", err)
	}

	if err := w.DeferredArray(len(buckets), b.writeBuckets(buckets), true, 0xff); err != nil {
		return fmt.Errorf("buckets: %v", err)
	}

	return nil
}

func (b *Builder) FlushTo(w io.WriteSeeker, rows RowsFunc) error {
	n, err := w.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	tableOffset := uint32(n) + 4

	if err := binary.Write(w, order, uint32(len(b.tables))); err != nil {
		return fmt.Errorf("flush to: num tables: %v", err)
	}

	dw := deferredwriter.New(w, order, tableOffset)

	writeDescription := true
	if err := dw.DeferredArray(len(b.tables)*2, func(w *writer, i int) (bool, error) {
		table := b.tables[i/2]

		if writeDescription {
			writeDescription = false
			return true, b.writeDescription(w, table)
		} else {
			writeDescription = true
			return true, b.writeRows(w, table, rows)
		}
	}, true); err != nil {
		return fmt.Errorf("flush to: %v", err)
	}

	return nil
}
