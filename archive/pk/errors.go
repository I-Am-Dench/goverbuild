package pk

import (
	"errors"
	"fmt"
)

type RecordError struct {
	Err   error
	Field string
}

func (err *RecordError) Error() string {
	return fmt.Sprintf("pk: record: %s: %s", err.Field, err.Err.Error())
}

func (err *RecordError) Unwrap() error {
	return err.Err
}

func (err *RecordError) Is(target error) bool {
	switch target.(type) {
	case *RecordError:
		return true
	default:
		return errors.Is(err.Err, target)
	}
}
