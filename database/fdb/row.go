package fdb

import (
	"fmt"
	"math"
)

type Row []Entry

func (r Row) Column(i int) (Entry, error) {
	if i >= len(r) {
		return nil, fmt.Errorf("out of range: %d", i)
	}
	return r[i], nil
}

func (r Row) Value(i int) (any, error) {
	col, err := r.Column(i)
	if err != nil {
		return nil, err
	}

	switch col.Variant() {
	case VariantNull:
		return nil, nil
	case VariantI32:
		return col.Int32(), nil
	case VariantU32:
		return col.Uint32(), nil
	case VariantReal:
		return col.Float32(), nil
	case VariantNVarChar, VariantText:
		return col.String()
	case VariantBool:
		return col.Bool(), nil
	case VariantI64:
		return col.Int64()
	case VariantU64:
		return col.Uint64()
	default:
		return nil, fmt.Errorf("unknown variant: %v", col.Variant())
	}
}

// Returns an int representing the first [Entry]
// of the row.
//
// If the variant is a
//
//   - [VariantNVarChar] or [VariantText], Id returns the [Sfhash]
//     of the string's bytes.
//   - [VariantBool], Id returns 1 if true and 0 if false
//   - [VariantReal], Id returns the underlying float32's bytes
//     as an int
//
// Otherwise, Id returns the [Entry]'s value casted to an int.
//
// Id returns an error if the variant is unrecognized or if the
// the variant is equal to [VariantNull].
func (r *Row) Id() (int, error) {
	if len(*r) == 0 {
		panic(fmt.Errorf("fdb: row: id: no entries"))
	}

	entry := (*r)[0]
	switch entry.Variant() {
	case VariantNull:
		return 0, fmt.Errorf("row id: %w", ErrNullData)
	case VariantI32:
		return int(entry.Int32()), nil
	case VariantU32:
		return int(entry.Uint32()), nil
	case VariantReal:
		return int(math.Float32bits(entry.Float32())), nil
	case VariantBool:
		v := 0
		if entry.Bool() {
			v = 1
		}
		return v, nil
	case VariantI64:
		v, err := entry.Int64()
		if err != nil {
			return 0, err
		}
		return int(v), nil
	case VariantU64:
		v, err := entry.Uint64()
		if err != nil {
			return 0, err
		}
		return int(v), nil
	case VariantNVarChar, VariantText:
		s, err := entry.String()
		if err != nil {
			return 0, err
		}

		return int(Sfhash([]byte(s))), nil
	default:
		return 0, fmt.Errorf("cannot read id for %s", entry.Variant())
	}
}
