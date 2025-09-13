package ldf_test

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"testing"
	"unsafe"

	"github.com/I-Am-Dench/goverbuild/encoding/ldf"
)

func checkBasic(expected Basic, format string) func(*testing.T) {
	return func(t *testing.T) {
		data := expected.Format(format)

		actual := Basic{}
		if err := ldf.UnmarshalText([]byte(data), &actual); err != nil {
			t.Fatal(err)
		}

		if expected.String != actual.String {
			t.Errorf("expected string %q but got %q", expected.String, actual.String)
		}

		if expected.Int32 != actual.Int32 {
			t.Errorf("expected int32 %d but got %d", expected.Int32, actual.Int32)
		}

		if expected.Float != actual.Float {
			t.Errorf("expected float %f but got %f", expected.Float, actual.Float)
		}

		if expected.Double != actual.Double {
			t.Errorf("expected double %g but got %g", expected.Double, actual.Double)
		}

		if expected.Uint32 != actual.Uint32 {
			t.Errorf("expected uint32 %d but got %d", expected.Uint32, actual.Uint32)
		}

		if expected.Boolean != actual.Boolean {
			t.Errorf("expected bool %t but got %t", bool(expected.Boolean), bool(actual.Boolean))
		}

	}
}

type Integers struct {
	Int  int  `ldf:"INT"`
	Uint uint `ldf:"UINT"`
}

func checkInts(expected Integers, format string) func(*testing.T) {
	return func(t *testing.T) {
		data := []byte(fmt.Sprintf(format, expected.Int, expected.Uint))

		actual := Integers{}
		if err := ldf.UnmarshalText(data, &actual); err != nil {
			t.Fatal(err)
		}

		if expected.Int != actual.Int {
			t.Errorf("expected int %d but got %d", expected.Int, actual.Int)
		}

		if expected.Uint != actual.Uint {
			t.Errorf("expected uint %d but got %d", expected.Uint, actual.Uint)
		}
	}
}

func checkStdStrings(expected Strings, format string) func(*testing.T) {
	return func(t *testing.T) {
		data := []byte(fmt.Sprintf(format, expected.Std8, expected.Std16))

		actual := Strings{}
		if err := ldf.UnmarshalText(data, &actual); err != nil {
			t.Fatal(err)
		}

		if expected.Std8 != actual.Std8 {
			t.Errorf("expected %q but got %q", expected.Std8, actual.Std8)
		}

		if expected.Std16 != actual.Std16 {
			t.Errorf("expected %q but got %q", expected.Std16, actual.Std16)
		}
	}
}

func checkMaps(t *testing.T, expected, actual map[string]any) {
	for key, expectedValue := range expected {
		actualValue, ok := actual[key]
		if !ok {
			t.Errorf("map does not contains %s", key)
			continue
		}

		if !reflect.ValueOf(expectedValue).Equal(reflect.ValueOf(actualValue)) {
			t.Errorf("%s: expected %v but got %v", key, expectedValue, actualValue)
		}
	}
}

