package sid0

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/I-Am-Dench/goverbuild/internal"
)

type DataReader struct {
	baseReader internal.ReadSeekerAt
	zlibReader io.ReadCloser

	chunkSize uint32
	totalSize int64
	bytesRead int64

	bytesDecompressed uint32
}

func (reader *DataReader) Read(p []byte) (n int, err error) {
	if reader.zlibReader == nil {
		if err := binary.Read(reader.baseReader, order, &reader.chunkSize); err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.EOF
			}

			return 0, fmt.Errorf("sd0: read: %w", err)
		}

		reader.bytesRead += 4
		zr, err := zlib.NewReader(io.NewSectionReader(reader.baseReader, reader.bytesRead, int64(reader.chunkSize)))
		if err != nil {
			return 0, fmt.Errorf("sd0: read: %w", err)
		}

		reader.zlibReader = zr
	}

	n, err = reader.zlibReader.Read(p)
	if errors.Is(err, io.EOF) {
		if reader.bytesRead >= reader.totalSize {
			return 0, io.EOF
		}

		reader.baseReader.Seek(int64(reader.chunkSize), io.SeekCurrent)
		reader.bytesRead += int64(reader.chunkSize)
		reader.zlibReader = nil
		err = nil
	}

	if err != nil {
		return 0, fmt.Errorf("sd0: read: %w", err)
	}

	reader.bytesDecompressed += uint32(n)

	return
}

func NewDataReader(r io.Reader, totalSize uint32) (*DataReader, error) {
	readSeeker, ok := r.(internal.ReadSeekerAt)
	if !ok {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("sd0: %w", err)
		}

		readSeeker = bytes.NewReader(data)
	}

	sig := [5]byte{}
	if _, err := r.Read(sig[:]); err != nil {
		return nil, fmt.Errorf("sd0: signature: %w", err)
	}

	if !bytes.Equal(sig[:], Signature) {
		return nil, fmt.Errorf("sd0: signature: invalid signature")
	}

	return &DataReader{
		baseReader: readSeeker,
		totalSize:  int64(totalSize),
		bytesRead:  5, // Size of signature
	}, nil
}
