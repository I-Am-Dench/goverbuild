package archive_test

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/I-Am-Dench/goverbuild/archive"
	"github.com/I-Am-Dench/goverbuild/archive/internal/binarytree"
	"github.com/I-Am-Dench/goverbuild/compress/segmented"
)

var (
	packSignature = append([]byte("ndpk"), 0x01, 0xff, 0x00)
	packDivider   = []byte{0xff, 0x00, 0x00, 0xdd, 0x00}
	order         = binary.LittleEndian
)

type PackRecord struct {
	Name string
	Data []byte

	Info archive.Info

	binarytree.Indices
	Compressed bool

	DataPointer uint32
}

func newRecord(name, data string, compressed bool) PackRecord {
	info, _ := calculateInfo([]byte(data))
	return PackRecord{
		Name:       name,
		Data:       []byte(data),
		Compressed: compressed,
		Info:       info,
	}
}

func (r *PackRecord) TreeIndices() *binarytree.Indices {
	return &r.Indices
}

func (r *PackRecord) appendChecksum(buf, checksum []byte) []byte {
	data := [36]byte{}
	hex.Encode(data[:32], checksum)
	return append(buf, data[:]...)
}

func (r *PackRecord) Marshal() []byte {
	buf := make([]byte, 0, 100)
	buf = order.AppendUint32(buf, archive.GetCrc(r.Name))
	buf = order.AppendUint32(buf, uint32(r.LowerIndex))
	buf = order.AppendUint32(buf, uint32(r.UpperIndex))

	buf = order.AppendUint32(buf, r.Info.UncompressedSize)
	buf = r.appendChecksum(buf, r.Info.UncompressedChecksum)
	buf = order.AppendUint32(buf, r.Info.CompressedSize)
	buf = r.appendChecksum(buf, r.Info.CompressedChecksum)

	buf = order.AppendUint32(buf, r.DataPointer)

	if r.Compressed {
		buf = append(buf, 1, 0, 0, 0)
	} else {
		buf = append(buf, 0, 0, 0, 0)
	}

	return buf
}

type TestPack struct {
	buf bytes.Buffer
}

func (p *TestPack) WriteData(records []*PackRecord) {
	for _, record := range records {
		record.DataPointer = uint32(p.buf.Len())
		p.buf.Write(record.Data)
		p.buf.Write(packDivider)
	}
}

func (p *TestPack) WriteRecords(records []*PackRecord) {
	ordered := slices.Clone(records)
	slices.SortFunc(ordered, func(a, b *PackRecord) int { return int(archive.GetCrc(a.Name)) - int(archive.GetCrc(b.Name)) })

	binarytree.UpdateIndices(ordered)

	binary.Write(&p.buf, order, uint32(len(records)))
	for _, record := range ordered {
		p.buf.Write(record.Marshal())
	}
}

func (p *TestPack) Generate(revision uint32, records []*PackRecord) {
	p.buf.Write(packSignature)

	p.WriteData(records)

	recordsPointer := p.buf.Len()
	p.WriteRecords(records)
	binary.Write(&p.buf, order, uint32(recordsPointer))
	binary.Write(&p.buf, order, revision)
}

func createCompressableData() []byte {
	const chars = "abcdefghijklmnopqrstuvwxyz1234567890"

	numBytes := rand.Intn(128) + 128
	data := make([]byte, 0, numBytes)

	c := rand.Intn(len(chars))
	for len(data) < numBytes {
		n := rand.Intn(numBytes - len(data))
		for i := 0; i < max(n, 5); i++ {
			data = append(data, byte(chars[c]))
		}
		c = (c + 1) % len(chars)
	}

	return data
}

func createData() ([]byte, bool) {
	if rand.Intn(2) == 1 {
		return createCompressableData(), true
	}

	numBytes := rand.Intn(128) + 128
	s := make([]byte, numBytes)
	for i := range s {
		s[i] = byte(rand.Int())
	}
	return s, false
}

