package archive

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

	"github.com/I-Am-Dench/goverbuild/archive/internal/binarytree"
	"github.com/I-Am-Dench/goverbuild/compress/segmented"
)

var (
	packSignature = append([]byte("ndpk"), 0x01, 0xff, 0x00)
	packDivider   = []byte{0xff, 0x00, 0x00, 0xdd, 0x00}
)

type PackRecord struct {
	r io.ReaderAt

	Crc uint32
	binarytree.Indices

	Info
	IsCompressed bool

	dataPointer uint32
}

// Returns an [io.Reader] for the record's uncompressed data.
// If the record is compressed, Section returns an error if it fails
// to create an sd0 decompressor.
func (r PackRecord) Section() (io.Reader, error) {
	reader := r.Raw()

	if r.IsCompressed {
		sd0, err := segmented.NewDataReader(reader)
		if err != nil {
			return nil, fmt.Errorf("section: %v", err)
		}
		reader = sd0
	}

	return reader, nil
}

// Returns [*PackRecord.Section] along with an [md5] hash tee'd
// from the returned [io.Reader]. Therefore, the returned [hash.Hash]
// will contain the data's uncompressed checksum once all data has
// been read out of the reader.
//
// Example:
//
//	reader, hash, err := r.SectionWithHash()
//	if err != nil {
//	    // ...
//	}
//
//	if _, err := io.Copy(io.Discard, reader); err != nil {
//	    // ...
//	}
//
//	fmt.Println(hash.Sum(nil)) // The final hash
func (r PackRecord) SectionWithHash() (io.Reader, hash.Hash, error) {
	reader, err := r.Section()
	if err != nil {
		return nil, nil, err
	}

	hash := md5.New()
	reader = io.TeeReader(reader, hash)
	return reader, hash, nil
}

// Returns an [io.Reader] for the record's uncompressed or compressed data.
func (r PackRecord) Raw() io.Reader {
	return io.NewSectionReader(r.r, int64(r.dataPointer), int64(r.DataSize()))
}

func (r PackRecord) DataSize() uint32 {
	if r.IsCompressed {
		return r.CompressedSize
	} else {
		return r.UncompressedSize
	}
}

func (r PackRecord) appendChecksum(buf, checksum []byte) []byte {
	data := [36]byte{}
	hex.Encode(data[:32], checksum)
	return append(buf, data[:]...)
}

func (r PackRecord) AppendBinary(b []byte) ([]byte, error) {
	const recordSize = 100

	buf := make([]byte, 0, recordSize)
	buf = order.AppendUint32(buf, r.Crc)
	buf = order.AppendUint32(buf, uint32(r.LowerIndex))
	buf = order.AppendUint32(buf, uint32(r.UpperIndex))

	buf = order.AppendUint32(buf, r.UncompressedSize)
	buf = r.appendChecksum(buf, r.UncompressedChecksum)
	buf = order.AppendUint32(buf, r.CompressedSize)
	buf = r.appendChecksum(buf, r.CompressedChecksum)

	buf = order.AppendUint32(buf, r.dataPointer)

	if r.IsCompressed {
		buf = append(buf, 1, 0, 0, 0)
	} else {
		buf = append(buf, 0, 0, 0, 0)
	}

	return append(b, buf...), nil
}

func (r PackRecord) MarshalBinary() (data []byte, err error) {
	return r.AppendBinary([]byte{})
}

func (r PackRecord) DataPointer() uint32 {
	return r.dataPointer
}

func (r *PackRecord) TreeIndices() *binarytree.Indices {
	return &r.Indices
}

// File type: [pk]
//
// [pk]: https://docs.lu-dev.net/en/latest/file-structures/pack.html
type Pack struct {
	f      *os.File
	dirty  bool
	closer bool

	records []*PackRecord

	numRecordsPointer uint32
	revision          uint32
}

