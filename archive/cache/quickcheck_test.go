package cache_test

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/cache"
)

const (
	NumFiles = 32
)

type Env struct {
	Dir       string
	CachePath string
	ResFiles  []string
}

func (env *Env) AddResFile(name ...string) (check, string, error) {
	resPath := fmt.Sprintf("file%02d.txt", len(env.ResFiles))
	if len(name) > 0 {
		resPath = name[0]
	}

	path := filepath.Join("res", resPath)

	file, err := os.OpenFile(filepath.Join(env.Dir, path), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return check{}, "", fmt.Errorf("add file: %w", err)
	}
	defer file.Close()

	env.ResFiles = append(env.ResFiles, path)

	hash := md5.New()
	w := io.MultiWriter(file, hash)

	written, err := w.Write(genData())
	if err != nil {
		return check{}, "", fmt.Errorf("add file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		return check{}, "", fmt.Errorf("add file: %w", err)
	}

	return check{
		Path:    archive.ToArchivePath(path),
		ModTime: stat.ModTime(),
		Size:    int64(written),
		Hash:    hash.Sum(nil),
	}, resPath, nil
}

func (env *Env) ModifyResFile(cachefile *cache.Cache, name string) error {
	file, err := env.OpenResFile(name)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(genData()); err != nil {
		return err
	}

	if err := cachefile.Push(filepath.Join("res", name), file); err != nil {
		return err
	}

	return nil
}

func (env *Env) OpenResFile(name string) (*os.File, error) {
	return os.OpenFile(filepath.Join(env.Dir, "res", name), os.O_RDWR, 0755)
}

func (env *Env) PushResFile(cachefile *cache.Cache, name ...string) error {
	quickcheck, resPath, err := env.AddResFile(name...)
	if err != nil {
		return err
	}

	file, err := env.OpenResFile(resPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return cachefile.Push(quickcheck.Path, file)
}

func (env *Env) CheckResFile(cachefile *cache.Cache, name string) error {
	file, err := env.OpenResFile(name)
	if err != nil {
		return err
	}
	defer file.Close()

	path := filepath.Join("res", name)

	actual, ok := cachefile.Get(path)
	if !ok {
		return fmt.Errorf("%s does not exist", path)
	}

	expected, err := Check(file)
	if err != nil {
		return err
	}

	errs := []error{}
	if expected.Size != actual.Size() {
		errs = append(errs, fmt.Errorf("expected size %d but got %d", expected.Size, actual.Size()))
	}

	if !bytes.Equal(expected.Hash, actual.Hash()) {
		errs = append(errs, fmt.Errorf("expected hash %x but got %x", expected.Hash, actual.Hash()))
	}

	return errors.Join(errs...)
}

func (env *Env) Test(cachefile *cache.Cache) error {
	expectedData := &bytes.Buffer{}
	cachefile.ForEach(func(qc cache.QuickCheck) bool {
		fmt.Fprintf(expectedData, "%s,%d.%06d,%d,%x\n", qc.Path(), qc.LastModified().Unix(), qc.LastModified().Nanosecond(), qc.Size(), qc.Hash())
		return true
	})

	actualData := &bytes.Buffer{}
	if _, err := cachefile.WriteTo(actualData); err != nil {
		return err
	}

	expectedLines := strings.Split(expectedData.String(), "\n")
	slices.Sort(expectedLines)

	actualLines := strings.Split(actualData.String(), "\n")
	slices.Sort(actualLines)

	expected := strings.Join(expectedLines, "")
	actual := strings.Join(actualLines, "")

	if !bytes.Equal([]byte(expected), []byte(actual)) {
		return fmt.Errorf("got different cachefile data:\n===EXPECTED===\n%s\n===ACTUAL===\n%s", expected, actual)
	}

	return nil
}

func (env *Env) TestFlushed(cachefile *cache.Cache) error {
	file, err := os.Open(env.CachePath)
	if err != nil {
		return err
	}

	expectedLines := 0
	cachefile.ForEach(func(cache.QuickCheck) bool {
		expectedLines++
		return true
	})

	actualLines := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if len(strings.TrimSpace(scanner.Text())) > 0 {
			actualLines++
		}
	}
	file.Close()

	if expectedLines != actualLines {
		return fmt.Errorf("got different num cachefile lines: expected %d but got %d", expectedLines, actualLines)
	}

	return env.Test(cachefile)
}

func (env *Env) CompareLines(expected []string) error {
	data, err := os.ReadFile(env.CachePath)
	if err != nil {
		return err
	}

	actual := strings.Split(string(data), "\n")
	sort.Strings(actual)

	if len(expected) != len(actual) {
		return fmt.Errorf("expected %d lines but got %d", len(expected), len(actual))
	}

	errs := []error{}
	for i, line := range actual {
		expectedLine := strings.TrimSpace(expected[i])
		actualLine := strings.TrimSpace(line)

		if expectedLine != actualLine {
			errs = append(errs, fmt.Errorf("expected %s but got %s", expectedLine, actualLine))
		}
	}

	return errors.Join(errs...)
}

func (env *Env) CleanUp() error {
	return os.RemoveAll(env.Dir)
}

type check struct {
	Path    string
	ModTime time.Time
	Size    int64
	Hash    []byte
}

func (check *check) WriteTo(w io.Writer) (int64, error) {
	n, err := fmt.Fprintf(w, "%s,%d.%06d,%d,%x\n", check.Path, check.ModTime.Unix(), check.ModTime.Nanosecond(), check.Size, check.Hash)
	return int64(n), err
}

func Check(file *os.File) (check, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return check{}, err
	}

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return check{}, err
	}

	stat, err := file.Stat()
	if err != nil {
		return check{}, err
	}

	return check{
		ModTime: stat.ModTime(),
		Size:    stat.Size(),
		Hash:    hash.Sum(nil),
	}, nil
}

