package pack

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/I-Am-Dench/goverbuild/archive/internal"
	"github.com/I-Am-Dench/goverbuild/compress/sid0"
)

type Record struct {
	r *io.SectionReader

	Crc      uint32
	CrcLower int32
	CrcUpper int32

	OriginalSize uint32
	OriginalHash []byte

	CompressedSize uint32
	CompressedHash []byte

	dataPointer uint32

	IsCompressed bool
}

func (record *Record) Section() (io.Reader, error) {
	if record.IsCompressed {
		reader, err := sid0.NewDataReader(record.r, record.CompressedSize)
		if err != nil {
			return nil, fmt.Errorf("pack: record: section: %w", err)
		}
		return reader, nil
	}

	return record.r, nil
}

type Pack struct {
	closer io.Closer

	records []*Record
}

func (pack *Pack) Records() []*Record {
	return pack.records
}

func (pack *Pack) Search(path string) (*Record, bool) {
	crc := internal.GetCrc(path)

	i := sort.Search(len(pack.records), func(i int) bool { return pack.records[i].Crc >= crc })
	if i < len(pack.records) && pack.records[i].Crc == crc {
		return pack.records[i], true
	}
	return nil, false
}

func (pack *Pack) Close() error {
	if pack.closer != nil {
		return pack.closer.Close()
	}

	return nil
}

func readHash(r io.ReadSeeker) ([]byte, error) {
	hashBuf := [36]byte{}
	if _, err := r.Read(hashBuf[:]); err != nil {
		return nil, err
	}

	hash, err := hex.DecodeString(string(hashBuf[:len(hashBuf)-4]))
	if err != nil {
		return nil, err
	}

	return hash, nil
}

func readRecord(r io.ReadSeeker) (*Record, error) {
	record := &Record{}

	if err := binary.Read(r, order, &record.Crc); err != nil {
		return nil, &RecordError{err, "index"}
	}

	if err := binary.Read(r, order, &record.CrcLower); err != nil {
		return nil, &RecordError{err, "crcLower"}
	}

	if err := binary.Read(r, order, &record.CrcUpper); err != nil {
		return nil, &RecordError{err, "crcUpper"}
	}

	if err := binary.Read(r, order, &record.OriginalSize); err != nil {
		return nil, &RecordError{err, "originalSize"}
	}

	originalHash, err := readHash(r)
	if err != nil {
		return nil, &RecordError{err, "originalHash"}
	}
	record.OriginalHash = originalHash

	if err := binary.Read(r, order, &record.CompressedSize); err != nil {
		return nil, &RecordError{err, "compressedSize"}
	}

	compressedHash, err := readHash(r)
	if err != nil {
		return nil, &RecordError{err, "compressedHash"}
	}
	record.CompressedHash = compressedHash

	if err := binary.Read(r, order, &record.dataPointer); err != nil {
		return nil, &RecordError{err, "dataPointer"}
	}

	boolData := [4]byte{}
	if _, err := r.Read(boolData[:]); err != nil {
		return nil, &RecordError{err, "isCompressed"}
	}

	record.IsCompressed = boolData[0] != 0

	return record, nil
}

func parseHeader(r io.ReadSeeker) (uint32, error) {
	sig := [7]byte{}
	if _, err := r.Read(sig[:]); err != nil {
		return 0, fmt.Errorf("pack: parseHeader: signature: %w", err)
	}

	if !bytes.Equal(sig[:], Signature) {
		return 0, fmt.Errorf("pack: parseHeader: invalid signature")
	}

	if _, err := r.Seek(-8, io.SeekEnd); err != nil {
		return 0, fmt.Errorf("pack: parseHeader: numRecordsPointer: %w", err)
	}

	var numRecordsPointer uint32
	if err := binary.Read(r, order, &numRecordsPointer); err != nil {
		return 0, fmt.Errorf("pack: parseHeader: numRecordsPointer: %w", err)
	}

	return numRecordsPointer, nil
}

func readRecords(r internal.ReadSeekerAt, size uint32) ([]*Record, error) {
	errs := []error{}
	records := make([]*Record, 0, size)

	for i := 0; i < int(size); i++ {
		record, err := readRecord(r)
		if err != nil {
			errs = append(errs, err)
		} else {
			records = append(records, record)
		}
	}

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("pack: readRecords: %w", err)
	}

	for _, record := range records {
		size := record.OriginalSize
		if record.IsCompressed {
			size = record.CompressedSize
		}
		record.r = io.NewSectionReader(r, int64(record.dataPointer), int64(size))
	}

	if len(errs) > 0 {
		return records, errors.Join(errs...)
	}

	return records, nil
}

// Uses the provided io.Reader to parse a pack file per the specification found here: https://docs.lu-dev.net/en/latest/file-structures/pack.html
//
// If any errors occur while parsing the signature, numRecords pointer, or numRecords, this function is guaranteed to return a nil *Pack. Otherwise,
// a valid reference to *Pack is returned.
//
// If an error occurs while parsing ANY individual record, that error is of the type *RecordError and all *RecordError errors are returned together
// through a call to errors.join(errs...) along with a list of valid records within the returned *Pack. In this way, if a group of *RecordError's are
// returned, the caller can use errors.Is(err, &RecordError{}) to verify that there MAY be some valid records that can be used. However, this feature
// should ONLY be used for diagnostic purposes, and it is encouraged to always discard a *Pack if ANY errors are returned even if there may still
// be records available.
func Read(r io.Reader) (*Pack, error) {
	readSeeker, ok := r.(internal.ReadSeekerAt)
	if !ok {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("pack: %w", err)
		}

		readSeeker = bytes.NewReader(data)
	}

	numRecordsPointer, err := parseHeader(readSeeker)
	if err != nil {
		return nil, err
	}

	if _, err := readSeeker.Seek(int64(numRecordsPointer), io.SeekStart); err != nil {
		return nil, fmt.Errorf("pack: numRecords: %w", err)
	}

	var numRecords uint32
	if err := binary.Read(readSeeker, order, &numRecords); err != nil {
		return nil, fmt.Errorf("pack: numRecords: %w", err)
	}

	pack := &Pack{}
	if closer, ok := r.(io.Closer); ok {
		pack.closer = closer
	}

	records, err := readRecords(readSeeker, numRecords)
	pack.records = records

	return pack, err
}

// Opens a file with the provided name and returns the resulting *Pack from Read. If Read returns any error, this function
// closes the opened file before passing the error back to the caller. If no error is returned from Read, the file remains
// opened and must be closed by the caller.
func Open(path string) (*Pack, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("pack: open reader: %w", err)
	}

	file.Seek(0, io.SeekStart)

	reader, err := Read(file)
	if err != nil {
		file.Close()
		return nil, err
	}

	return reader, nil
}
