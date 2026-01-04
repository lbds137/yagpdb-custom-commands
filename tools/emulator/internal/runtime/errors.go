package runtime

import "errors"

// Execution limit errors
var (
	ErrTooManyOps     = errors.New("too many operations")
	ErrTimeout        = errors.New("execution timeout exceeded")
	ErrOutputTooLarge = errors.New("output too large")
	ErrStackTooDeep   = errors.New("execCC stack too deep")
)
