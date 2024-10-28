// // REFACTOR: Catalog should probably stream files from .pki files instead of buffering all files into a slice,
// // but this is fine for the initial implementation.
// //
// // The resulting API for reading catalog files should be along the lines of:
// //
// //	for catalog.Next() {
// //	    file := catalog.File()
// //	    // do something with the file
// //	}
package catalog

// import (
// 	"bytes"
// 	"encoding/binary"
// 	"errors"
// 	"fmt"
// 	"io"
// 	"os"
// 	"sort"

// 	"github.com/I-Am-Dench/goverbuild/archive"
// 	"github.com/I-Am-Dench/goverbuild/internal"
// )

// type File struct {
// 	Name string

// 	Crc      uint32
// 	CrcLower int32
// 	CrcUpper int32

// 	IsCompressed bool
// }

// type Catalog struct {
// 	Version int32

// 	Files []*File
// }

// func (catalog *Catalog) Search(path string) (*File, bool) {
// 	crc := archive.GetCrc(path)

// 	i := sort.Search(len(catalog.Files), func(i int) bool { return catalog.Files[i].Crc >= crc })
// 	if i < len(catalog.Files) && catalog.Files[i].Crc == crc {
// 		return catalog.Files[i], true
// 	}
// 	return nil, false
// }

// func readFile(r internal.ReadSeekerAt, packnames []string) (*File, error) {
// 	file := &File{}

// 	if err := binary.Read(r, order, &file.Crc); err != nil {
// 		return nil, &ReadFileError{err, "crc"}
// 	}

// 	if err := binary.Read(r, order, &file.CrcLower); err != nil {
// 		return nil, &ReadFileError{err, "crcLower"}
// 	}

// 	if err := binary.Read(r, order, &file.CrcUpper); err != nil {
// 		return nil, &ReadFileError{err, "crcUpper"}
// 	}

// 	var packnameIndex uint32
// 	if err := binary.Read(r, order, &packnameIndex); err != nil {
// 		return nil, &ReadFileError{err, "packnameIndex"}
// 	}

// 	if packnameIndex >= uint32(len(packnames)) {
// 		return nil, &ReadFileError{fmt.Errorf("index out of bounds %d", packnameIndex), "packname"}
// 	}

// 	file.Name = packnames[packnameIndex]

// 	compression := [4]byte{}
// 	if err := binary.Read(r, order, compression[:]); err != nil {
// 		return nil, &ReadFileError{err, "isCompressed"}
// 	}

// 	file.IsCompressed = compression[0] != 0
// 	return file, nil
// }

// func readFiles(r internal.ReadSeekerAt) ([]*File, error) {
// 	var numFilenames uint32
// 	if err := binary.Read(r, order, &numFilenames); err != nil {
// 		return nil, fmt.Errorf("catalog: parseFiles: numFilenames: %w", err)
// 	}

// 	packnames := make([]string, 0, numFilenames)
// 	for i := uint32(0); i < numFilenames; i++ {
// 		var size uint32
// 		if err := binary.Read(r, order, &size); err != nil {
// 			return nil, fmt.Errorf("catalog: parseFiles: filename: %w", err)
// 		}

// 		data := make([]byte, size)
// 		if _, err := r.Read(data); err != nil {
// 			return nil, fmt.Errorf("catalog: parseFiles: filename: %w", err)
// 		}

// 		packnames = append(packnames, string(data))
// 	}

// 	var numFiles uint32
// 	if err := binary.Read(r, order, &numFiles); err != nil {
// 		return nil, fmt.Errorf("catalog: parseFiles: %w", err)
// 	}

// 	errs := []error{}

// 	files := make([]*File, 0, numFiles)
// 	for i := uint32(0); i < numFiles; i++ {
// 		file, err := readFile(r, packnames)
// 		if err != nil {
// 			errs = append(errs, err)
// 		} else {
// 			files = append(files, file)
// 		}
// 	}

// 	return files, errors.Join(errs...)
// }

// // Uses the provided io.Reader to parse a catalog file per the specification found here: https://docs.lu-dev.net/en/latest/file-structures/catalog.html
// //
// // This function is guaranteed to return a nil *Catalog when it encounters ANY error.
// func Read(r io.Reader) (*Catalog, error) {
// 	readSeeker, ok := r.(internal.ReadSeekerAt)
// 	if !ok {
// 		data, err := io.ReadAll(r)
// 		if err != nil {
// 			return nil, fmt.Errorf("catalog: %w", err)
// 		}

// 		readSeeker = bytes.NewReader(data)
// 	}

// 	catalog := &Catalog{}
// 	if err := binary.Read(readSeeker, order, &catalog.Version); err != nil {
// 		return nil, fmt.Errorf("catalog: %w", err)
// 	}

// 	if catalog.Version != CatalogVersion {
// 		return nil, fmt.Errorf("catalog: unsupported version: %d", catalog.Version)
// 	}

// 	files, err := readFiles(readSeeker)
// 	if err != nil {
// 		return nil, err
// 	}
// 	catalog.Files = files

// 	return catalog, nil
// }

// // Opens a file with the provided name and returns the resulting *Catalog from Read.
// // This function always closes the opened file whether Read returned an error or not.
// func Open(path string) (*Catalog, error) {
// 	file, err := os.OpenFile(path, os.O_RDONLY, 0755)
// 	if err != nil {
// 		return nil, fmt.Errorf("catalog: open: %w", err)
// 	}
// 	defer file.Close()

// 	file.Seek(0, io.SeekStart)

// 	catalog, err := Read(file)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return catalog, err
// }
