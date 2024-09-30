package cache

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/I-Am-Dench/goverbuild/archive"
)

var (
	ErrMismatchedQuickCheck = errors.New("mismatched quick check entry")
)

func FormatQuickCheckTime(t time.Time) string {
	return fmt.Sprintf("%d.%06d", t.Unix(), t.Nanosecond())
}

type quickCheck struct {
	path         string
	lastModified time.Time
	size         int64
	hash         []byte
}

func (qc *quickCheck) Path() string {
	return qc.path
}

func (qc *quickCheck) LastModified() time.Time {
	return qc.lastModified
}

func (qc *quickCheck) Size() int64 {
	return qc.size
}

func (qc *quickCheck) Hash() []byte {
	return qc.hash
}

func (qc *quickCheck) Check(file *os.File) error {
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}

	expected := fmt.Sprintf("%s,%d", FormatQuickCheckTime(qc.lastModified), qc.size)
	actual := fmt.Sprintf("%s,%d", FormatQuickCheckTime(stat.ModTime()), stat.Size())
	if expected != actual {
		return fmt.Errorf("%w: (expected: %s) != (actual: %s)", ErrMismatchedQuickCheck, expected, actual)
	}

	return nil
}

type QuickCheck interface {
	Path() string
	LastModified() time.Time
	Size() int64
	Hash() []byte

	Check(*os.File) error
}

type Cache struct {
	sm sync.Map

	canFlush atomic.Bool
	// flushMux sync.Mutex

	f *os.File

	flushThreshold int

	modified atomic.Uint32

	addedMux sync.RWMutex
	added    []*quickCheck
}

func (cache *Cache) parseModTime(s string) (time.Time, error) {
	rawSeconds, rawNanoseconds, ok := strings.Cut(s, ".")
	if !ok {
		return time.Time{}, errors.New("parse mod time: no nanoseconds")
	}

	seconds, err := strconv.ParseInt(rawSeconds, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse mod time: %w", err)
	}

	nanoseconds, err := strconv.ParseInt(rawNanoseconds, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse mod time: %w", err)
	}

	return time.Unix(seconds, nanoseconds), nil
}

func (cache *Cache) parseLine(line string) (*quickCheck, error) {
	parts := strings.Split(line, ",")
	if len(parts) < 4 {
		return nil, errors.New("parse line: malformed line")
	}

	modTime, err := cache.parseModTime(parts[1])
	if err != nil {
		return nil, fmt.Errorf("parse line: %w", err)
	}

	size, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse line: %w", err)
	}

	hash, err := hex.DecodeString(parts[3])
	if err != nil {
		return nil, fmt.Errorf("parse line: %w", err)
	}

	return &quickCheck{
		path:         archive.ToArchivePath(parts[0]),
		lastModified: modTime,
		size:         size,
		hash:         hash,
	}, nil
}

func (cache *Cache) ForEach(f func(qc QuickCheck) bool) {
	cache.sm.Range(func(key, value any) bool {
		return f(value.(QuickCheck))
	})
}

func (cache *Cache) WriteTo(w io.Writer) (n int64, err error) {
	written := int64(0)
	cache.ForEach(func(qc QuickCheck) bool {
		var n int
		n, err = fmt.Fprintf(w, "%s,%d.%06d,%d,%x\n", qc.Path(), qc.LastModified().Unix(), qc.LastModified().Nanosecond(), qc.Size(), qc.Hash())
		written += int64(n)
		return err == nil
	})

	if err == nil {
		n = written
	}

	return
}

func (cache *Cache) flushAll() error {
	stat, err := cache.f.Stat()
	if err != nil {
		return err
	}

	if _, err := cache.f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	originalSize := stat.Size()

	written, err := cache.WriteTo(cache.f)
	if err != nil {
		return err
	}

	if written < originalSize {
		return cache.f.Truncate(written)
	}

	return nil
}

// func (cache *Cache) flushAll() error {
// 	stat, err := cache.f.Stat()
// 	if err != nil {
// 		return err
// 	}

// 	if _, err := cache.f.Seek(0, io.SeekStart); err != nil {
// 		return err
// 	}

// 	size := stat.Size()

