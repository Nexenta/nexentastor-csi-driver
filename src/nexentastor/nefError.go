package nexentastor

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