func calculateInfo(data []byte) (info archive.Info, compressedData []byte) {
	buf := bytes.NewBuffer(data)

	uncompressedSize := buf.Len()
	uncompressedChecksum := md5.New()

	compressedChecksum := md5.New()

	compressedBuf := bytes.Buffer{}

	w := segmented.NewDataWriter(io.MultiWriter(&compressedBuf, compressedChecksum))
	buf.WriteTo(io.MultiWriter(w, uncompressedChecksum))
	w.Close()

	return archive.Info{
		UncompressedSize:     uint32(uncompressedSize),
		UncompressedChecksum: uncompressedChecksum.Sum(nil),

		CompressedSize:     uint32(w.BytesWritten()),
		CompressedChecksum: compressedChecksum.Sum(nil),
	}, compressedBuf.Bytes()
}

type Env struct {
	Dir string

	Revision uint32
	Records  []*PackRecord
}

func (e *Env) TestPack(t *testing.T, name string, f func(*archive.Pack)) {
	file, err := os.Create(filepath.Join(e.Dir, "store.pk"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()

	pack, err := archive.NewPack(file)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pack.Close(); err != nil {
			t.Error(err)
		}
	}()

	f(pack)

	if err := pack.Flush(); err != nil {
		t.Fatal(err)
	}
}

func (e *Env) UpdateRecord(i int) *PackRecord {
	record := e.Records[i]
	records := slices.Delete(e.Records, i, i+1)

	data, compressable := createData()

	info, compressed := calculateInfo(data)
	if compressable {
		data = compressed
	}

	record.Data = data
	record.Compressed = compressable
	record.Info = info
	e.Records = append(records, record)

	return record
}

func (e *Env) Dump(t *testing.T, expected, actual []byte) {
	dir := filepath.Join("testdata", "dump")

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Logf("dump: %v", err)
		return
	}

	name := filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "-"))
	if err := os.WriteFile(fmt.Sprint(name, ".expected.bin"), expected, 0664); err != nil {
		t.Logf("dump: %v", err)
	}

	if err := os.WriteFile(fmt.Sprint(name, ".actual.bin"), actual, 0664); err != nil {
		t.Logf("dump: %v", err)
	}
}

func (e *Env) Check(t *testing.T, pack *archive.Pack) {
	records := pack.Records()
	if len(records) != len(e.Records) {
		t.Errorf("expected %d records but got %d", len(records), len(e.Records))
		return
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

	expected := TestPack{}
	expected.Generate(e.Revision, e.Records)

	actual := bytes.Buffer{}
	if _, err := pack.WriteTo(&actual); err != nil {
		t.Fatal(err)
	}

	if actual.Len() != expected.buf.Len() {
		t.Errorf("expected %d bytes but got %d", expected.buf.Len(), actual.Len())
		e.Dump(t, expected.buf.Bytes(), actual.Bytes())
		return
	}

	if !bytes.Equal(expected.buf.Bytes(), actual.Bytes()) {
		t.Errorf("expected data does not match")
		e.Dump(t, expected.buf.Bytes(), actual.Bytes())
	}
}

func setup(t *testing.T, tempDir string) (*Env, func(), error) {
	dir, err := os.MkdirTemp("testdata", tempDir)
	if err != nil {
		return nil, nil, fmt.Errorf("setup: %v", err)
	}

	records := make([]*PackRecord, 20)
	for i := range records {
		data, compressable := createData()

		info, compressed := calculateInfo(data)
		if compressable {
			data = compressed
		}

		records[i] = &PackRecord{
			Name: fmt.Sprint("files/data", i),
			Data: data,

			Compressed: compressable,
			Info:       info,
		}
	}

	return &Env{
			Dir:     dir,
			Records: records,
		}, func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Log(err)
			}
		}, nil
}

func testStore(env *Env) func(t *testing.T) {
	return func(t *testing.T) {
		env.TestPack(t, "store.pk", func(pack *archive.Pack) {
			for _, record := range env.Records {
				t.Logf("Storing %s; numBytes=%d compressed=%t", record.Name, len(record.Data), record.Compressed)
				if err := pack.Store(record.Name, record.Info, record.Compressed, bytes.NewReader(record.Data)); err != nil {
					t.Fatal(err)
				}
			}

			env.Check(t, pack)
		})
	}
}

