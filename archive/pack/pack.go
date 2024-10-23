package pack

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"slices"
	"sort"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/internal/binarytree"
	"github.com/I-Am-Dench/goverbuild/compress/segmented"
)

type Record struct {
	r io.ReaderAt

	Crc uint32
	binarytree.Indices

	archive.Info
	IsCompressed bool

	dataPointer uint32
}

// Returns an io.Reader and md5 hash writer for the record's data. If the record is compressed,
// the underlying io.Reader is wrapped by a *segmented.DataReader.
//
// The hash.Hash value contains the md5 chunksum for the uncompressed data, but only for
// the data read out of the io.Reader.
//
// Example:
//
//	reader, hash, err := record.Section()
//
//	if err != nil {
//	    return err
//	}
//
//	// The contents of the reader must be read before calling hash.Sum(nil)
//	if err := io.Copy(file, reader); err != nil {
//	    return err
//	}
//
//	fmt.Println(hash.Sum(nil)) // the resulting checksum
func (record *Record) Section() (io.Reader, hash.Hash, error) {
	size := record.UncompressedSize
	if record.IsCompressed {
		size = record.CompressedSize
	}
	reader := io.Reader(io.NewSectionReader(record.r, int64(record.dataPointer), int64(size)))

	if record.IsCompressed {
		// sd0, err := segmented.NewDataReader(reader, record.CompressedSize)
		sd0, err := segmented.NewDataReader(reader)
		if err != nil {
			return nil, nil, fmt.Errorf("pack: record: section: %w", err)
		}
		reader = sd0
	}

	hash := md5.New()
	reader = io.TeeReader(reader, hash)

	return reader, hash, nil
}

func (record *Record) DataSize() uint32 {
	if record.IsCompressed {
		return record.CompressedSize
	} else {
		return record.UncompressedSize
	}
}

func (record *Record) writeChecksum(w io.Writer, checksum []byte) error {
	buf := [36]byte{}
	hex.Encode(buf[:32], checksum)

	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	return nil
}

func (record *Record) WriteTo(w io.Writer) (n int64, err error) {
	if err := errors.Join(
		binary.Write(w, order, record.Crc),
		binary.Write(w, order, record.LowerIndex),
		binary.Write(w, order, record.UpperIndex),

		binary.Write(w, order, record.UncompressedSize),
		record.writeChecksum(w, record.UncompressedChecksum),
		binary.Write(w, order, record.CompressedSize),
		record.writeChecksum(w, record.CompressedChecksum),

		binary.Write(w, order, record.dataPointer),

		binary.Write(w, order, record.IsCompressed),
		binary.Write(w, order, []byte{0, 0, 0}), // padding for IsCompressed
	); err != nil {
		return 0, fmt.Errorf("record: write to: %w", err)
	}

	return 100, nil
}

func (record *Record) DataPointer() uint32 {
	return record.dataPointer
}

func (record *Record) TreeIndices() *binarytree.Indices {
	return &record.Indices
}

type Pack struct {
	f      *os.File
	dirty  bool
	closer bool

	records []*Record

	numRecordsPointer uint32
	revision          uint32
}

func (pack *Pack) Records() []*Record {
	if pack.records == nil {
		_, err := pack.ReadRecords()
		if err != nil {
			pack.records = []*Record{}
		}
	}

	return pack.records
}

func (pack *Pack) Revision() uint32 {
	return pack.revision
}

func (pack *Pack) Search(path string) (*Record, bool) {
	crc := archive.GetCrc(path)

	records := pack.Records()
	i := sort.Search(len(records), func(i int) bool { return records[i].Crc >= crc })
	if i < len(records) && records[i].Crc == crc {
		return records[i], true
	}
	return nil, false
}

func (pack *Pack) updateRecordsTree() {
	binarytree.UpdateIndices(pack.Records())
}

// Updates record i's internal state, replaces record i's written data with data after i,
// and then appends r's data.
func (pack *Pack) updateRecord(records []*Record, index int, info archive.Info, compressed bool, r io.Reader) error {
	modified := records[index]
	modified.Info = info
	modified.IsCompressed = compressed

	// Sorting by data pointer for sequential writes
	slices.SortFunc(records, func(a, b *Record) int { return int(a.dataPointer) - int(b.dataPointer) })

	if _, err := pack.f.Seek(int64(modified.dataPointer), io.SeekStart); err != nil {
		return err
	}

	n := int64(modified.dataPointer)

	modifiedIndex := sort.Search(len(records), func(i int) bool { return records[i].dataPointer >= modified.dataPointer })
	for i := modifiedIndex + 1; i < len(records); i++ {
		record := records[i]

		newDataPointer := uint32(n)

		section := io.NewSectionReader(pack.f, int64(record.dataPointer), int64(record.DataSize()))
		if written, err := io.Copy(pack.f, section); err != nil {
			return err
		} else {
			n += written
		}

		if written, err := pack.f.Write(Terminator); err != nil {
			return err
		} else {
			n += int64(written)
		}

		record.dataPointer = newDataPointer
	}

	modified.dataPointer = uint32(n)
	if written, err := io.Copy(pack.f, r); err != nil {
		return err
	} else {
		n += written
	}

	if written, err := pack.f.Write(Terminator); err != nil {
		return err
	} else {
		n += int64(written)
	}

	pack.numRecordsPointer = uint32(n)

	return nil
}

