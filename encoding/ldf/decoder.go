package ldf

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf16"
)

func splitLineComma(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.IndexAny(data, ",\n"); i >= 0 {
		return i + 1, data[0:i], nil
	}

	if atEOF {
		return len(data), data, nil
	}

	return
}

type TokenError struct {
	Err  error
	Line string
}

func (err *TokenError) Error() string {
	return fmt.Sprintf("ldf: token: %s: %s", err.Err, err.Line)
}

func (err *TokenError) Unwrap() error {
	return err.Err
}

type Token struct {
	Name     string
	Type     ValueType
	RawValue string
}

func (t *Token) TrimmedValue() string {
	return strings.TrimSpace(t.RawValue)
}

type TextDecoder struct {
	s   *bufio.Scanner
	err error

	token Token
}

func NewTextDecoder(r io.Reader) *TextDecoder {
	scanner := bufio.NewScanner(r)
	scanner.Split(splitLineComma)

	return &TextDecoder{
		s: scanner,
	}
}

func (d *TextDecoder) decodeToken(rawToken string) (Token, error) {
	key, valueWithType, ok := strings.Cut(rawToken, "=")
	if !ok {
		return Token{}, &TokenError{errors.New("missing key-value pair"), rawToken}
	}

	rawValueType, value, ok := strings.Cut(valueWithType, ":")
	if !ok {
		return Token{}, &TokenError{errors.New("missing value type"), rawToken}
	}

	valueType, err := strconv.Atoi(strings.TrimSpace(rawValueType))
	if err != nil {
		return Token{}, &TokenError{fmt.Errorf("value type is not a number: %s", rawValueType), rawToken}
	}

	if valueType < 0 || valueType > int(StringUtf8) {
		return Token{}, &TokenError{fmt.Errorf("invalid value type: %d", valueType), rawToken}
	}

	return Token{
		Name:     key,
		Type:     ValueType(valueType),
		RawValue: value,
	}, nil
}

func (d *TextDecoder) Next() bool {
	ok := d.s.Scan()
	if !ok {
		d.err = d.s.Err()
		return false
	}

	rawToken := d.s.Text()
	for len(rawToken) == 0 && d.s.Scan() {
		rawToken = d.s.Text()
	}

	if (len(strings.TrimSpace(rawToken))) == 0 {
		return false
	}

	token, err := d.decodeToken(rawToken)
	if err != nil {
		d.err = err
		return false
	}

	d.token = token

	return true
}

func (d *TextDecoder) Token() (Token, error) {
	return d.token, d.err
}

func (d *TextDecoder) Err() error {
	return d.err
}

func (d *TextDecoder) setStructField(field reflect.Value, token Token) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("setStructField: %v", r)
		}
	}()

	switch token.Type {
	case StringUtf16:
		trimmed := strings.TrimRight(token.RawValue, "\r")

		if field.Kind() == reflect.String {
			field.SetString(trimmed)
			break
		}

		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Uint16 {
			field.Set(reflect.ValueOf(utf16.Encode([]rune(trimmed))))
		}
	case Signed32:
		i, err := strconv.ParseInt(token.TrimmedValue(), 10, 32)
		if err != nil {
			return err
		}

		field.SetInt(i)
	case Float:
		f, err := strconv.ParseFloat(token.TrimmedValue(), 32)
		if err != nil {
			return err
		}

		field.SetFloat(f)
	case Double:
		f, err := strconv.ParseFloat(token.TrimmedValue(), 64)
		if err != nil {
			return err
		}

		field.SetFloat(f)
	case Unsigned32:
		i, err := strconv.ParseUint(token.TrimmedValue(), 10, 32)
		if err != nil {
			return err
		}

		field.SetUint(i)
	case Bool:
		field.SetBool(token.TrimmedValue() == "1")
	case Unsigned64:
		i, err := strconv.ParseUint(token.TrimmedValue(), 10, 64)
		if err != nil {
			return err
		}

		field.SetUint(i)
	case Signed64:
		i, err := strconv.ParseInt(token.TrimmedValue(), 10, 64)
		if err != nil {
			return err
		}

		field.SetInt(i)
	case StringUtf8:
		trimmed := strings.TrimRight(token.RawValue, "\r")

		if field.Kind() == reflect.String {
			field.SetString(trimmed)
			break
		}

		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Uint8 {
			field.Set(reflect.ValueOf([]byte(trimmed)))
		}
	default:
		return fmt.Errorf("cannot decode ldf type: %v", token.Type)
	}

	return nil
}

func (d *TextDecoder) Decode(v any) error {
	if reflect.TypeOf(v).Kind() != reflect.Pointer {
		return nil
	}

	if reflect.ValueOf(v).IsNil() {
		return errors.New("ldf: decode: v cannot be nil")
	}

	tokens := make(map[string]Token)

	for d.Next() {
		token, err := d.Token()
		if err != nil {
			return err
		}

		tokens[strings.TrimSpace(token.Name)] = token
	}

	if err := d.Err(); err != nil {
		return err
	}

	structValue := reflect.ValueOf(v).Elem()
	typeInfo := getTypeInfo(structValue.Type())

	for i, field := range typeInfo.fields {
		if field.ignore {
			continue
		}

		token, ok := tokens[field.name]
		if !ok {
			continue
		}

		value := structValue.Field(i)
		if !value.CanSet() {
			continue
		}

		if err := d.setStructField(value, token); err != nil {
			return fmt.Errorf("ldf: decode: %w", err)
		}
	}

	return nil
}

// UnmarshalText parser the textual LDF encoded data into v.
// UnmarshalText returns an error if v is nil.
//
// Fields may be separated by either a comma or newlines character.
//
// UnmarshalText returns an error if the encoded value type does
// not match the struct field's type.
//
// If the field type is a slice type, a new slice is created.
func UnmarshalText(data []byte, v any) error {
	buf := bytes.NewBuffer(data)
	return NewTextDecoder(buf).Decode(v)
}
