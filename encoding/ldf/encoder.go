package ldf

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"unicode/utf16"
)

var (
	string16Type      = reflect.TypeOf(String16([]uint16{}))
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
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

func isEmpty(v reflect.Value) bool {
	if v.IsZero() {
		return true
	}

	if v.Kind() == reflect.Array || v.Kind() == reflect.Slice {
		return v.Len() == 0
	}

	return false
}

func (e *TextEncoder) encode(buf *bytes.Buffer, key string, valueType ValueType, value string) error {
	if e.wroteLine {
		buf.WriteString(e.delim)
	}

	buf.WriteString(key)
	buf.WriteRune('=')
	buf.WriteString(strconv.Itoa(int(valueType)))
	buf.WriteRune(':')
	buf.WriteString(value)

	if _, err := e.w.Write(buf.Bytes()); err != nil {
		return err
	}

	buf.Reset()
	e.wroteLine = true

	return nil
}

func (e *TextEncoder) getEncoding(value reflect.Value, raw bool) (string, ValueType, error) {
	if value.Type() == string16Type {
		return value.Interface().(String16).String(), ValueTypeString, nil
	}

	if value.Type().Implements(textMarshalerType) {
		data, err := value.Interface().(encoding.TextMarshaler).MarshalText()
		if err != nil {
			return "", 0, err
		}

		t := ValueTypeString
		if raw {
			t = ValueTypeUtf8
		}

		return string(data), t, nil
	}

	switch k := value.Kind(); k {
	case reflect.String:
		t := ValueTypeString
		if raw {
			t = ValueTypeUtf8
		}
		return value.String(), t, nil
	case reflect.Int, reflect.Int32, reflect.Int64:
		t := ValueTypeI64
		if k == reflect.Int32 {
			t = ValueTypeI32
		}
		return strconv.FormatInt(value.Int(), 10), t, nil
	case reflect.Float32:
		return strconv.FormatFloat(value.Float(), 'g', -1, 32), ValueTypeFloat, nil
	case reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'g', -1, 64), ValueTypeDouble, nil
	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		t := ValueTypeU64
		if k == reflect.Uint32 {
			t = ValueTypeU32
		}
		return strconv.FormatUint(value.Uint(), 10), t, nil
	case reflect.Bool:
		v := "0"
		if value.Bool() {
			v = "1"
		}
		return v, ValueTypeBool, nil
	case reflect.Interface:
		if value.IsNil() {
			return "", 0, errors.New("cannot encode nil")
		}
		return e.getEncoding(value.Elem(), raw)
	case reflect.Slice:
		sliceType := value.Type().Elem()

		if sliceType.Kind() == reflect.Uint16 {
			s := utf16.Decode(value.Interface().([]uint16))
			return string(s), ValueTypeString, nil
		}

		if sliceType.Kind() == reflect.Uint8 {
			s := value.Interface().([]uint8)
			return string(s), ValueTypeUtf8, nil
		}

		fallthrough
	default:
		return "", 0, fmt.Errorf("cannot encode value: %v", value.Type())
	}
}

func (e *TextEncoder) encodeStruct(value reflect.Value) error {
	typeInfo := getTypeInfo(value.Type())

	buf := bytes.Buffer{}
	for i, fieldInfo := range typeInfo.fields {
		if fieldInfo.ignore {
			continue
		}

		field := value.Field(i)
		if fieldInfo.omitEmpty && isEmpty(field) {
			continue
		}

		if fieldInfo.embedded {
			if err := e.encodeStruct(field); err != nil {
				return err
			}
			continue
		}

		encodedValue, valueType, err := e.getEncoding(field, fieldInfo.raw)
		if err != nil {
			return fmt.Errorf("ldf: encode: %s: %v", fieldInfo.name, err)
		}

		if err := e.encode(&buf, fieldInfo.name, valueType, encodedValue); err != nil {
			return fmt.Errorf("ldf: encode: %s: %v", fieldInfo.name, err)
		}
	}

	return nil
}

func (e *TextEncoder) encodeMap(value reflect.Value) error {
	iter := value.MapRange()

	buf := bytes.Buffer{}
	for iter.Next() {
		key := iter.Key()
		if key.Kind() != reflect.String {
			return fmt.Errorf("ldf: encode: cannot encode %s as key", key.Type())
		}

		value := iter.Value()
		encodedValue, valueType, err := e.getEncoding(value, false)
		if err != nil {
			return fmt.Errorf("ldf: encode: %s: %v", key.String(), err)
		}

		if err := e.encode(&buf, key.String(), valueType, encodedValue); err != nil {
			return fmt.Errorf("ldf: encode: %s: %v", key.String(), err)
		}
	}

	return nil
}

func (e *TextEncoder) Encode(v any) error {
	if v == nil {
		return nil
	}

	value := reflect.Indirect(reflect.ValueOf(v))
	switch value.Kind() {
	case reflect.Struct:
		return e.encodeStruct(value)
	case reflect.Map:
		return e.encodeMap(value)
	default:
		return fmt.Errorf("ldf: encode: cannot encode %v", value.Kind())
	}
}

// MarshalText returns the textual LDF encoding of v.
//
// MarshalText only encodes top-level structs and maps.
// If MarshalText encounters a type it cannot encode it
// will return an error.
//
// The encoding of fields can be customized through the "ldf" key
// in the field's tag as a comma-separated list. The first option
// is always the custom field name. Using the name "-" will ignore
// the field. The other options can be specified in any order.
// These options include:
//
//   - omitempty: indicates that the struct field should not be
//     encoded if the value is equal to its type's zero value or
//     if the value's length is 0.
//   - raw (string only): indicates that the field should be
//     encoded as the [ValueType], [ValueTypeUtf8].
//
// Strings are encoded with the value type [ValueTypeString].
// To encode a string in utf-16, use either the [String16] or
// []uint16 types.
//
// Fields with type []uint8 (or []byte) are encoded with the
// value type [ValueTypeUtf8].
//
// Fields with type int or uint are encoded to [ValueTypeI64] and
// [ValueTypeU64] respectively.
//
// Fields with an interface type will encode the value contained
// within the interface.
//
// Fields that implement the [encoding.TextMarshaler] interface are
// treated as string types and obey the "raw" option.
//
// Embedded structs are encoded as if their fields were at the same
// level as their parent struct. Field tags on embedded structs are
// ignored.
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
//
// Map key types must be a string. Map values follow the same encoding
// rules as struct fields.
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
