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

// TODO add util function
//if nefErr, ok := err.(*ns.NefError); ok {
// 	log.Errorf("ISCLUSTERWITH NEF ERROR: %v", nefErr.Code)
// } else {
// 	log.Errorf("ISCLUSTERWITH ERROR: %v", err)
// }
