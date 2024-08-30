package pk

import "encoding/binary"

var (
	Signature  = append([]byte("ndpk"), []byte{0x01, 0xff, 0x00}...)
	Terminator = []byte{0xff, 0x00, 0x00, 0xdd, 0x00}

	order = binary.LittleEndian
)