func (pack *Pack) Store(path string, info archive.Info, compressed bool, r io.Reader) (err error) {
	defer pack.updateRecordsTree()

	if !pack.dirty {
		pack.revision++
	}
	pack.dirty = true

	crc := archive.GetCrc(path)

	records := pack.Records()
	i := sort.Search(len(records), func(i int) bool { return records[i].Crc >= crc })
	if i < len(records) && records[i].Crc == crc {
		if err := pack.updateRecord(slices.Clone(records), i, info, compressed, r); err != nil {
			return fmt.Errorf("pack: store: %w", err)
		}
		return nil
	}

	record := &Record{
		r: pack.f,

		Crc: crc,

		Info:         info,
		IsCompressed: compressed,

		dataPointer: pack.numRecordsPointer,
	}

	if i >= len(records) {
		records = append(records, record)
	} else {
		records = slices.Insert(records, i, record)
	}

	pack.records = records

	if _, err := pack.f.Seek(int64(pack.numRecordsPointer), io.SeekStart); err != nil {
		return fmt.Errorf("pack: store: %w", err)
	}

	if written, err := io.Copy(pack.f, r); err != nil {
		return fmt.Errorf("pack: store: %w", err)
	} else {
		pack.numRecordsPointer += uint32(written)
	}

	if written, err := pack.f.Write(Terminator); err != nil {
		return fmt.Errorf("pack: store: %w", err)
	} else {
		pack.numRecordsPointer += uint32(written)
	}

	return nil
}

func (pack *Pack) writeRecords(w io.Writer, records []*Record) (n int64, err error) {
	if err := binary.Write(w, order, uint32(len(records))); err != nil {
		return 0, err
	}
	n += 4

	for _, record := range records {
		if written, err := record.WriteTo(w); err != nil {
			return n, err
		} else {
			n += written
		}
	}
	return
}

func (pack *Pack) flush(w io.Writer, recordsPointer uint32) (n int64, err error) {
	pack.numRecordsPointer = recordsPointer

	if written, err := pack.writeRecords(w, pack.Records()); err != nil {
		return 0, err
	} else {
		n += written
	}

	if err := binary.Write(w, order, pack.numRecordsPointer); err != nil {
		return 0, err
	}
	n += 4

	if err := binary.Write(w, order, pack.revision); err != nil {
		return n, err
	}
	n += 4

	return
}

func (pack *Pack) Flush() error {
	if pack.dirty {
		if _, err := pack.f.Seek(int64(pack.numRecordsPointer), io.SeekStart); err != nil {
			return err
		}

		if _, err := pack.flush(pack.f, pack.numRecordsPointer); err != nil {
			return fmt.Errorf("pack: flush: %w", err)
		}
		pack.dirty = false
	}
	return nil
}

func (pack *Pack) writeRecordData(w io.Writer, record *Record) (n int64, err error) {
	size := record.UncompressedSize
	if record.IsCompressed {
		size = record.CompressedSize
	}

	section := io.NewSectionReader(record.r, int64(record.dataPointer), int64(size))
	if written, err := io.Copy(w, section); err != nil {
		return 0, err
	} else {
		n += written
	}

	if written, err := w.Write(Terminator); err != nil {
		return 0, err
	} else {
		n += int64(written)
	}

	return
}

func (pack *Pack) WriteTo(w io.Writer) (n int64, err error) {
	if written, err := w.Write(Signature); err != nil {
		return n, fmt.Errorf("pack: write to: %w", err)
	} else {
		n += int64(written)
	}

	records := slices.Clone(pack.Records())
	slices.SortFunc(records, func(a, b *Record) int { return int(a.dataPointer) - int(b.dataPointer) })

	for _, record := range records {
		if written, err := pack.writeRecordData(w, record); err != nil {
			return n, fmt.Errorf("pack: write to: %w", err)
		} else {
			n += written
		}
	}

	if written, err := pack.flush(w, uint32(n)); err != nil {
		return n, fmt.Errorf("pack: write to: %w", err)
	} else {
		n += written
	}

	return
}

