package ldf

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
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

func (encoder *TextEncoder) encodeValue(value reflect.Value, forceUtf16 bool) (string, ValueType, error) {
	if value.Type() == utf16Type {
		return value.Interface().(Utf16String).String(), StringUtf16, nil
	}

	switch k := value.Kind(); k {
	case reflect.String:
		t := StringUtf8
		if forceUtf16 {
			t = StringUtf16
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

// func (encoder *TextEncoder) valueToString(value reflect.Value) (string, error) {
// 	if value.Type() == utf16Type {
// 		return value.Interface().(Utf16String).String(), nil
// 	}

// 	switch value.Kind() {
// 	case reflect.String:
// 		return value.String(), nil
// 	case reflect.Int, reflect.Int32, reflect.Int64:
// 		return fmt.Sprint(value.Int()), nil
// 	case reflect.Float32:
// 		return fmt.Sprint(float32(value.Float())), nil
// 	case reflect.Float64:
// 		return fmt.Sprint(value.Float()), nil
// 	case reflect.Uint, reflect.Uint32, reflect.Uint64:
// 		return fmt.Sprint(value.Uint()), nil
// 	case reflect.Bool:
// 		if value.Bool() {
// 			return "1", nil
// 		} else {
// 			return "0", nil
// 		}
// 	case reflect.Slice:
// 		sliceType := value.Type().Elem()

// 		if sliceType.Kind() == reflect.Uint16 {
// 			return string(utf16.Decode(value.Interface().([]uint16))), nil
// 		}

// 		if sliceType.Kind() == reflect.Uint8 {
// 			return string(value.Interface().([]byte)), nil
// 		}

// 		fallthrough
// 	default:
// 		return "", fmt.Errorf("cannot marshal type: %v", value.Type())
// 	}
// }

// func (encoder *TextEncoder) getValueType(value reflect.Value) ValueType {
// 	switch value.Kind() {
// 	case reflect.String:
// 		return StringUtf16
// 	case reflect.Int32:
// 		return Signed32
// 	case reflect.Float32:
// 		return Float
// 	case reflect.Float64:
// 		return Double
// 	case reflect.Uint32:
// 		return Unsigned32
// 	case reflect.Bool:
// 		return Bool
// 	case reflect.Uint64:
// 		return Unsigned64
// 	case reflect.Int64:
// 		return Signed64
// 	default:
// 		panic(fmt.Errorf("cannot get value type for kind: %v", value.Kind()))
// 	}
// }

func (encoder *TextEncoder) Encode(v any) error {
	if v == nil {
		return nil
	}

	structValue := reflect.Indirect(reflect.ValueOf(v))
	structType := structValue.Type()

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		tagValue := field.Tag.Get("ldf")

		if tagValue == "-" {
			continue
		}

		value := structValue.Field(i)
		if !value.IsValid() {
			continue
		}

		tokenName, directive, _ := strings.Cut(tagValue, ",")
		if len(tokenName) == 0 {
			tokenName = field.Name
		}

		if directive == "omitempty" && value.IsZero() {
			continue
		}

		rawValue, valueType, err := encoder.encodeValue(value, directive == "utf16")
		if err != nil {
			return fmt.Errorf("ldf: encoder: %w", err)
		}

		if encoder.wroteLine {
			fmt.Fprint(encoder.w, encoder.delim)
		}

		fmt.Fprint(
			encoder.w,
			tokenName,
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
