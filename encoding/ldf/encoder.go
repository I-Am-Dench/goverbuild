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

func (encoder *TextEncoder) encodeValue(value reflect.Value, raw bool) (string, ValueType, error) {
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

func (encoder *TextEncoder) Encode(v any) error {
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

		rawValue, valueType, err := encoder.encodeValue(value, field.raw)
		if err != nil {
			return fmt.Errorf("ldf: encoder: %w", err)
		}

		if encoder.wroteLine {
			fmt.Fprint(encoder.w, encoder.delim)
		}

		fmt.Fprint(
			encoder.w,
			field.name,
			"=",
			int(valueType),
			":",
			rawValue,
		)

		encoder.wroteLine = true
	}

	return nil
}

func MarshalText(v any) ([]byte, error) {
	buf := bytes.Buffer{}
	if err := NewTextEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func MarshalTextLines(v any) ([]byte, error) {
	buf := bytes.Buffer{}
	if err := NewTextEncoder(&buf, ",\n").Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