func (pack *Pack) readHash(r io.Reader) ([]byte, error) {
	buf := [36]byte{}
	if _, err := r.Read(buf[:]); err != nil {
		return nil, err
	}

	hash, err := hex.DecodeString(string(buf[:len(buf)-4]))
	if err != nil {
		return nil, err
	}

	return hash, nil
}

func (pack *Pack) readRecord(r io.Reader) (*Record, error) {
	record := &Record{}

	if err := binary.Read(r, order, &record.Crc); err != nil {
		return nil, &RecordError{err, "index"}
	}

	if err := binary.Read(r, order, &record.LowerIndex); err != nil {
		return nil, &RecordError{err, "lower index"}
	}

	if err := binary.Read(r, order, &record.UpperIndex); err != nil {
		return nil, &RecordError{err, "upper index"}
	}

	if err := binary.Read(r, order, &record.UncompressedSize); err != nil {
		return nil, &RecordError{err, "uncompressed size"}
	}

	uchecksum, err := pack.readHash(r)
	if err != nil {
		return nil, &RecordError{err, "uncompressed checksum"}
	}
	record.UncompressedChecksum = uchecksum

	if err := binary.Read(r, order, &record.CompressedSize); err != nil {
		return nil, &RecordError{err, "compressed size"}
	}

	cchecksum, err := pack.readHash(r)
	if err != nil {
		return nil, &RecordError{err, "compressed checksum"}
	}
	record.CompressedChecksum = cchecksum

	if err := binary.Read(r, order, &record.dataPointer); err != nil {
		return nil, &RecordError{err, "data pointer"}
	}

	boolData := [4]byte{}
	if _, err := r.Read(boolData[:]); err != nil {
		return nil, &RecordError{err, "is compressed"}
	}

	record.IsCompressed = boolData[0] != 0
	record.r = pack.f

	return record, nil
}

func (pack *Pack) ReadRecords() ([]*Record, error) {
	if pack.dirty {
		return pack.records, nil
	}

	if _, err := pack.f.Seek(int64(pack.numRecordsPointer), io.SeekStart); err != nil {
		return nil, fmt.Errorf("pack: read records: %w", err)
	}

	var numRecords uint32
	if err := binary.Read(pack.f, order, &numRecords); err != nil {
		return nil, fmt.Errorf("pack: read records: %w", err)
	}

	pack.records = make([]*Record, 0, numRecords)
	for i := 0; i < int(numRecords); i++ {
		record, err := pack.readRecord(pack.f)
		if err != nil {
			return nil, err
		}

		pack.records = append(pack.records, record)
	}

	return pack.records, nil
}

func (pack *Pack) parseHeader(r io.ReadSeeker) (err error) {
	sig := [7]byte{}
	if _, err := r.Read(sig[:]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return fmt.Errorf("pack: header: signature: %w", err)
	}

	if !bytes.Equal(sig[:], Signature) {
		return fmt.Errorf("pack: header: invalid signature")
	}

	if _, err := r.Seek(-8, io.SeekEnd); err != nil {
		return fmt.Errorf("pack: header: %w", err)
	}

	if err := binary.Read(r, order, &pack.numRecordsPointer); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return fmt.Errorf("pack: header: numRecordsPointer: %w", err)
	}

	if err := binary.Read(r, order, &pack.revision); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return fmt.Errorf("pack: header: revision: %w", err)
	}

	return nil
}

func (pack *Pack) Close() (err error) {
	defer func() {
		if pack.closer {
			if e := pack.f.Close(); err == nil {
				err = e
			}
		}
	}()

	err = pack.Flush()

	return
}

func (pack *Pack) init() error {
	if _, err := pack.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("pack: init: %w", err)
	}

	if _, err := pack.f.Write(Signature); err != nil {
		return fmt.Errorf("pack: init: %w", err)
	}

	pack.dirty = true
	pack.numRecordsPointer = uint32(len(Signature))
	return nil
}

func New(file *os.File) (*Pack, error) {
	pack := &Pack{
		f: file,
	}

	err := pack.parseHeader(file)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		if err := pack.init(); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return pack, nil
}

func Open(path string) (*Pack, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0755)
	if err != nil {
		return nil, fmt.Errorf("pack: open: %w", err)
	}

	pack, err := New(file)
	if err != nil {
		file.Close()
		return nil, err
	}

	if _, err := pack.ReadRecords(); err != nil {
		file.Close()
		return nil, err
	}

	pack.closer = true

	return pack, nil
}
