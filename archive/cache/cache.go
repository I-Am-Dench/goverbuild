package cache

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/I-Am-Dench/goverbuild/archive"
)

type QuickCheck interface {
	Path() string
	ModTime() time.Time
	Size() int64
	Checksum() []byte

	Check(os.FileInfo, archive.Info) error
}

var quickCheckPattern = regexp.MustCompile(`^([^,]+),([0-9]+\.[0-9]{6}),([0-9]+),([0-9a-fA-F]+)$`)

const (
	fieldFileName = iota + 1
	fieldModTime
	fieldSize
	fieldChecksum
)

type quickCheck struct {
	path     string
	modTime  time.Time
	size     int64
	checksum []byte
}

func (q *quickCheck) MarshalText() ([]byte, error) {
	return fmt.Appendf([]byte{}, "%s,%s,%d,%x\n", q.path, FormatTime(q.modTime), q.size, q.Checksum()), nil
}

func (q *quickCheck) parseModTime(s string) (time.Time, error) {
	rawSeconds, rawNanoseconds, ok := strings.Cut(s, ".")
	if !ok {
		return time.Time{}, errors.New("parse mod time: no nanoseconds")
	}

	seconds, err := strconv.ParseInt(rawSeconds, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse mod time: %v", err)
	}

	nanoseconds, err := strconv.ParseInt(rawNanoseconds, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse mod time: %v", err)
	}
	nanoseconds *= microToNano

	return time.Unix(seconds, nanoseconds), nil
}

func (q *quickCheck) UnmarshalText(text []byte) error {
	text = bytes.TrimSpace(text)

	matches := quickCheckPattern.FindSubmatch(text)
	if matches == nil {
		return fmt.Errorf("quick check: malformed line: %s", string(text))
	}

	modTime, err := q.parseModTime(string(matches[fieldModTime]))
	if err != nil {
		return fmt.Errorf("quick check: %v", err)
	}

	size, _ := strconv.ParseInt(string(matches[fieldSize]), 10, 64)
	checksum, _ := hex.DecodeString(string(matches[fieldChecksum]))

	q.path = filepath.FromSlash(string(matches[fieldFileName]))
	q.modTime = modTime
	q.size = size
	q.checksum = checksum

	return nil
}

func (q *quickCheck) Path() string {
	return q.path
}

func (q *quickCheck) ModTime() time.Time {
	return q.modTime
}

func (q *quickCheck) Size() int64 {
	return q.size
}

func (q *quickCheck) Checksum() []byte {
	return q.checksum
}

func (q *quickCheck) Check(stat os.FileInfo, info archive.Info) error {
	if !RoundTime(stat.ModTime()).Equal(q.modTime) || stat.Size() != q.size {
		return fmt.Errorf("entry does not match stat: (expected: %s,%d) != (actual: %s,%d)", FormatTime(q.modTime), q.size, FormatTime(stat.ModTime()), stat.Size())
	}

	if int64(info.UncompressedSize) != q.size || !bytes.Equal(info.UncompressedChecksum, q.checksum) {
		return fmt.Errorf("entry does not match info: (expected: %d,%x) != (actual: %d,%x)", q.size, q.checksum, info.UncompressedSize, info.UncompressedChecksum)
	}

	return nil
}

type RangeFunc = func(qc QuickCheck) bool

// File type: txt
//
// The cache file, named quickcheck.txt in the orignial patcher,
// stored a list of comma-separated values for the states of unpacked
// client resources from when the patcher was last run. If any of
// the non-path values (modification time, size, or uncompressed checksum)
// have changed, the unpacked resource should be reinstalled.
//
// Each line in the file takes the form:
//
//	%s,%d.%06d,%d,%x
//
// With these values associated with an unpacked resource:
//  1. The relative path
//  2. The modification time seconds
//  3. The modification time milliseconds (padded to 6 digits)
//  4. The size of the resource
//  5. The md5 hash of the resource
type Cache struct {
	sm sync.Map
}

func (c *Cache) store(path string, qc *quickCheck) {
	c.sm.Store(strings.ToLower(path), qc)
}

func (c *Cache) Store(path string, stat os.FileInfo, info archive.Info) {
	qc := &quickCheck{
		path:     filepath.FromSlash(path),
		modTime:  RoundTime(stat.ModTime()),
		size:     stat.Size(),
		checksum: info.UncompressedChecksum,
	}
	c.store(qc.path, qc)
}

func (c *Cache) Load(path string) (QuickCheck, bool) {
	v, ok := c.sm.Load(strings.ToLower(filepath.FromSlash(path)))
	if !ok {
		return nil, false
	}
	return v.(QuickCheck), true
}

func (c *Cache) Range(f RangeFunc) {
	c.sm.Range(func(key, value any) bool {
		return f(value.(QuickCheck))
	})
}

func (c *Cache) All() iter.Seq[QuickCheck] {
	return func(yield func(QuickCheck) bool) {
		c.sm.Range(func(_, value any) bool {
			return yield(value.(QuickCheck))
		})
	}
}

func (c *Cache) Len() (length int) {
	c.sm.Range(func(key, value any) bool {
		length++
		return true
	})
	return length
}

// Creates a [*Cache] from the entries scanned from
// the provided [io.Reader]. Read immediately
// returns an error if it fails to scan a line or
// parse a quick check entry.
func Read(r io.Reader) (*Cache, error) {
	scanner := bufio.NewScanner(r)

	cache := &Cache{}
	for scanner.Scan() {
		qc := &quickCheck{}
		if err := qc.UnmarshalText(scanner.Bytes()); err != nil {
			return nil, fmt.Errorf("cache: read: %v", err)
		}
		cache.store(qc.path, qc)
	}

	if scanner.Err() != nil {
		return nil, fmt.Errorf("cache: read: %v", scanner.Err())
	}

	return cache, nil
}

// Writes a [*Cache]'s quick check entries to the
// provided [io.Writer].
func Write(w io.Writer, c *Cache) (err error) {
	c.sm.Range(func(key, value any) bool {
		qc := value.(*quickCheck)
		text, _ := qc.MarshalText()
		if _, err = w.Write(text); err != nil {
			return false
		}
		return true
	})

	if err != nil {
		err = fmt.Errorf("cache: write: %v", err)
	}

	return nil
}

// Creates a [*Cache] from the entries scanned from
// the named file. Read immediately
// returns an error if it fails to scan a line or
// parse a quick check entry.
func ReadFile(name string) (*Cache, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("cache: read: %w", err)
	}
	defer file.Close()

	cache, err := Read(file)
	if err != nil {
		return nil, err
	}

	return cache, nil
}

// Writes a [*Cache]'s quick check entries to the
// named file.
func WriteFile(name string, c *Cache) error {
	file, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("cache: write: %w", err)
	}
	defer file.Close()

	return Write(file, c)
}
