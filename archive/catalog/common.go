package catalog

import "encoding/binary"

const (
	CatalogVersion = 3
)

var (
	order = binary.LittleEndian
)
