package cache_test

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/cache"
)

type Entry struct {
	Name     string
	Size     int64
	ModTime  time.Time
	Checksum []byte
}

type TestCache map[string]Entry

type File struct {
	Name     string
	Checksum []byte
}

func createData() []byte {
	num := rand.Intn(128) + 128
	data := make([]byte, num)
	for i := range data {
		data[i] = byte(rand.Int())
	}
	return data
}

func createFile(dir, name string) (File, error) {
	file, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return File{}, err
	}
	defer file.Close()

	data := createData()

	checksum := md5.New()
	checksum.Write(data)

	if _, err := file.Write(data); err != nil {
		return File{}, err
	}

	return File{
		Name:     name,
		Checksum: checksum.Sum(nil),
	}, nil
}

type Files []File

func (files *Files) Add(dir, name string) error {
	file, err := createFile(dir, name)
	if err != nil {
		return err
	}

	*files = append(*files, file)
	return nil
}

func (files Files) Update(dir string, index int) error {
	name := files[index].Name
	file, err := createFile(dir, name)
	if err != nil {
		return err
	}
	files[index] = file
	return nil
}

func createFiles(dir string, n int) (Files, error) {
	files := Files{}
	for i := 0; i < n; i++ {
		if err := files.Add(dir, fmt.Sprintf("res/file%02d", i)); err != nil {
			return nil, err
		}
	}
	return files, nil
}

type Env struct {
	Dir       string
	CachePath string

	Files     Files
	CacheFile *cache.Cache
}

func (env *Env) Stat(name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(env.Dir, name))
}

func setup(numFiles int, threshold ...int) (*Env, func(t *testing.T), error) {
	thresh := 0
	if len(threshold) > 0 {
		thresh = threshold[0]
	}

	dir, err := os.MkdirTemp(".", "cache_*.temp")
	if err != nil {
		return nil, nil, fmt.Errorf("setup: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "res"), 0755); err != nil {
		return nil, nil, fmt.Errorf("setup: %w", err)
	}

	files, err := createFiles(dir, numFiles)
	if err != nil {
		return nil, nil, fmt.Errorf("setup: %w", err)
	}

	cachePath := filepath.Join(dir, "quickcheck.txt")
	cachefile, err := cache.Open(cachePath, thresh)
	if err != nil {
		return nil, nil, fmt.Errorf("setup: %w", err)
	}

	cleanup := func(t *testing.T) {
		if os.Getenv("KEEP_TESTDATA") != "1" {
			if err := os.RemoveAll(dir); err != nil {
				t.Log(err)
			}
		}
	}

	return &Env{
		Dir:       dir,
		CachePath: cachePath,

		Files:     files,
		CacheFile: cachefile,
	}, cleanup, nil
}

func generateCache(env *Env, files Files) (TestCache, error) {
	c := TestCache{}
	for _, file := range files {
		stat, err := env.Stat(file.Name)
		if err != nil {
			return nil, err
		}

		c[strings.ToLower(filepath.FromSlash(file.Name))] = Entry{
			Name:     file.Name,
			ModTime:  cache.RoundTime(stat.ModTime()),
			Size:     stat.Size(),
			Checksum: file.Checksum,
		}
	}
	return c, nil
}

func generateCacheLines(env *Env, files Files) ([]string, error) {
	c, err := generateCache(env, files)
	if err != nil {
		return nil, err
	}

	lines := []string{}
	for _, entry := range c {
		lines = append(lines, fmt.Sprintf("%s,%s,%d,%x", filepath.FromSlash(entry.Name), cache.FormatTime(entry.ModTime), entry.Size, entry.Checksum))
	}
	return lines, nil
}

func cacheLen(cachefile *cache.Cache) int {
	l := 0
	cachefile.ForEach(func(qc cache.QuickCheck) bool {
		l++
		return true
	})
	return l
}

func checkCache(t *testing.T, cachefile *cache.Cache, expected TestCache) {
	if length := cacheLen(cachefile); len(expected) != length {
		t.Fatalf("expected %d entries but got %d", len(expected), length)
	}

	cachefile.ForEach(func(qc cache.QuickCheck) bool {
		entry, ok := expected[strings.ToLower(qc.Path())]
		if !ok {
			t.Errorf("%s: unexpected entry", qc.Path())
			return true
		}

		if !entry.ModTime.Equal(qc.ModTime()) {
			t.Errorf("%s: expected mod time (%d.%d) but got (%d.%d)", entry.Name, entry.ModTime.Unix(), entry.ModTime.Nanosecond(), qc.ModTime().Unix(), qc.ModTime().Nanosecond())
		}

		if entry.Size != qc.Size() {
			t.Errorf("%s: expected size %d but got %d", entry.Name, entry.Size, qc.Size())
		}

		if !bytes.Equal(entry.Checksum, qc.Checksum()) {
			t.Errorf("%s: expected %x but got %x", entry.Name, entry.Checksum, qc.Checksum())
		}

		return true
	})
}

func testBasicStore(n int) func(t *testing.T) {
	return func(t *testing.T) {
		env, cleanup, err := setup(n)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := env.CacheFile.Close(); err != nil {
				t.Log(err)
			}
			cleanup(t)
		}()

		for _, file := range env.Files {
			stat, err := env.Stat(file.Name)
			if err != nil {
				t.Fatal(err)
			}

			if err := env.CacheFile.Store(file.Name, stat, archive.Info{
				UncompressedChecksum: file.Checksum,
			}); err != nil {
				t.Fatal(err)
			}
		}

		expected, err := generateCache(env, env.Files)
		if err != nil {
			t.Fatal(err)
		}

		checkCache(t, env.CacheFile, expected)
	}
}

