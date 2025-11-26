package archive

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/I-Am-Dench/goverbuild/archive/internal/binarytree"
)

const (
	CatalogVersion = 3
)

type CatalogRecord struct {
	PackName string

	Crc uint32
	binarytree.Indices

	IsCompressed bool
}

func (r *CatalogRecord) TreeIndices() *binarytree.Indices {
	return &r.Indices
}

type CatalogEntry struct {
	Path         string
	IsCompressed bool
}

// A mapping between a pack name and a list of file names
type CatalogEntries = map[string][]CatalogEntry

// File type: [pki]
//
// [pki]: https://docs.lu-dev.net/en/latest/file-structures/catalog.html
type Catalog struct {
	f      *os.File
	closer bool
	dirty  bool

	Version int32

	packNames []string
	records   []*CatalogRecord
}

func (c *Catalog) PackNames() []string {
	return c.packNames
}

func (c *Catalog) Records() []*CatalogRecord {
	return c.records
}

func (c *Catalog) Search(path string) (*CatalogRecord, bool) {
	crc := GetCrc(path)

	i := sort.Search(len(c.records), func(i int) bool { return c.records[i].Crc >= crc })
	if i < len(c.records) && c.records[i].Crc == crc {
		return c.records[i], true
	}
	return nil, false
}

func (c *Catalog) readPackNames() ([]string, error) {
	var numFiles uint32
	if err := binary.Read(c.f, order, &numFiles); err != nil {
		return nil, fmt.Errorf("read pack names: %v", err)
	}

	names := make([]string, numFiles)
	for i := range names {
		var size uint32
		if err := binary.Read(c.f, order, &size); err != nil {
			return nil, fmt.Errorf("read pack names: %v", err)
		}

		data := make([]byte, size)
		if _, err := c.f.Read(data); err != nil {
			return nil, fmt.Errorf("read pack names: %v", err)
		}

		names[i] = filepath.ToSlash(string(data))
	}

	return names, nil
}

func (c *Catalog) readRecord(packNames []string) (*CatalogRecord, error) {
	data := [20]byte{}
	if _, err := c.f.Read(data[:]); err != nil {
		return nil, fmt.Errorf("read record: %v", err)
	}

	record := &CatalogRecord{}
	var packNameIndex uint32

	buf := bytes.NewBuffer(data[:])
	binary.Read(buf, order, &record.Crc)
	binary.Read(buf, order, &record.LowerIndex)
	binary.Read(buf, order, &record.UpperIndex)
	binary.Read(buf, order, &packNameIndex)

	if packNameIndex >= uint32(len(packNames)) {
		return nil, fmt.Errorf("read record: index out of bounds: %d", packNameIndex)
	}

	compression := [4]byte{}
	buf.Read(compression[:])

	record.PackName = packNames[packNameIndex]
	record.IsCompressed = compression[0] != 0

	return record, nil
}

func (c *Catalog) readRecords(packNames []string) ([]*CatalogRecord, error) {
	var numRecords uint32
	if err := binary.Read(c.f, order, &numRecords); err != nil {
		return nil, fmt.Errorf("read records: %v", err)
	}

	records := make([]*CatalogRecord, numRecords)
	for i := range records {
		record, err := c.readRecord(packNames)
		if err != nil {
			return nil, fmt.Errorf("read records: %v", err)
		}

		records[i] = record
	}

	return records, nil
}

func (c *Catalog) update(record *CatalogRecord) bool {
	i := sort.Search(len(c.records), func(i int) bool { return c.records[i].Crc >= record.Crc })
	if i < len(c.records) && c.records[i].Crc == record.Crc {
		c.records[i] = record
		return true
	}
	return false
}

// Adds or updates a collection of entries to the [*Catalog].
//
// One or more calls to Store should be followed by
// a call to either [*Catalog.Flush] or [*Catalog.Close] to write the list
// of pack names and the list of records to the underlying [*os.File].
//
// [CatalogEntry]'s with duplicate path names, but different pack names, DO NOT
// have a determined order in which the final pack name is chosen.
func (c *Catalog) Store(entries CatalogEntries) error {
	c.dirty = true

	records := []*CatalogRecord{}
	for packName, entries := range entries {
		if slices.Index(c.packNames, packName) < 0 {
			c.packNames = append(c.packNames, filepath.ToSlash(packName))
		}

		for _, entry := range entries {
			record := &CatalogRecord{
				PackName:     packName,
				Crc:          GetCrc(entry.Path),
				IsCompressed: entry.IsCompressed,
			}

			if !c.update(record) {
				records = append(records, record)
			}
		}
	}
	c.records = append(c.records, records...)
	slices.SortFunc(c.records, func(a, b *CatalogRecord) int { return int(a.Crc) - int(b.Crc) })
	binarytree.UpdateIndices(c.records)

	return nil
}

