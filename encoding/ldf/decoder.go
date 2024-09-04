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

	if len(rawToken) == 0 {
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

func (decoder *TextDecoder) setStructField(field reflect.Value, token Token) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("setStructField: %v", r)
		}
	}()

	switch token.Type.Kind() {
	case reflect.String:
		field.SetString(strings.TrimRight(token.RawValue, "\r"))
	case reflect.Int32:
		i, err := strconv.ParseInt(token.TrimmedValue(), 10, 32)
		if err != nil {
			return err
		}

		field.SetInt(i)
	case reflect.Float32:
		f, err := strconv.ParseFloat(token.TrimmedValue(), 32)
		if err != nil {
			return err
		}

		field.SetFloat(f)
	case reflect.Float64:
		f, err := strconv.ParseFloat(token.TrimmedValue(), 64)
		if err != nil {
			return err
		}

		field.SetFloat(f)
	case reflect.Uint32:
		i, err := strconv.ParseUint(token.TrimmedValue(), 10, 32)
		if err != nil {
			return err
		}

		field.SetUint(i)
	case reflect.Bool:
		field.SetBool(token.TrimmedValue() == "1")
	case reflect.Uint64:
		i, err := strconv.ParseUint(token.TrimmedValue(), 10, 64)
		if err != nil {
			return err
		}

		field.SetUint(i)
	case reflect.Int64:
		i, err := strconv.ParseInt(token.TrimmedValue(), 10, 64)
		if err != nil {
			return err
		}

		field.SetInt(i)
	default:
		return fmt.Errorf("cannot decode ldf type: %v", token.Type.Kind())
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

	structValue := reflect.ValueOf(v).Elem()
	structType := structValue.Type()

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		tagValue := field.Tag.Get("ldf")

		if len(tagValue) <= 0 || tagValue == "-" {
			continue
		}

		tokenName, _, _ := strings.Cut(tagValue, ",")

		token, ok := tokens[tokenName]
		if !ok {
			continue
		}

		value := structValue.Field(i)
		if !value.IsValid() || !value.CanSet() {
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
