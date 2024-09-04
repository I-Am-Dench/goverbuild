package ldf

import "reflect"

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

func (t ValueType) Kind() reflect.Kind {
	switch t {
	case StringUtf16, StringUtf8:
		return reflect.String
	case Signed32:
		return reflect.Int32
	case Float:
		return reflect.Float32
	case Double:
		return reflect.Float64
	case Unsigned32:
		return reflect.Uint32
	case Bool:
		return reflect.Bool
	case Unsigned64:
		return reflect.Uint64
	case Signed64:
		return reflect.Int64
	default:
		return reflect.Invalid
	}
}
