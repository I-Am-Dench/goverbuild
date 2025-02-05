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

type Bucket struct {
	r io.ReadSeeker

	base     int64
	next     uint32
	finished bool

	row *Row
	err error
}

func (b *Bucket) readRow(r io.ReadSeeker) (row *Row, nextOffset uint32, err error) {
	var rowDataOffset uint32
	if err = errors.Join(
		binary.Read(r, order, &rowDataOffset),
		binary.Read(r, order, &nextOffset),
	); err != nil {
		return
	}

	if _, err = r.Seek(int64(rowDataOffset), io.SeekStart); err != nil {
		return
	}

	var (
		numColumns,
		dataArrayOffset uint32
	)
	if err = errors.Join(
		binary.Read(r, order, &numColumns),
		binary.Read(r, order, &dataArrayOffset),
	); err != nil {
		return
	}

	if _, err = r.Seek(int64(dataArrayOffset), io.SeekStart); err != nil {
		return
	}

	entries := make([]Entry, numColumns)
	for i := range entries {
		if err := errors.Join(
			binary.Read(r, order, &entries[i].Variant),
			binary.Read(r, order, &entries[i].data),
		); err != nil {
			return nil, 0, err
		}
		entries[i].r = r
	}

	return &Row{entries}, nextOffset, nil
}

func (b *Bucket) Next() bool {
	if b.finished {
		b.err = nil
		return false
	}

	b.row, b.next, b.err = b.readRow(b.r)
	b.finished = b.next == noData || b.err != nil

	return b.err == nil
}

func (b *Bucket) Row() *Row {
	return b.row
}

func (b *Bucket) Err() error {
	return b.err
}

func (b *Bucket) Reset() error {
	b.finished = false
	b.err = nil
	b.row = nil

	if _, err := b.r.Seek(b.base, io.SeekStart); err != nil {
		return err
	}

	return nil
}

type Rows struct {
	r io.ReadSeeker

	base       int64
	numBuckets int

	bucketIndex int
	bucket      *Bucket

	err error
}

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

func (r *Rows) Row() *Row {
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

type HashTable struct {
	r          io.ReadSeeker
	base       int64
	numBuckets int
}

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

func (h *HashTable) Find(id int) (*Row, error) {
	bucket, err := h.Bucket(id % h.numBuckets)
	if errors.Is(err, ErrNullData) {
		return nil, fmt.Errorf("hash table: %w", ErrRowNotFound)
	}

	if err != nil {
		return nil, fmt.Errorf("hash table: %v", err)
	}

	for bucket.Next() {
		rowId, err := bucket.Row().Id()
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

func (h *HashTable) FindString(id string) (*Row, error) {
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
