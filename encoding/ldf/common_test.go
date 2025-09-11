package ldf_test

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/I-Am-Dench/goverbuild/encoding/ldf"
)

const (
	formatCommasOnly = "STRING=0:%s,INT32=1:%d,FLOAT=3:%v,DOUBLE=4:%v,UINT32=5:%d,BOOLEAN=7:%t"

	formatNewlines = `
STRING=0:%s
INT32=1:%d
FLOAT=3:%f
DOUBLE=4:%v
UINT32=5:%d
BOOLEAN=7:%t`

	formatWhitespace = `

	STRING=0:%s

INT32=1:%d
FLOAT=3:%f

DOUBLE=4:%v
UINT32=5:%d

      BOOLEAN=7:%t
	`

	formatMixedCommasAndNewlines = `STRING=0:%s
INT32=1:%d,
FLOAT=3:%f
DOUBLE=4:%v
UINT32=5:%d,
BOOLEAN=7:%t`

	formatCarriageReturns = "STRING=0:%s\r\nINT32=1:%d\r\nFLOAT=3:%f\r\nDOUBLE=4:%v\r\nUINT32=5:%d\r\nBOOLEAN=7:%t"

	formatNewlineCommas = `STRING=0:%s,
INT32=1:%d,
FLOAT=3:%v,
DOUBLE=4:%v,
UINT32=5:%d,
BOOLEAN=7:%t`
)

type LdfBool bool

func (b LdfBool) Format(f fmt.State, verb rune) {
	switch verb {
	case 't':
		out := "0"
		if b {
			out = "1"
		}
		f.Write([]byte(out))
	default:
		fmt.Fprintf(f, fmt.FormatString(f, verb), bool(b))
	}
}

type Basic struct {
	String  string  `ldf:"STRING"`
	Int32   int32   `ldf:"INT32"`
	Float   float32 `ldf:"FLOAT"`
	Double  float64 `ldf:"DOUBLE"`
	Uint32  uint32  `ldf:"UINT32"`
	Boolean LdfBool `ldf:"BOOLEAN"`
}

func (basic Basic) Format(format string) string {
	return fmt.Sprintf(format, basic.String, basic.Int32, basic.Float, basic.Double, basic.Uint32, basic.Boolean)
}

type Strings struct {
	Std8  string `ldf:"STD8,raw"`
	Std16 string `ldf:"STD16"`

	U16   ldf.String16 `ldf:"U16"`
	Bytes []byte       `ldf:"BYTES"`
}

func (strings Strings) Format(format string) string {
	return fmt.Sprintf(format, strings.Std8, strings.Std16, strings.U16.String(), string(strings.Bytes))
}

type Ints []int

func (l Ints) MarshalText() ([]byte, error) {
	buf := bytes.Buffer{}
	for i, n := range l {
		if i != 0 {
			buf.WriteRune(';')
		}
		buf.WriteString(strconv.Itoa(n))
	}

	return buf.Bytes(), nil
}

func (l *Ints) UnmarshalText(text []byte) error {
	parts := bytes.Split(text, []byte(";"))

	items := []int{}
	for _, part := range parts {
		n, err := strconv.Atoi(string(part))
		if err != nil {
			return fmt.Errorf("could not parse int: %v", err)
		}
		items = append(items, n)
	}
	*l = items

	return nil
}

type WithEncodings struct {
	IntList Ints `ldf:"int_list"`
}

type SubStruct struct {
	A int `ldf:"a"`
}

type Embedded struct {
	SubStruct
	B float32 `ldf:"b"`
}

func (e Embedded) Format(format string) string {
	return fmt.Sprintf(format, e.A, e.B)
}
