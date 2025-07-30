package pack_test

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"slices"
	"testing"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/internal/binarytree"
	"github.com/I-Am-Dench/goverbuild/archive/pack"
	"github.com/I-Am-Dench/goverbuild/compress/segmented"
)

const (
	NumInitial = 10
	NumUpdates = 20
	NumAppends = 15
)

var (
	order = binary.LittleEndian
)

type PackBuf struct {
	*bytes.Buffer
}

func (buf *PackBuf) WriteSignature() {
	buf.Write(pack.Signature)
}

func (buf *PackBuf) WriteData(files ...*File) {
	for _, file := range files {
		file.DataPointer = uint32(buf.Len())
		buf.Write(file.Data)
		buf.Write(pack.Terminator)
	}
}

func (buf *PackBuf) writeChecksum(data []byte) {
	b := [36]byte{}
	hex.Encode(b[:32], data)
	buf.Write(b[:])
}

func (buf *PackBuf) WriteRecords(files []*File) {
	ordered := slices.Clone(files)
	slices.SortFunc(ordered, func(a, b *File) int { return int(archive.GetCrc(a.Name)) - int(archive.GetCrc(b.Name)) })

	binarytree.UpdateIndices(ordered)

	binary.Write(buf, order, uint32(len(files)))
	for _, file := range ordered {
		binary.Write(buf, order, archive.GetCrc(file.Name))
		binary.Write(buf, order, file.LowerIndex)
		binary.Write(buf, order, file.UpperIndex)

		binary.Write(buf, order, file.Info.UncompressedSize)
		buf.writeChecksum(file.Info.UncompressedChecksum)
		binary.Write(buf, order, file.Info.CompressedSize)
		buf.writeChecksum(file.Info.CompressedChecksum)

		binary.Write(buf, order, file.DataPointer)

		c := [4]byte{}
		if file.Compressed {
			c[0] = 1
		}
		buf.Write(c[:])
	}
}

func (buf *PackBuf) WriteTail(recordsPointer, revision uint32) {
	binary.Write(buf, order, recordsPointer)
	binary.Write(buf, order, revision)
}

type File struct {
	Name string
	Data []byte

	Info archive.Info

	binarytree.Indices
	Compressed bool

	DataPointer uint32
}

func (file *File) TreeIndices() *binarytree.Indices {
	return &file.Indices
}

func createData() []byte {
	numBytes := rand.Intn(128) + 128
	s := make([]byte, numBytes)
	for i := range s {
		s[i] = byte(rand.Int())
	}
	return s
}

func calculateInfo(data []byte) archive.Info {
	buf := bytes.NewBuffer(data)

	uncompressedSize := buf.Len()
	uncompressedChecksum := md5.New()

	compressedChecksum := md5.New()

	writer := segmented.NewDataWriter(io.MultiWriter(io.Discard, compressedChecksum))
	buf.WriteTo(io.MultiWriter(writer, uncompressedChecksum))
	writer.Close()

	return archive.Info{
		UncompressedSize:     uint32(uncompressedSize),
		UncompressedChecksum: uncompressedChecksum.Sum(nil),

		CompressedSize:     uint32(writer.BytesWritten()),
		CompressedChecksum: compressedChecksum.Sum(nil),
	}
}

func popFile(files []*File, i int) ([]*File, *File) {
	file := files[i]
	return slices.Delete(files, i, i+1), file
}

func updateFile(files []*File, i int) ([]*File, *File) {
	var file *File
	files, file = popFile(files, i)

	file.Data = createData()
	file.Info = calculateInfo(file.Data)
	files = append(files, file)

	return files, file
}

type Env struct {
	Revision uint32
	Files    []*File
}

func setup() (*Env, func(*testing.T), error) {
	if err := os.MkdirAll("testdata", 0755); err != nil {
		return nil, nil, fmt.Errorf("setup: %w", err)
	}

	files := make([]*File, NumInitial)
	for i := range files {
		data := createData()

		files[i] = &File{
			Name: fmt.Sprintf("files/data%d", i),
			Data: data,

			Info: calculateInfo(data),
		}
	}

	return &Env{
			Files: files,
		}, func(t *testing.T) {
			if err := os.RemoveAll("testdata"); err != nil {
				t.Log(err)
			}
		}, nil
}

func generateExpectedPack(revision uint32, files []*File) PackBuf {
	pack := PackBuf{&bytes.Buffer{}}
	pack.WriteSignature()

	pack.WriteData(files...)

	recordsPointer := pack.Len()
	pack.WriteRecords(files)
	pack.WriteTail(uint32(recordsPointer), revision)

	return pack
}