func genData() []byte {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(rand.Int())
	}
	return data
}

func setup(dir string) (*Env, func(*testing.T), error) {
	env := &Env{
		Dir:       dir,
		CachePath: filepath.Join(dir, "quickcheck.txt"),
		ResFiles:  []string{},
	}

	cleanup := func(t *testing.T) {
		if os.Getenv("KEEP_TESTDATA") != "1" {
			if err := env.CleanUp(); err != nil {
				t.Logf("cleanup: %v", err)
			}
		}
	}

	if err := os.MkdirAll(filepath.Join(env.Dir, "res"), 0755); err != nil {
		return nil, cleanup, err
	}

	cacheFile, err := os.OpenFile(env.CachePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return nil, cleanup, err
	}
	defer cacheFile.Close()

	for i := 0; i < NumFiles; i++ {
		check, _, err := env.AddResFile()
		if err != nil {
			return nil, cleanup, err
		}

		if _, err := check.WriteTo(cacheFile); err != nil {
			return nil, cleanup, err
		}
	}

	return env, cleanup, nil
}

func TestQuickCheckBasic(t *testing.T) {
	env, cleanup, err := setup("testdata")
	defer cleanup(t)

	if err != nil {
		t.Fatalf("quick check: setup: %v", err)
	}

	cachefile, err := cache.Open(env.CachePath)
	if err != nil {
		t.Fatalf("quick check: %v", err)
	}
	defer cachefile.Close()

	t.Log("-- test: all files match")
	cachefile.ForEach(func(qc cache.QuickCheck) bool {
		file, err := os.Open(filepath.Join(env.Dir, qc.Path()))
		if err != nil {
			t.Errorf("all files match: %v", err)
		}

		if err := qc.Check(file); err != nil {
			t.Errorf("all files match: %v", err)
		}

		file.Close()

		return true
	})

	t.Log("-- test: add files")
	if err := errors.Join(
		env.PushResFile(cachefile, "added.txt"),
		env.PushResFile(cachefile, "to_be_modified.txt"),
	); err != nil {
		t.Fatalf("add files: %v", err)
	}

	if err := env.Test(cachefile); err != nil {
		t.Errorf("add files: %v", err)
	}

	t.Log("-- test: modify file (no changes)")
	file, err := env.OpenResFile("to_be_modified.txt")
	if err != nil {
		t.Fatalf("modify file (no change): %v", err)
	}

	if err := cachefile.Push("res/to_be_modified.txt", file); err != nil {
		file.Close()
		t.Fatalf("modify file (no changes): %v", err)
	}
	file.Close()

	if err := env.Test(cachefile); err != nil {
		t.Errorf("modify file (no changes): %v", err)
	}

	t.Log("-- test: modify file (with changes)")
	file, err = env.OpenResFile("to_be_modified.txt")
	if err != nil {
		t.Fatalf("modify file (with changes): %v", err)
	}

	if _, err := file.Write(genData()); err != nil {
		file.Close()
		t.Fatalf("modify file (with changes): %v", err)
	}

	if err := cachefile.Push("res/to_be_modified.txt", file); err != nil {
		file.Close()
		t.Fatalf("modify file (with changes): %v", err)
	}
	file.Close()

	if err := env.CheckResFile(cachefile, "to_be_modified.txt"); err != nil {
		t.Errorf("modify file (with changes): %v", err)
	}

	if err := env.Test(cachefile); err != nil {
		t.Errorf("modify file (with changes): %v", err)
	}

	t.Log("-- test: hard flush")
	if err := cachefile.Flush(); err != nil {
		t.Fatalf("hard flush: %v", err)
	}

	if err := env.TestFlushed(cachefile); err != nil {
		t.Errorf("hard flush: %v", err)
	}
}

