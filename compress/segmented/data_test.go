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

var dataSignature = append([]byte("sd0"), 0x01, 0xff)

func createData() []byte {
	const chars = "abcdefghijklmnopqrstuvwxyz"

	numBytes := int(rand.Float32()*float32(segmented.DefaultChunkSize*2)) + (segmented.DefaultChunkSize * 2)
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
	final.Write(dataSignature)

	for buf.Len() > 0 {
		chunk := buf.Next(segmented.DefaultChunkSize)

		compressed := &bytes.Buffer{}
		writer, _ := zlib.NewWriterLevel(compressed, zlib.BestCompression)
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

func testWrite(t *testing.T, data []byte) {
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

	if actual.Len() != int(writer.BytesWritten()) {
		t.Errorf("data writer: writer compressed %d bytes but returned %d", actual.Len(), writer.BytesWritten())
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

func TestDataWriter(t *testing.T) {
	t.Run("short", func(t *testing.T) {
		testWrite(t, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	})

	for i := 0; i < 10; i++ {
		t.Run("random_data", func(t *testing.T) {
			testWrite(t, createData())
		})
	}
}

func testRead(t *testing.T, expected []byte) {
	compressed := compress(expected)
	t.Logf("data reader: testing %d compressed bytes", compressed.Len())

	t.Logf("data reader: expecting %d uncompressed bytes", len(expected))

	reader, err := segmented.NewDataReader(compressed)
	if err != nil {
		t.Fatal(err)
	}

	actual := &bytes.Buffer{}
	if _, err := io.Copy(actual, reader); err != nil {
		t.Fatalf("data reader: %v", err)
	}

	t.Logf("data reader: uncompressed %d bytes", actual.Len())

	if len(expected) != actual.Len() {
		t.Errorf("data reader: expected %d bytes but got %d", len(expected), actual.Len())
		dump(bytes.NewBuffer(expected), actual)
		return
	}

	if !bytes.Equal(expected, actual.Bytes()) {
		t.Errorf("data reader: data does not match")
		dump(bytes.NewBuffer(expected), actual)
		return
	}
}

func TestDataReader(t *testing.T) {
	t.Run("short", func(t *testing.T) {
		testRead(t, []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbccccccccc"))
	})

	for i := 0; i < 10; i++ {
		t.Run("random_data", func(t *testing.T) {
			testRead(t, createData())
		})
	}
}