// Returns a slice of [*PackRecord] for [*Pack], p.
//
// Records should never return a nil slice.
func (p *Pack) Records() []*PackRecord {
	if len(p.records) == 0 {
		_, err := p.ReadRecords()
		if err != nil {
			p.records = []*PackRecord{}
		}
	}
	return p.records
}

func (p Pack) Revision() uint32 {
	return p.revision
}

func (p *Pack) Search(path string) (*PackRecord, bool) {
	crc := GetCrc(path)

	records := p.Records()
	i := sort.Search(len(records), func(i int) bool { return records[i].Crc >= crc })
	if i < len(records) && records[i].Crc == crc {
		return records[i], true
	}
	return nil, false
}

// Updates record i's internal state, replaces record i's written data with data after i,
// and then appends r's data.
func (p *Pack) updateRecord(records []*PackRecord, index int, info Info, compressed bool, r io.Reader) error {
	modified := records[index]
	modified.Info = info
	modified.IsCompressed = compressed

	// Sorting by data pointer for sequential writes
	slices.SortFunc(records, func(a, b *PackRecord) int { return int(a.dataPointer) - int(b.dataPointer) })

	if _, err := p.f.Seek(int64(modified.dataPointer), io.SeekStart); err != nil {
		return err
	}

	n := int64(modified.dataPointer)

	modifiedIndex := sort.Search(len(records), func(i int) bool { return records[i].dataPointer >= modified.dataPointer })
	for i := modifiedIndex + 1; i < len(records); i++ {
		record := records[i]

		newDataPointer := uint32(n)

		section := io.NewSectionReader(p.f, int64(record.dataPointer), int64(record.DataSize()))
		if written, err := io.Copy(p.f, section); err != nil {
			return err
		} else {
			n += written
		}

		if written, err := p.f.Write(packDivider); err != nil {
			return err
		} else {
			n += int64(written)
		}

		record.dataPointer = newDataPointer
	}

	modified.dataPointer = uint32(n)
	if written, err := io.Copy(p.f, r); err != nil {
		return err
	} else {
		n += written
	}

	if written, err := p.f.Write(packDivider); err != nil {
		return err
	} else {
		n += int64(written)
	}

	p.numRecordsPointer = uint32(n)

	return nil
}

func (p Pack) writeRecords(w io.Writer, records []*PackRecord) (n int64, err error) {
	if err := binary.Write(w, order, uint32(len(records))); err != nil {
		return 0, err
	}
	n += 4

	for _, record := range records {
		data, _ := record.MarshalBinary()

		if written, err := w.Write(data); err != nil {
			return n, fmt.Errorf("write record: %v", err)
		} else {
			n += int64(written)
		}
	}

	return n, nil
}

func (p *Pack) flush(w io.Writer, recordsPointer uint32) (n int64, err error) {
	p.numRecordsPointer = recordsPointer

	if written, err := p.writeRecords(w, p.Records()); err != nil {
		return 0, err
	} else {
		n += written
	}

	if err := binary.Write(w, order, p.numRecordsPointer); err != nil {
		return n, err
	}
	n += 4

	if err := binary.Write(w, order, p.revision); err != nil {
		return n, err
	}
	n += 4

	return n, nil
}

// Writes the list of records and trailer data to the end of the underlying [*os.File].
//
// If the written data is less than the total size of the file,
// the file is truncated to the number of written bytes.
func (p *Pack) Flush() error {
	if !p.dirty {
		return nil
	}

	if _, err := p.f.Seek(int64(p.numRecordsPointer), io.SeekStart); err != nil {
		return fmt.Errorf("flush: %v", err)
	}

	if _, err := p.flush(p.f, p.numRecordsPointer); err != nil {
		return fmt.Errorf("flush: %v", err)
	}
	p.dirty = false

	written, err := p.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("flush: %v", err)
	}

	stat, err := p.f.Stat()
	if err != nil {
		return fmt.Errorf("flush: %v", err)
	}

	if written < stat.Size() {
		if err := p.f.Truncate(written); err != nil {
			return fmt.Errorf("flush: %v", err)
		}
	}

	return nil
}

