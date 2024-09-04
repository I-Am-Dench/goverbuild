package ldf_test

import (
	"errors"
	"fmt"
	"testing"
	"unsafe"

	"github.com/I-Am-Dench/goverbuild/encoding/ldf"
)

func testUnmarshalBasic(expected Basic, format string) error {
	data := []byte(fmt.Sprintf(format, expected.String, expected.Int32, expected.Float, expected.Double, expected.Uint32, expected.Boolean))

	actual := Basic{}
	if err := ldf.Unmarshal(data, &actual); err != nil {
		return err
	}

	errs := []error{}

	if actual.String != expected.String {
		errs = append(errs, fmt.Errorf("expected string \"%s\" but got \"%s\"", expected.String, actual.String))
	}

	if actual.Int32 != expected.Int32 {
		errs = append(errs, fmt.Errorf("expected int32 %d but got %d", expected.Int32, actual.Int32))
	}

	if actual.Float != expected.Float {
		errs = append(errs, fmt.Errorf("expected float %f but got %f", expected.Float, actual.Float))
	}

	if actual.Double != expected.Double {
		errs = append(errs, fmt.Errorf("expected double %g but got %g", expected.Double, actual.Double))
	}

	if actual.Uint32 != expected.Uint32 {
		errs = append(errs, fmt.Errorf("expected uint32 %d but got %d", expected.Uint32, actual.Uint32))
	}

	if actual.Boolean != expected.Boolean {
		errs = append(errs, fmt.Errorf("expected %t but got %t", bool(expected.Boolean), bool(actual.Boolean)))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func TestUnmarshal(t *testing.T) {
	expected := Basic{
		String:  "Save Imagination! :)",
		Int32:   int32(2010),
		Float:   float32(39.99),
		Double:  float64(3.14159265358932384),
		Uint32:  uint32(4051612861),
		Boolean: true,
	}

	if err := testUnmarshalBasic(expected, formatCommasOnly); err != nil {
		t.Errorf("test unmarshal commas only: %v", err)
	}

	if err := testUnmarshalBasic(expected, formatNewlines); err != nil {
		t.Errorf("test unmarshal newlines only: %v", err)
	}

	if err := testUnmarshalBasic(expected, formatWhitespace); err != nil {
		t.Errorf("test unmarshal whitespace: %v", err)
	}

	if err := testUnmarshalBasic(expected, formatMixedCommasAndNewlines); err != nil {
		t.Errorf("test unmarshal mixed commas and newlines: %v", err)
	}

	if err := testUnmarshalBasic(expected, formatCarriageReturns); err != nil {
		t.Errorf("test unmarshal carriage returns: %v", err)
	}
}

type Integers struct {
	Int  int  `ldf:"INT"`
	Uint uint `ldf:"UINT"`
}

func testUnmarshalIntegers(expected Integers, format string) error {
	data := []byte(fmt.Sprintf(format, expected.Int, expected.Uint))

	actual := Integers{}
	if err := ldf.Unmarshal(data, &actual); err != nil {
		return err
	}

	if actual.Int != expected.Int {
		return fmt.Errorf("expected int %d but got %d", expected.Int, actual.Int)
	}

	if actual.Uint != expected.Uint {
		return fmt.Errorf("expected uint %d but got %d", expected.Uint, actual.Uint)
	}

	return nil
}

func TestUnmarshalIntegers(t *testing.T) {
	expected := Integers{
		Int:  -180015668,
		Uint: 2401893510,
	}

	if err := testUnmarshalIntegers(expected, "INT=1:%d,UINT=5:%d"); err != nil {
		t.Errorf("test unmarshal integers: 32 bits: %v", err)
	}

	if err := testUnmarshalIntegers(expected, "INT=9:%d,UINT=8:%d"); err != nil {
		t.Errorf("test unmarshal integers: 64 bits: %v", err)
	}

	data := "INT=9:-9223372036854775808,UINT=8:18446744073709551615"

	largeInts := Integers{}
	if err := ldf.Unmarshal([]byte(data), &largeInts); err != nil {
		t.Errorf("test unmarshal integers: 64 bit value: %v", err)
	}

	if unsafe.Sizeof(int(0)) == 8 {
		if largeInts.Int != -9223372036854775808 {
			t.Errorf("test unmarshal integers: int{64}: expected -9223372036854775808 but got %d", largeInts.Int)
		}

		if largeInts.Uint != 18446744073709551615 {
			t.Errorf("test unmarshal integers: uint{64}: expected 18446744073709551615 but got %d", largeInts.Uint)
		}
	} else if unsafe.Sizeof(int(0)) == 4 {
		if largeInts.Int != 0 {
			t.Errorf("test unmarshal integers: int{32}: expected 0 but got %d", largeInts.Int)
		}

		if largeInts.Uint != 4294967295 {
			t.Errorf("test unmarshal integers: uint{32}: expected 4294967295 but got %d", largeInts.Uint)
		}
	}
}
