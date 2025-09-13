package cache_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/cache"
)

type Entry struct {
	Path     string
	modTime  time.Time
	size     int64
	Checksum []byte
}

func (e Entry) Name() string {
	return e.Path
}

func (e Entry) Size() int64 {
	return e.size
}

func (e Entry) Mode() fs.FileMode {
	return 0
}

func (e Entry) ModTime() time.Time {
	return e.modTime
}

func (e Entry) IsDir() bool {
	return false
}

func (e Entry) Sys() any {
	return nil
}

func newEntry(path string, seconds, milliseconds, size int64, checksum string) Entry {
	data, _ := hex.DecodeString(checksum)
	return Entry{
		Path:     path,
		modTime:  time.Unix(seconds, milliseconds*1000),
		size:     size,
		Checksum: data,
	}
}

func generateChecksum() []byte {
	buf := make([]byte, 32)
	for i := range buf {
		buf[i] = byte(rand.Int())
	}
	return buf
}

func generateEntries(num int) []Entry {
	entries := make([]Entry, num)
	for i := range entries {
		entries[i] = Entry{
			Path:     fmt.Sprint("files/", "data", i),
			size:     rand.Int63(),
			modTime:  time.Now(),
			Checksum: generateChecksum(),
		}
		time.Sleep(time.Duration(rand.Intn(1000000)) * time.Nanosecond) // skew mod time
	}
	return entries
}

func checkEntry(t *testing.T, expected Entry, actual cache.QuickCheck) {
	expectedPath := filepath.ToSlash(expected.Path)
	actualPath := filepath.ToSlash(actual.Path())

	if expectedPath != actualPath {
		t.Errorf("%s: expected path %s but got %s", expected.Path, expectedPath, actualPath)
	}

	if expected.size != actual.Size() {
		t.Errorf("%s: expected size %d but got %d", expected.Path, expected.size, actual.Size())
	}

	if !cache.RoundTime(expected.modTime).Equal(actual.ModTime()) {
		t.Errorf("%s: expected mod time %s but got %s", expected.Path, cache.FormatTime(expected.modTime), cache.FormatTime(actual.ModTime()))
	}

	if !bytes.Equal(expected.Checksum, actual.Checksum()) {
		t.Errorf("%s: expected checksum %x but got %x", expected.Path, expected.Checksum, actual.Checksum())
	}
}

func checkReader(t *testing.T, r io.Reader, expectedEntries []Entry) {
	cacheFile, err := cache.Read(r)
	if err != nil {
		t.Fatal(err)
	}

	if len(expectedEntries) != cacheFile.Len() {
		t.Fatalf("expected %d entries but got %d", len(expectedEntries), cacheFile.Len())
	}

	for _, expected := range expectedEntries {
		actual, ok := cacheFile.Load(expected.Path)
		if ok {
			checkEntry(t, expected, actual)
		} else {
			t.Errorf("failed to find %s", expected.Path)
		}
	}
}

