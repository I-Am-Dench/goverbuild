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

	data, err = ldf.MarshalTextLines(v)
	if err != nil {
		t.Fatalf("test marshal: newline commas: %v", err)
	}

	expectedData = v.Format(formatNewlineCommas)
	if !bytes.Equal(data, []byte(expectedData)) {
		t.Errorf("test marshal: newline commas:\nexpected = \"%s\"\nactual  = \"%s\"", expectedData, string(data))
	}

}
