package segmented

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

type DataReader struct {
	baseReader io.Reader
	zlibReader io.ReadCloser

	chunkSize      uint32
	chunkBytesRead int64
}

func (reader *DataReader) Read(p []byte) (n int, err error) {
	if reader.zlibReader == nil {
		if err = binary.Read(reader.baseReader, order, &reader.chunkSize); err != nil {
			if err == io.EOF {
				return
			}
			return 0, fmt.Errorf("sd0: read: %w", err)
		}

		reader.zlibReader, err = zlib.NewReader(io.LimitReader(reader.baseReader, int64(reader.chunkSize)))
		if err != nil {
			return 0, fmt.Errorf("sd0: read: %w", err)
		}
	}

	n, err = reader.zlibReader.Read(p)
	reader.chunkBytesRead += int64(n)

	if err == io.EOF {
		if reader.chunkBytesRead < int64(reader.chunkSize) {
			return n, fmt.Errorf("sd0: read: %w", io.ErrShortBuffer)
		}

		err = nil
		reader.chunkBytesRead = 0
		reader.zlibReader = nil
		return
	}

	if err != nil {
		return n, fmt.Errorf("sd0: read: %w", err)
	}

	return
}

// Creates a new sd0 decompressor. NewDataReader returns a nil
// *DataReader if the function fails to verify the signature.
func NewDataReader(r io.Reader) (*DataReader, error) {
	sig := [5]byte{}
	if _, err := r.Read(sig[:]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, fmt.Errorf("sd0: read: signature: %w", err)
	}

	if !bytes.Equal(sig[:], Signature) {
		return nil, fmt.Errorf("sd0: read: invalid signature")
	}

	return &DataReader{
		baseReader: r,
	}, nil
}
