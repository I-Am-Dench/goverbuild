package fdb

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/I-Am-Dench/goverbuild/database/fdb/internal/deferredwriter"
)

var (
	order = binary.LittleEndian
)

type Variant uint32

const (
	VariantNull = Variant(iota)
	VariantI32
	VariantU32
	VariantReal
	VariantNVarChar
	VariantBool
	VariantI64
	VariantU64
	VariantText
)

func (v Variant) String() string {
	switch v {
	case VariantNull:
		return "null"
	case VariantI32:
		return "i32"
	case VariantU32:
		return "u32"
	case VariantReal:
		return "f4"
	case VariantNVarChar:
		return "nvarchar"
	case VariantBool:
		return "bool"
	case VariantI64:
		return "i64"
	case VariantU64:
		return "u64"
	case VariantText:
		return "text"
	default:
		return fmt.Sprintf("Variant(%d)", v)
	}
}

func readNullTerminatedBytes(r io.Reader) ([]byte, error) {
	c := make([]byte, 0, 32)
	for {
		b := [1]byte{}
		_, err := r.Read(b[:])
		if err == io.EOF {
			return c, nil
		}

		if err != nil {
			return nil, fmt.Errorf("read null terminated bytes: %v", err)
		}

		if b[0] == 0 {
			return c, nil
		} else {
			c = append(c, b[0])
		}
	}
}

func ReadZString(r io.Reader) (string, error) {
	b, err := readNullTerminatedBytes(r)
	if err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}

func WriteZString(w io.Writer, s string) (int, error) {
	return deferredwriter.WriteZString(w, s)
}

func bitCeil(n int) int {
	if n == 0 {
		return 0
	}

	c := 1
	for c < n {
		c <<= 1
	}

	return c
}

// THE CODE BELOW HAS BEEN TRANSLATE FROM THE FOLLOWING RESOURCES:
//   - Original implementation: https://www.azillionmonkeys.com/qed/hash.html
//   - Xiphoseer's implementation: https://docs.rs/sfhash/latest/src/sfhash/lib.rs.html
func Sfhash(b []byte) uint32 {
	if len(b) == 0 || b == nil {
		return 0
	}

	// Decouple fdb byte order from hash byte order
	order := binary.LittleEndian

	hash := uint32(len(b))
	rem := len(b) & 3

	for l := len(b) >> 2; l > 0; l-- {
		hash += uint32(order.Uint16(b))
		temp := (uint32(order.Uint16(b[2:])) << 11) ^ hash
		hash = (hash << 16) ^ temp
		hash += hash >> 11
		b = b[4:]
	}

	switch rem {
	case 3:
		hash += uint32(order.Uint16(b))
		hash ^= hash << 16
		hash ^= uint32(b[2]) << 18
		hash += hash >> 11
	case 2:
		hash += uint32(order.Uint16(b))
		hash ^= hash << 11
		hash += hash >> 17
	case 1:
		hash += uint32(b[0])
		hash ^= hash << 10
		hash += hash >> 1
	}

	hash ^= hash << 3
	hash += hash >> 5
	hash ^= hash << 4
	hash += hash >> 17
	hash ^= hash << 25
	hash += hash >> 6

	return hash
}
