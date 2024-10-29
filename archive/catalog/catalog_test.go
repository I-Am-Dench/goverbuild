package catalog_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/catalog"
	"github.com/I-Am-Dench/goverbuild/archive/internal/binarytree"
)

const (
	NumPacks = 10
)

var (
	order = binary.LittleEndian
)

type Files map[string][]string

func (files *Files) AsRecords() map[uint32]string {
	records := map[uint32]string{}

	for packName, files := range *files {
		for _, file := range files {
			records[archive.GetCrc(file)] = packName
		}
	}

	return records
}

var fileNumber = 1

func generateFileNames() []string {
	num := rand.Intn(20) + 1
	files := make([]string, num)
	for i := range files {
		files[i] = fmt.Sprintf("client/files/date%d", fileNumber)
		fileNumber++
	}
	return files
}

func generateFiles() Files {
	packNames := make([]string, NumPacks)
	for i := range packNames {
		packNames[i] = fmt.Sprintf("packs/pack%d.pk", i)
	}

	files := Files{}
	for _, name := range packNames {
		files[name] = generateFileNames()
	}

	return files
}

func generateCatalogData(packNames []string, files Files) []byte {
	records := []*catalog.Record{}
	indices := map[string]int{}

	for packName, files := range files {
		for _, file := range files {
			records = append(records, &catalog.Record{
				PackName: packName,
				Crc:      archive.GetCrc(file),
			})
		}
	}
	slices.SortFunc(records, func(a, b *catalog.Record) int { return int(a.Crc) - int(b.Crc) })
	binarytree.UpdateIndices(records)

	for i, n := range packNames {
		indices[n] = i
	}

	buf := &bytes.Buffer{}
	binary.Write(buf, order, int32(3))
	binary.Write(buf, order, uint32(len(packNames)))

	for _, name := range packNames {
		binary.Write(buf, order, uint32(len(name)))
		buf.Write([]byte(name))
	}

	binary.Write(buf, order, uint32(len(records)))
	for _, record := range records {
		binary.Write(buf, order, record.Crc)
		binary.Write(buf, order, record.LowerIndex)
		binary.Write(buf, order, record.UpperIndex)
		binary.Write(buf, order, uint32(indices[record.PackName]))
		buf.Write([]byte{0, 0, 0, 0})
	}

	return buf.Bytes()
}

func checkPackNames(t *testing.T, expectedFiles Files, actual []string) {
	expected := []string{}
	for name := range expectedFiles {
		expected = append(expected, name)
	}

	if len(expected) != len(actual) {
		t.Errorf("expected %d packs but got %d", len(expected), len(actual))
		return
	}

	slices.Sort(expected)
	slices.Sort(actual)

	if !slices.Equal(expected, actual) {
		t.Errorf("got different pack names:\nexpected = %v\nactual   = %v", expected, actual)
	}
}

func testExpectedFiles(catalog *catalog.Catalog, expectedFiles Files) func(t *testing.T) {
	return func(t *testing.T) {
		checkPackNames(t, expectedFiles, slices.Clone(catalog.PackNames))

		records := expectedFiles.AsRecords()
		if len(records) != len(catalog.Records) {
			t.Errorf("expected %d records but got %d", len(records), len(catalog.Records))
		}

		for _, record := range catalog.Records {
			expectedPack, ok := records[record.Crc]
			if !ok {
				t.Errorf("%d: crc not expected", record.Crc)
				continue
			}

			if expectedPack != record.PackName {
				t.Errorf("%d: expected pack %s but got %s", record.Crc, expectedPack, record.PackName)
			}
		}

		// Using catalog.PackNames gives us the correct order for the expected data
		expectedData := generateCatalogData(catalog.PackNames, expectedFiles)

		actualData := &bytes.Buffer{}
		if _, err := catalog.WriteTo(actualData); err != nil {
			t.Fatalf("expected %d bytes but got %d", len(expectedData), actualData.Len())
		}

		if !bytes.Equal(expectedData, actualData.Bytes()) {
			t.Errorf("got different catalog data:\nexpected = %v\nactual   = %v", expectedData, actualData.Bytes())
		}
	}
}

func TestWrite(t *testing.T) {
	for i := 0; i < 10; i++ {
		files := generateFiles()
		t.Run("write", testExpectedFiles(catalog.New(files), files))
	}
}

func TestRead(t *testing.T) {

	for i := 0; i < 10; i++ {
		files := generateFiles()

		packNames := []string{}
		for name := range files {
			packNames = append(packNames, name)
		}

		expectedData := generateCatalogData(packNames, files)
		t.Run("read", func(t *testing.T) {
			catalog, err := catalog.ReadFrom(bytes.NewBuffer(expectedData))
			if err != nil {
				t.Fatal(err)
			}
			testExpectedFiles(catalog, files)
		})
	}
}