func (c *Catalog) writePackNames() (map[string]int, error) {
	buf := []byte{}
	buf = order.AppendUint32(buf, uint32(len(c.packNames)))

	indices := make(map[string]int)
	for i, name := range c.packNames {
		buf = order.AppendUint32(buf, uint32(len(name)))
		buf = append(buf, []byte(name)...)
		indices[name] = i
	}

	if _, err := c.f.Write(buf); err != nil {
		return nil, fmt.Errorf("pack names: %v", err)
	}

	return indices, nil
}

func (c *Catalog) writeRecord(record *CatalogRecord, indices map[string]int) error {
	const recordSize = 20

	i, ok := indices[record.PackName]
	if !ok {
		return fmt.Errorf("unknown pack name: %s", record.PackName)
	}

	buf := make([]byte, 0, recordSize)
	buf = order.AppendUint32(buf, record.Crc)
	buf = order.AppendUint32(buf, uint32(record.LowerIndex))
	buf = order.AppendUint32(buf, uint32(record.UpperIndex))
	buf = order.AppendUint32(buf, uint32(i))

	if record.IsCompressed {
		buf = append(buf, 1, 0, 0, 0)
	} else {
		buf = append(buf, 0, 0, 0, 0)
	}

	if _, err := c.f.Write(buf); err != nil {
		return err
	}

	return nil
}

func (c *Catalog) Flush() error {
	if !c.dirty {
		return nil
	}

	if _, err := c.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("flush: %v", err)
	}

	if err := binary.Write(c.f, order, c.Version); err != nil {
		return fmt.Errorf("flush: %v", err)
	}

	indices, err := c.writePackNames()
	if err != nil {
		return fmt.Errorf("flush: %v", err)
	}

	if err := binary.Write(c.f, order, uint32(len(c.records))); err != nil {
		return fmt.Errorf("flush: %v", err)
	}

	for _, record := range c.records {
		if err := c.writeRecord(record, indices); err != nil {
			return fmt.Errorf("flush: %v", err)
		}
	}
	c.dirty = false

	return nil
}

func (c *Catalog) init() error {
	emptyCatalog := []byte{3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	if _, err := c.f.Write(emptyCatalog); err != nil {
		return fmt.Errorf("init: %v", err)
	}

	c.packNames = []string{}
	c.records = []*CatalogRecord{}

	return nil
}

// Flushes the contents of the [*Catalog], and then closes
// the underlying [*os.File] ONLY if the [*Catalog] was created
// through a call to OpenCatalog.
func (c *Catalog) Close() (err error) {
	err = c.Flush()
	if c.closer {
		if e := c.f.Close(); err == nil {
			err = e
		}
	}
	return err
}

// Creates a [*Catalog] with the provided [*os.File].
//
// If NewCatalog fails to verify the file version,
// [*Catalog] will write an empty catalog to the file.
func NewCatalog(file *os.File) (*Catalog, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}

	catalog := &Catalog{
		f:       file,
		Version: CatalogVersion,
	}
	if err := binary.Read(file, order, &catalog.Version); err == io.EOF {
		if err := catalog.init(); err != nil {
			return nil, fmt.Errorf("catalog: %v", err)
		}
		return catalog, nil
	} else if err != nil {
		return nil, fmt.Errorf("catalog: %v", err)
	}

	if catalog.Version != CatalogVersion {
		return nil, fmt.Errorf("catalog: unsupported version: %d", catalog.Version)
	}

	packNames, err := catalog.readPackNames()
	if err != nil {
		return nil, fmt.Errorf("catalog: %v", err)
	}
	catalog.packNames = packNames

	records, err := catalog.readRecords(packNames)
	if err != nil {
		return nil, fmt.Errorf("catalog: %v", err)
	}
	catalog.records = records

	return catalog, nil
}

// Creates a [*Catalog] with the named [*os.File].
//
// If OpenCatalog fails to initialize the [*Catalog],
// the file is closed.
//
// Calling [*Catalog.Close] on a [*Catalog] created through a call
// from OpenCatalog causes the underlying file to be closed.
func OpenCatalog(name string) (*Catalog, error) {
	file, err := os.OpenFile(name, os.O_RDWR, 0664)
	if err != nil {
		return nil, fmt.Errorf("catalog: open: %w", err)
	}

	catalog, err := NewCatalog(file)
	if err != nil {
		file.Close()
		return nil, err
	}
	catalog.closer = true

	return catalog, nil
}
