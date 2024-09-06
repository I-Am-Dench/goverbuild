package sid0

import "encoding/binary"

var (
	Signature = append([]byte("sd0"), []byte{0x01, 0xff}...)

	order = binary.LittleEndian
)
