package catalog

import "fmt"

type RecordError struct {
	Err   error
	Field string
}

func (err *RecordError) Error() string {
	return fmt.Sprintf("record: %s: %s", err.Field, err.Err.Error())
}

// type ReadFileError struct {
// 	Err   error
// 	Field string
// }

// func (err *ReadFileError) Error() string {
// 	return fmt.Sprintf("catalog: file: %s: %s", err.Field, err.Err.Error())
// }

// func (err *ReadFileError) Unwrap() error {
// 	return err.Err
// }

// func (err *ReadFileError) Is(target error) bool {
// 	switch target.(type) {
// 	case *ReadFileError:
// 		return true
// 	default:
// 		return errors.Is(err, target)
// 	}
// }