func sortedLines(cachefile *cache.Cache) []string {
	buf := &bytes.Buffer{}
	cachefile.WriteTo(buf)

	lines := strings.Split(buf.String(), "\n")
	sort.Strings(lines)

	return lines
}

func TestQuickCheckFlush(t *testing.T) {
	env, cleanup, err := setup("testdata")
	defer cleanup(t)

	if err != nil {
		t.Fatalf("quick check: setup: %v", err)
	}

	cachefile, err := cache.Open(env.CachePath, 10)
	if err != nil {
		t.Fatalf("quick check: setup: %v", err)
	}
	defer cachefile.Close()

	expectedLines := sortedLines(cachefile)

	t.Log("-- test: add 1")
	if err := env.PushResFile(cachefile); err != nil {
		t.Fatalf("add 1: %v", err)
	}

	if err := env.CompareLines(expectedLines); err != nil {
		t.Errorf("add 1: %v", err)
	}

	t.Log("-- test: add 5")
	if err := errors.Join(
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
	); err != nil {
		t.Fatalf("add 5: %v", err)
	}

	if err := env.CompareLines(expectedLines); err != nil {
		t.Errorf("add 5: %v", err)
	}

	t.Log("-- test: add 4 (causes flush)")
	if err := errors.Join(
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
	); err != nil {
		t.Fatalf("add 4 (causes flush): %v", err)
	}

	if err := env.TestFlushed(cachefile); err != nil {
		t.Errorf("add 4 (causes flush): %v", err)
	}

	expectedLines = sortedLines(cachefile)

	t.Log("-- test: add 9")
	if err := errors.Join(
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
	); err != nil {
		t.Fatalf("add 9: %v", err)
	}

	if err := env.CompareLines(expectedLines); err != nil {
		t.Errorf("add 9: %v", err)
	}

	t.Log("-- test: add 6 (causes flush)")
	if err := errors.Join(
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile),
		env.PushResFile(cachefile, "modify_me_1.txt"),
		env.PushResFile(cachefile, "modify_me_2.txt"),
		env.PushResFile(cachefile),
	); err != nil {
		t.Fatalf("add 6 (causes flush): %v", err)
	}

	expectedLines = sortedLines(cachefile)
	expectedLines = expectedLines[:len(expectedLines)-5]

	if err := env.CompareLines(expectedLines); err != nil {
		t.Errorf("add 6 (causes flush): %v", err)
	}

	t.Log("-- test: hard flush")
	if err := cachefile.Flush(); err != nil {
		t.Errorf("hard flush: %v", err)
	}

	if err := env.TestFlushed(cachefile); err != nil {
		t.Errorf("hard flush: %v", err)
	}

	expectedLines = sortedLines(cachefile)

	t.Log("-- test: modify files (hard flush)")
	if err := errors.Join(
		env.ModifyResFile(cachefile, "modify_me_1.txt"),
		env.ModifyResFile(cachefile, "modify_me_2.txt"),
	); err != nil {
		t.Fatalf("modify files: %v", err)
	}

	if err := env.CompareLines(expectedLines); err != nil {
		t.Errorf("modify files: %v", err)
	}

	if err := cachefile.Flush(); err != nil {
		t.Errorf("modify files: %v", err)
	}

	if err := env.TestFlushed(cachefile); err != nil {
		t.Errorf("modify files: %v", err)
	}
}
