package cache

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/I-Am-Dench/goverbuild/archive"
)

type quickCheck struct {
	path         string
	lastModified time.Time
	size         int64
	hash         []byte
}

func (qc *quickCheck) Path() string {
	return qc.path
}

func (qc *quickCheck) ModTime() time.Time {
	return qc.lastModified
}

func (qc *quickCheck) Size() int64 {
	return qc.size
}

func (qc *quickCheck) Checksum() []byte {
	return qc.hash
}

func (qc *quickCheck) Check(stat os.FileInfo, info archive.Info) error {
	if !(RoundTime(stat.ModTime()).Equal(qc.lastModified) && stat.Size() == qc.size) {
		return fmt.Errorf("quickcheck: entry does not match disk: (expected: %s,%d) != (actual: %s,%d)", FormatTime(qc.lastModified), qc.size, FormatTime(stat.ModTime()), stat.Size())
	}

	if !(int64(info.UncompressedSize) == qc.size && bytes.Equal(info.UncompressedChecksum, qc.hash)) {
		return fmt.Errorf("quickcheck: entry does not match info: (expected: %d,%x) != (actual: %d,%x)", qc.size, qc.hash, info.UncompressedSize, info.UncompressedChecksum)
	}

	return nil
}

type QuickCheck interface {
	Path() string
	ModTime() time.Time
	Size() int64
	Checksum() []byte

	Check(os.FileInfo, archive.Info) error
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
	nanoseconds *= microToNano

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
		path:         strings.ToLower(filepath.FromSlash(parts[0])),
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
		n, err = fmt.Fprintf(w, "%s,%s,%d,%x\n", qc.Path(), FormatTime(qc.ModTime()), qc.Size(), qc.Checksum())
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
		if _, err := fmt.Fprintf(cache.f, "%s,%s,%d,%x\n", qc.Path(), FormatTime(qc.ModTime()), qc.Size(), qc.Checksum()); err != nil {
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

func (cache *Cache) shouldFlush(threshold int) bool {
	cache.addedMux.RLock()
	defer cache.addedMux.RUnlock()
	return uint32(len(cache.added))+cache.modified.Load() >= uint32(threshold)
}

func (cache *Cache) store(qc *quickCheck) error {
	_, ok := cache.sm.LoadOrStore(qc.Path(), qc)

	if ok {
		cache.sm.Store(qc.Path(), qc)
		cache.modified.Add(1)
	} else {
		cache.addedMux.Lock()
		cache.added = append(cache.added, qc)
		cache.addedMux.Unlock()
	}

	if cache.shouldFlush(cache.flushThreshold) && cache.canFlush.Swap(false) {
		return cache.varFlush()
	}

	return nil
}

// Returns the QuickCheck value for the given path.
func (cache *Cache) Get(path string) (QuickCheck, bool) {
	v, ok := cache.sm.Load(strings.ToLower(filepath.FromSlash(path)))
	if !ok {
		return nil, false
	}
	return v.(QuickCheck), true
}

// Sets the QuickCheck of the provided file to the given path as its key. The path is
// always passed through filepath.FromSlash before being stored.
//
// If the number of changes (# modifications + # additions) >= the configured flush threshold,
// the cache's contents are flushed to underlying *os.File. The result of this flush is NOT
// guaranteed to be equivalent to calling Flush.
func (cache *Cache) Store(path string, stat os.FileInfo, info archive.Info) error {
	qc := &quickCheck{
		path:         strings.ToLower(filepath.FromSlash(path)),
		lastModified: RoundTime(stat.ModTime()),
		size:         stat.Size(),
		hash:         info.UncompressedChecksum,
	}

	if err := cache.store(qc); err != nil {
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
	if cache.shouldFlush(1) {
		if err := cache.Flush(); err != nil {
			return err
		}
	}

	return cache.f.Close()
}

// Creates a new *Cache with an optional flush threshold.
// The provided *os.File is used as persistent storage for the cache's
// QuickCheck values. New does NOT parse and store the contents of the
// given file.
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

// Creates a new *Cache with an optional flush threshold from the file
// specified by name. Open creates the file if it doesn't already exist.
//
// If opening the file does not return an error, Open parses and stores the
// contents of that file. If an error occurs when reading the file, the file is
// immediately closed and that error is returned.
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
