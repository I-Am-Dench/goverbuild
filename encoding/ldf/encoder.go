package ldf

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"unicode/utf16"
)

var (
	utf16Type = reflect.TypeOf(Utf16String([]uint16{}))
)

type TextEncoder struct {
	w io.Writer

	delim     string
	wroteLine bool
}

func NewTextEncoder(w io.Writer, delim ...string) *TextEncoder {
	d := ","
	if len(delim) > 0 {
		d = delim[0]
	}

	return &TextEncoder{w, d, false}
}

func (e *TextEncoder) encodeValue(value reflect.Value, raw bool) (string, ValueType, error) {
	if value.Type() == utf16Type {
		return value.Interface().(Utf16String).String(), StringUtf16, nil
	}

	switch k := value.Kind(); k {
	case reflect.String:
		t := StringUtf16
		if raw {
			t = StringUtf8
		}
		return value.String(), t, nil
	case reflect.Int, reflect.Int32, reflect.Int64:
		t := Signed64
		if k == reflect.Int32 {
			t = Signed32
		}

		return strconv.FormatInt(value.Int(), 10), t, nil
	case reflect.Float32:
		return fmt.Sprint(float32(value.Float())), Float, nil
	case reflect.Float64:
		return fmt.Sprint(value.Float()), Double, nil
	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		t := Unsigned64
		if k == reflect.Uint32 {
			t = Unsigned32
		}

		return strconv.FormatUint(value.Uint(), 10), t, nil
	case reflect.Bool:
		v := "0"
		if value.Bool() {
			v = "1"
		}
		return v, Bool, nil
	case reflect.Slice:
		sliceType := value.Type().Elem()

		if sliceType.Kind() == reflect.Uint16 {
			return string(utf16.Decode(value.Interface().([]uint16))), StringUtf16, nil
		}

		if sliceType.Kind() == reflect.Uint8 {
			return string(value.Interface().([]byte)), StringUtf8, nil
		}

		fallthrough
	default:
		return "", 0, fmt.Errorf("cannot marshal type: %v", value.Type())
	}
}

func (e *TextEncoder) Encode(v any) error {
	if v == nil {
		return nil
	}

	structValue := reflect.Indirect(reflect.ValueOf(v))
	typeInfo := getTypeInfo(structValue.Type())

	for i, field := range typeInfo.fields {
		if field.ignore {
			continue
		}

		value := structValue.Field(i)
		if field.omitEmpty && value.IsZero() {
			continue
		}

		rawValue, valueType, err := e.encodeValue(value, field.raw)
		if err != nil {
			return fmt.Errorf("ldf: encode: %w", err)
		}

		if e.wroteLine {
			fmt.Fprint(e.w, e.delim)
		}

		fmt.Fprint(
			e.w,
			field.name,
			"=",
			int(valueType),
			":",
			rawValue,
		)

		e.wroteLine = true
	}

	return nil
}

// MarshalText returns the textual LDF encoding of v.
//
// MarshalText only encodes the top level struct. If MarshalText
// encounters a type it cannot encode it will return an error.
//
// The encoding of fields can be customized through the "ldf" key
// in the field's tag as a comma-separated list. The first option
// is always the custom field name. Using the name "-" will ignore
// the field. The other options can be specified in any order.
// These options include:
//
//   - omitempty: indicates that the struct field should not be
//     encoded if the value is equal to its type's zero value.
//   - raw (string only): indicates that the field should be
//     encoded as the [ValueType], [StringUtf8].
//
// Strings are encoded with the value type [StringUtf16] while
// retaining their utf-8 encoding. To encode a string in utf-16,
// use either the [Utf16String] or []uint16 types.
//
// Fields with type []uint8 (or []byte) are encoded with the
// value type [StringUtf8].
//
// Fields with type int or uint are encoded to [Signed64] and [Unsigned64]
// respectively.
//
// Example fields:
//
//	// Encodes: "Field=0:"
//	Field string
//
//	// Encodes: "MyField=0:"
//	Field string `ldf:"MyField"`
//
//	// Encodes: "MyField=0:" if Field is not empty
//	Field string `ldf:"MyField,omitempty"`
//
//	// Encodes: "Field=13:"
//	Field string `ldf:",raw"`
func MarshalText(v any) ([]byte, error) {
	buf := bytes.Buffer{}
	if err := NewTextEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MarshalLines returns the textual LDF encoding of v with
// each field separated by a comma and newline character.
//
// See [MarshalText] for details.
func MarshalLines(v any) ([]byte, error) {
	buf := bytes.Buffer{}
	if err := NewTextEncoder(&buf, ",\n").Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
