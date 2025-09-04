package fdb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var (
	ErrRowNotFound = errors.New("row not found")
	ErrNullData    = errors.New("null data")
)

const (
	noData uint32 = 0xffffffff
)

// Represents a single linked list of buckets.
type Bucket struct {
	r io.ReadSeeker

	base     int64
	next     uint32
	finished bool

	row Row
	err error
}

func (b *Bucket) readRow(r io.ReadSeeker) (row Row, nextOffset uint32, err error) {
	if b.next == noData {
		return nil, noData, nil
	}

	if _, err := r.Seek(int64(b.next), io.SeekStart); err != nil {
		return nil, 0, err
	}

	data := [8]byte{}
	if _, err := r.Read(data[:]); err != nil {
		return nil, 0, err
	}

	rowDataOffset := order.Uint32(data[:])
	nextOffset = order.Uint32(data[4:])

	if _, err = r.Seek(int64(rowDataOffset), io.SeekStart); err != nil {
		return nil, 0, err
	}

	if _, err := r.Read(data[:]); err != nil {
		return nil, 0, err
	}

	numColumns := order.Uint32(data[:])
	dataArrayOffset := order.Uint32(data[4:])

	if _, err = r.Seek(int64(dataArrayOffset), io.SeekStart); err != nil {
		return nil, 0, err
	}

	entries := make([]Entry, numColumns)
	for i := range entries {
		if _, err := r.Read(data[:]); err != nil {
			return nil, 0, err
		}

		e := &readerEntry{r: r}
		e.variant = Variant(order.Uint32(data[:]))
		e.data = order.Uint32(data[4:])

		entries[i] = e
	}

	return Row(entries), nextOffset, nil
}

// Advances to the next [Row].
func (b *Bucket) Next() bool {
	if b.finished {
		b.err = nil
		return false
	}

	b.row, b.next, b.err = b.readRow(b.r)
	b.finished = b.next == noData || b.err != nil

	return b.err == nil
}

func (b *Bucket) Row() Row {
	return b.row
}

func (b *Bucket) Err() error {
	return b.err
}

// Resets to the first bucket in the linked list.
func (b *Bucket) Reset() error {
	b.finished = false
	b.next = uint32(b.base)
	b.err = nil
	b.row = nil

	return nil
}

// Represents all rows within a [*HashTable].
type Rows struct {
	r io.ReadSeeker

	base       int64
	numBuckets int

	bucketIndex int
	bucket      *Bucket

	err error
}

// Returns the [*Bucket] located at index, i. Bucket panics
// if i >= # of buckets. If no bucket exists at i, Bucket returns
// an [ErrNullData] error.
func (r *Rows) Bucket(i int) (*Bucket, error) {
	if i >= r.numBuckets {
		panic(fmt.Errorf("fdb: rows: bucket: out of range: %d", i))
	}

	if _, err := r.r.Seek(r.base+int64(i*4), io.SeekStart); err != nil {
		return nil, err
	}

	var listOffset uint32
	if err := binary.Read(r.r, order, &listOffset); err != nil {
		return nil, err
	}

	if listOffset == noData {
		return nil, ErrNullData
	}

	b := &Bucket{
		r:    r.r,
		base: int64(listOffset),
	}
	if err := b.Reset(); err != nil {
		return nil, err
	}

	return b, nil
}

func (r *Rows) nextBucket() (*Bucket, error) {
	r.bucketIndex++
	if r.bucketIndex >= r.numBuckets {
		return nil, fmt.Errorf("rows: next bucket: %w", ErrNullData)
	}

	bucket, err := r.Bucket(r.bucketIndex)
	if errors.Is(err, ErrNullData) {
		return r.nextBucket()
	}

	if err != nil {
		return nil, fmt.Errorf("rows: next bucket: %w", err)
	}

	return bucket, nil
}

