package segmented

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	DefaultCompressionLevel = zlib.BestCompression
)

// NOTE: The live game used Python's zlib library to create
// sd0 files. Due to zlib implementation differences, DataWriter
// is NOT guaranteed to produce equivalent sd0 files as
// generated by lib_segmented.py within the original patcher.
//
// This is only an issue for compressing, not decompressing.
type DataWriter struct {
	baseWriter io.Writer
	zlibWriter *zlib.Writer

	buf *bytes.Buffer

	bytesLeft    int
	bytesWritten int64

	wroteSignature bool
}

func (writer *DataWriter) flush() (n int, err error) {
	if err := writer.zlibWriter.Close(); err != nil {
		return 0, err
	}

	size := writer.buf.Len()
	if err := binary.Write(writer.baseWriter, order, uint32(size)); err != nil {
		return 0, err
	}
	n += 4

	if written, err := io.Copy(writer.baseWriter, writer.buf); err != nil {
		return 0, err
	} else {
		n += int(written)
	}

	writer.zlibWriter, _ = zlib.NewWriterLevel(writer.buf, 9)
	writer.bytesLeft = ChunkSize
	return
}

func (writer *DataWriter) Write(p []byte) (n int, err error) {
	defer func() {
		if n < len(p) && err == nil {
			err = fmt.Errorf("sd0: write: %w", io.ErrShortWrite)
		}
	}()

	if !writer.wroteSignature {
		writer.wroteSignature = true
		if written, err := writer.baseWriter.Write(Signature); err != nil {
			return 0, fmt.Errorf("sd0: writer: signature: %w", err)
		} else {
			writer.bytesWritten += int64(written)
		}
	}

	extra := []byte{}
	if writer.bytesLeft < len(p) {
		written, _ := writer.zlibWriter.Write(p[:writer.bytesLeft])
		extra = p[writer.bytesLeft:]
		n += written
		writer.bytesLeft = 0
	} else {
		written, _ := writer.zlibWriter.Write(p)
		n += written
		writer.bytesLeft -= written
	}

	if writer.bytesLeft <= 0 {
		if written, err := writer.flush(); err != nil {
			return n, err
		} else {
			writer.bytesWritten += int64(written)
		}
	}

	if len(extra) > 0 {
		if written, err := writer.Write(extra); err != nil {
			return n, err
		} else {
			n += written
		}
	}

	return
}

func (writer *DataWriter) BytesWritten() int64 {
	return writer.bytesWritten
}

func (writer *DataWriter) Close() error {
	if writer.buf.Len() > 0 {
		if written, err := writer.flush(); err != nil {
			return fmt.Errorf("sd0: close: %w", err)
		} else {
			writer.bytesWritten += int64(written)
		}
	}
	return nil
}

func NewDataWriter(w io.Writer) *DataWriter {
	buf := &bytes.Buffer{}
	zw, _ := zlib.NewWriterLevel(buf, DefaultCompressionLevel)

	return &DataWriter{
		baseWriter: w,
		zlibWriter: zw,

		buf: buf,

		bytesLeft: ChunkSize,
	}
}
