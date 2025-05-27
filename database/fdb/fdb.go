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

type DB struct {
	f *os.File

	tables []*Table
}

func (db *DB) Tables() []*Table {
	return db.tables
}

func (db *DB) FindTable(name string) (*Table, bool) {
	for _, table := range db.tables {
		if table.Name == name {
			return table, true
		}
	}
	return nil, false
}

func (db *DB) readColumns(r io.ReadSeeker, offset uint32, numColumns int) ([]*Column, error) {
	if _, err := r.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("read columns: %w", err)
	}

	columnData := make([]struct {
		DataType    Variant
		NamePointer uint32
	}, numColumns)
	for i := range columnData {
		if err := errors.Join(
			binary.Read(r, order, &columnData[i].DataType),
			binary.Read(r, order, &columnData[i].NamePointer),
		); err != nil {
			return nil, fmt.Errorf("read columns: %w", err)
		}
	}

	columns := make([]*Column, numColumns)
	for i, data := range columnData {
		if _, err := r.Seek(int64(data.NamePointer), io.SeekStart); err != nil {
			return nil, fmt.Errorf("read columns: %w", err)
		}

		name, err := ReadNullTerminatedString(r)
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

func (db *DB) readHashTable(r io.ReadSeeker, offset uint32) (*HashTable, error) {
	if _, err := r.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("read hash table: %w", err)
	}

	var (
		numBuckets,
		bucketsOffset uint32
	)
	if err := errors.Join(
		binary.Read(r, order, &numBuckets),
		binary.Read(r, order, &bucketsOffset),
	); err != nil {
		return nil, fmt.Errorf("read hash table: %w", err)
	}

	return &HashTable{
		r:          r,
		base:       int64(bucketsOffset),
		numBuckets: int(numBuckets),
	}, nil
}

func (db *DB) readTable(r io.ReadSeeker, description, hashTable uint32) (*Table, error) {
	if _, err := r.Seek(int64(description), io.SeekStart); err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	var (
		numColumns,
		namePointer,
		columnOffset uint32
	)

	if err := errors.Join(
		binary.Read(r, order, &numColumns),
		binary.Read(r, order, &namePointer),
		binary.Read(r, order, &columnOffset),
	); err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	if _, err := r.Seek(int64(namePointer), io.SeekStart); err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	name, err := ReadNullTerminatedString(r)
	if err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	columns, err := db.readColumns(r, columnOffset, int(numColumns))
	if err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	ht, err := db.readHashTable(r, hashTable)
	if err != nil {
		return nil, fmt.Errorf("read table: %w", err)
	}

	return &Table{
		name,
		columns,
		ht,
	}, nil
}

func (db *DB) init() error {
	var (
		numTables,
		tablesOffset uint32
	)

	if err := errors.Join(
		binary.Read(db.f, order, &numTables),
		binary.Read(db.f, order, &tablesOffset),
	); err != nil {
		return fmt.Errorf("init: %v", err)
	}

	if _, err := db.f.Seek(int64(tablesOffset), io.SeekStart); err != nil {
		return fmt.Errorf("init: tables offset: %v", err)
	}

	tableOffsets := make([]struct{ Description, HashTable uint32 }, numTables)
	for i := range tableOffsets {
		if err := errors.Join(
			binary.Read(db.f, order, &tableOffsets[i].Description),
			binary.Read(db.f, order, &tableOffsets[i].HashTable),
		); err != nil {
			return fmt.Errorf("init: table offsets: %v", err)
		}
	}

	for _, offsets := range tableOffsets {
		table, err := db.readTable(db.f, offsets.Description, offsets.HashTable)
		if err != nil {
			return fmt.Errorf("init: %v", err)
		}

		db.tables = append(db.tables, table)
	}

	return nil
}

func (db *DB) Close() error {
	return db.f.Close()
}

func Open(name string) (*DB, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("fdb: %w", err)
	}

	db := &DB{
		f: file,

		tables: []*Table{},
	}
	if err := db.init(); err != nil {
		return nil, fmt.Errorf("fdb: %v", err)
	}

	return db, nil
}
