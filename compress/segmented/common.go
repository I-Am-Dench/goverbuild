package segmented

import "encoding/binary"

var (
	Signature = append([]byte("sd0"), []byte{0x01, 0xff}...)
	ChunkSize = 0x40000

	order = binary.LittleEndian
)
