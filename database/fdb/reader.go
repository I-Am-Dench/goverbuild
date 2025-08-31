package fdb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
		return nil, fmt.Errorf("read columns: %w", err)
	}

	columnData := make([]struct {
		DataType    Variant
		NamePointer uint32
	}, numColumns)
	for i := range columnData {
		if err := errors.Join(
			binary.Read(rs, order, &columnData[i].DataType),
			binary.Read(rs, order, &columnData[i].NamePointer),
		); err != nil {
			return nil, fmt.Errorf("read columns: %w", err)
		}
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
		return nil, fmt.Errorf("read hash table: %w", err)
	}

	var (
		numBuckets,
		bucketsOffset uint32
	)
	if err := errors.Join(
		binary.Read(rs, order, &numBuckets),
		binary.Read(rs, order, &bucketsOffset),
	); err != nil {
		return nil, fmt.Errorf("read hash table: %w", err)
	}

	return &HashTable{
		r:          rs,
		base:       int64(bucketsOffset),
		numBuckets: int(numBuckets),
	}, nil
}

func (r *Reader) readTable(rs io.ReadSeeker, description, hashTable uint32) (*Table, error) {
	if _, err := rs.Seek(int64(description), io.SeekStart); err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	var (
		numColumns,
		namePointer,
		columnOffset uint32
	)

	if err := errors.Join(
		binary.Read(rs, order, &numColumns),
		binary.Read(rs, order, &namePointer),
		binary.Read(rs, order, &columnOffset),
	); err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

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
	var (
		numTables,
		tablesOffset uint32
	)

	if err := errors.Join(
		binary.Read(r.f, order, &numTables),
		binary.Read(r.f, order, &tablesOffset),
	); err != nil {
		return fmt.Errorf("init: %v", err)
	}

	if _, err := r.f.Seek(int64(tablesOffset), io.SeekStart); err != nil {
		return fmt.Errorf("init: tables offset: %v", err)
	}

	tableOffsets := make([]struct{ Description, HashTable uint32 }, numTables)
	for i := range tableOffsets {
		if err := errors.Join(
			binary.Read(r.f, order, &tableOffsets[i].Description),
			binary.Read(r.f, order, &tableOffsets[i].HashTable),
		); err != nil {
			return fmt.Errorf("init: table offsets: %v", err)
		}
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

func (r *Reader) Close() error {
	if r.closer {
		return r.f.Close()
	}
	return nil
}

func New(file *os.File) (*Reader, error) {
	r := &Reader{
		f:      file,
		tables: []*Table{},
	}
	if err := r.init(); err != nil {
		return nil, fmt.Errorf("fdb: %v", err)
	}

	return r, nil
}

func Open(name string) (*Reader, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("fdb: %w", err)
	}

	r, err := New(file)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("fdb: %v", err)
	}
	r.closer = true

	return r, nil
}
