package archive

import (
	"path/filepath"
	"strings"

	"github.com/snksoft/crc"
)

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

func ToSysPath(s string) string {
	return strings.ReplaceAll(s, "\\", string(filepath.Separator))
}

func GetCrc(s string) uint32 {
	cleaned := ToArchivePath(s)
	data := append([]byte(cleaned), []byte{0x00, 0x00, 0x00, 0x00}...)

	hash := crc.NewHashWithTable(Crc)
	hash.Write(data)
	return hash.CRC32()
}
