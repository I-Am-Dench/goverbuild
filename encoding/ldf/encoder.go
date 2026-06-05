package ldf

import (
	"bytes"
	"encoding"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"unicode/utf16"
)

// A key-value pair which can be directly encoded/decoded.
type Entry struct {
	Key   string
	Value any
}

type TextEncoder struct {
	w   io.Writer
	buf []byte

	delim     string
	wroteLine bool
}

func NewTextEncoder(w io.Writer, delim ...string) *TextEncoder {
	d := ","
	if len(delim) > 0 {
		d = delim[0]
	}
	return &TextEncoder{w, []byte{}, d, false}
}

func (e *TextEncoder) Reset(w io.Writer) {
	e.w = w
	e.wroteLine = false
}

func (e *TextEncoder) write(key string, valueType ValueType, value string) error {
	buf := e.buf[:0]

	if e.wroteLine {
		buf = append(buf, e.delim...)
	}
	buf = append(buf, key...)
	buf = append(buf, '=')
	buf = strconv.AppendInt(buf, int64(valueType), 10)
	buf = append(buf, ':')
	buf = append(buf, value...)
	e.buf = buf

	if _, err := e.w.Write(buf); err != nil {
		return err
	}

	e.wroteLine = true
	return nil
}

func (e TextEncoder) encodeAny(v any) (ValueType, string, error) {
	switch val := v.(type) {
	case encoding.TextMarshaler:
		data, err := val.MarshalText()
		if err != nil {
			return 0, "", err
		}
		return ValueTypeString, string(data), nil
	case string:
		return ValueTypeString, val, nil
	case String16:
		return ValueTypeString, val.String(), nil
	case []uint16:
		return ValueTypeString, string(utf16.Decode(val)), nil
	case int32:
		return ValueTypeI32, strconv.FormatInt(int64(val), 10), nil
	case float32:
		return ValueTypeFloat, strconv.FormatFloat(float64(val), 'g', -1, 32), nil
	case float64:
		return ValueTypeDouble, strconv.FormatFloat(val, 'g', -1, 64), nil
	case uint32:
		return ValueTypeU32, strconv.FormatUint(uint64(val), 10), nil
	case bool:
		if val {
			return ValueTypeBool, "1", nil
		} else {
			return ValueTypeBool, "0", nil
		}
	case uint:
		return ValueTypeU64, strconv.FormatUint(uint64(val), 10), nil
	case uint64:
		return ValueTypeU64, strconv.FormatUint(val, 10), nil
	case int:
		return ValueTypeI64, strconv.FormatInt(int64(val), 10), nil
	case int64:
		return ValueTypeI64, strconv.FormatInt(val, 10), nil
	case []uint8:
		return ValueTypeUtf8, string(val), nil
	default:
		return 0, "", fmt.Errorf("cannot encode %T", v)
	}
}

func (e TextEncoder) encodeValue(value reflect.Value) (ValueType, string, error) {
	if marshaler, ok := value.Interface().(encoding.TextMarshaler); ok {
		data, err := marshaler.MarshalText()
		if err != nil {
			return 0, "", err
		}
		return ValueTypeString, string(data), nil
	}

	if s, ok := value.Interface().(String16); ok {
		return ValueTypeString, s.String(), nil
	}

	switch value.Kind() {
	case reflect.String:
		return e.encodeAny(value.String())
	case reflect.Int32:
		return e.encodeAny(int32(value.Int()))
	case reflect.Float32:
		return e.encodeAny(float32(value.Float()))
	case reflect.Float64:
		return e.encodeAny(value.Float())
	case reflect.Uint32:
		return e.encodeAny(uint32(value.Uint()))
	case reflect.Bool:
		return e.encodeAny(value.Bool())
	case reflect.Uint, reflect.Uint64:
		return e.encodeAny(value.Uint())
	case reflect.Int, reflect.Int64:
		return e.encodeAny(value.Int())
	case reflect.Slice:
		elemKind := value.Type().Elem().Kind()
		if elemKind == reflect.Uint16 || elemKind == reflect.Uint8 {
			return e.encodeAny(value.Interface())
		}
		fallthrough
	default:
		return 0, "", fmt.Errorf("cannot encode %v", value.Kind())
	}
}

func (e *TextEncoder) encodeKeyValue(entry Entry) error {
	valueType, value, err := e.encodeAny(entry.Value)
	if err != nil {
		return err
	}
	return e.write(entry.Key, valueType, value)
}

func (e *TextEncoder) encodeMapAny(m Map) error {
	for k, v := range m {
		valueType, value, err := e.encodeAny(v)
		if err != nil {
			return err
		}

		if err := e.write(k, valueType, value); err != nil {
			return err
		}
	}
	return nil
}

func (e *TextEncoder) encode(v any) error {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case Entry:
		return e.encodeKeyValue(val)
	case []Entry:
		for _, kv := range val {
			if err := e.encodeKeyValue(kv); err != nil {
				return err
			}
		}
		return nil
	case Map:
		return e.encodeMapAny(val)
	}

	value := reflect.Indirect(reflect.ValueOf(v))
	switch value.Kind() {
	case reflect.Struct, reflect.Map:
		marshal := getArshaler(value.Type()).textMarshal
		if err := marshal(e, value); err != nil {
			return err
		}
	default:
		return fmt.Errorf("cannot encode %v", value.Type())
	}

	return nil
}

func (e *TextEncoder) Encode(v any) error {
	if err := e.encode(v); err != nil {
		return fmt.Errorf("ldf: encode: %v", err)
	}
	return nil
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
// Strings, [String16], and []uint16s are encoded as [ValueTypeString].
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
//
// Maps embedded in a struct will capture the remaining LDF keys
// not mapped to a struct field.
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
