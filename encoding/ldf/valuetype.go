package ldf

import (
	"fmt"
	"unicode/utf16"
)

type ValueType int

const (
	ValueTypeString = ValueType(iota) // An encoding-independent string (could be utf-8 or utf-16 depending on usage).
	ValueTypeI32
	_
	ValueTypeFloat
	ValueTypeDouble
	ValueTypeU32
	_
	ValueTypeBool
	ValueTypeU64
	ValueTypeI64
	_
	_
	_
	ValueTypeUtf8 // A string intended to always be encoded in utf-8.
)

func (t ValueType) String() string {
	switch t {
	case ValueTypeString:
		return "String"
	case ValueTypeI32:
		return "Signed32"
	case ValueTypeFloat:
		return "Float"
	case ValueTypeDouble:
		return "Double"
	case ValueTypeU32:
		return "Unsigned32"
	case ValueTypeBool:
		return "Bool"
	case ValueTypeU64:
		return "Unsigned64"
	case ValueTypeI64:
		return "Signed64"
	case ValueTypeUtf8:
		return "Utf8"
	default:
		return fmt.Sprintf("ValueType(%d)", t)
	}
}

type String16 []uint16

func ToString16(s string) String16 {
	return utf16.Encode([]rune(s))
}

func (s String16) String() string {
	return string(utf16.Decode(s))
}
