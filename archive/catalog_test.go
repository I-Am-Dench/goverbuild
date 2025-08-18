package archive_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/internal/binarytree"
)

type TestCatalog struct {
	buf bytes.Buffer
}

func (c *TestCatalog) WriteRecord(record *archive.CatalogRecord, indices map[string]int) {
	i := indices[record.PackName]

	buf := make([]byte, 0, 20)
	buf = order.AppendUint32(buf, record.Crc)
	buf = order.AppendUint32(buf, uint32(record.LowerIndex))
	buf = order.AppendUint32(buf, uint32(record.UpperIndex))
	buf = order.AppendUint32(buf, uint32(i))

	if record.IsCompressed {
		buf = append(buf, 1, 0, 0, 0)
	} else {
		buf = append(buf, 0, 0, 0, 0)
	}

	c.buf.Write(buf)
}

func (c *TestCatalog) Generate(packNames []string, entries archive.CatalogEntries) {
	binary.Write(&c.buf, order, uint32(archive.CatalogVersion))

	indices := make(map[string]int)
	binary.Write(&c.buf, order, uint32(len(packNames)))

	for i, name := range packNames {
		binary.Write(&c.buf, order, uint32(len(name)))
		c.buf.Write([]byte(name))
		indices[name] = i
	}

	records := []*archive.CatalogRecord{}
	for packName, entries := range entries {
		for _, entry := range entries {
			records = append(records, &archive.CatalogRecord{
				PackName:     packName,
				Crc:          archive.GetCrc(entry.Path),
				IsCompressed: entry.IsCompressed,
			})
		}
	}

	slices.SortFunc(records, func(a, b *archive.CatalogRecord) int { return int(a.Crc) - int(b.Crc) })
	binarytree.UpdateIndices(records)

	binary.Write(&c.buf, order, uint32(len(records)))
	for _, record := range records {
		c.WriteRecord(record, indices)
	}
}

func (e *Env) CheckCatalog(t *testing.T, expectedEntries archive.CatalogEntries, catalog *archive.Catalog, f *os.File) {
	if err := catalog.Flush(); err != nil {
		t.Fatal(err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	expected := TestCatalog{}
	expected.Generate(catalog.PackNames(), expectedEntries)

	actual, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(expected.buf.Bytes(), actual) {
		t.Error("written data did not match")
		e.Dump(t, expected.buf.Bytes(), actual)
	}
}

func createCatalogEntries() archive.CatalogEntries {
	entries := archive.CatalogEntries{}

	numPacks := rand.Intn(5) + 5
	for i := 0; i < numPacks; i++ {
		packEntries := make([]archive.CatalogEntry, rand.Intn(10)+10)
		for j := range packEntries {
			compressed := false
			if rand.Intn(2) == 0 {
				compressed = true
			}

			packEntries[j] = archive.CatalogEntry{
				Path:         fmt.Sprint("pack", i, "/", "file", j),
				IsCompressed: compressed,
			}
		}

		entries[fmt.Sprint("pack", i, ".pk")] = packEntries
	}

	return entries
}

func testCatalogStore(env *Env) func(*testing.T) {
	return func(t *testing.T) {
		file, err := os.Create(filepath.Join(env.Dir, "store.pki"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		catalog, err := archive.NewCatalog(file)
		if err != nil {
			t.Fatal(err)
		}

		expectedEntries := createCatalogEntries()
		if err := catalog.Store(expectedEntries); err != nil {
			t.Fatal(err)
		}

		env.CheckCatalog(t, expectedEntries, catalog, file)
	}
}

func testCatalogUpdate(env *Env) func(*testing.T) {
	return func(t *testing.T) {
		file, err := os.Create(filepath.Join(env.Dir, "update.pki"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		catalog, err := archive.NewCatalog(file)
		if err != nil {
			t.Fatal(err)
		}

		expectedEntries := createCatalogEntries()
		if err := catalog.Store(expectedEntries); err != nil {
			t.Fatal(err)
		}

		env.CheckCatalog(t, expectedEntries, catalog, file)

		for _, entries := range expectedEntries {
			for i := range entries {
				entries[i].IsCompressed = !entries[i].IsCompressed
			}
		}

		for packName, entries := range expectedEntries {
			packEntries := make([]archive.CatalogEntry, rand.Intn(5)+5)
			for i := range packEntries {
				compressed := false
				if rand.Intn(2) == 0 {
					compressed = true
				}

				packEntries[i] = archive.CatalogEntry{
					Path:         fmt.Sprint("update/", packName, "/", 100+i, ".pk"),
					IsCompressed: compressed,
				}
			}

			expectedEntries[packName] = append(entries, packEntries...)
		}

		if err := catalog.Store(expectedEntries); err != nil {
			t.Fatal(err)
		}

		env.CheckCatalog(t, expectedEntries, catalog, file)
	}
}

func TestCatalogWrite(t *testing.T) {
	env, teardown, err := setup(t, "catalog*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	t.Run("store", testCatalogStore(env))
	t.Run("update", testCatalogUpdate(env))
}

func checkCatalogRecord(t *testing.T, expectedPackName string, expected archive.CatalogEntry, actual *archive.CatalogRecord) {
	if expectedPackName != actual.PackName {
		t.Errorf("%s: expected pack name %s but got %s", expected.Path, expectedPackName, actual.PackName)
	}

	if expected.IsCompressed != actual.IsCompressed {
		t.Errorf("%s: expected compressed %t but got %t", expected.Path, expected.IsCompressed, actual.IsCompressed)
	}
}

func testCatalogRead(catalogName string, entries archive.CatalogEntries) func(*testing.T) {
	return func(t *testing.T) {
		catalogPath := filepath.Join("testdata", catalogName)

		file, err := os.Open(catalogPath)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		catalog, err := archive.NewCatalog(file)
		if err != nil {
			t.Fatal(err)
		}

		if catalog.Version != archive.CatalogVersion {
			t.Errorf("expected version %d but got %d", archive.CatalogVersion, catalog.Version)
		}

		numExpectedRecords := 0
		for _, records := range entries {
			numExpectedRecords += len(records)
		}

		if len(catalog.Records()) != numExpectedRecords {
			t.Fatalf("expected %d records but got %d", numExpectedRecords, len(catalog.Records()))
		}

		for expectedPackName, expectedRecord := range entries {
			for _, expected := range expectedRecord {
				actual, ok := catalog.Search(expected.Path)
				if ok {
					checkCatalogRecord(t, expectedPackName, expected, actual)
				} else {
					t.Errorf("failed to find %s", expected.Path)
				}
			}
		}
	}
}

func TestCatalogRead(t *testing.T) {
	t.Run("read_basic", testCatalogRead("read_basic.pki", archive.CatalogEntries{
		"packs/pack1.pk": []archive.CatalogEntry{
			{"files/raw1", false},
			{"files/raw2", false},
			{"files/raw3", false},
			{"files/raw4", false},
			{"files/raw5", false},
		},
		"packs/pack2.pk": []archive.CatalogEntry{
			{"files/compressed1", true},
			{"files/compressed2", true},
			{"files/compressed3", true},
			{"files/compressed4", true},
			{"files/compressed5", true},
		},
	}))

	t.Run("empty", testCatalogRead("empty.pki", archive.CatalogEntries{}))
}