func testStoreUpdates(numFiles, numUpdates int) func(t *testing.T) {
	return func(t *testing.T) {
		env, cleanup, err := setup(numFiles)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := env.CacheFile.Close(); err != nil {
				t.Log(err)
			}
			cleanup(t)
		}()

		for _, file := range env.Files {
			stat, err := env.Stat(file.Name)
			if err != nil {
				t.Fatal(err)
			}

			if err := env.CacheFile.Store(file.Name, stat, archive.Info{
				UncompressedChecksum: file.Checksum,
			}); err != nil {
				t.Fatal(err)
			}
		}

		for i := 0; i < numUpdates; i++ {
			index := rand.Intn(len(env.Files))
			if err := env.Files.Update(env.Dir, index); err != nil {
				t.Fatal(err)
			}

			file := env.Files[index]

			stat, err := env.Stat(file.Name)
			if err != nil {
				t.Fatal(err)
			}

			if err := env.CacheFile.Store(file.Name, stat, archive.Info{
				UncompressedChecksum: file.Checksum,
			}); err != nil {
				t.Fatal(err)
			}
		}

		expected, err := generateCache(env, env.Files)
		if err != nil {
			t.Fatal(err)
		}

		checkCache(t, env.CacheFile, expected)
	}
}

func TestStore(t *testing.T) {
	for i := 1; i <= 12; i++ {
		t.Run(fmt.Sprintf("store_few_%d", i), testBasicStore(i))
	}

	for i := 0; i < 10; i++ {
		n := rand.Intn(32) + 24
		t.Run("store_many", testBasicStore(n))
	}

	for i := 0; i < 5; i++ {
		numFiles := rand.Intn(8) + 1
		numUpdates := rand.Intn(12)
		t.Run("update", testStoreUpdates(numFiles, numUpdates))
	}
}

func getLines(buf *bytes.Buffer) []string {
	lines := []string{}
	for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
		s := strings.TrimSpace(string(line))
		if len(s) > 0 {
			lines = append(lines, strings.ToLower(s))
		}
	}
	return lines
}

func compareLines(t *testing.T, expected, actual []string) {

	if len(expected) != len(actual) {
		t.Errorf("expected %d lines but got %d", len(expected), len(actual))
		return
	}
	sort.Strings(expected)
	sort.Strings(actual)

	for i, a := range expected {
		if a != actual[i] {
			t.Errorf("expected:\n%v\n\nactual:\n%v", expected, actual)
			return
		}
	}
}

func checkLines(t *testing.T, env *Env) {
	expectedLines, err := generateCacheLines(env, env.Files)
	if err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	env.CacheFile.WriteTo(buf)

	compareLines(t, expectedLines, getLines(buf))
}

func testWrite(numFiles int) func(t *testing.T) {
	return func(t *testing.T) {
		env, cleanup, err := setup(numFiles)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := env.CacheFile.Close(); err != nil {
				t.Log(err)
			}
			cleanup(t)
		}()

		for _, file := range env.Files {
			stat, err := env.Stat(file.Name)
			if err != nil {
				t.Fatal(err)
			}

			if err := env.CacheFile.Store(file.Name, stat, archive.Info{
				UncompressedChecksum: file.Checksum,
			}); err != nil {
				t.Fatal(err)
			}
		}

		checkLines(t, env)
	}
}

func testWriteUpdates(numFiles, numUpdates int) func(t *testing.T) {
	return func(t *testing.T) {
		env, cleanup, err := setup(numFiles)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := env.CacheFile.Close(); err != nil {
				t.Log(err)
			}
			cleanup(t)
		}()

		for _, file := range env.Files {
			stat, err := env.Stat(file.Name)
			if err != nil {
				t.Fatal(err)
			}

			if err := env.CacheFile.Store(file.Name, stat, archive.Info{
				UncompressedChecksum: file.Checksum,
			}); err != nil {
				t.Fatal(err)
			}
		}

		for i := 0; i < numUpdates; i++ {
			index := rand.Intn(len(env.Files))
			if err := env.Files.Update(env.Dir, index); err != nil {
				t.Fatal(err)
			}

			file := env.Files[index]

			stat, err := env.Stat(file.Name)
			if err != nil {
				t.Fatal(err)
			}

			if err := env.CacheFile.Store(file.Name, stat, archive.Info{
				UncompressedChecksum: file.Checksum,
			}); err != nil {
				t.Fatal(err)
			}

			checkLines(t, env)
		}
	}
}