func testUpdate(env *Env) func(t *testing.T) {
	const numUpdates = 20

	return func(t *testing.T) {
		env.TestPack(t, "update.pk", func(pack *archive.Pack) {
			for _, record := range env.Records {
				if err := pack.Store(record.Name, record.Info, record.Compressed, bytes.NewReader(record.Data)); err != nil {
					t.Fatal(err)
				}
			}

			for n := 0; n < numUpdates; n++ {
				i := rand.Intn(len(env.Records))

				updated := env.UpdateRecord(i)
				if err := pack.Store(updated.Name, updated.Info, updated.Compressed, bytes.NewReader(updated.Data)); err != nil {
					t.Fatal(err)
				}

				env.Check(t, pack)
			}
		})
	}
}

func testAppend(env *Env) func(t *testing.T) {
	const numAppends = 15

	return func(t *testing.T) {
		env.TestPack(t, "append.pk", func(pack *archive.Pack) {
			for _, record := range env.Records {
				if err := pack.Store(record.Name, record.Info, record.Compressed, bytes.NewReader(record.Data)); err != nil {
					t.Fatal(err)
				}
			}

			appended := make([]*PackRecord, numAppends)
			for i := range appended {
				data, compressable := createData()

				info, compressed := calculateInfo(data)
				if compressable {
					data = compressed
				}

				record := &PackRecord{
					Name: fmt.Sprint("files/append", i),
					Data: data,

					Compressed: compressable,
					Info:       info,
				}
				appended[i] = record

				t.Logf("Appending %s; numBytes=%d compressed=%t", record.Name, len(data), record.Compressed)
				if err := pack.Store(record.Name, record.Info, record.Compressed, bytes.NewReader(record.Data)); err != nil {
					t.Fatal(err)
				}
			}

			env.Records = append(env.Records, appended...)

			env.Check(t, pack)
		})
	}
}

