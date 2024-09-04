package ldf

import "fmt"

type TokenError struct {
	Err  error
	Line string
}

func (err *TokenError) Error() string {
	return fmt.Sprintf("ldf: token: %s: %s", err.Err, err.Line)
}

func (err *TokenError) Unwrap() error {
	return err.Err
}
