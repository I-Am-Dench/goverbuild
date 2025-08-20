package archive

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/snksoft/crc"
)

var (
	order = binary.LittleEndian
)

type Info struct {
	UncompressedSize     uint32
	UncompressedChecksum []byte

	CompressedSize     uint32
	CompressedChecksum []byte
}

func (info Info) verify(file *os.File, expectedSize int64, expectedChecksum []byte) error {
	stat, err := file.Stat()
	if err != nil {
		return err
	}

	checksum := md5.New()
	if _, err := io.Copy(checksum, file); err != nil {
		return err
	}

	sum := checksum.Sum(nil)
	if !(stat.Size() == expectedSize && bytes.Equal(sum, expectedChecksum)) {
		return fmt.Errorf("verify: (expected: %d,%x) != (actual: %d,%x)", expectedSize, expectedChecksum, stat.Size(), sum)
	}
	return nil
}

func (info Info) VerifyUncompressed(file *os.File) error {
	return info.verify(file, int64(info.UncompressedSize), info.UncompressedChecksum)
}

func (info Info) VerifyCompressed(file *os.File) error {
	return info.verify(file, int64(info.CompressedSize), info.CompressedChecksum)
}

var crcTable = crc.NewTable(&crc.Parameters{
	Width:      32,
	Polynomial: 0x04c11db7,
	Init:       0xffffffff,
})

func ToArchivePath(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ToLower(s), "/", "\\"))
}

func GetCrc(s string) uint32 {
	cleaned := ToArchivePath(s)
	data := append([]byte(cleaned), []byte{0x00, 0x00, 0x00, 0x00}...)

	hash := crc.NewHashWithTable(crcTable)
	hash.Write(data)
	return hash.CRC32()
}

var (
	ErrNotCataloged    = errors.New("not cataloged")
	ErrPackNotExist    = errors.New("pack does not exist")
	ErrCatalogMismatch = errors.New("catalog mismatch")
)

// A wrapper for a [*Catalog] and its corresponding [*Pack]'s.
type Archive struct {
	root   string
	closer bool

	catalog *Catalog
	packs   map[string]*Pack
}

// Returns a [*Pack], opening it if necessary, for a provided path
// recorded in the [Archive]'s catalog. FindPack will also return the
// [*CatalogRecord] associated with that path.
//
// Pack paths are cleaned before being joined with the root directory.
//
// FindPack returns an [ErrPackNotExist] error ONLY IF [OpenPack]
// returns an [os.ErrNotExist] error.
func (a *Archive) FindPack(path string) (*Pack, *CatalogRecord, error) {
	record, ok := a.catalog.Search(path)
	if !ok {
		return nil, nil, ErrNotCataloged
	}

	pack, ok := a.packs[strings.ToLower(record.PackName)]
	if ok {
		return pack, record, nil
	}

	pack, err := OpenPack(filepath.Join(a.root, filepath.Clean(record.PackName)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("%w: %s", ErrPackNotExist, record.PackName)
	}

	if err != nil {
		return nil, nil, err
	}

	a.packs[strings.ToLower(record.PackName)] = pack
	return pack, record, nil
}

// Stores the given file data in the [*Pack] corresponding
// to the provided path in the catalog.
//
// NOTE: The original client's Pack.dll IGNORES file data if
// the provided `compressed` flag DOES NOT match the corresponding
// `compressed` flag for that path in the catalog. Instead of
// ignoring the file data, Store will return a wrapped [ErrCatalogMismatch] error,
// which can be tested using [errors.Is].
func (a *Archive) Store(path string, info Info, compressed bool, r io.Reader) error {
	pack, record, err := a.FindPack(path)
	if err != nil {
		return fmt.Errorf("find pack: %w", err)
	}

	if compressed != record.IsCompressed {
		return fmt.Errorf("%w: wrong compression state", ErrCatalogMismatch)
	}

	return pack.Store(path, info, compressed, r)
}

// Closes all open packs within the [Archive], returning
// all errors returned by each [*Pack.Close], and then closes
// the underlying [*Catalog] ONLY if the [Archive] was created
// through a call to Open.
func (a *Archive) Close() error {
	errs := []error{}
	for _, pack := range a.packs {
		if err := pack.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if a.closer {
		if err := a.catalog.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// Creates an [Archive] with the provided root directory
// and [*Catalog].
//
// All packs opened from [Archive.FindPack] are opened relative
// to root.
func New(root string, catalog *Catalog) Archive {
	return Archive{
		root:    root,
		catalog: catalog,
		packs:   make(map[string]*Pack),
	}
}

// Creates an [Archive] with the provided root directory
// and named [*Catalog].
//
// Calling [*Archive.Close] on an Archive created through a call
// from Open causes the underlying catalog to be closed.
func Open(root, catalogPath string) (Archive, error) {
	catalog, err := OpenCatalog(catalogPath)
	if err != nil {
		return Archive{}, fmt.Errorf("archive: %w", err)
	}

	archive := New(root, catalog)
	archive.closer = true

	return archive, nil
}
