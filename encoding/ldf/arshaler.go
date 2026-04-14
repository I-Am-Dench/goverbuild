package ldf

import (
	"encoding"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
)

// type (
// 	textMarshalerFunc   func(enc *TextEncoder, value reflect.Value) error
// 	textUnmarshalerFunc func(dec *TextDecoder, value reflect.Value) error
// )

var (
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
	string16Type        = reflect.TypeFor[String16]()
)

type (
	textMarshalerFunc   func(d *TextEncoder, value reflect.Value) error
	textUnmarshalerFunc func(d *TextDecoder, value reflect.Value, seq TokenSeq) error

	tokenMap = map[string]Token
)

type arshaler struct {
	textMarshal   textMarshalerFunc
	textUnmarshal textUnmarshalerFunc
}

var arshalerMap sync.Map // map[reflect.Type]*arshaler

func getArshaler(t reflect.Type) *arshaler {
	if v, ok := arshalerMap.Load(t); ok {
		return v.(*arshaler)
	}

	arsh := makeArshaler(t)

	v, _ := arshalerMap.LoadOrStore(t, arsh)
	return v.(*arshaler)
}

func makeArshaler(t reflect.Type) *arshaler {
	switch t.Kind() {
	case reflect.Struct:
		return makeStructArshaler(t)
	case reflect.Map:
		return makeMapArshaler(t)
	default:
		return &arshaler{
			textMarshal: func(enc *TextEncoder, value reflect.Value) error {
				return fmt.Errorf("cannot encode %v", t)
			},
			textUnmarshal: func(*TextDecoder, reflect.Value, TokenSeq) error {
				return fmt.Errorf("cannot decode %v", t)
			},
		}
	}
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

func getUnmarshaler(v reflect.Value) (reflect.Value, bool) {
	if v.Type().Implements(textUnmarshalerType) {
		return v, true
	}

	if addr := v.Addr(); addr.Type().Implements(textUnmarshalerType) {
		return addr, true
	}
	return reflect.Value{}, false
}

func toTokenMap(seq TokenSeq) (tokenMap, error) {
	m := make(map[string]Token)
	for token, err := range seq {
		if err != nil {
			return nil, err
		}
		m[token.Key] = token
	}
	return m, nil
}

func toTokenSeq(m tokenMap, exclude []string) TokenSeq {
	return func(yield func(Token, error) bool) {
		for _, token := range m {
			if !slices.Contains(exclude, token.Key) && !yield(token, nil) {
				return
			}
		}
	}
}

type fieldInfo struct {
	name      string
	ignore    bool
	embedded  bool
	goType    reflect.Type
	omitEmpty bool
	raw       bool
}

func setStructField(field reflect.Value, fieldInfo fieldInfo, decodedValue any) (err error) {
	defer func() {
		if r := recover(); err == nil && r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	switch v := decodedValue.(type) {
	case string:
		if fieldInfo.goType == string16Type {
			field.Set(reflect.ValueOf(ToString16(v)))
		} else {
			field.SetString(v)
		}
	case int32:
		field.SetInt(int64(v))
	case int64:
		field.SetInt(v)
	case int:
		field.SetInt(int64(v))
	case float32:
		field.SetFloat(float64(v))
	case float64:
		field.SetFloat(v)
	case uint32:
		field.SetUint(uint64(v))
	case uint64:
		field.SetUint(v)
	case uint:
		field.SetUint(uint64(v))
	case bool:
		field.SetBool(v)
	case []uint8:
		if fieldInfo.goType.Kind() == reflect.String {
			field.SetString(string(v))
		} else {
			field.SetBytes(v)
		}
	default:
		return fmt.Errorf("unhandled decoded value: %T", v)
	}
	return nil
}

func makeStructArshaler(t reflect.Type) *arshaler {
	fields := []fieldInfo{}
	fieldNames := []string{}

	n := t.NumField()
	for i := range n {
		f := t.Field(i)
		tag := f.Tag.Get("ldf")

		name, options, _ := strings.Cut(tag, ",")
		if len(name) == 0 {
			name = f.Name
		}

		fieldInfo := fieldInfo{
			name:     name,
			embedded: f.Anonymous,
			ignore:   name == "-" || !(f.IsExported() || f.Anonymous),
			goType:   f.Type,
		}
		if !fieldInfo.ignore {
			for len(options) > 0 {
				var option string
				option, options, _ = strings.Cut(options, ",")

				switch option {
				case "omitempty":
					fieldInfo.omitEmpty = true
				case "raw":
					// "raw" option can't be applied to String16's
					fieldInfo.raw = t.Kind() != reflect.Slice || t.Elem().Kind() != reflect.Uint16
				}
			}
		}

		if !fieldInfo.ignore {
			fieldNames = append(fieldNames, fieldInfo.name)
		}
		fields = append(fields, fieldInfo)
	}

	return &arshaler{
		textMarshal: func(enc *TextEncoder, structValue reflect.Value) error {
			for i, fieldInfo := range fields {
				if fieldInfo.ignore {
					continue
				}

				field := structValue.Field(i)
				if fieldInfo.omitEmpty && isEmpty(field) {
					continue
				}

				if fieldInfo.embedded {
					if field.Kind() == reflect.Struct || field.Kind() == reflect.Map {
						marshal := getArshaler(field.Type()).textMarshal
						if err := marshal(enc, field); err != nil {
							return fmt.Errorf("%v: %v", field.Type(), err)
						}
					}
					continue
				}

				valueType, encodedValue, err := enc.encodeValue(field)
				if err != nil {
					return fmt.Errorf("%s: %v", fieldInfo.name, err)
				}

				if fieldInfo.raw {
					valueType = ValueTypeUtf8
				}

				if err := enc.write(fieldInfo.name, valueType, encodedValue); err != nil {
					return fmt.Errorf("%s: %v", fieldInfo.name, err)
				}
			}

			return nil
		},
		textUnmarshal: func(dec *TextDecoder, value reflect.Value, seq TokenSeq) (err error) {
			defer func() {
				if r := recover(); err == nil && r != nil {
					err = fmt.Errorf("%v", r)
				}
			}()

			tokens, err := toTokenMap(seq)
			if err != nil {
				return err
			}

			for i, fieldInfo := range fields {
				if fieldInfo.ignore {
					continue
				}

				field := value.Field(i)

				if fieldInfo.embedded {
					unmarshal := getArshaler(fieldInfo.goType).textUnmarshal
					if err := unmarshal(dec, field, toTokenSeq(tokens, fieldNames)); err != nil {
						return fmt.Errorf("%s: %v", fieldInfo.goType, err)
					}
					continue
				}

				token, ok := tokens[fieldInfo.name]
				if !ok {
					continue
				}

				if fieldInfo.goType.Kind() == reflect.Pointer {
					elem := fieldInfo.goType.Elem()
					if elem.Kind() != reflect.Slice {
						field.Set(reflect.New(elem))
					}
				}

				if v, ok := getUnmarshaler(field); ok && (token.Type == ValueTypeString || token.Type == ValueTypeUtf8) {
					if err := v.Interface().(encoding.TextUnmarshaler).UnmarshalText(token.Value); err != nil {
						return fmt.Errorf("%s: %v", fieldInfo.name, err)
					}
					return nil
				}

				decodedValue, err := dec.decodeAny(token.Type, token.Value)
				if err != nil {
					return err
				}

				if err := setStructField(field, fieldInfo, decodedValue); err != nil {
					return fmt.Errorf("%s: %v", fieldInfo.name, err)
				}
			}
			return nil
		},
	}
}

func makeMapArshaler(t reflect.Type) *arshaler {
	if t.Key().Kind() != reflect.String {
		return &arshaler{
			textMarshal: func(enc *TextEncoder, value reflect.Value) error {
				return fmt.Errorf("cannot encode %v: maps must have a string key", t)
			},
			textUnmarshal: func(*TextDecoder, reflect.Value, TokenSeq) error {
				return fmt.Errorf("cannot decode %v: maps must have a string key", t)
			},
		}
	}

	elemType := t.Elem()

	return &arshaler{
		textMarshal: func(enc *TextEncoder, value reflect.Value) error {
			iter := value.MapRange()

			for iter.Next() {
				key := iter.Key().String()
				value := iter.Value()

				valueType, encodedValue, err := enc.encodeAny(value)
				if err != nil {
					return fmt.Errorf("%s: %v", key, err)
				}

				if err := enc.write(iter.Key().String(), valueType, encodedValue); err != nil {
					return fmt.Errorf("%s: %v", key, err)
				}
			}

			return nil
		},
		textUnmarshal: func(dec *TextDecoder, value reflect.Value, seq TokenSeq) error {
			if value.IsNil() {
				value.Set(reflect.MakeMap(t))
			}

			for token, err := range seq {
				if err != nil {
					return err
				}

				decodedValue, err := dec.decodeValue(token, elemType)
				if err != nil {
					return fmt.Errorf("%s: %v", token.Key, err)
				}

				value.SetMapIndex(reflect.ValueOf(token.Key), decodedValue)
			}
			return nil
		},
	}
}
