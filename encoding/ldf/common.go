package ldf

import (
	"fmt"
	"unicode/utf16"
)

type ValueType int

const (
	StringUtf16 = ValueType(iota)
	Signed32
	_
	Float
	Double
	Unsigned32
	_
	Bool
	Unsigned64
	Signed64
	_
	_
	_
	StringUtf8
)

func (t ValueType) String() string {
	switch t {
	case StringUtf16:
		return "StringUtf16"
	case Signed32:
		return "Signed32"
	case Float:
		return "Float"
	case Double:
		return "Double"
	case Unsigned32:
		return "Unsigned32"
	case Bool:
		return "Bool"
	case Unsigned64:
		return "Unsigned64"
	case Signed64:
		return "Signed64"
	case StringUtf8:
		return "StringUtf8"
	default:
		return fmt.Sprintf("ValueType(%d)", t)
	}
}

type Utf16String []uint16

func StringToUtf16(s string) Utf16String {
	return utf16.Encode([]rune(s))
}

func (s Utf16String) String() string {
	return string(utf16.Decode(s))
}
