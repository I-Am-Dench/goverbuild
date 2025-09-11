package ldf_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/I-Am-Dench/goverbuild/encoding/ldf"
)

type Formatter interface {
	Format(string) string
}

func checkExpected(t *testing.T, expected, actual []byte) {
	if !bytes.Equal(expected, actual) {
		t.Errorf("\nexpected = \"%s\"\nactual   = \"%s\"", string(expected), string(actual))
	}
}

func checkMarshalText(t *testing.T, formatter Formatter, format string) {
	data, err := ldf.MarshalText(formatter)
	if err != nil {
		t.Fatal(err)
	}

	checkExpected(t, []byte(formatter.Format(format)), data)
}

func checkMarshalLines(t *testing.T, formatter Formatter, format string) {
	data, err := ldf.MarshalLines(formatter)
	if err != nil {
		t.Fatal(err)
	}

	checkExpected(t, []byte(formatter.Format(format)), data)
}

func TestEncode(t *testing.T) {

	t.Run("commas_only", func(t *testing.T) {
		v := Basic{
			String:  "If just being born is the greatest act of creation. Then what are you suppose to do after that? Isn't everything that comes next just sort of a disappointment? Slowly entropying until we deflate into a pile of mush?",
			Int32:   2123311855,
			Float:   0.2394242421,
			Double:  -15555313.199119,
			Uint32:  2340432028,
			Boolean: true,
		}

		checkMarshalText(t, v, formatCommasOnly)
		checkMarshalLines(t, v, formatNewlineCommas)
	})

	t.Run("strings", func(t *testing.T) {
		v := Strings{
			Std8:  "Crazy? I was crazy once.",
			Std16: "They put me in a room. A rubber room. A rubber room of rats.",
			U16:   ldf.ToString16("And the rats made me crazy."),
			Bytes: []byte("I can't think of any more interesting strings."),
		}

		checkMarshalText(t, v, "STD8=13:%s,STD16=0:%s,U16=0:%s,BYTES=13:%s")
	})

	t.Run("omitempty", func(t *testing.T) {
		v := struct {
			Empty    string `ldf:"empty,omitempty"`
			NotEmpty string `ldf:"not_empty,omitempty"`
		}{
			Empty:    "",
			NotEmpty: "NOT EMPTY",
		}

		actual, err := ldf.MarshalText(v)
		if err != nil {
			t.Fatal(err)
		}

		checkExpected(t, []byte("not_empty=0:NOT EMPTY"), actual)
	})

	t.Run("text_marshaler", func(t *testing.T) {
		v := WithEncodings{
			IntList: Ints{38, 24, 93, 70, 37},
		}

		actual, err := ldf.MarshalText(v)
		if err != nil {
			t.Fatal(err)
		}

		checkExpected(t, []byte("int_list=0:38;24;93;70;37"), actual)
	})

	t.Run("map", func(t *testing.T) {
		v := map[string]any{
			"String":  "An encoded string",
			"Int32":   int32(-159610504),
			"Float":   float32(95.8676),
			"Double":  float64(177.566233582597),
			"Uint32":  uint32(1808189996),
			"Bool":    true,
			"Uint64":  uint64(1420086543),
			"Int64":   int64(462309426),
			"IntList": Ints{22, -74, 24, -39, 98},
		}

		data, err := ldf.MarshalText(v)
		if err != nil {
			t.Fatal(err)
		}

		expected := []string{
			"String=0:An encoded string",
			"Int32=1:-159610504",
			"Uint32=5:1808189996",
			"Float=3:95.8676",
			"Double=4:177.566233582597",
			"Bool=7:1",
			"Uint64=8:1420086543",
			"Int64=9:462309426",
			"IntList=0:22;-74;24;-39;98",
		}
		actual := strings.Split(string(data), ",")

		slices.Sort(expected)
		slices.Sort(actual)

		if !slices.Equal(expected, actual) {
			t.Errorf("\nexpected = \"%s\"\nactual   = \"%s\"", strings.Join(expected, ","), strings.Join(actual, ","))
		}
	})

	t.Run("embedded", func(t *testing.T) {
		v := Embedded{
			SubStruct: SubStruct{
				A: 7345,
			},
			B: -56.597155501,
		}

		checkMarshalText(t, v, "a=9:%d,b=3:%f")
	})
}
