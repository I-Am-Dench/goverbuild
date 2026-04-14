package ldf

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"iter"
	"reflect"
	"regexp"
	"strconv"
	"unicode/utf16"
)

var delimPattern = regexp.MustCompile("(,|\r\n|\n)")

type Token struct {
	Key   string
	Type  ValueType
	Value []byte
}

type TokenSeq = iter.Seq2[Token, error]

type TextDecoder struct {
	delim *regexp.Regexp
	s     *bufio.Scanner
	err   error

	token Token
}

func NewTextDecoder(r io.Reader) *TextDecoder {
	d := &TextDecoder{
		delim: delimPattern,
	}
	d.Reset(r)

	return d
}

func (d *TextDecoder) SetDelim(delim *regexp.Regexp) {
	d.delim = delim
}

func (d *TextDecoder) splitDelim(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	match := d.delim.FindIndex(data)
	if match != nil {
		return match[1], data[:match[0]], nil
	}

	if atEOF {
		return len(data), data, nil
	}

	return 0, nil, nil
}

func (d TextDecoder) decodeToken(rawToken []byte) (Token, error) {
	key, valueWithType, ok := bytes.Cut(rawToken, []byte("="))
	if !ok {
		return Token{}, fmt.Errorf("missing key-value pair: %s", rawToken)
	}

	rawValueType, value, ok := bytes.Cut(valueWithType, []byte(":"))
	if !ok {
		return Token{}, fmt.Errorf("missing value type: %s", rawToken)
	}

	valueType, err := strconv.ParseInt(string(bytes.TrimSpace(rawValueType)), 10, 8)
	if err != nil {
		return Token{}, fmt.Errorf("invalid value type: %s: %v", rawToken, err)
	}

	if valueType < 0 || valueType > int64(ValueTypeUtf8) {
		return Token{}, fmt.Errorf("invalid value type: %s: %d", rawToken, valueType)
	}

	return Token{
		Key:   string(bytes.TrimSpace(key)),
		Type:  ValueType(valueType),
		Value: value,
	}, nil
}

func (d *TextDecoder) Reset(r io.Reader) {
	d.s = bufio.NewScanner(r)
	d.s.Split(d.splitDelim)
}

func (d *TextDecoder) Next() bool {
	for d.s.Scan() {
		rawToken := d.s.Bytes()
		if len(bytes.TrimSpace(rawToken)) == 0 {
			continue
		}

		token, err := d.decodeToken(rawToken)
		if err != nil {
			d.err = fmt.Errorf("invalid token: %v", err)
			return false
		}

		d.token = token
		return true
	}

	if d.s.Err() != nil {
		d.err = d.s.Err()
	}

	return false
}

func (d TextDecoder) Token() Token {
	return d.token
}

func (d TextDecoder) Err() error {
	return d.err
}

func (d *TextDecoder) All() TokenSeq {
	return func(yield func(Token, error) bool) {
		for d.Next() {
			if !yield(d.Token(), nil) {
				return
			}
		}

		if d.Err() != nil {
			yield(Token{}, d.Err())
		}
	}
}

func (d *TextDecoder) decodeAny(valueType ValueType, data []byte) (any, error) {
	switch valueType {
	case ValueTypeString:
		return string(data), nil
	case ValueTypeI32:
		v, err := strconv.ParseInt(string(data), 10, 32)
		if err != nil {
			return nil, err
		}
		return int32(v), nil
	case ValueTypeFloat:
		v, err := strconv.ParseFloat(string(data), 32)
		if err != nil {
			return nil, err
		}
		return float32(v), nil
	case ValueTypeDouble:
		v, err := strconv.ParseFloat(string(data), 64)
		if err != nil {
			return nil, err
		}
		return v, nil
	case ValueTypeU32:
		v, err := strconv.ParseUint(string(data), 10, 32)
		if err != nil {
			return nil, err
		}
		return uint32(v), nil
	case ValueTypeBool:
		return bytes.Equal(data, []byte("1")), nil
	case ValueTypeU64:
		v, err := strconv.ParseUint(string(data), 10, 64)
		if err != nil {
			return nil, err
		}
		return v, nil
	case ValueTypeI64:
		v, err := strconv.ParseInt(string(data), 10, 64)
		if err != nil {
			return nil, err
		}
		return v, nil
	case ValueTypeUtf8:
		return data, nil
	default:
		return nil, fmt.Errorf("cannot decode %v", valueType)
	}
}

func (d *TextDecoder) decodeMapAny(m Map, seq TokenSeq) error {
	for token := range seq {
		v, err := d.decodeAny(token.Type, token.Value)
		if err != nil {
			return err
		}
		m[token.Key] = v
	}
	return d.Err()
}

