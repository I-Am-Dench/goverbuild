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
	"regexp"
	"strconv"
)

var (
	SectionHeader = regexp.MustCompile(`\[([a-zA-Z0-9]+)]`)
)

type Sections = map[string][][]byte

type File struct {
	name string

	originalSize int
	originalHash []byte

	compressedSize int
	compressedHash []byte
}

func (file *File) Name() string {
	return file.name
}

func (file *File) String() string {
	return file.name
}

type Manifest struct {
	Version int
	Name    string

	Files []*File
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

func parseFile(line []byte) (*File, error) {
	parts := bytes.Split(line, []byte(","))
	if len(parts) < 6 {
		return nil, fmt.Errorf("manifest: malformed file line: %s", line)
	}

	originalSize, err := strconv.Atoi(string(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("manifest: malformed file size: %s", line)
	}

	originalHash, _ := hex.DecodeString(string(parts[2]))

	compressedSize, err := strconv.Atoi(string(parts[3]))
	if err != nil {
		return nil, fmt.Errorf("manifest: malformed compressed file size: %s", line)
	}

	compressedHash, _ := hex.DecodeString(string(parts[4]))

	checkHash, _ := hex.DecodeString(string(parts[5]))

	hash := md5.New()
	hash.Write(bytes.Join(parts[:5], []byte(",")))
	if !bytes.Equal(hash.Sum(nil), checkHash) {
		return nil, &MismatchedMd5HashError{string(line)}
	}

	return &File{
		name: string(parts[0]),

		originalSize: originalSize,
		originalHash: originalHash,

		compressedSize: compressedSize,
		compressedHash: compressedHash,
	}, nil
}

// Uses the provided io.Reader to parse a manifest file per the specification found here: https://docs.lu-dev.net/en/latest/file-structures/manifest.html
//
// Manifest sections are not required to be in a specific order. The [version] section is allowed to come after the [files] section.
func Read(r io.Reader) (*Manifest, error) {
	sections := parseSections(r)

	manifest := &Manifest{
		Files: []*File{},
	}

	version, ok := sections["version"]
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

	files, ok := sections["files"]
	if !ok {
		manifest.Files = []*File{}
		return manifest, nil
	}

	errs := []error{}
	for _, line := range files {
		file, err := parseFile(line)
		if err != nil {
			errs = append(errs, err)
		} else {
			manifest.Files = append(manifest.Files, file)
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
	file, err := os.OpenFile(name, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	defer file.Close()

	return Read(file)
}
