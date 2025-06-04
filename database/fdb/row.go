package fdb

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type Entry interface {
	Variant() Variant
	RawData() uint32

	Int32() int32
	Uint32() uint32
	Float32() float32
	String() (string, error)
	Bool() bool
	Int64() (int64, error)
	Uint64() (uint64, error)

	IsNull() bool
}

type readerEntry struct {
	r    io.ReadSeeker
	data uint32

	variant Variant
}

func (e *readerEntry) Variant() Variant {
	return e.variant
}

func (e *readerEntry) RawData() uint32 {
	return e.data
}

func (e *readerEntry) Int32() int32 {
	return int32(e.data)
}

func (e *readerEntry) Uint32() uint32 {
	return uint32(e.data)
}

func (e *readerEntry) Float32() float32 {
	return math.Float32frombits(e.data)
}

func (e *readerEntry) String() (s string, err error) {
	if _, err := e.r.Seek(int64(e.data), io.SeekStart); err != nil {
		return "", err
	}

	s, err = ReadZString(e.r)
	if err != nil {
		return "", err
	}

	return s, nil
}

func (e *readerEntry) Bool() bool {
	return e.data != 0
}

func (e *readerEntry) Int64() (v int64, err error) {
	if _, err := e.r.Seek(int64(e.data), io.SeekStart); err != nil {
		return 0, err
	}

	if err := binary.Read(e.r, order, &v); err != nil {
		return 0, err
	}

	return v, nil
}

func (e *readerEntry) Uint64() (v uint64, err error) {
	if _, err := e.r.Seek(int64(e.data), io.SeekStart); err != nil {
		return 0, err
	}

	if err := binary.Read(e.r, order, &v); err != nil {
		return 0, err
	}

	return v, nil
}

func (e *readerEntry) IsNull() bool {
	return e.variant == NullVariant
}

type Row []Entry

func (r *Row) Column(col int) (Entry, error) {
	if col >= len(*r) {
		return nil, fmt.Errorf("out of range: %d", col)
	}
	return (*r)[col], nil
}

func (r *Row) Id() (int, error) {
	if len(*r) == 0 {
		panic(fmt.Errorf("fdb: row: id: no entries"))
	}

	entry := (*r)[0]
	switch entry.Variant() {
	case NullVariant:
		return 0, ErrNullData
	case I32Variant:
		return int(entry.Int32()), nil
	case U32Variant:
		return int(entry.Uint32()), nil
	case RealVariant:
		return int(math.Float32bits(entry.Float32())), nil
	case BoolVariant:
		v := 0
		if entry.Bool() {
			v = 1
		}
		return v, nil
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
		return 0, fmt.Errorf("cannot read id for %s", entry.Variant())
	}
}
