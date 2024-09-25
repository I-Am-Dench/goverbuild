package cache_test

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
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

const (
	NumFiles = 25
)

type Env struct {
	Dir       string
	CachePath string
	ResFiles  []string
}

func (env *Env) AddResFile(name ...string) (check, error) {
	resPath := fmt.Sprintf("file%02d.txt", len(env.ResFiles))
	if len(name) > 0 {
		resPath = name[0]
	}

	path := filepath.Join("res", resPath)

	file, err := os.OpenFile(filepath.Join(env.Dir, path), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return check{}, fmt.Errorf("add file: %w", err)
	}
	defer file.Close()

	env.ResFiles = append(env.ResFiles, path)

	hash := md5.New()
	w := io.MultiWriter(file, hash)

	written, err := w.Write(genData())
	if err != nil {
		return check{}, fmt.Errorf("add file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		return check{}, fmt.Errorf("add file: %w", err)
	}

	return check{
		Path:    archive.ToArchivePath(path),
		ModTime: cache.SysModTime(stat),
		Size:    int64(written),
		Hash:    hash.Sum(nil),
	}, nil
}

func (env *Env) OpenResFile(name string) (*os.File, error) {
	return os.Open(filepath.Join(env.Dir, "res", name))
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
		check, err := env.AddResFile()
		if err != nil {
			return nil, cleanup, err
		}

		if _, err := check.WriteTo(cacheFile); err != nil {
			return nil, cleanup, err
		}
	}

	return env, cleanup, nil
}

// func sortedQuickCheck(cachefile *cache.Cache) []cache.QuickCheck {
// 	quickcheck := []cache.QuickCheck{}
// 	cachefile.ForEach(func(qc cache.QuickCheck) bool {
// 		quickcheck = append(quickcheck, qc)
// 		return true
// 	})

// 	sort.Slice(quickcheck, func(i, j int) bool {
// 		return strings.Compare(quickcheck[i].Path(), quickcheck[j].Path()) < 0
// 	})

// 	return quickcheck
// }

func TestQuickCheck(t *testing.T) {
	env, cleanup, err := setup("testdata")
	defer cleanup(t)

	if err != nil {
		t.Fatalf("quick check: setup: %v", err)
	}

	quickcheck, err := cache.Open(env.CachePath)
	if err != nil {
		t.Fatalf("quick check: %v", err)
	}
	defer quickcheck.Close()

	t.Log("-- test: all files match")
	quickcheck.ForEach(func(qc cache.QuickCheck) bool {
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
	addedCheck, err := env.AddResFile("added.txt")
	if err != nil {
		t.Fatalf("add files: %v", err)
	}

	file, err := env.OpenResFile("added.txt")
	if err != nil {
		t.Fatalf("add files: %v", err)
	}

	if err := quickcheck.Push(addedCheck.Path, file); err != nil {
		file.Close()
		t.Fatalf("add files: %v", err)
	}
	file.Close()

	if err := env.Test(quickcheck); err != nil {
		t.Errorf("add files: %v", err)
	}
}
