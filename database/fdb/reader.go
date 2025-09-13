package fdb

import (
	"fmt"
	"io"
	"iter"
	"os"
)

type Column struct {
	Variant Variant
	Name    string
}

type Table struct {
	Name    string
	Columns []*Column

	hashTable *HashTable
}

func (t *Table) HashTable() *HashTable {
	return t.hashTable
}

func (t *Table) Rows() iter.Seq2[Row, error] {
	return func(yield func(Row, error) bool) {
		if t.hashTable == nil {
			return
		}

		rows, err := t.hashTable.Rows()
		if err != nil {
			yield(nil, err)
			return
		}

		for rows.Next() {
			if !yield(rows.Row(), nil) {
				return
			}
		}

		if err := rows.Err(); err != nil {
			yield(nil, err)
			return
		}
	}
}

// File type: [fdb]
//
// [fdb]: https://docs.lu-dev.net/en/latest/file-structures/database.html
type Reader struct {
	f      *os.File
	closer bool

	tables []*Table
}

func (r *Reader) Tables() []*Table {
	return r.tables
}

func (r *Reader) FindTable(name string) (*Table, bool) {
	for _, table := range r.tables {
		if table.Name == name {
			return table, true
		}
	}
	return nil, false
}

func (r *Reader) readColumns(rs io.ReadSeeker, offset uint32, numColumns int) ([]*Column, error) {
	if _, err := rs.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("read columns: %v", err)
	}

	data := [8]byte{}

	columnData := make([]struct {
		DataType    Variant
		NamePointer uint32
	}, numColumns)
	for i := range columnData {
		if _, err := rs.Read(data[:]); err != nil {
			return nil, fmt.Errorf("read columns: %v", err)
		}

		columnData[i].DataType = Variant(order.Uint32(data[:]))
		columnData[i].NamePointer = order.Uint32(data[4:])
	}

	columns := make([]*Column, numColumns)
	for i, data := range columnData {
		if _, err := rs.Seek(int64(data.NamePointer), io.SeekStart); err != nil {
			return nil, fmt.Errorf("read columns: %w", err)
		}

		name, err := ReadZString(rs)
		if err != nil {
			return nil, fmt.Errorf("read columns: %w", err)
		}

		columns[i] = &Column{
			Variant: data.DataType,
			Name:    name,
		}
	}

	return columns, nil
}

func (r *Reader) readHashTable(rs io.ReadSeeker, offset uint32) (*HashTable, error) {
	if _, err := rs.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("read hash table: %v", err)
	}

	data := [8]byte{}
	if _, err := rs.Read(data[:]); err != nil {
		return nil, fmt.Errorf("read hash table: %v", err)
	}

	numBuckets := order.Uint32(data[:])
	bucketsOffset := order.Uint32(data[4:])

	return &HashTable{
		r:          rs,
		base:       int64(bucketsOffset),
		numBuckets: int(numBuckets),
	}, nil
}

func (r *Reader) readTable(rs io.ReadSeeker, description, hashTable uint32) (*Table, error) {
	if _, err := rs.Seek(int64(description), io.SeekStart); err != nil {
		return nil, fmt.Errorf("read table: %v", err)
	}

	data := [12]byte{}
	if _, err := rs.Read(data[:]); err != nil {
		return nil, fmt.Errorf("read table: %v", err)
	}

	numColumns := order.Uint32(data[:])
	namePointer := order.Uint32(data[4:])
	columnOffset := order.Uint32(data[8:])

	if _, err := rs.Seek(int64(namePointer), io.SeekStart); err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	name, err := ReadZString(rs)
	if err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	columns, err := r.readColumns(rs, columnOffset, int(numColumns))
	if err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	ht, err := r.readHashTable(rs, hashTable)
	if err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	return &Table{
		name,
		columns,
		ht,
	}, nil
}

func (r *Reader) init() error {
	data := [8]byte{}
	if _, err := r.f.Read(data[:]); err != nil {
		return fmt.Errorf("init: %v", err)
	}

	numTables := order.Uint32(data[:])
	tablesOffset := order.Uint32(data[4:])

	if _, err := r.f.Seek(int64(tablesOffset), io.SeekStart); err != nil {
		return fmt.Errorf("init: tables offset: %v", err)
	}

	tableOffsets := make([]struct{ Description, HashTable uint32 }, numTables)
	for i := range tableOffsets {
		if _, err := r.f.Read(data[:]); err != nil {
			return fmt.Errorf("init: table offsets: %v", err)
		}

		tableOffsets[i].Description = order.Uint32(data[:])
		tableOffsets[i].HashTable = order.Uint32(data[4:])
	}

	for _, offsets := range tableOffsets {
		table, err := r.readTable(r.f, offsets.Description, offsets.HashTable)
		if err != nil {
			return fmt.Errorf("init: %v", err)
		}

		r.tables = append(r.tables, table)
	}

	return nil
}

// Closes the underlying [*os.File] only if the Reader
// was created by a call to [OpenReader].
func (r *Reader) Close() error {
	if r.closer {
		return r.f.Close()
	}
	return nil
}

// Creates a [*Reader] with the provided [*os.File].
func NewReader(file *os.File) (*Reader, error) {
	r := &Reader{
		f:      file,
		tables: []*Table{},
	}
	if err := r.init(); err != nil {
		return nil, fmt.Errorf("fdb: %v", err)
	}

	return r, nil
}

// Creates a [*Reader] with the named [*os.File].
func OpenReader(name string) (*Reader, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("fdb: %w", err)
	}

	r, err := NewReader(file)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("fdb: %v", err)
	}
	r.closer = true

	return r, nil
}
