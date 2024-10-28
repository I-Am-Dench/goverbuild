package catalog

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/internal/binarytree"
)

type Record struct {
	PackName string

	Crc uint32
	binarytree.Indices

	IsCompressed bool
}

func (record *Record) TreeIndices() *binarytree.Indices {
	return &record.Indices
}

type Catalog struct {
	Version int32

	PackNames []string
	Records   []*Record
}

func (catalog *Catalog) Search(path string) (*Record, bool) {
	crc := archive.GetCrc(path)

	i := sort.Search(len(catalog.Records), func(i int) bool { return catalog.Records[i].Crc >= crc })
	if i < len(catalog.Records) && catalog.Records[i].Crc == crc {
		return catalog.Records[i], true
	}
	return nil, false
}

func (catalog *Catalog) writePackNames(w io.Writer) (indices map[string]int, n int64, err error) {
	if err := binary.Write(w, order, uint32(len(catalog.PackNames))); err != nil {
		return nil, 0, fmt.Errorf("pack names: %w", err)
	}
	n += 4

	indices = map[string]int{}
	for i, name := range catalog.PackNames {
		if err := binary.Write(w, order, uint32(len(name))); err != nil {
			return nil, n, fmt.Errorf("pack names: %w", err)
		}
		n += 4

		if written, err := w.Write([]byte(name)); err != nil {
			return nil, n, fmt.Errorf("pack names: %w", err)
		} else {
			n += int64(written)
		}

		indices[name] = i
	}

	return
}

func (catalog *Catalog) writeRecords(w io.Writer, indices map[string]int) (n int64, err error) {
	records := catalog.Records

	slices.SortFunc(records, func(a, b *Record) int { return int(a.Crc) - int(b.Crc) })
	binarytree.UpdateIndices(records)

	if err := binary.Write(w, order, uint32(len(records))); err != nil {
		return 0, fmt.Errorf("records: %w", err)
	}

	for _, record := range records {
		if err := binary.Write(w, order, record.Crc); err != nil {
			return n, fmt.Errorf("record: %w", err)
		}
		n += 4

		if err := binary.Write(w, order, record.LowerIndex); err != nil {
			return n, fmt.Errorf("record: %w", err)
		}
		n += 4

		if err := binary.Write(w, order, record.UpperIndex); err != nil {
			return n, fmt.Errorf("record: %w", err)
		}
		n += 4

		if err := binary.Write(w, order, uint32(indices[record.PackName])); err != nil {
			return n, fmt.Errorf("record: %w", err)
		}
		n += 4

		compression := [4]byte{}
		if record.IsCompressed {
			compression[0] = 1
		}

		if _, err := w.Write(compression[:]); err != nil {
			return n, fmt.Errorf("record: %w", err)
		}
		n += 4
	}

	return
}

func (catalog *Catalog) WriteTo(w io.Writer) (n int64, err error) {
	if err := binary.Write(w, order, catalog.Version); err != nil {
		return 0, fmt.Errorf("catalog: write to: %w", err)
	}

	indices, written, err := catalog.writePackNames(w)
	if err != nil {
		return written, fmt.Errorf("catalog: write to: %w", err)
	}
	n += written

	if written, err := catalog.writeRecords(w, indices); err != nil {
		return n, fmt.Errorf("catalog: write to: %w", err)
	} else {
		n += written
	}

	return
}

func New(files map[string][]string) *Catalog {
	records := []*Record{}
	packNames := []string{}
	for packName, files := range files {
		packNames = append(packNames, packName)
		for _, file := range files {
			records = append(records, &Record{
				PackName: packName,
				Crc:      archive.GetCrc(file),
			})
		}
	}
	slices.SortFunc(records, func(a, b *Record) int { return int(a.Crc) - int(b.Crc) })
	binarytree.UpdateIndices(records)

	return &Catalog{
		Version:   CatalogVersion,
		Records:   records,
		PackNames: packNames,
	}
}

func readPackNames(r io.Reader) ([]string, error) {
	var numFiles uint32
	if err := binary.Read(r, order, &numFiles); err != nil {
		return nil, fmt.Errorf("catalog: read from: packNames: %w", err)
	}

	names := make([]string, 0, numFiles)
	for i := 0; i < int(numFiles); i++ {
		var size uint32
		if err := binary.Read(r, order, &size); err != nil {
			return nil, fmt.Errorf("catalog: read from: packNames: %w", err)
		}

		data := make([]byte, size)
		if _, err := r.Read(data); err != nil {
			return nil, fmt.Errorf("catalog: read from: packNames: %w", err)
		}

		names = append(names, string(data))
	}

	return names, nil
}

func readRecord(r io.Reader, packNames []string) (*Record, error) {
	record := &Record{}
	if err := binary.Read(r, order, &record.Crc); err != nil {
		return nil, &RecordError{err, "crc"}
	}

	if err := binary.Read(r, order, &record.LowerIndex); err != nil {
		return nil, &RecordError{err, "lower index"}
	}

	if err := binary.Read(r, order, &record.UpperIndex); err != nil {
		return nil, &RecordError{err, "upper index"}
	}

	var packNameIndex uint32
	if err := binary.Read(r, order, &packNameIndex); err != nil {
		return nil, &RecordError{err, "pack name index"}
	}

	if packNameIndex >= uint32(len(packNames)) {
		return nil, &RecordError{fmt.Errorf("index out of bounds %d", packNameIndex), "pack name"}
	}

	record.PackName = packNames[packNameIndex]

	compression := [4]byte{}
	if err := binary.Read(r, order, compression[:]); err != nil {
		return nil, &RecordError{err, "compression"}
	}
	record.IsCompressed = compression[0] != 0

	return record, nil
}

func readRecords(r io.Reader, packNames []string) ([]*Record, error) {
	var numRecords uint32
	if err := binary.Read(r, order, &numRecords); err != nil {
		return nil, fmt.Errorf("catalog: read from: records: %w", err)
	}

	records := make([]*Record, 0, numRecords)
	for i := 0; i < int(numRecords); i++ {
		record, err := readRecord(r, packNames)
		if err == nil {
			records = append(records, record)
		} else {
			return nil, fmt.Errorf("catalog: read from: %w", err)
		}
	}

	return records, nil
}

func ReadFrom(r io.Reader) (*Catalog, error) {
	catalog := &Catalog{}
	if err := binary.Read(r, order, &catalog.Version); err != nil {
		return nil, fmt.Errorf("catalog: read from: %w", err)
	}

	if catalog.Version != CatalogVersion {
		return nil, fmt.Errorf("catalog: read from: unsupported version: %d", catalog.Version)
	}

	names, err := readPackNames(r)
	if err != nil {
		return nil, err
	}

	records, err := readRecords(r, names)
	if err != nil {
		return nil, err
	}

	catalog.PackNames = names
	catalog.Records = records

	return catalog, nil
}

func ReadFile(name string) (*Catalog, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("catalog: read: %w", err)
	}
	defer file.Close()

	return ReadFrom(file)
}

func WriteFile(name string, catalog *Catalog) error {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("catalog: write: %w", err)
	}
	defer file.Close()

	_, err = catalog.WriteTo(file)
	return err
}
