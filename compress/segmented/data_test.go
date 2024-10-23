package segmented_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
	"log"
	"math/rand"
	"os"
	"testing"

	"github.com/I-Am-Dench/goverbuild/compress/segmented"
)

func createData() []byte {
	const chars = "abcdefghijklmnopqrstuvwxyz"

	numBytes := int(rand.Float32()*float32(segmented.ChunkSize*2)) + (segmented.ChunkSize * 2)
	data := make([]byte, 0, numBytes)

	c := 0
	for len(data) < numBytes {
		n := rand.Intn(50) + 1
		for i := 0; i < n; i++ {
			data = append(data, byte(chars[c]))
		}
		c = (c + 1) % len(chars)
	}

	return data
}

func compress(data []byte) *bytes.Buffer {
	buf := bytes.NewBuffer(data)

	final := &bytes.Buffer{}
	final.Write(segmented.Signature)

	for buf.Len() > 0 {
		chunk := buf.Next(segmented.ChunkSize)

		compressed := &bytes.Buffer{}
		writer, _ := zlib.NewWriterLevel(compressed, 9)
		writer.Write(chunk)
		writer.Close()

		binary.Write(final, binary.LittleEndian, uint32(compressed.Len()))
		io.Copy(final, compressed)
	}

	return final
}

func writeFile(name string, data *bytes.Buffer) error {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, data); err != nil {
		return err
	}

	return nil
}

func dump(expected, actual *bytes.Buffer) {
	if err := os.MkdirAll("testdata", 0755); err != nil {
		log.Fatalf("dump: %v", err)
	}

	if err := writeFile("testdata/expected.bin", expected); err != nil {
		log.Printf("dump: %v", err)
	}

	if err := writeFile("testdata/actual.bin", actual); err != nil {
		log.Printf("dump: %v", err)
	}

	log.Println("dumped testdata")
}

func test(t *testing.T, data []byte) {
	t.Logf("data writer: testing %d uncompressed bytes", len(data))

	expected := compress(data)
	t.Logf("data writer: expecting %d compressed bytes", expected.Len())

	actual := &bytes.Buffer{}

	buf := bytes.NewBuffer(data)
	writer := segmented.NewDataWriter(actual)

	if _, err := io.Copy(writer, buf); err != nil {
		t.Fatalf("data writer: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("data writer: %v", err)
	}

	t.Logf("data writer: compressed %d bytes", actual.Len())

	if actual.Len() != int(writer.BytesCompressed()) {
		t.Errorf("data writer: writer compressed %d bytes but returned %d", actual.Len(), writer.BytesCompressed())
	}

	if expected.Len() != actual.Len() {
		t.Errorf("data writer: expected %d bytes but got %d", expected.Len(), actual.Len())
		dump(expected, actual)
		return
	}

	if !bytes.Equal(expected.Bytes(), actual.Bytes()) {
		t.Errorf("data writer: data does not match")
		dump(expected, actual)
		return
	}
}

func testRandomData(t *testing.T) {
	test(t, createData())
}

func TestDataWriter(t *testing.T) {
	t.Run("short", func(t *testing.T) {
		test(t, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	})

	for i := 0; i < 10; i++ {
		t.Run("random_data", testRandomData)
	}
}
