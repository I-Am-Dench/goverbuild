package manifest

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/I-Am-Dench/goverbuild/archive"
)

var (
	SectionHeader = regexp.MustCompile(`\[([a-zA-Z0-9]+)]`)
)

type Sections = map[string][][]byte

type Entry struct {
	Path string
	archive.Info
}

type Manifest struct {
	Version int
	Name    string

	// Contains all manifest file section data without line endings.
	//
	// This includes custom section data and the raw, unparsed [version] and [files] data.
	Sections map[string][][]byte

	Entries []*Entry

	byPath map[string]*Entry
}

func (manifest *Manifest) GetEntry(path string) (*Entry, bool) {
	f, ok := manifest.byPath[strings.ToLower(filepath.ToSlash(path))]
	return f, ok
}

func (manifest *Manifest) WriteTo(w io.Writer) (int64, error) {
	versionChecksum := md5.New()
	fmt.Fprintf(versionChecksum, "%d", manifest.Version)

	versionName := manifest.Name
	if len(versionName) == 0 {
		versionName = "0"
	}

	written, err := fmt.Fprintf(w, "[version]\n%d,%x,%s\n", manifest.Version, versionChecksum.Sum(nil), versionName)
	if err != nil {
		return 0, fmt.Errorf("manifest: write version: %w", err)
	}

	if n, err := fmt.Fprintf(w, "[files]\n"); err != nil {
		return 0, fmt.Errorf("manifest: write files: %w", err)
	} else {
		written += n
	}

	for _, entry := range manifest.Entries {
		buf := &bytes.Buffer{}
		fmt.Fprintf(buf, "%s,%d,%x,%d,%x", entry.Path, entry.UncompressedSize, entry.UncompressedChecksum, entry.CompressedSize, entry.CompressedChecksum)

		checksum := md5.New()
		checksum.Write(buf.Bytes())
		fmt.Fprintf(buf, ",%x\n", checksum.Sum(nil))

		if n, err := w.Write(buf.Bytes()); err != nil {
			return 0, fmt.Errorf("manifest: write files: %w", err)
		} else {
			written += n
		}
	}

	for name, section := range manifest.Sections {
		if name == "files" || name == "version" {
			continue
		}

		if n, err := fmt.Fprintf(w, "[%s]\n", name); err != nil {
			return 0, fmt.Errorf("manifest: write section: %s: %w", name, err)
		} else {
			written += n
		}

		for _, line := range section {
			if n, err := fmt.Fprintf(w, "%s\n", line); err != nil {
				return 0, fmt.Errorf("manifest: write section: %s: %w", name, err)
			} else {
				written += n
			}
		}
	}

	return int64(written), nil
}

func parseSections(r io.Reader) Sections {
	sections := Sections{}

	lines := [][]byte{}
	currentSection := ""

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Bytes()

		match := SectionHeader.FindSubmatch(line)
		if len(match) == 0 {
			if len(line) > 0 {
				data := make([]byte, len(line))
				copy(data, line)
				lines = append(lines, data)
			}
		} else {
			if len(currentSection) > 0 {
				sections[currentSection] = lines
			}

			lines = [][]byte{}
			currentSection = string(match[1])
			sections[currentSection] = lines
		}
	}

	if len(currentSection) > 0 {
		sections[currentSection] = lines
	}

	return sections
}

func parseVersion(line []byte) (int, string, error) {
	parts := bytes.Split(line, []byte(","))
	if len(parts) < 2 {
		return 0, "", fmt.Errorf("manifest: malformed version line: %s", line)
	}

	version, err := strconv.Atoi(string(parts[0]))
	if err != nil {
		return 0, "", fmt.Errorf("manifest: malformed version number")
	}

	checkHash, _ := hex.DecodeString(string(parts[1]))

	hash := md5.New()
	hash.Write(parts[0])
	if !bytes.Equal(hash.Sum(nil), checkHash) {
		return 0, "", &MismatchedMd5HashError{string(line)}
	}

	name := ""
	if len(parts) > 2 {
		name = string(parts[2])
	}

	return version, name, nil
}

func parseEntry(line []byte) (*Entry, error) {
	parts := bytes.Split(line, []byte(","))
	if len(parts) < 6 {
		return nil, fmt.Errorf("manifest: malformed file line: %s", line)
	}

	uncompressedSize, err := strconv.Atoi(string(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("manifest: malformed file size: %s", line)
	}

	uncompressedChecksum, _ := hex.DecodeString(string(parts[2]))

	compressedSize, err := strconv.Atoi(string(parts[3]))
	if err != nil {
		return nil, fmt.Errorf("manifest: malformed compressed file size: %s", line)
	}

	compressedChecksum, _ := hex.DecodeString(string(parts[4]))

	checkHash, _ := hex.DecodeString(string(parts[5]))

	hash := md5.New()
	hash.Write(bytes.Join(parts[:5], []byte(",")))
	if !bytes.Equal(hash.Sum(nil), checkHash) {
		return nil, &MismatchedMd5HashError{string(line)}
	}

	return &Entry{
		Path: filepath.ToSlash(string(parts[0])),
		Info: archive.Info{
			UncompressedSize:     uint32(uncompressedSize),
			UncompressedChecksum: uncompressedChecksum,

			CompressedSize:     uint32(compressedSize),
			CompressedChecksum: compressedChecksum,
		},
	}, nil
}

// Uses the provided io.Reader to parse a manifest file per the specification found here: https://docs.lu-dev.net/en/latest/file-structures/manifest.html
//
// Manifest sections are not required to be in a specific order. The [version] section is allowed to come after the [files] section.
func Read(r io.Reader) (*Manifest, error) {
	manifest := &Manifest{
		Sections: parseSections(r),
		Entries:  []*Entry{},
		byPath:   map[string]*Entry{},
	}

	version, ok := manifest.Sections["version"]
	if ok && len(version) > 0 {
		vers, name, err := parseVersion(version[0])
		if errors.Is(err, ErrMismatchedHash) {
			return nil, err
		}

		if err == nil {
			manifest.Version = vers
			manifest.Name = name
		}
	}

	files, ok := manifest.Sections["files"]
	if !ok {
		manifest.Entries = []*Entry{}
		manifest.byPath = map[string]*Entry{}
		return manifest, nil
	}

	errs := []error{}
	for _, line := range files {
		entry, err := parseEntry(line)
		if err != nil {
			errs = append(errs, err)
		} else {
			manifest.Entries = append(manifest.Entries, entry)
			manifest.byPath[strings.ToLower(entry.Path)] = entry
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return manifest, nil
}

// Opens a file with the provided name and returns the resulting *Manifest from Read.
// This function always closes the opened file whether Read returned an error or not.
func Open(name string) (*Manifest, error) {
	file, err := os.OpenFile(name, os.O_RDONLY, 0755)
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	defer file.Close()

	return Read(file)
}

// Writes the given *Manifest to the file specified by name.
func Write(name string, manifest *Manifest) error {
	file, err := os.OpenFile(name, os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	defer file.Close()

	if _, err := manifest.WriteTo(file); err != nil {
		return err
	}

	return nil
}
