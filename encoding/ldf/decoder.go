package ldf

import (
	"bufio"
	"bytes"
	"encoding"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

type Token struct {
	Name  string
	Type  ValueType
	Value []byte
}

var delimPattern = regexp.MustCompile("(,|\r\n|\n)")

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

type TextDecoder struct {
	delim *regexp.Regexp
	s     *bufio.Scanner
	err   error

	token Token
}

func NewTextDecoder(r io.Reader) *TextDecoder {
	d := &TextDecoder{
		delim: delimPattern,
		s:     bufio.NewScanner(r),
	}
	d.s.Split(d.splitDelim)

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

func (d *TextDecoder) decodeToken(rawToken []byte) (Token, error) {
	key, valueWithType, ok := bytes.Cut(rawToken, []byte("="))
	if !ok {
		return Token{}, fmt.Errorf("missing key-value pair: %s", rawToken)
	}

	rawValueType, value, ok := bytes.Cut(valueWithType, []byte(":"))
	if !ok {
		return Token{}, fmt.Errorf("missing value type: %s", rawToken)
	}

	valueType, err := strconv.Atoi(string(bytes.TrimSpace(rawValueType)))
	if err != nil {
		return Token{}, fmt.Errorf("value type is not a number: %s: %s", rawValueType, rawToken)
	}

	if valueType < 0 || valueType > int(ValueTypeUtf8) {
		return Token{}, fmt.Errorf("invalid value type: %d: %s", valueType, rawToken)
	}

	return Token{
		Name:  string(key),
		Type:  ValueType(valueType),
		Value: value,
	}, nil
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

func (d *TextDecoder) Token() Token {
	return d.token
}

func (d *TextDecoder) Err() error {
	return d.err
}

func (d *TextDecoder) tokens() (map[string]Token, error) {
	tokens := make(map[string]Token)

	for d.Next() {
		token := d.Token()
		tokens[strings.TrimSpace(token.Name)] = token
	}

	if err := d.Err(); err != nil {
		return nil, err
	}

	return tokens, nil
}

func (d *TextDecoder) getUnmarshaler(value reflect.Value) (encoding.TextUnmarshaler, bool) {
	if value.Kind() != reflect.Pointer {
		value = value.Addr()
	}

	if value.Type().Implements(textUnmarshalerType) {
		return value.Interface().(encoding.TextUnmarshaler), true
	}

	return nil, false
}

func (d *TextDecoder) getValue(token Token, valueType reflect.Type) (reflect.Value, error) {
	switch token.Type {
	case ValueTypeString:
		if valueType == string16Type || (valueType.Kind() == reflect.Slice && valueType.Elem().Kind() == reflect.Uint16) {
			runes := []rune(string(token.Value))
			return reflect.ValueOf(utf16.Encode(runes)), nil
		}

		return reflect.ValueOf(string(token.Value)), nil
	case ValueTypeI32:
		i, err := strconv.ParseInt(string(token.Value), 10, 32)
		if err != nil {
			return reflect.Value{}, err
		}

		return reflect.ValueOf(int32(i)), nil
	case ValueTypeFloat:
		f, err := strconv.ParseFloat(string(token.Value), 32)
		if err != nil {
			return reflect.Value{}, err
		}

		return reflect.ValueOf(float32(f)), nil
	case ValueTypeDouble:
		f, err := strconv.ParseFloat(string(token.Value), 64)
		if err != nil {
			return reflect.Value{}, err
		}

		return reflect.ValueOf(f), nil
	case ValueTypeU32:
		i, err := strconv.ParseUint(string(token.Value), 10, 32)
		if err != nil {
			return reflect.Value{}, err
		}

		return reflect.ValueOf(uint32(i)), nil
	case ValueTypeBool:
		return reflect.ValueOf(string(token.Value) == "1"), nil
	case ValueTypeU64:
		i, err := strconv.ParseUint(string(token.Value), 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}

		return reflect.ValueOf(i), nil
	case ValueTypeI64:
		i, err := strconv.ParseInt(string(token.Value), 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}

		return reflect.ValueOf(i), nil
	case ValueTypeUtf8:
		data := append([]byte(nil), token.Value...)
		if valueType.Kind() == reflect.String {
			return reflect.ValueOf(string(data)), nil
		}

		return reflect.ValueOf(data), nil
	default:
		return reflect.Value{}, fmt.Errorf("cannot decode value type: %v", token.Value)
	}
}

func (d *TextDecoder) setStructField(value reflect.Value, token Token) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	unmarshaler, ok := d.getUnmarshaler(value)
	if ok && (token.Type == ValueTypeString || token.Type == ValueTypeUtf8) {
		return unmarshaler.UnmarshalText(token.Value)
	}

	valueType := value.Type()

	v, err := d.getValue(token, valueType)
	if err != nil {
		return err
	}

	value.Set(v.Convert(valueType))
	return nil
}

func (d *TextDecoder) decodeStruct(structValue reflect.Value, tokens map[string]Token) error {
	typeInfo := getTypeInfo(structValue.Type())
	for i, field := range typeInfo.fields {
		if field.ignore {
			continue
		}

		value := structValue.Field(i)
		if field.embedded {
			if err := d.decodeStruct(value, tokens); err != nil {
				return err
			}
			continue
		}

		token, ok := tokens[field.name]
		if !ok {
			continue
		}

		if !value.IsValid() || !value.CanSet() {
			continue
		}

		if err := d.setStructField(value, token); err != nil {
			return fmt.Errorf("ldf: decode: %s: %v", field.name, err)
		}
	}

	return nil
}

func (d *TextDecoder) setMapValue(mapValue reflect.Value, token Token) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()

	valueType := mapValue.Type().Elem()

	v, err := d.getValue(token, valueType)
	if err != nil {
		return err
	}

	if !v.Type().AssignableTo(valueType) {
		return fmt.Errorf("cannot assign %v to %v", v.Type(), valueType)
	}

	mapValue.SetMapIndex(reflect.ValueOf(token.Name), v)

	return nil
}

func (d *TextDecoder) decodeMap(mapValue reflect.Value) error {
	if mapValue.Type().Key().Kind() != reflect.String {
		return fmt.Errorf("ldf: decode: map key type must be a string")
	}

	tokens, err := d.tokens()
	if err != nil {
		return fmt.Errorf("ldf: decode: %v", err)
	}

	for key, token := range tokens {
		if err := d.setMapValue(mapValue, token); err != nil {
			return fmt.Errorf("ldf: decode: %s: %v", key, err)
		}
	}

	return nil
}

func (d *TextDecoder) Decode(v any) error {
	if v == nil {
		return fmt.Errorf("ldf: decode: v cannot be nil")
	}

	value := reflect.ValueOf(v)
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return fmt.Errorf("ldf: decode: v cannot be nil")
		}

		value = reflect.Indirect(value)
		if value.Kind() != reflect.Struct {
			return fmt.Errorf("ldf: decode: cannot decode %v", value.Kind())
		}

		tokens, err := d.tokens()
		if err != nil {
			return fmt.Errorf("ldf: decode: %v", err)
		}

		return d.decodeStruct(value, tokens)
	case reflect.Map:
		return d.decodeMap(value)
	default:
		return fmt.Errorf("ldf: decode: cannot decode %v", value.Kind())
	}
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
// If the field type is a slice, a new slice is created.
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
