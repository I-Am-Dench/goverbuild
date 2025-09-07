package ldf_test

import (
	"bytes"
	"testing"

	"github.com/I-Am-Dench/goverbuild/encoding/ldf"
)

func TestMarshal(t *testing.T) {
	v := Basic{
		String:  "If just being born is the greatest act of creation. Then what are you suppose to do after that? Isn't everything that comes next just sort of a disappointment? Slowly entropying until we deflate into a pile of mush?",
		Int32:   2123311855,
		Float:   0.2394242421,
		Double:  -15555313.199119,
		Uint32:  2340432028,
		Boolean: true,
	}

	data, err := ldf.MarshalText(v)
	if err != nil {
		t.Fatalf("test marshal: commas only: %v", err)
	}

	expectedData := v.Format(formatCommasOnly)
	if !bytes.Equal(data, []byte(expectedData)) {
		t.Errorf("test marshal: commas only:\nexpected = \"%s\"\nactual   = \"%s\"", expectedData, string(data))
	}

	data, err = ldf.MarshalLines(v)
	if err != nil {
		t.Fatalf("test marshal: newline commas: %v", err)
	}

	expectedData = v.Format(formatNewlineCommas)
	if !bytes.Equal(data, []byte(expectedData)) {
		t.Errorf("test marshal: newline commas:\nexpected = \"%s\"\nactual  = \"%s\"", expectedData, string(data))
	}

}

func TestMarshalStrings(t *testing.T) {
	v := Strings{
		Std8:  "Crazy? I was crazy once.",
		Std16: "They put me in a room. A rubber room. A rubber room of rats.",
		U16:   ldf.StringToUtf16("And the rats made me crazy."),
		Bytes: []byte("I can't think of any more interesting strings."),
	}

	data, err := ldf.MarshalText(v)
	if err != nil {
		t.Fatalf("test marshal strings: %v", err)
	}

	expectedData := v.Format("STD8=13:%s,STD16=0:%s,U16=0:%s,BYTES=13:%s")
	if !bytes.Equal(data, []byte(expectedData)) {
		t.Errorf("test marshal strings:\nexpected = \"%s\"\nactual   = \"%s\"", expectedData, string(data))
	}
}

func TestMarshalOmit(t *testing.T) {
	v := struct {
		Empty    string `ldf:"empty,omitempty"`
		NotEmpty string `ldf:"not_empty,omitempty"`
	}{
		Empty:    "",
		NotEmpty: "NOT EMPTY",
	}

	expected := []byte("not_empty=0:NOT EMPTY")

	actual, err := ldf.MarshalText(v)
	if err != nil {
		t.Fatalf("test marshal omit: %v", err)
	}

	if !bytes.Equal(actual, expected) {
		t.Errorf("test marshal omit:\nexpected = \"%s\"\nactual   = \"%s\"", expected, actual)
	}
}
