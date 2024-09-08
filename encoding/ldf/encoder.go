package ldf

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
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

func (encoder *TextEncoder) valueToString(value reflect.Value) (string, error) {
	switch value.Kind() {
	case reflect.String:
		return value.String(), nil
	case reflect.Int, reflect.Int32, reflect.Int64:
		return fmt.Sprint(value.Int()), nil
	case reflect.Float32:
		return fmt.Sprint(float32(value.Float())), nil
	case reflect.Float64:
		return fmt.Sprint(value.Float()), nil
	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		return fmt.Sprint(value.Uint()), nil
	case reflect.Bool:
		if value.Bool() {
			return "1", nil
		} else {
			return "0", nil
		}
	default:
		return "", fmt.Errorf("cannot marshal type: %v", value.Type())
	}
}

func (encoder *TextEncoder) getValueType(kind reflect.Kind) ValueType {
	switch kind {
	case reflect.String:
		return StringUtf16
	case reflect.Int32:
		return Signed32
	case reflect.Float32:
		return Float
	case reflect.Float64:
		return Double
	case reflect.Uint32:
		return Unsigned32
	case reflect.Bool:
		return Bool
	case reflect.Uint64:
		return Unsigned64
	case reflect.Int64:
		return Signed64
	default:
		panic(fmt.Errorf("cannot get value type for kind: %v", kind))
	}
}

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

		rawValue, err := encoder.valueToString(value)
		if err != nil {
			return fmt.Errorf("ldf: encode: %w", err)
		}

		if encoder.wroteLine {
			fmt.Fprint(encoder.w, encoder.delim)
		}

		fmt.Fprint(
			encoder.w,
			tokenName,
			"=",
			int(encoder.getValueType(value.Kind())),
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
