package internal

import (
	"io"
)

type ReadSeekerAt interface {
	io.ReadSeeker
	io.ReaderAt
}