// Adds or updates a record to the [*Pack].
//
// One or more calls to Store should be followed by
// a call to either [*Pack.Flush] or [*Pack.Close] to write the record list
// and trailer data to the underlying [*os.File].
func (p *Pack) Store(path string, info Info, compressed bool, r io.Reader) (err error) {
	defer func() {
		binarytree.UpdateIndices(p.records)
	}()

	// Records must be read BEFORE marking the pack as dirty
	// since ReadRecords will contain the current state of Pack.records
	// if it's dirty, which may be empty if the first method called on
	// Pack is Store.
	records, err := p.ReadRecords()
	if err != nil {
		return fmt.Errorf("store: %v", err)
	}

	if !p.dirty {
		p.revision++
	}
	p.dirty = true

	crc := GetCrc(path)

	i := sort.Search(len(records), func(i int) bool { return records[i].Crc >= crc })
	if i < len(records) && records[i].Crc == crc {
		if err := p.updateRecord(slices.Clone(records), i, info, compressed, r); err != nil {
			return fmt.Errorf("store: %v", err)
		}
		return nil
	}

	record := &PackRecord{
		r: p.f,

		Crc: crc,

		Info:         info,
		IsCompressed: compressed,

		dataPointer: p.numRecordsPointer,
	}

	if i >= len(records) {
		records = append(records, record)
	} else {
		records = slices.Insert(records, i, record)
	}

	p.records = records

	if _, err := p.f.Seek(int64(p.numRecordsPointer), io.SeekStart); err != nil {
		return fmt.Errorf("store: %v", err)
	}

	if written, err := io.Copy(p.f, r); err != nil {
		return fmt.Errorf("store: %v", err)
	} else {
		p.numRecordsPointer += uint32(written)
	}

	if written, err := p.f.Write(packDivider); err != nil {
		return fmt.Errorf("store: %v", err)
	} else {
		p.numRecordsPointer += uint32(written)
	}

	return nil
}

func (p *Pack) WriteTo(w io.Writer) (n int64, err error) {
	if written, err := w.Write(packSignature); err != nil {
		return 0, fmt.Errorf("write to: %v", err)
	} else {
		n += int64(written)
	}

	records := slices.Clone(p.Records())
	slices.SortFunc(records, func(a, b *PackRecord) int { return int(a.dataPointer) - int(b.dataPointer) })

	for _, record := range records {
		if written, err := io.Copy(w, record.Raw()); err != nil {
			return 0, fmt.Errorf("write to: %v", err)
		} else {
			n += written
		}

		if written, err := w.Write(packDivider); err != nil {
			return n, fmt.Errorf("write to: %v", err)
		} else {
			n += int64(written)
		}
	}

	if written, err := p.flush(w, uint32(n)); err != nil {
		return n, fmt.Errorf("write to: %v", err)
	} else {
		n += written
	}

	return n, nil
}