// 	written := int64(0)
// 	cache.ForEach(func(qc QuickCheck) bool {
// 		var n int
// 		n, err = fmt.Fprintf(cache.f, "%s,%d.%06d,%d,%x\n", qc.Path(), qc.LastModified().Unix(), qc.LastModified().Nanosecond(), qc.Size(), qc.Hash())
// 		written += int64(n)
// 		return err == nil
// 	})

// 	if err != nil {
// 		return err
// 	}

// 	if written < size {
// 		return cache.f.Truncate(written)
// 	}

// 	return nil
// }

func (cache *Cache) flushAdded() error {
	var err error
	cache.ForEach(func(qc QuickCheck) bool {
		_, err = fmt.Fprintf(cache.f, "%s,%d.%06d,%d,%x\n", qc.Path(), qc.LastModified().Unix(), qc.LastModified().Nanosecond(), qc.Size(), qc.Hash())
		return err == nil
	})
	return err
}

func (cache *Cache) flush(all bool) (err error) {
	cache.addedMux.Lock()
	defer cache.addedMux.Unlock()

	if all {
		err = cache.flushAll()
	} else {
		err = cache.flushAdded()
	}

	cache.modified.Store(0)
	cache.added = cache.added[:0]
	cache.canFlush.Store(true)

	return err
}

func (cache *Cache) varFlush() error {
	// If a previously loaded entry has been modified,
	// we'll have to rewrite the whole file since there will most
	// likely be entries that appear after this one, which need to be shifted down.
	// Rewritting the whole file is the simplest solution.
	rewrite := cache.modified.Load() > 0

	return cache.flush(rewrite)
}

// func (cache *Cache) flush() error {
// 	// If a previously loaded entry has been modified,
// 	// we'll have to rewrite the whole file since there will most
// 	// likely be entries that appear after this one, which need to be shifted down.
// 	// Rewritting the whole file is the simpilest solution.
// 	rewrite := cache.modified.Load() > 0

// 	if rewrite {
// 		return cache.flushAll()
// 	} else {
// 		return cache.flushAdded()
// 	}
// }

func (cache *Cache) Flush() error {
	if err := cache.flush(true); err != nil {
		return fmt.Errorf("cache: flush: %w", err)
	}
	return nil
}

func (cache *Cache) shouldFlush() bool {
	cache.addedMux.RLock()
	defer cache.addedMux.RUnlock()
	return uint32(len(cache.added))+cache.modified.Load() > uint32(cache.flushThreshold)
}

func (cache *Cache) push(qc *quickCheck) error {
	_, ok := cache.sm.LoadOrStore(qc.Path(), qc)

	if ok {
		cache.sm.Store(qc.Path(), qc)
		cache.modified.Add(1)
	} else {
		cache.addedMux.Lock()
		cache.added = append(cache.added, qc)
		cache.addedMux.Unlock()
	}

	if cache.shouldFlush() && cache.canFlush.Swap(false) {
		return cache.varFlush()
	}

	return nil
}

func (cache *Cache) Push(path string, file *os.File) error {
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("cache: add: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("cache: add: %w", err)
	}

	qc := &quickCheck{
		path:         path,
		lastModified: stat.ModTime(),
		size:         stat.Size(),
		hash:         hash.Sum(nil),
	}

	if err := cache.push(qc); err != nil {
		return fmt.Errorf("cache: add: %w", err)
	}

	return nil
}

func (cache *Cache) Load() error {
	if _, err := cache.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("cache: load: %w", err)
	}

	scanner := bufio.NewScanner(cache.f)

	for scanner.Scan() {
		qc, err := cache.parseLine(scanner.Text())
		if err == nil {
			cache.push(qc)
		}
	}

	return nil
}

func (cache *Cache) Close() error {
	if cache.shouldFlush() {
		if err := cache.Flush(); err != nil {
			return err
		}
	}

	return cache.f.Close()
}

func New(f *os.File, flush ...int) *Cache {
	threshold := 0
	if len(flush) > 0 {
		threshold = flush[0]
	}

	cache := &Cache{
		f: f,

		flushThreshold: threshold,

		added: []*quickCheck{},
	}

	return cache
}

func Open(name string, flush ...int) (*Cache, error) {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0755)
	if err != nil {
		return nil, fmt.Errorf("cache: open: %w", err)
	}

	cache := New(file, flush...)

	if err := cache.Load(); err != nil {
		file.Close()
		return nil, err
	}

	return cache, nil
}
