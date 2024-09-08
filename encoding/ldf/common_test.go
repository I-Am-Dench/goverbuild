package ldf_test

import (
	"fmt"

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
	String  string  `ldf:"STRING,utf16"`
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
	Std8  string `ldf:"STD8"`
	Std16 string `ldf:"STD16,utf16"`

	U16   ldf.Utf16String `ldf:"U16"`
	Bytes []byte          `ldf:"BYTES"`
}

func (strings Strings) Format(format string) string {
	return fmt.Sprintf(format, strings.Std8, strings.Std16, strings.U16.String(), string(strings.Bytes))
}
