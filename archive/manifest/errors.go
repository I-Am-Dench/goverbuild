package manifest

import (
	"errors"
	"fmt"
)

var (
	ErrMismatchedHash = errors.New("manifest: mismatched hash")
)

type MismatchedMd5HashError struct {
	Line string
}

func (err *MismatchedMd5HashError) Error() string {
	return fmt.Sprintf("manifest: mismatched md5 hash: %s", err.Line)
}

func (err *MismatchedMd5HashError) Unwrap() error {
	return ErrMismatchedHash
}