func (p Pack) readHash(r io.Reader) ([]byte, error) {
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

func (p *Pack) readRecord(r io.Reader) (*PackRecord, error) {
	data := [100]byte{}
	if _, err := r.Read(data[:]); err != nil {
		return nil, fmt.Errorf("read record: %v", err)
	}

	record := &PackRecord{}

	buf := bytes.NewBuffer(data[:])
	binary.Read(buf, order, &record.Crc)
	binary.Read(buf, order, &record.LowerIndex)
	binary.Read(buf, order, &record.UpperIndex)

	binary.Read(buf, order, &record.UncompressedSize)

	uncompressedChecksum, err := p.readHash(buf)
	if err != nil {
		return nil, fmt.Errorf("read record: uncompressed checksum: %v", err)
	}
	record.UncompressedChecksum = uncompressedChecksum

	binary.Read(buf, order, &record.CompressedSize)

	compressedChecksum, err := p.readHash(buf)
	if err != nil {
		return nil, fmt.Errorf("read record: compressed checksum: %v", err)
	}
	record.CompressedChecksum = compressedChecksum

	binary.Read(buf, order, &record.dataPointer)

	boolData := [4]byte{}
	if _, err := buf.Read(boolData[:]); err != nil {
		return nil, fmt.Errorf("read records: is compressed")
	}

	record.IsCompressed = boolData[0] != 0
	record.r = p.f

	return record, nil
}

// Reads the record data from the underlying [*os.File]
// and returns the resulting slice.
func (p *Pack) ReadRecords() ([]*PackRecord, error) {
	if p.dirty {
		return p.records, nil
	}

	if _, err := p.f.Seek(int64(p.numRecordsPointer), io.SeekStart); err != nil {
		return nil, fmt.Errorf("read records: %v", err)
	}

	var numRecords uint32
	if err := binary.Read(p.f, order, &numRecords); err != nil {
		return nil, fmt.Errorf("read records: %v", err)
	}

	p.records = make([]*PackRecord, 0, numRecords)
	for i := 0; i < int(numRecords); i++ {
		record, err := p.readRecord(p.f)
		if err != nil {
			return nil, err
		}

		p.records = append(p.records, record)
	}

	return p.records, nil
}

func readHeaderErr(err error, format string) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return fmt.Errorf(format, err)
}

func (p *Pack) readHeader() error {
	sig := [7]byte{}
	if _, err := p.f.Read(sig[:]); err != nil {
		return readHeaderErr(err, "header: signature: %w")
	}

	if !bytes.Equal(sig[:], packSignature) {
		return fmt.Errorf("header: invalid signature")
	}

	if _, err := p.f.Seek(-8, io.SeekEnd); err != nil {
		return fmt.Errorf("header: %v", err)
	}

	if err := binary.Read(p.f, order, &p.numRecordsPointer); err != nil {
		return readHeaderErr(err, "header: numRecordsPointer: %v")
	}

	if err := binary.Read(p.f, order, &p.revision); err != nil {
		return readHeaderErr(err, "header: revision: %v")
	}

	return nil
}

func (p *Pack) init() error {
	if _, err := p.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("init: %v", err)
	}

	if _, err := p.f.Write(packSignature); err != nil {
		return fmt.Errorf("init: %v", err)
	}

	p.dirty = true
	p.numRecordsPointer = uint32(len(packSignature))
	return nil
}

// Flushes the contents of the [*Pack], and then closes
// the underlying [*os.File] ONLY if the [*Pack] was created
// through a call to OpenPack.
func (p *Pack) Close() (err error) {
	err = p.Flush()
	if p.closer {
		if e := p.f.Close(); err == nil {
			err = e
		}
	}

	return err
}

// Creates a [*Pack] with the provided [*os.File].
//
// If NewPack fails to verify the file signature,
// [*Pack] will write the signature to the beginning of the file.
func NewPack(file *os.File) (*Pack, error) {
	pack := &Pack{
		f: file,
	}

	if err := pack.readHeader(); errors.Is(err, io.ErrUnexpectedEOF) {
		if err := pack.init(); err != nil {
			return nil, fmt.Errorf("pack: %v", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("pack: %v", err)
	}

	return pack, nil
}

// Creates a [*Pack] with the named [*os.File].
//
// If OpenPack fails to initialize the [*Pack],
// the file is closed.
//
// Calling [*Pack.Close] on a [*Pack] created through a call
// from OpenPack causes the underlying file to be closed.
func OpenPack(path string) (*Pack, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0664)
	if err != nil {
		return nil, fmt.Errorf("pack: open: %w", err)
	}

	pack, err := NewPack(file)
	if err != nil {
		file.Close()
		return nil, err
	}
	pack.closer = true

	return pack, nil
}
