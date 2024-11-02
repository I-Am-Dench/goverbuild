package archive

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/snksoft/crc"
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

var (
	Crc = crc.NewTable(&crc.Parameters{
		Width:      32,
		Polynomial: 0x04c11db7,
		Init:       0xffffffff,
	})
)

func ToArchivePath(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ToLower(s), "/", "\\"))
}

func GetCrc(s string) uint32 {
	cleaned := ToArchivePath(s)
	data := append([]byte(cleaned), []byte{0x00, 0x00, 0x00, 0x00}...)

	hash := crc.NewHashWithTable(Crc)
	hash.Write(data)
	return hash.CRC32()
}
