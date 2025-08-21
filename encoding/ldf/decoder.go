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

func (token *Token) TrimmedValue() string {
	return strings.TrimSpace(token.RawValue)
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

func (decoder *TextDecoder) decodeToken(rawToken string) (Token, error) {
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

func (decoder *TextDecoder) Next() bool {
	ok := decoder.s.Scan()
	if !ok {
		decoder.err = decoder.s.Err()
		return false
	}

	rawToken := decoder.s.Text()
	for len(rawToken) == 0 && decoder.s.Scan() {
		rawToken = decoder.s.Text()
	}

	if (len(strings.TrimSpace(rawToken))) == 0 {
		return false
	}

	token, err := decoder.decodeToken(rawToken)
	if err != nil {
		decoder.err = err
		return false
	}

	decoder.token = token

	return true
}

func (decoder *TextDecoder) Token() (Token, error) {
	return decoder.token, decoder.err
}

func (decoder *TextDecoder) Err() error {
	return decoder.err
}

func (decoder *TextDecoder) setStructField(field reflect.Value, token Token) (err error) {
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

func (decoder *TextDecoder) Decode(v any) error {
	if v == nil || reflect.TypeOf(v).Kind() != reflect.Pointer {
		return nil
	}

	if reflect.ValueOf(v).IsNil() {
		return errors.New("ldf: decode: reference cannot be a nil pointer")
	}

	tokens := make(map[string]Token)

	for decoder.Next() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}

		tokens[strings.TrimSpace(token.Name)] = token
	}

	if err := decoder.Err(); err != nil {
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

		if err := decoder.setStructField(value, token); err != nil {
			return fmt.Errorf("ldf: decode: %w", err)
		}
	}

	return nil
}

func Unmarshal(data []byte, v any) error {
	buf := bytes.NewBuffer(data)
	return NewTextDecoder(buf).Decode(v)
}
