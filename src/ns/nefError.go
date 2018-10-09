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

// GetNefErrorCode - treats an error as NefError and returns its code in case of success
func GetNefErrorCode(err error) string {
	if nefErr, ok := err.(*NefError); ok {
		return nefErr.Code
	}
	return ""
}

// IsAlreadyExistNefError - treats an error as NefError and returns true if its code is "EEXIST"
func IsAlreadyExistNefError(err error) bool {
	return GetNefErrorCode(err) == "EEXIST"
}