func TestDecode(t *testing.T) {
	basic := Basic{
		String:  "Save Imagination! :)",
		Int32:   2010,
		Float:   float32(39.99),
		Double:  float64(3.14159265358932384),
		Uint32:  4051612861,
		Boolean: true,
	}

	ints := Integers{
		Int:  -180015668,
		Uint: 2401893510,
	}

	stdStrings := Strings{
		Std8:  "DO NOT GO TO PORTOBELLO",
		Std16: "WORST MISTAKE OF MY LIFE",
	}

	t.Run("commas_only", checkBasic(basic, formatCommasOnly))
	t.Run("newlines_only", checkBasic(basic, formatNewlines))
	t.Run("whitespace", checkBasic(basic, formatWhitespace))
	t.Run("mixed_commas_and_newlines", checkBasic(basic, formatMixedCommasAndNewlines))
	t.Run("cariage_returns", checkBasic(basic, formatCarriageReturns))

	t.Run("ints_32", checkInts(ints, "INT=1:%d,UINT=5:%d"))
	t.Run("ints_64", checkInts(ints, "INT=9:%d,UINT=8:%d"))
	t.Run("large_ints", func(t *testing.T) {
		data := []byte("INT=9:-9223372036854775808,UINT=8:18446744073709551615")

		largeInts := Integers{}
		if err := ldf.UnmarshalText(data, &largeInts); err != nil {
			t.Fatal(err)
		}

		if unsafe.Sizeof(int(0)) == 8 {
			if largeInts.Int != -9223372036854775808 {
				t.Errorf("int{64}: expected -9223372036854775808 but got %d", largeInts.Int)
			}

			if largeInts.Uint != 18446744073709551615 {
				t.Errorf("uint{64}: expected 18446744073709551615 but got %d", largeInts.Uint)
			}
		} else if unsafe.Sizeof(int(0)) == 4 {
			if largeInts.Int != 0 {
				t.Errorf("int{32}: expected 0 but got %d", largeInts.Int)
			}

			if largeInts.Uint != 4294967295 {
				t.Errorf("uint{32}: expected 4294967295 but got %d", largeInts.Uint)
			}
		}
	})

	t.Run("std_strings", checkStdStrings(stdStrings, "STD8=0:%s,STD16=0:%s"))
	t.Run("std_strings_raw", checkStdStrings(stdStrings, "STD8=13:%s,STD16=13:%s"))

	t.Run("encoded_strings", func(t *testing.T) {
		expected := Strings{
			U16:   ldf.ToString16("Check this out: ӔŦഹ"),
			Bytes: []byte("https://www.tcfriedrich.com"),
		}

		data := []byte(fmt.Sprintf("U16=0:%s,BYTES=13:%s", expected.U16.String(), string(expected.Bytes)))

		actual := Strings{}
		if err := ldf.UnmarshalText(data, &actual); err != nil {
			t.Fatal(err)
		}

		if !slices.Equal(actual.U16, expected.U16) {
			t.Errorf("expected %v but got %v", expected.U16, actual.U16)
		}

		if !bytes.Equal(actual.Bytes, expected.Bytes) {
			t.Errorf("expected %v but got %v", expected.Bytes, actual.Bytes)
		}
	})

	t.Run("text_unmarshaler", func(t *testing.T) {
		expected := Ints{-100, -53, -55, 6, 85}

		data := []byte("int_list=0:-100;-53;-55;6;85")

		actual := WithEncodings{}
		if err := ldf.UnmarshalText(data, &actual); err != nil {
			t.Fatal(err)
		}

		if !slices.Equal(expected, actual.IntList) {
			t.Errorf("expected %v but got %v", expected, actual.IntList)
		}
	})

	t.Run("map", func(t *testing.T) {
		expected := map[string]any{
			"String": "An encoded string",
			"Int32":  int32(-159610504),
			"Float":  float32(95.8676),
			"Double": float64(177.566233582597),
			"Uint32": uint32(1808189996),
			"Bool":   true,
			"Uint64": uint64(1420086543),
			"Int64":  int64(462309426),
		}

		data := []byte("Int64=9:462309426,Float=3:95.8676,Bool=7:1,Uint64=8:1420086543,Uint32=5:1808189996,String=0:An encoded string,Int32=1:-159610504,Double=4:177.566233582597")

		actual := map[string]any{}
		if err := ldf.UnmarshalText(data, actual); err != nil {
			t.Fatal(err)
		}

		checkMaps(t, expected, actual)
	})

	t.Run("embedded", func(t *testing.T) {
		expected := Embedded{
			SubStruct: SubStruct{
				A: -621,
			},
			B: 238.2281277,
		}

		data := []byte("a=9:-621,b=3:238.2281277")

		actual := Embedded{}
		if err := ldf.UnmarshalText(data, &actual); err != nil {
			t.Fatal(err)
		}

		if expected.A != actual.A {
			t.Errorf("expected int %d but got %d", expected.A, actual.A)
		}

		if expected.B != actual.B {
			t.Errorf("expected float32 %f but got %f", expected.B, actual.B)
		}
	})

	t.Run("delim", func(t *testing.T) {
		expected := Strings{
			Std8:  "Some text with a comma, I guess...",
			Std16: "Mermaid Man says: \"BUY MORE CARDS\"",
		}

		data := []byte(fmt.Sprintf("STD8=0:%s\nSTD16=13:%s", expected.Std8, expected.Std16))

		decoder := ldf.NewTextDecoder(bytes.NewReader(data))
		decoder.SetDelim(regexp.MustCompile("\n"))

		actual := Strings{}
		if err := decoder.Decode(&actual); err != nil {
			t.Fatal(err)
		}

		if expected.Std8 != actual.Std8 {
			t.Errorf("expected %q but got %q", expected.Std8, actual.Std8)
		}

		if expected.Std16 != actual.Std16 {
			t.Errorf("expected %q but got %q", expected.Std16, actual.Std16)
		}
	})
}
