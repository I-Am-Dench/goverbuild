package archive

import (
	"strings"

	"github.com/snksoft/crc"
)

type Info struct {
	UncompressedSize     uint32
	UncompressedChecksum []byte

	CompressedSize     uint32
	CompressedChecksum []byte
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