func testRead(name string, expectedEntries []Entry) func(*testing.T) {
	return func(t *testing.T) {
		file, err := os.Open(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		checkReader(t, file, expectedEntries)
	}
}

func checkIterator(t *testing.T, r io.Reader, expectedEntries []Entry) {
	cacheFile, err := cache.Read(r)
	if err != nil {
		t.Fatal(err)
	}

	actualEntries := slices.Collect(cacheFile.All())
	if len(actualEntries) != len(expectedEntries) {
		t.Errorf("expected %d entries but got %d", len(expectedEntries), len(actualEntries))
		return
	}

	slices.SortFunc(expectedEntries, func(a, b Entry) int { return strings.Compare(a.Path, b.Path) })
	slices.SortFunc(actualEntries, func(a, b cache.QuickCheck) int { return strings.Compare(a.Path(), b.Path()) })

	for i, expected := range expectedEntries {
		checkEntry(t, expected, actualEntries[i])
	}
}

func testIterator(name string, expectedEntries []Entry) func(*testing.T) {
	return func(t *testing.T) {
		file, err := os.Open(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		checkIterator(t, file, expectedEntries)
	}
}

func TestRead(t *testing.T) {
	t.Run("read_basic", testRead("basic.txt", []Entry{
		newEntry("client\\awesomium.dll", 1312891366, 283039, 21562880, "e00896c0ecc03a375dcd54b9b1034b54"),
		newEntry("client\\awesomiumprocess.exe", 1312891378, 381248, 451176, "9c5b7ab68910e0be65419c617b8394fc"),
		newEntry("client\\binkw32.dll", 1312891372, 640674, 174080, "80d353fadd34bb551b912289eae596af"),
		newEntry("client\\cop.dll", 1312891368, 59216, 3862528, "da50f2b835941cd72b8596fb2716ef44"),
		newEntry("client\\d3dx9_34.dll", 1312891359, 959406, 3497832, "1ca939918ed1b930059b3a882de6f648"),
		newEntry("client\\fmod_event.dll", 1312891338, 617272, 307200, "203956a75fe0d8ac6906793bdfe0d211"),
		newEntry("client\\fmodex.dll", 1312891341, 396550, 843776, "7d040207c78542104a8790ab695bc9c0"),
		newEntry("client\\icudt42.dll", 1312891335, 763987, 10941440, "0c5bd1f7a69a176d6029a8c598a13261"),
		newEntry("client\\legouniverse.exe", 1320337851, 961618, 23029352, "29d6870c6e9229cafd58d0e613d10f89"),
		newEntry("client\\locale\\locale.xml", 1320337856, 54617, 9047759, "fda4d857c7ce8cfa1e7d44b2333b64f2"),
		newEntry("client\\locales\\en-us.dll", 1312891352, 465657, 111104, "a0ac7b4b394e345177de08a668b47672"),
		newEntry("client\\lwo.cfg.default", 1312891341, 703581, 6104, "9c7d30d4701e1406e1c52cac8f7d5593"),
		newEntry("client\\lwoclient.state", 1312891322, 676678, 2260, "8b9b37ea73e2d242463f697840085b35"),
	}))

	t.Run("basic_iterator", testIterator("basic.txt", []Entry{
		newEntry("client\\awesomium.dll", 1312891366, 283039, 21562880, "e00896c0ecc03a375dcd54b9b1034b54"),
		newEntry("client\\awesomiumprocess.exe", 1312891378, 381248, 451176, "9c5b7ab68910e0be65419c617b8394fc"),
		newEntry("client\\binkw32.dll", 1312891372, 640674, 174080, "80d353fadd34bb551b912289eae596af"),
		newEntry("client\\cop.dll", 1312891368, 59216, 3862528, "da50f2b835941cd72b8596fb2716ef44"),
		newEntry("client\\d3dx9_34.dll", 1312891359, 959406, 3497832, "1ca939918ed1b930059b3a882de6f648"),
		newEntry("client\\fmod_event.dll", 1312891338, 617272, 307200, "203956a75fe0d8ac6906793bdfe0d211"),
		newEntry("client\\fmodex.dll", 1312891341, 396550, 843776, "7d040207c78542104a8790ab695bc9c0"),
		newEntry("client\\icudt42.dll", 1312891335, 763987, 10941440, "0c5bd1f7a69a176d6029a8c598a13261"),
		newEntry("client\\legouniverse.exe", 1320337851, 961618, 23029352, "29d6870c6e9229cafd58d0e613d10f89"),
		newEntry("client\\locale\\locale.xml", 1320337856, 54617, 9047759, "fda4d857c7ce8cfa1e7d44b2333b64f2"),
		newEntry("client\\locales\\en-us.dll", 1312891352, 465657, 111104, "a0ac7b4b394e345177de08a668b47672"),
		newEntry("client\\lwo.cfg.default", 1312891341, 703581, 6104, "9c7d30d4701e1406e1c52cac8f7d5593"),
		newEntry("client\\lwoclient.state", 1312891322, 676678, 2260, "8b9b37ea73e2d242463f697840085b35"),
	}))
}

func testWrite(numEntries int) func(*testing.T) {
	return func(t *testing.T) {
		expectedEntries := generateEntries(numEntries)

		c := cache.Cache{}
		for _, entry := range expectedEntries {
			c.Store(entry.Path, entry, archive.Info{
				UncompressedChecksum: entry.Checksum,
			})
		}

		buf := bytes.Buffer{}
		if err := cache.Write(&buf, &c); err != nil {
			t.Fatal(err)
		}

		checkReader(t, bytes.NewReader(buf.Bytes()), expectedEntries)
	}
}

func TestWrite(t *testing.T) {
	t.Run("write_1", testWrite(1))
	t.Run("write_10", testWrite(10))
	t.Run("write_20", testWrite(20))
	t.Run("write_100", testWrite(100))
	t.Run("write_1000", testWrite(1000))
}

func TestRoundNanoseconds(t *testing.T) {
	type Time struct {
		Seconds, Nano int64
	}

	type TestCase struct {
		Time     Time
		Expected int64
	}

	for _, test := range []TestCase{
		{Time{0, 5}, 0},
		{Time{1729218809, 315336400}, 315337000},
		{Time{1729218809, 330893400}, 330894000},
		{Time{1729218819, 196368500}, 196369000},
		{Time{1729218819, 870770400}, 870771000},
		{Time{1729218819, 897075500}, 897075000},
		{Time{1729219363, 510861500}, 510861000},
	} {
		actual := cache.RoundTime(time.Unix(test.Time.Seconds, test.Time.Nano))
		if test.Time.Seconds != actual.Unix() || test.Expected != int64(actual.Nanosecond()) {
			t.Errorf("%d.%d: expected %d.%06d but got %d.%06d", test.Time.Seconds, test.Time.Nano, test.Time.Seconds, test.Expected, actual.Unix(), actual.Nanosecond())
		}
	}
}