func checkPack(t *testing.T, pack *pack.Pack, files []*File, revision uint32) {
	records := pack.Records()
	if len(records) != len(files) {
		t.Errorf("expected %d files but got %d", len(files), len(records))
	}

	for i := 1; i < len(records); i++ {
		a := records[i-1]
		b := records[i]
		if a.Crc > b.Crc {
			t.Errorf("bad record order: %d > %d", a.Crc, b.Crc)
		} else if a.Crc == b.Crc {
			t.Errorf("duplicate records: %d", a.Crc)
		}
	}

	actual := &bytes.Buffer{}
	if _, err := pack.WriteTo(actual); err != nil {
		t.Fatal(err)
	}

	expected := generateExpectedPack(revision, files)

	if actual.Len() != expected.Len() {
		t.Fatalf("expected %d bytes but got %d", expected.Len(), actual.Len())
	}

	if !bytes.Equal(expected.Bytes(), actual.Bytes()) {
		t.Errorf("\nexpected = %v\nactual   = %v", expected.Bytes(), actual.Bytes())
		t.Fatal("written data does not match")
	}
}

func testStore(pack *pack.Pack, env *Env) func(t *testing.T) {
	return func(t *testing.T) {
		for _, file := range env.Files {
			if err := pack.Store(file.Name, file.Info, file.Compressed, bytes.NewBuffer(file.Data)); err != nil {
				t.Fatal(err)
			}
		}

		checkPack(t, pack, env.Files, env.Revision)

		env.Revision++
		if err := pack.Flush(); err != nil {
			t.Fatal(err)
		}
	}
}

func testUpdate(pack *pack.Pack, env *Env) func(t *testing.T) {
	return func(t *testing.T) {
		for n := 0; n < NumUpdates; n++ {
			i := rand.Intn(len(env.Files))

			var updated *File
			env.Files, updated = updateFile(env.Files, i)

			if err := pack.Store(updated.Name, updated.Info, updated.Compressed, bytes.NewBuffer(updated.Data)); err != nil {
				t.Fatal(err)
			}

			checkPack(t, pack, env.Files, env.Revision)
		}

		env.Revision++
		if err := pack.Flush(); err != nil {
			t.Fatal(err)
		}
	}
}

func testAppend(pack *pack.Pack, env *Env) func(t *testing.T) {
	return func(t *testing.T) {
		appended := make([]*File, NumAppends)
		for i := range appended {
			data := createData()

			appended[i] = &File{
				Name: fmt.Sprintf("files/append%d", i),
				Data: data,

				Info: calculateInfo(data),
			}

			name := appended[i].Name
			t.Logf("%s -> %d", name, archive.GetCrc(name))
		}

		for _, file := range appended {
			if err := pack.Store(file.Name, file.Info, file.Compressed, bytes.NewBuffer(file.Data)); err != nil {
				t.Fatal(err)
			}
		}

		env.Files = append(env.Files, appended...)

		checkPack(t, pack, env.Files, env.Revision)
	}
}

func testSave(f *os.File, p *pack.Pack, env *Env) func(t *testing.T) {
	return func(t *testing.T) {
		data := createData()
		file := &File{
			Name: "files/save/somedata",
			Data: data,

			Info: calculateInfo(data),
		}
		env.Files = append(env.Files, file)

		if err := p.Store(file.Name, file.Info, file.Compressed, bytes.NewBuffer(file.Data)); err != nil {
			t.Fatal(err)
		}

		if err := p.Close(); err != nil {
			t.Fatal(err)
		}

		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		pack, err := pack.Open("testdata/pack.pk")
		if err != nil {
			t.Fatal(err)
		}
		defer pack.Close()

		checkPack(t, pack, env.Files, env.Revision)
	}
}

func TestStore(t *testing.T) {
	env, cleanup, err := setup()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer cleanup(t)

	for _, file := range env.Files {
		t.Logf("%s -> %d", file.Name, archive.GetCrc(file.Name))
	}

	file, err := os.OpenFile("testdata/pack.pk", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	pack, err := pack.New(file)
	if err != nil {
		t.Fatal(err)
	}
	defer pack.Close()

	t.Run("store", testStore(pack, env))
	t.Run("update", testUpdate(pack, env))
	t.Run("append", testAppend(pack, env))
	t.Run("save", testSave(file, pack, env))
}

func testTruncate(t *testing.T) {
	const (
		numFilesSize = 4
		tailSize     = 8
	)

	var EmptyPackSize = len(pack.Signature) + numFilesSize + tailSize

	file, err := os.OpenFile("testdata/pack.pk", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0755)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if _, err := file.Write(createData()); err != nil {
		t.Fatal(err)
	}

	stat, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("filled pack with %d random bytes", stat.Size())

	pack, err := pack.New(file)
	if err != nil {
		t.Fatal(err)
	}

	if err := pack.Flush(); err != nil {
		t.Fatal(err)
	}

	stat, err = file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if stat.Size() != int64(EmptyPackSize) {
		t.Errorf("empty pack: expected %d bytes but got %d", EmptyPackSize, stat.Size())
	}
}

func TestRead(t *testing.T) {
	_, cleanup, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t)

	t.Run("trunate_empty", testTruncate)
}
