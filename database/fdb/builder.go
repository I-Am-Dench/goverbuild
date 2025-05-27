package fdb

import (
	"encoding/binary"
	"fmt"
	"io"
	"slices"
	"strings"
)

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

func (b *Builder) writeDescription(w io.WriteSeeker, table *Table) (n int64, err error) {

	// if err := binary.Write(w, order, uint32(len(table.Columns))); err != nil {
	// 	return 0, fmt.Errorf("num columns: %w", err)
	// }
	// n += 4

	// pos, err := w.Seek(0, io.SeekCurrent)
	// if err != nil {
	// 	return 0, fmt.Errorf("table name: %w", err)
	// }

	// b.stringPool.Add(uint32(pos), table.Name)
	// if err := binary.Write(w, order, uint32(0)); err != nil {
	// 	return 0, fmt.Errorf("table name: %w", err)
	// }
	// n += 4

	// if err := binary.Write(w, order, pos+8); err != nil {
	// 	return 0, fmt.Errorf("column offset: %w", err)
	// }
	// n += 4

	// for _, column := range table.Columns {
	// 	if err := binary.Write(w, order, column.Variant); err != nil {
	// 		return 0, fmt.Errorf("variant: %s: %v", column.Name, err)
	// 	}
	// 	n += 4

	// }

	return 0, nil
}

func (b *Builder) writeRows(w io.WriteSeeker, table *Table, rows RowsFunc) (n int64, err error) {
	return 0, nil
}

func (b *Builder) FlushTo(w io.WriteSeeker, rows RowsFunc) error {
	n, err := w.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	tableOffset := uint32(n) + 8

	if err := binary.Write(w, order, uint32(len(b.tables))); err != nil {
		return fmt.Errorf("flush to: num tables: %v", err)
	}

	if err := binary.Write(w, order, tableOffset); err != nil {
		return fmt.Errorf("flush to: table offset: %v", err)
	}

	dw := newDeferredWriter(w, tableOffset)

	writeDescription := true
	if err := dw.Array(len(b.tables)*2, 4, func(w io.WriteSeeker, i int) (n int64, err error) {
		table := b.tables[i/2]

		if writeDescription {
			writeDescription = false
			return b.writeDescription(w, table)
		} else {
			writeDescription = true
			return b.writeRows(w, table, rows)
		}
	}); err != nil {
		return fmt.Errorf("flush to: %v", err)
	}

	return nil
}

// type stringPool struct {
// 	Strings map[string]uint32
// 	Defered map[uint32]string
// }

// func (p *stringPool) Add(home uint32, s string) {
// 	if p.Defered == nil {
// 		p.Defered = map[uint32]string{}
// 	}

// 	p.Defered[home] = s
// }

// func (p *stringPool) writeString(w io.WriteSeeker, s string, address uint32) (written int, err error) {
// 	const alignment = 4

// 	if _, err := w.Seek(int64(address), io.SeekStart); err != nil {
// 		return 0, err
// 	}

// 	n, err := WriteNullTerminatedString(s, w)
// 	if err != nil {
// 		return 0, err
// 	}
// 	written += n

// 	// Alignment padding; Suggested by documentation
// 	cur := address + uint32(written)
// 	if padding := alignment - (cur % alignment); padding < alignment {
// 		zeros := [3]byte{}
// 		if _, err := w.Write(zeros[:padding]); err != nil {
// 			return 0, err
// 		}
// 		written += int(padding)
// 	}

// 	return written, nil
// }

// func (p *stringPool) Flush(w io.WriteSeeker, base uint32) (n int, err error) {
// 	if p.Defered == nil {
// 		return 0, nil
// 	}

// 	for home, text := range p.Defered {
// 		address, ok := p.Strings[text]
// 		if !ok {
// 			written, err := p.writeString(w, text, base)
// 			if err != nil {
// 				return 0, fmt.Errorf("string pool: flush: %v", err)
// 			}
// 			n += written

// 			p.Strings[text] = base
// 			address = base

// 			base += uint32(written)
// 		}

// 		if _, err := w.Seek(int64(home), io.SeekStart); err != nil {
// 			return 0, fmt.Errorf("string pool: flush: %v", err)
// 		}

// 		if err := binary.Write(w, order, address); err != nil {
// 			return 0, fmt.Errorf("string pool: flush: %v", err)
// 		}
// 	}

// 	clear(p.Defered)
// 	return n, nil
// }

// type RowsFunc func(tableName string) func() (row Row, err error)

// type Builder struct {
// 	stringPool stringPool
// 	tables     []*Table
// }

// func NewBuilder(tables []*Table) *Builder {
// 	slices.SortFunc(tables, func(a, b *Table) int {
// 		return strings.Compare(a.Name, b.Name)
// 	})

// 	return &Builder{
// 		stringPool: stringPool{
// 			Strings: map[string]uint32{},
// 		},
// 		tables: tables,
// 	}
// }

// func (b *Builder) writeHeader(w io.WriteSeeker) (tableOffset uint32, err error) {
// 	n, err := w.Seek(0, io.SeekCurrent)
// 	if err != nil {
// 		return 0, err
// 	}
// 	tableOffset = uint32(n) + 8

