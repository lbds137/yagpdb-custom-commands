package runtime

import "errors"

// Execution limit errors, worded as YAGPDB words them (common/templates/context_funcs.go).
var (
	ErrTooManyCalls    = errors.New("too many calls to this function")
	ErrTooManyAPICalls = errors.New("too many potential Discord API calls")
)
