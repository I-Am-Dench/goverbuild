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

func (qc *quickCheck) SysPath() string {
	return archive.ToSysPath(qc.path)
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
	SysPath() string
	LastModified() time.Time
	Size() int64
	Hash() []byte

	Check(*os.File) error
}

type RangeFunc = func(qc QuickCheck) bool

type Cache struct {
	sm sync.Map

	canFlush atomic.Bool

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

func (cache *Cache) ForEach(f RangeFunc) {
	cache.sm.Range(func(key, value any) bool {
		return f(value.(QuickCheck))
	})
}

// Writes the cache's contents to the provided writer.
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

func (cache *Cache) flushAdded() error {
	for _, qc := range cache.added {
		if _, err := fmt.Fprintf(cache.f, "%s,%d.%06d,%d,%x\n", qc.Path(), qc.LastModified().Unix(), qc.LastModified().Nanosecond(), qc.Size(), qc.Hash()); err != nil {
			return err
		}
	}

	return nil
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
	// If a previously loaded entry has been modified, we'll have
	// to rewrite the whole file since there will most likely be
	// entries that appear after this one, which need to be shifted down.
	// There are definitely ways to optimize which parts of the file needs
	// to rewritten, but rewritting the whole is the simplest solution.
	rewrite := cache.modified.Load() > 0

	return cache.flush(rewrite)
}

// Writes the cache's contents, in it's entirety, to the underlying *os.File, and resets
// the cache's internal state.
func (cache *Cache) Flush() error {
	if err := cache.flush(true); err != nil {
		return fmt.Errorf("cache: flush: %w", err)
	}
	return nil
}

func (cache *Cache) shouldFlush() bool {
	cache.addedMux.RLock()
	defer cache.addedMux.RUnlock()
	return uint32(len(cache.added))+cache.modified.Load() >= uint32(cache.flushThreshold)
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

// Returns the QuickCheck value for the given path.
func (cache *Cache) Get(path string) (QuickCheck, bool) {
	v, ok := cache.sm.Load(archive.ToArchivePath(path))
	if !ok {
		return nil, false
	}
	return v.(QuickCheck), true
}

// Sets the QuickCheck of the provided file to the given path as its key. The path is
// always passed through archive.ToArchivePath before being stored.
//
// If the number of changes (# modifications + # additions) >= the configured flush threshold,
// the cache's contents are flushed to underlying *os.File. The result of this flush is NOT
// guaranteed to be equivalent to calling Flush.
func (cache *Cache) Push(path string, file *os.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("cache: add: %w", err)
	}

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("cache: add: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("cache: add: %w", err)
	}

	qc := &quickCheck{
		path:         archive.ToArchivePath(path),
		lastModified: stat.ModTime(),
		size:         stat.Size(),
		hash:         hash.Sum(nil),
	}

	if err := cache.push(qc); err != nil {
		return fmt.Errorf("cache: add: %w", err)
	}

	return nil
}

func (cache *Cache) FlushThreshold(threshold int) {
	cache.flushThreshold = threshold
}

// Parses the contents of provided io.Reader and stores valid QuickCheck
// values into the cache. Read returns the first parse error that occurs
// or any error that is returned from the provided io.Reader. Valid QuickCheck
// values parsed prior to a parse error are still stored.
func (cache *Cache) Read(r io.Reader) error {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		qc, err := cache.parseLine(scanner.Text())
		if err != nil {
			return fmt.Errorf("cache: read: %w", err)
		}

		cache.sm.Store(qc.Path(), qc)
	}

	if scanner.Err() != nil {
		return fmt.Errorf("cache: read: %w", scanner.Err())
	}

	return nil
}

// Flushes the cache's contents, if necessary, to the underlying *os.File,
// and then closes the file.
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
	cache.canFlush.Store(true)

	return cache
}

func Open(name string, flush ...int) (*Cache, error) {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		return nil, fmt.Errorf("cache: open: %w", err)
	}

	cache := New(file, flush...)

	if err := cache.Read(file); err != nil {
		file.Close()
		return nil, err
	}

	return cache, nil
}