// 	if err := binary.Write(w, order, uint32(len(b.tables))); err != nil {
// 		return 0, fmt.Errorf("num tables: %w", err)
// 	}

// 	if err := binary.Write(w, order, tableOffset); err != nil {
// 		return 0, fmt.Errorf("table offset: %w", err)
// 	}

// 	return tableOffset, nil
// }

// func (b *Builder) writeTable(w io.WriteSeeker, table *Table, address uint32) (pos uint32, err error) {
// 	pos = address

// 	if err := binary.Write(w, order, uint32(len(table.Columns))); err != nil {
// 		return 0, fmt.Errorf("num columns: %w", err)
// 	}
// 	pos += 4

// 	if err := binary.Write(w, order, uint32(0)); err != nil {
// 		return 0, fmt.Errorf("table name: %w", err)
// 	}
// 	b.stringPool.Add(pos, table.Name)
// 	pos += 4

// 	if err := binary.Write(w, order, pos+4); err != nil {
// 		return 0, fmt.Errorf("column offset: %w", err)
// 	}
// 	pos += 4

// 	for _, column := range table.Columns {
// 		if err := binary.Write(w, order, column.Variant); err != nil {
// 			return 0, fmt.Errorf("variant: %s: %v", column.Name, err)
// 		}
// 		pos += 4

// 		if err := binary.Write(w, order, uint32(0)); err != nil {
// 			return 0, fmt.Errorf("column name: %s: %w", column.Name, err)
// 		}
// 		b.stringPool.Add(pos, column.Name)
// 		pos += 4
// 	}

// 	written, err := b.stringPool.Flush(w, pos)
// 	if err != nil {
// 		return 0, err
// 	}
// 	pos += uint32(written)

// 	return pos, nil
// }

// func (b *Builder) collectBuckets(table *Table, f RowsFunc) ([][]Row, error) {
// 	if len(table.Columns) == 0 {
// 		return [][]Row{}, nil
// 	}

// 	rowsById := map[uint32][]Row{}

// 	iter := f(table.Name)
// 	row, err := iter()
// 	for ; err == nil; row, err = iter() {
// 		if len(table.Columns) != len(row) {
// 			return nil, fmt.Errorf("%s: mismatched columns: expected %d columns but got %d", table.Name, len(table.Columns), len(row))
// 		}

// 		key, err := row.Id()
// 		if err != nil {
// 			return nil, err
// 		}

// 		rows := rowsById[uint32(key)]
// 		rowsById[uint32(key)] = append(rows, row)
// 	}

// 	if err != io.EOF {
// 		return nil, err
// 	}

// 	buckets := make([][]Row, bitCeil(len(rowsById)))
// 	for id, rows := range rowsById {
// 		index := id % uint32(len(buckets))
// 		buckets[index] = append(buckets[index], rows...)
// 	}

// 	return buckets, nil
// }

// func (b *Builder) writeRows(w io.WriteSeeker, table *Table, f RowsFunc, address uint32) (pos uint32, err error) {
// 	pos = address

// 	buckets, err := b.collectBuckets(table, f)
// 	if err != nil {
// 		return 0, fmt.Errorf("buckets: %w", err)
// 	}

// 	if err := binary.Write(w, order, uint32(len(buckets))); err != nil {
// 		return 0, fmt.Errorf("buckets: %w", err)
// 	}
// 	pos += 4

// 	for i := 0; i < len(buckets); i++ {
// 		if err := binary.Write(w, order, noData); err != nil {
// 			return 0, fmt.Errorf("buckets: %w", err)
// 		}
// 		pos += 4
// 	}

// 	return pos, nil
// }

// func (b *Builder) FlushTo(w io.WriteSeeker, rows RowsFunc) error {
// 	tableOffset, err := b.writeHeader(w)
// 	if err != nil {
// 		return fmt.Errorf("flush to: %v", err)
// 	}

// 	for range b.tables {
// 		// Allocate initial table array, initialized to 0xffffffff
// 		if err := errors.Join(
// 			binary.Write(w, order, noData),
// 			binary.Write(w, order, noData),
// 		); err != nil {
// 			return fmt.Errorf("flush to: %v", err)
// 		}
// 	}

// 	descriptionOffset := tableOffset + uint32(len(b.tables)*8)

// 	for i, table := range b.tables {
// 		if _, err := w.Seek(int64(tableOffset)+int64(i*8), io.SeekStart); err != nil {
// 			return fmt.Errorf("flush to: %v", err)
// 		}

// 		if err := binary.Write(w, order, descriptionOffset); err != nil {
// 			return fmt.Errorf("flush to: description offset: %v", err)
// 		}

// 		if _, err := w.Seek(int64(descriptionOffset), io.SeekStart); err != nil {
// 			return fmt.Errorf("flush to: description offset: %v", err)
// 		}

// 		pos, err := b.writeTable(w, table, uint32(descriptionOffset))
// 		if err != nil {
// 			return fmt.Errorf("flush to: description: %v", err)
// 		}

// 		pos, err = b.writeRows(w, table, rows, pos)
// 		if err != nil {
// 			return fmt.Errorf("flush to: rows: %v", err)
// 		}
// 		descriptionOffset = pos
// 	}

// 	return nil
// }
