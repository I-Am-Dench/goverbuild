package manifest

import "fmt"

type MismatchedChecksumError struct {
	Expected, Calculated []byte
}

func (e *MismatchedChecksumError) Error() string {
	return fmt.Sprintf("manifest: mismatched checksum: expected %x but got %x", e.Expected, e.Calculated)
}
