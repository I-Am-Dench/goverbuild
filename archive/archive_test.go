package archive_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/I-Am-Dench/goverbuild/archive"
)

var Expected = archive.Info{
	UncompressedSize:     2964,
	UncompressedChecksum: []byte{143, 178, 135, 9, 197, 251, 68, 83, 23, 131, 129, 223, 160, 192, 182, 252},
	CompressedSize:       1222,
	CompressedChecksum:   []byte{224, 178, 115, 85, 37, 167, 233, 132, 236, 232, 8, 114, 169, 0, 69, 95},
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
		t.Errorf("expected uncompressed checksum %x but got %x", Expected.UncompressedChecksum, info.UncompressedChecksum)
	}

	if Expected.CompressedSize != info.CompressedSize {
		t.Errorf("expected compressed size %d but got %d", Expected.CompressedSize, info.CompressedSize)
	}

	if !bytes.Equal(Expected.CompressedChecksum, info.CompressedChecksum) {
		t.Errorf("expected compressed checksum %x but got %x", Expected.CompressedChecksum, info.CompressedChecksum)
	}
}