func (d *TextDecoder) decodeValue(token Token, rtype reflect.Type) (reflect.Value, error) {
	switch token.Type {
	case ValueTypeString:
		if rtype.Kind() == reflect.Slice {
			if rtype.Elem().Kind() == reflect.Uint16 {
				s := string(token.Value)
				return reflect.ValueOf(utf16.Encode([]rune(s))), nil
			}
			return reflect.Value{}, fmt.Errorf("cannot decode %v", rtype)
		}
		return reflect.ValueOf(string(token.Value)), nil
	case ValueTypeI32:
		v, err := strconv.ParseInt(string(token.Value), 10, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int32(v)), nil
	case ValueTypeFloat:
		v, err := strconv.ParseFloat(string(token.Value), 32)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(float32(v)), nil
	case ValueTypeDouble:
		v, err := strconv.ParseFloat(string(token.Value), 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(v), nil
	case ValueTypeU32:
		v, err := strconv.ParseUint(string(token.Value), 10, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint32(v)), nil
	case ValueTypeBool:
		return reflect.ValueOf(bytes.Equal(token.Value, []byte("1"))), nil
	case ValueTypeU64:
		v, err := strconv.ParseUint(string(token.Value), 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(v), nil
	case ValueTypeI64:
		v, err := strconv.ParseInt(string(token.Value), 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(v), nil
	case ValueTypeUtf8:
		return reflect.ValueOf(token.Value), nil
	default:
		return reflect.Value{}, fmt.Errorf("unhandled value type: %v", token.Type)
	}
}

func first(seq TokenSeq) (token Token, err error, ok bool) {
	seq(func(t Token, e error) bool {
		token = t
		err = e
		ok = true
		return false
	})
	return token, err, ok
}

func (d *TextDecoder) decode(v any, seq TokenSeq) error {
	if v == nil {
		return errors.New("cannot decode nil")
	}

	switch val := v.(type) {
	case *KeyValue:
		if token, err, ok := first(seq); ok {
			if err != nil {
				return err
			}

			v, err := d.decodeAny(token.Type, token.Value)
			if err != nil {
				return err
			}

			*val = KeyValue{
				Key:   token.Key,
				Value: v,
			}
		}
		return nil
	case *[]KeyValue:
		values := []KeyValue{}
		for token, err := range seq {
			if err != nil {
				return err
			}

			v, err := d.decodeAny(token.Type, token.Value)
			if err != nil {
				return err
			}

			values = append(values, KeyValue{
				Key:   token.Key,
				Value: v,
			})
		}
		*val = values

		return nil
	case Map:
		return d.decodeMapAny(val, seq)
	}

	value := reflect.ValueOf(v)
	if value.Kind() != reflect.Pointer {
		return fmt.Errorf("cannot decode %v", value.Type())
	}

	value = reflect.Indirect(value)
	switch value.Kind() {
	case reflect.Struct, reflect.Map:
		unmarshal := getArshaler(value.Type()).textUnmarshal
		if err := unmarshal(d, value, d.All()); err != nil {
			return err
		}
	default:
		return fmt.Errorf("cannot decode %v", value.Type())
	}

	return nil
}

func (d *TextDecoder) Decode(v any) error {
	if err := d.decode(v, d.All()); err != nil {
		return fmt.Errorf("ldf: decode: %v", err)
	}
	return nil
}

// UnmarshalText parses the textual LDF encoded data into v.
// UnmarshalText only decodes pointers to structs or maps with
// a string key type.
//
// Fields, by default, are delimited by a comma, a newline
// character, or a CRLF. The delimiter can be changed on a custom
// decoder by calling [TextDecoder.SetDelim].
//
// UnmarshalText returns an error if the encoded value type does
// not match the struct field's type.
//
// If the field type is a slice, a new slice is created, except
// when the slice implements [encoding.TextUnmarshaler].
//
// Fields that implement the [encoding.TextUnmarshaler] interface are
// only unmarshaled if the value type is either [ValueTypeString] or
// [ValueTypeUtf8]. If the field's type is not a pointer, the pointer
// type to that field is checked for compatibility with [encoding.TextUnmarshaler].
//
// When decoding a map:
//   - [ValueTypeUtf8] is decoded as a []uint8. If the map's value
//     type is a string, it is instead decoded as a string.
//   - [ValueTypeString] is decoded as a string. If the map's value
//     type is a [String16] or []uint16, it is decoded as a []uint16.
//   - [encoding.TextUnmarshaler] is not supported.
func UnmarshalText(data []byte, v any) error {
	buf := bytes.NewBuffer(data)
	return NewTextDecoder(buf).Decode(v)
}
