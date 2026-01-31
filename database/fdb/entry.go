package fdb

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"
)

type Entry interface {
	Variant() Variant

	Int32() int32
	Uint32() uint32
	Float32() float32
	String() (string, error)
	Bool() bool
	Int64() (int64, error)
	Uint64() (uint64, error)
}

type readerEntry struct {
	r    io.ReadSeeker
	data uint32

	variant Variant
}

func (e readerEntry) Variant() Variant {
	return e.variant
}

func (e readerEntry) Int32() int32 {
	return int32(e.data)
}

func (e readerEntry) Uint32() uint32 {
	return uint32(e.data)
}

func (e readerEntry) Float32() float32 {
	return math.Float32frombits(e.data)
}

func (e readerEntry) String() (s string, err error) {
	if _, err := e.r.Seek(int64(e.data), io.SeekStart); err != nil {
		return "", err
	}

	s, err = ReadZString(e.r)
	if err != nil {
		return "", err
	}

	return s, nil
}

func (e readerEntry) Bool() bool {
	return e.data != 0
}

func (e readerEntry) Int64() (v int64, err error) {
	if _, err := e.r.Seek(int64(e.data), io.SeekStart); err != nil {
		return 0, err
	}

	if err := binary.Read(e.r, order, &v); err != nil {
		return 0, err
	}

	return v, nil
}

func (e readerEntry) Uint64() (v uint64, err error) {
	if _, err := e.r.Seek(int64(e.data), io.SeekStart); err != nil {
		return 0, err
	}

	if err := binary.Read(e.r, order, &v); err != nil {
		return 0, err
	}

	return v, nil
}

var _ sql.Scanner = (*DataEntry)(nil)

type DataEntry struct {
	variant Variant
	data    any
}

func (e DataEntry) Variant() Variant {
	return e.variant
}

func (e DataEntry) Int32() int32 {
	return e.data.(int32)
}

func (e DataEntry) Uint32() uint32 {
	return e.data.(uint32)
}

func (e DataEntry) Float32() float32 {
	return e.data.(float32)
}

func (e DataEntry) String() (string, error) {
	return e.data.(string), nil
}

func (e DataEntry) Bool() bool {
	return e.data.(bool)
}

func (e DataEntry) Int64() (int64, error) {
	return e.data.(int64), nil
}

func (e DataEntry) Uint64() (uint64, error) {
	return e.data.(uint64), nil
}

func (e DataEntry) IsNull() bool {
	return e.data == nil
}

func (e *DataEntry) scanString(s string) error {
	if e.variant == VariantNull {
		e.variant = VariantNVarChar
	}

	if e.variant != VariantNVarChar && e.variant != VariantText {
		return fmt.Errorf("cannot scan string into %v", e.variant)
	}

	e.data = s
	return nil
}

func (e *DataEntry) scanFloat64(f float64) error {
	if e.variant == VariantNull {
		e.variant = VariantReal
	}

	if e.variant != VariantReal {
		return fmt.Errorf("cannot scan float64 into %v", e.variant)
	}

	e.data = float32(f)
	return nil
}

func (e *DataEntry) scanBool(b bool) error {
	if e.variant == VariantNull {
		e.variant = VariantBool
	}

	if e.variant != VariantBool {
		return fmt.Errorf("cannot scan bool into %v", e.variant)
	}

	e.data = b
	return nil
}

func (e *DataEntry) scanInt64(i int64) error {
	if e.variant == VariantNull {
		e.variant = VariantI64
	}

	switch e.variant {
	case VariantI32:
		e.data = int32(i)
	case VariantI64:
		e.data = i
	case VariantBool:
		e.data = i != 0
	default:
		return fmt.Errorf("cannot scan int64 into %v", e.variant)
	}

	return nil
}

func (e *DataEntry) scanUint64(i uint64) error {
	if e.variant == VariantNull {
		e.variant = VariantI64
	}

	switch e.variant {
	case VariantU32:
		e.data = uint32(i)
	case VariantU64:
		e.data = i
	case VariantBool:
		e.data = i != 0
	default:
		return fmt.Errorf("cannot scan uint64 into %v", e.variant)
	}

	return nil
}

func (e *DataEntry) scanTime(t time.Time) error {
	if e.variant == VariantNull {
		e.variant = VariantI64
	}

	if e.variant != VariantI64 {
		return fmt.Errorf("cannot scan time.Time into %v", e.variant)
	}

	e.data = t.Unix()
	return nil
}

// Method satisfying the [sql.Scanner] interface.
//
// When [*DataEntry.Variant] is not equal to [VariantNull],
// the variant will be treated as a hint for what type the value
// should be casted to.
//
// When [*DataEntry.Variant] is equal to [VariantNull], the variant
// will be set to the [Variant] corresponding to value's type.
//
// The variant is always set to [VariantNull] if value is nil.
//
// If the variant is equal to [VariantBool] and value is either an
// int64 or uint64, [*DataEntry]'s value will be set to false if
// value is 0 and true otherwise.
//
// If the provided value is of type [time.Time] and the
// variant is equal to [VariantI64], [*DataEntry]'s value will
// be set to the number of seconds since epoch for that time.
func (e *DataEntry) Scan(value any) error {
	if value == nil {
		e.data = nil
		e.variant = VariantNull
		return nil
	}

	switch v := value.(type) {
	case string:
		return e.scanString(v)
	case float64:
		return e.scanFloat64(v)
	case bool:
		return e.scanBool(v)
	case int64:
		return e.scanInt64(v)
	case uint64:
		return e.scanUint64(v)
	case time.Time:
		return e.scanTime(v)
	default:
		return fmt.Errorf("cannot scan %T", v)
	}
}

// Creates a new [*DataEntry] with the provided
// [Variant] and optional data.
//
// Ensuring that the [Variant] corresponds to the
// provided data is the job of the caller.
func NewEntry(variant Variant, data ...any) *DataEntry {
	entry := &DataEntry{
		variant: variant,
	}
	if len(data) > 0 {
		entry.data = data[0]
	}

	return entry
}
