package ns

import (
	"fmt"
)

// NefError - nef error format
type NefError struct {
	Err  error
	Code string
}

func (e *NefError) Error() string {
	return fmt.Sprintf("%v [code: %v]", e.Err, e.Code)
}

// IsNefError - true if error is an instance of NefError
func IsNefError(err error) bool {
	_, ok := err.(*NefError)
	return ok
}

// GetNefErrorCode - treats an error as NsError and returns its code in case of success
func GetNefErrorCode(err error) string {
	if nefErr, ok := err.(*NefError); ok {
		return nefErr.Code
	}
	return ""
}
