package fdb

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type Entry struct {
	r    io.ReadSeeker
	data uint32

	Variant Variant
}

func (e *Entry) Int32() int32 {
	return int32(e.data)
}

func (e *Entry) Uint32() uint32 {
	return e.data
}

func (e *Entry) Float32() float32 {
	return math.Float32frombits(e.data)
}

func (e *Entry) String() (s string, err error) {
	if _, err := e.r.Seek(int64(e.data), io.SeekStart); err != nil {
		return "", err
	}

	s, err = ReadNullTerminatedString(e.r)
	if err != nil {
		return "", err
	}

	return s, nil
}

func (e *Entry) Bool() bool {
	return e.data == 1
}

func (e *Entry) Int64() (v int64, err error) {
	if _, err := e.r.Seek(int64(e.data), io.SeekStart); err != nil {
		return 0, err
	}

	if err := binary.Read(e.r, order, &v); err != nil {
		return 0, err
	}

	return v, nil
}

func (e *Entry) Uint64() (v uint64, err error) {
	if _, err := e.r.Seek(int64(e.data), io.SeekStart); err != nil {
		return 0, err
	}

	if err := binary.Read(e.r, order, &v); err != nil {
		return 0, err
	}

	return v, nil
}

type Row struct {
	entries []Entry
}

func (r *Row) Column(col int) (Entry, error) {
	if col >= len(r.entries) {
		return Entry{}, fmt.Errorf("out of range: %d", col)
	}
	return r.entries[col], nil
}

func (r *Row) Id() (int, error) {
	if len(r.entries) == 0 {
		panic(fmt.Errorf("fdb: row: id: no entries"))
	}

	entry := r.entries[0]
	switch entry.Variant {
	case NullVariant:
		return 0, ErrNullData
	case I32Variant, RealVariant, BoolVariant:
		return int(int32(entry.data)), nil
	case U32Variant:
		return int(uint32(entry.data)), nil
	case I64Variant:
		v, err := entry.Int64()
		if err != nil {
			return 0, err
		}
		return int(v), nil
	case U64Variant:
		v, err := entry.Uint64()
		if err != nil {
			return 0, err
		}
		return int(v), nil
	case NVarCharVariant, TextVariant:
		s, err := entry.String()
		if err != nil {
			return 0, err
		}

		return int(Sfhash([]byte(s))), nil
	default:
		return 0, fmt.Errorf("cannot read id for %s", entry.Variant)
	}
}
