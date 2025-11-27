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
	"slices"
	"strconv"
	"strings"

	"github.com/I-Am-Dench/goverbuild/archive"
)

var (
	sectionHeaderPattern = regexp.MustCompile(`\[([a-zA-Z0-9]+)]`)
	entryPattern         = regexp.MustCompile(`^([^,]+),([0-9]+),([0-9a-fA-F]+),([0-9]+),([0-9a-fA-F]+),([0-9a-fA-F]+)$`)
)

const (
	fieldFileName = iota + 1
	fieldUncompressedSize
	fieldUncompressedChecksum
	fieldCompressedSize
	fieldCompressedChecksum
	fieldEntryChecksum
)

type Sections = map[string][][]byte

type Entry struct {
	Path string
	archive.Info
}

func (e *Entry) MarshalText() ([]byte, error) {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "%s,%d,%x,%d,%x", e.Path, e.UncompressedSize, e.UncompressedChecksum, e.CompressedSize, e.CompressedChecksum)

	checksum := md5.New()
	checksum.Write(buf.Bytes())
	fmt.Fprintf(buf, ",%x", checksum.Sum(nil))

	return buf.Bytes(), nil
}

func (e *Entry) UnmarshalText(text []byte) error {
	text = bytes.TrimSpace(text)

	matches := entryPattern.FindSubmatch(text)
	if matches == nil {
		return fmt.Errorf("entry: malformed line: %s", string(text))
	}

	uncompressedSize, err := strconv.ParseUint(string(matches[fieldUncompressedSize]), 10, 32)
	if err != nil {
		return fmt.Errorf("entry: %v", err)
	}

	uncompressedChecksum, _ := hex.DecodeString(string(matches[fieldUncompressedChecksum]))

	compressedSize, err := strconv.ParseUint(string(matches[fieldCompressedSize]), 10, 32)
	if err != nil {
		return fmt.Errorf("entry: %v", err)
	}

	compressedChecksum, _ := hex.DecodeString(string(matches[fieldCompressedChecksum]))

	entryChecksum, _ := hex.DecodeString(string(matches[fieldEntryChecksum]))

	hash := md5.New()
	hash.Write(bytes.Join(matches[1:len(matches)-1], []byte(",")))
	if actual := hash.Sum(nil); !bytes.Equal(actual, entryChecksum) {
		return &MismatchedChecksumError{entryChecksum, actual}
	}

	e.Path = strings.ToLower(filepath.ToSlash(string(matches[fieldFileName])))
	e.Info = archive.Info{
		UncompressedSize:     uint32(uncompressedSize),
		UncompressedChecksum: uncompressedChecksum,
		CompressedSize:       uint32(compressedSize),
		CompressedChecksum:   compressedChecksum,
	}

	return nil
}

// File type: [txt]
//
// [txt]: https://docs.lu-dev.net/en/latest/file-structures/manifest.html
type Manifest struct {
	Version int
	Name    string

	// Contains all manifest file section data without line endings.
	//
	// This includes custom section data and the raw, unparsed [version] and [files] data
	Sections map[string][][]byte

	entries map[string]Entry
}

func (m Manifest) Entries() []Entry {
	entries := []Entry{}
	for _, entry := range m.entries {
		entries = append(entries, entry)
	}
	return entries
}

func (m Manifest) GetEntry(path string) (Entry, bool) {
	f, ok := m.entries[strings.ToLower(filepath.ToSlash(path))]
	return f, ok
}

func (m *Manifest) AddEntries(entries ...Entry) {
	if m.entries == nil {
		m.entries = make(map[string]Entry)
	}

	for _, entry := range entries {
		entry.Path = strings.ToLower(filepath.ToSlash(entry.Path))
		m.entries[entry.Path] = entry
	}
}

func parseSections(r io.Reader) Sections {
	sections := Sections{}

	lines := [][]byte{}
	currentSection := ""

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Bytes()

		match := sectionHeaderPattern.FindSubmatch(line)
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
	if actual := hash.Sum(nil); !bytes.Equal(actual, checkHash) {
		return 0, "", &MismatchedChecksumError{checkHash, actual}
	}

	name := ""
	if len(parts) > 2 {
		name = string(parts[2])
	}

	return version, name, nil
}

// Uses the provided io.Reader to parse a manifest file per the specification found here: https://docs.lu-dev.net/en/latest/file-structures/manifest.html
//
// Manifest sections are not required to be in a specific order. The [version] section is allowed to come after the [files] section.
func Read(r io.Reader) (*Manifest, error) {
	manifest := &Manifest{
		Sections: parseSections(r),
		entries:  make(map[string]Entry),
	}

	version, ok := manifest.Sections["version"]
	if ok && len(version) > 0 {
		vers, name, err := parseVersion(version[0])

		checksumErr := &MismatchedChecksumError{}
		if errors.As(err, &checksumErr) {
			return nil, err
		}

		if err == nil {
			manifest.Version = vers
			manifest.Name = name
		}
	}

	files, ok := manifest.Sections["files"]
	if !ok {
		return manifest, nil
	}

	errs := []error{}
	for _, line := range files {
		entry := Entry{}
		if err := entry.UnmarshalText(line); err != nil {
			errs = append(errs, err)
		} else {
			manifest.entries[entry.Path] = entry
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return manifest, nil
}

// Writes the provided [*Manifest] to the provided [io.Writer].
//
// Manifest entries are written lexicographically by their paths.
func Write(w io.Writer, manifest *Manifest) error {
	versionChecksum := md5.New()
	fmt.Fprintf(versionChecksum, "%d", manifest.Version)

	versionName := manifest.Name
	if len(versionName) == 0 {
		versionName = "0"
	}

	if _, err := fmt.Fprintf(w, "[version]\n%d,%x,%s\n", manifest.Version, versionChecksum.Sum(nil), versionName); err != nil {
		return fmt.Errorf("manifest: write version: %w", err)
	}

	if _, err := io.WriteString(w, "[files]\n"); err != nil {
		return fmt.Errorf("manifest: write files: %w", err)
	}

	entries := manifest.Entries()
	slices.SortFunc(entries, func(a, b Entry) int {
		return strings.Compare(a.Path, b.Path)
	})

	for _, entry := range entries {
		data, _ := entry.MarshalText()

		if _, err := w.Write(append(data, []byte("\n")...)); err != nil {
			return fmt.Errorf("manifest: write files: %w", err)
		}
	}

	for name, section := range manifest.Sections {
		if name == "files" || name == "version" {
			continue
		}

		if _, err := fmt.Fprint(w, "[", name, "]\n"); err != nil {
			return fmt.Errorf("manifest: write section: %s: %w", name, err)
		}

		for _, line := range section {
			if _, err := fmt.Fprintln(w, string(line)); err != nil {
				return fmt.Errorf("manifest write section: %s: %w", name, err)
			}
		}
	}

	return nil
}

// Opens a file with the provided name and returns the resulting [*Manifest] from [Read].
// This function always closes the opened file whether [Read] returned an error or not.
func ReadFile(name string) (*Manifest, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	defer file.Close()

	return Read(file)
}

// Writes the given [*Manifest] to the file specified by name.
func WriteFile(name string, manifest *Manifest) error {
	file, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	defer file.Close()

	return Write(file, manifest)
}
