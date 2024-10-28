package catalog

import (
	"fmt"
)

type RecordError struct {
	Err   error
	Field string
}

func (err *RecordError) Error() string {
	return fmt.Sprintf("record: %s: %s", err.Field, err.Err.Error())
}

func (err *RecordError) Unwrap() error {
	return err.Err
}