func TestWrite(t *testing.T) {
	for i := 1; i <= 1; i++ {
		t.Run(fmt.Sprintf("write_lines_%d", i), testWrite(i))
	}

	for i := 0; i < 10; i++ {
		numFiles := rand.Intn(8) + 1
		numUpdates := rand.Intn(12)
		t.Run("write_updates", testWriteUpdates(numFiles, numUpdates))
	}
}

func readLines(env *Env) ([]string, error) {
	file, err := os.Open(env.CachePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return getLines(bytes.NewBuffer(data)), nil
}

func testFlush(numFiles, threshold int) func(t *testing.T) {
	return func(t *testing.T) {
		env, cleanup, err := setup(numFiles, threshold)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := env.CacheFile.Close(); err != nil {
				t.Log(err)
			}
			cleanup(t)
		}()

		expectedFiles := Files{}
		toFlush := Files{}

		for _, file := range env.Files {
			toFlush = append(toFlush, file)
			if len(toFlush) >= threshold {
				expectedFiles = append(expectedFiles, toFlush...)
				toFlush = toFlush[:0]
			}

			stat, err := env.Stat(file.Name)
			if err != nil {
				t.Fatal(err)
			}

			if err := env.CacheFile.Store(file.Name, stat, archive.Info{
				UncompressedChecksum: file.Checksum,
			}); err != nil {
				t.Fatal(err)
			}

			expectedLines, err := generateCacheLines(env, expectedFiles)
			if err != nil {
				t.Fatal(err)
			}

			actualLines, err := readLines(env)
			if err != nil {
				t.Fatal(err)
			}

			compareLines(t, expectedLines, actualLines)
		}
	}
}

func TestFlush(t *testing.T) {
	t.Run("flush_5_1", testFlush(5, 1))
	t.Run("flush_1_10", testFlush(1, 10))
	t.Run("flush_5_10", testFlush(5, 10))
	t.Run("flush_10_10", testFlush(10, 10))
	t.Run("flush_20_10", testFlush(20, 10))

	t.Run("close_flush", func(t *testing.T) {
		env, cleanup, err := setup(20, 10)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup(t)
		defer env.CacheFile.Close()

		for _, file := range env.Files[:5] {
			stat, err := env.Stat(file.Name)
			if err != nil {
				t.Fatal(err)
			}

			if err := env.CacheFile.Store(file.Name, stat, archive.Info{
				UncompressedChecksum: file.Checksum,
			}); err != nil {
				t.Fatal(err)
			}
		}

		expectedLines, err := generateCacheLines(env, Files{})
		if err != nil {
			t.Fatal(err)
		}

		actualLines, err := readLines(env)
		if err != nil {
			t.Fatal(err)
		}

		compareLines(t, expectedLines, actualLines)

		for _, file := range env.Files[5:] {
			stat, err := env.Stat(file.Name)
			if err != nil {
				t.Fatal(err)
			}

			if err := env.CacheFile.Store(file.Name, stat, archive.Info{
				UncompressedChecksum: file.Checksum,
			}); err != nil {
				t.Fatal(err)
			}
		}

		if err := env.CacheFile.Close(); err != nil {
			t.Fatal(err)
		}

		expectedLines, err = generateCacheLines(env, env.Files)
		if err != nil {
			t.Fatal(err)
		}

		actualLines, err = readLines(env)
		if err != nil {
			t.Fatal(err)
		}

		compareLines(t, expectedLines, actualLines)
	})
}

func testRead(numFiles int) func(t *testing.T) {
	return func(t *testing.T) {
		env, cleanup, err := setup(numFiles)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup(t)
		defer func() {
			env.CacheFile.Close()
		}()

		if err := env.CacheFile.Close(); err != nil {
			t.Fatal(err)
		}

		lines, err := generateCacheLines(env, env.Files)
		if err != nil {
			t.Fatal(err)
		}

		file, err := os.OpenFile(env.CachePath, os.O_TRUNC|os.O_WRONLY, 0755)
		if err != nil {
			t.Fatal(err)
		}

		for _, line := range lines {
			if _, err := file.Write([]byte(line + "\n")); err != nil {
				file.Close()
				t.Fatal(err)
			}
		}
		file.Close()

		env.CacheFile, err = cache.Open(env.CachePath)
		if err != nil {
			t.Fatal(err)
		}

		expectedCache, err := generateCache(env, env.Files)
		if err != nil {
			t.Fatal(err)
		}

		checkCache(t, env.CacheFile, expectedCache)
	}
}

func TestRead(t *testing.T) {
	for i := 0; i < 5; i++ {
		t.Run("read", testRead(10))
		time.Sleep(time.Duration(rand.Intn(1000000)) * time.Nanosecond) // only to skew mod time
	}
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
