package archive_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/I-Am-Dench/goverbuild/archive"
)

var Expected = archive.Info{
	UncompressedSize:     2972,
	UncompressedChecksum: []byte{177, 30, 252, 122, 35, 42, 9, 24, 50, 111, 230, 37, 84, 80, 34, 154},
	CompressedSize:       1224,
	CompressedChecksum:   []byte{13, 121, 199, 209, 64, 177, 213, 237, 161, 197, 59, 231, 74, 119, 38, 68},
}

func TestCalculateInfo(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.txt")
	if err != nil {
		t.Fatal(err)
	}

	info, err := archive.CalculateInfoFromReader(bytes.NewReader(data), io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	if Expected.UncompressedSize != info.UncompressedSize {
		t.Errorf("expected uncompressed size %d but got %d", Expected.UncompressedSize, info.UncompressedSize)
	}

	if !bytes.Equal(Expected.UncompressedChecksum, info.UncompressedChecksum) {
		t.Errorf("expected uncompressed checksum %x but got %x", Expected.UncompressedSize, info.UncompressedSize)
	}

	if Expected.CompressedSize != info.CompressedSize {
		t.Errorf("expected uncompressed size %d but got %d", Expected.UncompressedSize, info.UncompressedSize)
	}

	if !bytes.Equal(Expected.CompressedChecksum, info.CompressedChecksum) {
		t.Errorf("expected uncompressed checksum %x but got %x", Expected.CompressedChecksum, info.CompressedChecksum)
	}
}