// Advances to the next [Row].
func (r *Rows) Next() bool {
	if r.bucketIndex >= r.numBuckets {
		return false
	}

	if r.bucket.Next() {
		r.err = nil
		return true
	}

	r.bucket, r.err = r.nextBucket()
	if r.err == ErrNullData {
		r.err = nil
		return false
	}

	if r.err != nil {
		return false
	}

	return r.Next()
}

func (r *Rows) Reset() error {
	r.bucketIndex = -1
	r.bucket = nil
	r.err = nil

	if _, err := r.r.Seek(r.base, io.SeekStart); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	bucket, err := r.nextBucket()
	if errors.Is(err, ErrNullData) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	r.bucket = bucket

	return nil
}

func (r *Rows) Row() Row {
	if r.bucket == nil {
		return nil
	}
	return r.bucket.Row()
}

func (r *Rows) Err() error {
	if r.bucket == nil {
		return nil
	}

	if r.err != nil {
		return r.err
	}

	return r.bucket.Err()
}

// FDB tables store their rows within hash tables
// where each hash table contains an array of linked lists
// of [Bucket]'s. Each bucket then contains a pointer
// to row data.
//
//	+-------+-------+-------+-------+-------+
//	|   0   |   1   |   2   |   3   |   4   |
//	+-------+-------+-------+-------+-------+
//	    |                       |
//	    V                       V
//	+-------+               +-------+
//	| ID: 0 | -> <row data> | ID: 8 | -> <row data>
//	+-------+               +-------+
//	    |
//	    V
//	+-------+
//	| ID: 5 | -> <row data>
//	+-------+
//
// Each row is identified by the first column's value
// converted to an integer. The bucket linked list that
// contains the row is then located at the index: ID % the # of rows.
type HashTable struct {
	r          io.ReadSeeker
	base       int64
	numBuckets int
}

// Returns the [*Bucket] located at index, i. Bucket panics
// if i >= # of buckets. If no bucket exists at i, Bucket returns
// an [ErrNullData] error.
func (h *HashTable) Bucket(i int) (*Bucket, error) {
	if i >= h.numBuckets {
		panic(fmt.Errorf("fdb: hash table: bucket: out of range: %d", i))
	}

	if _, err := h.r.Seek(h.base+int64(i*4), io.SeekStart); err != nil {
		return nil, err
	}

	var listOffset uint32
	if err := binary.Read(h.r, order, &listOffset); err != nil {
		return nil, err
	}

	if listOffset == noData {
		return nil, ErrNullData
	}

	b := &Bucket{
		r:    h.r,
		base: int64(listOffset),
	}
	if err := b.Reset(); err != nil {
		return nil, err
	}

	return b, nil
}

// Returns the first row corresponding to the provided id.
// If no row exists, Find returns a wrapped [ErrRowNotFound] error.
func (h *HashTable) Find(id int) (Row, error) {
	bucket, err := h.Bucket(id % h.numBuckets)
	if errors.Is(err, ErrNullData) {
		return nil, fmt.Errorf("hash table: %w", ErrRowNotFound)
	}

	if err != nil {
		return nil, fmt.Errorf("hash table: %v", err)
	}

	for bucket.Next() {
		row := bucket.Row()

		rowId, err := row.Id()
		if errors.Is(err, ErrNullData) {
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("hash table: %v", err)
		}

		if rowId == id {
			return bucket.Row(), nil
		}
	}

	return nil, fmt.Errorf("hash table: %w", ErrRowNotFound)
}

// Returns the first row corresponding to the provided id.
func (h *HashTable) FindString(id string) (Row, error) {
	return h.Find(int(Sfhash([]byte(id))))
}

func (h *HashTable) Rows() (*Rows, error) {
	r := &Rows{
		r: h.r,

		base:       h.base,
		numBuckets: h.numBuckets,
	}
	if err := r.Reset(); err != nil {
		return nil, err
	}

	return r, nil
}