func testSave(env *Env) func(t *testing.T) {
	return func(t *testing.T) {
		file, err := os.Create(filepath.Join(env.Dir, "save.pk"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		pack, err := archive.NewPack(file)
		if err != nil {
			t.Fatal(err)
		}

		for _, record := range env.Records {
			t.Logf("Saving %s; numBytes=%d compressed=%t", record.Name, len(record.Data), record.Compressed)
			if err := pack.Store(record.Name, record.Info, record.Compressed, bytes.NewReader(record.Data)); err != nil {
				t.Fatal(err)
			}
		}

		if err := pack.Close(); err != nil {
			t.Fatal(err)
		}

		if err := file.Close(); err != nil {
			t.Fatal(err)
		}

		pack, err = archive.OpenPack(filepath.Join(env.Dir, "save.pk"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := pack.Close(); err != nil {
				t.Error(err)
			}
		}()

		env.Check(t, pack)
	}
}

func testTruncate(env *Env) func(t *testing.T) {
	emptyPack := append(packSignature, 0, 0, 0, 0, 7, 0, 0, 0, 0, 0, 0, 0)

	return func(t *testing.T) {
		file, err := os.Create(filepath.Join(env.Dir, "truncate.pk"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		data, _ := createData()
		if _, err := file.Write(data); err != nil {
			t.Fatal(err)
		}

		stat, _ := file.Stat()
		t.Logf("filled pack with %d random bytes", stat.Size())

		pack, err := archive.NewPack(file)
		if err != nil {
			t.Fatal(err)
		}

		if err := pack.Flush(); err != nil {
			t.Fatal(err)
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}

		actual, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(emptyPack, actual) {
			t.Error("written data did not match empty pack")
			env.Dump(t, emptyPack, actual)
		}
	}
}

func TestPackWrite(t *testing.T) {
	env, teardown, err := setup(t, "pack*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	t.Run("store", testStore(env))
	t.Run("update", testUpdate(env))
	t.Run("append", testAppend(env))
	t.Run("save", testSave(env))
	t.Run("truncate", testTruncate(env))
}

func checkPackRecord(t *testing.T, expected PackRecord, actual *archive.PackRecord) {
	if expected.Compressed != actual.IsCompressed {
		t.Errorf("%s: expected compressed %t but got %t", expected.Name, expected.Compressed, actual.IsCompressed)
	}

	if expected.Info.UncompressedSize != actual.Info.UncompressedSize {
		t.Errorf("%s: expected uncompressed size %d but got %d", expected.Name, expected.Info.UncompressedSize, actual.UncompressedSize)
	}

	if !bytes.Equal(expected.Info.UncompressedChecksum, actual.UncompressedChecksum) {
		t.Errorf("%s: expected uncompressed checksum %x but got %x", expected.Name, expected.Info.UncompressedChecksum, actual.UncompressedChecksum)
	}

	if expected.Info.CompressedSize != actual.Info.CompressedSize {
		t.Errorf("%s: expected compressed size %d but got %d", expected.Name, expected.Info.CompressedSize, actual.CompressedSize)
	}

	if !bytes.Equal(expected.Info.CompressedChecksum, actual.CompressedChecksum) {
		t.Errorf("%s: expected compressed checksum %x but got %x", expected.Name, expected.Info.CompressedChecksum, actual.CompressedChecksum)
	}

	expectedRawData := []byte(expected.Data)
	if expected.Compressed {
		buf := bytes.Buffer{}
		w := segmented.NewDataWriter(&buf)
		w.Write(expected.Data)
		w.Close()
		expectedRawData = buf.Bytes()
	}

	section, err := actual.Section()
	if err != nil {
		t.Fatalf("%s: failed to get section: %v", expected.Name, err)
	}

	actualData, err := io.ReadAll(section)
	if err != nil {
		t.Fatalf("%s: failed to read section: %v", expected.Name, err)
	}

	if !bytes.Equal(expected.Data, actualData) {
		t.Errorf("%s: expected data =\n%s\nactual data =\n%s", expected.Name, expected.Data, actualData)
	}

	actualRawData, err := io.ReadAll(actual.Raw())
	if err != nil {
		t.Fatalf("%s: failed to read raw section: %v", expected.Name, err)
	}

	if !bytes.Equal(expectedRawData, actualRawData) {
		t.Errorf("%s: raw data did not match", expected.Name)
	}
}

func testPackRead(packName string, records []PackRecord) func(*testing.T) {
	return func(t *testing.T) {
		packPath := filepath.Join("testdata", packName)

		file, err := os.Open(packPath)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		pack, err := archive.NewPack(file)
		if err != nil {
			t.Fatal(err)
		}

		if len(pack.Records()) != len(records) {
			t.Fatalf("expected %d records but got %d", len(records), len(pack.Records()))
		}

		for _, expected := range records {
			actual, ok := pack.Search(expected.Name)
			if ok {
				checkPackRecord(t, expected, actual)
			} else {
				t.Errorf("failed to find %s", expected.Name)
			}
		}
	}
}

func TestPackRead(t *testing.T) {
	t.Run("read_one", testPackRead("read_one.pk", []PackRecord{
		newRecord("data1", "aaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", false),
	}))

	t.Run("read_basic", testPackRead("read_basic.pk", []PackRecord{
		newRecord("data1", "aaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbcccccccccccccccccdddddddddddeeeeee", false),
		newRecord("data2", "fffffffffffggggggggggggggggggggggggggggggggggggggggggggggggggggggggghhhhhhhiiiijjjjjjjjjjjjjjjjjj", false),
		newRecord("data3", "klmnop", false),
		newRecord("data4", "", false),
		newRecord("data5", "qqqqqqqqrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrsssssssssssssssssssssssssssssssssssssssttuvvvvvvvvwwwwwwwwwwwwwwwwwwwwwwwwwwwwwxxxxxyyyyyyzzz", false),
	}))

	t.Run("read_compressed", testPackRead("read_compressed.pk", []PackRecord{
		newRecord("data1", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa111111111111111111111111111", true),
		newRecord("data2", "bbbbbbbbbbbbbbbbbbbb2222222222222222222222222222222222222222", true),
		newRecord("data3", "ccccccccccccccc33333333333", true),
		newRecord("data4", "dddddddddddddddddddddddddddddddddddddddddddd444444444444444444", true),
		newRecord("data5", "eeeeeeeeeeeeeeeeeeeeeeee55555555555555555555555555555", true),
		newRecord("data6", "", true),
	}))

	t.Run("empty", testPackRead("empty.pk", []PackRecord{}))
}
