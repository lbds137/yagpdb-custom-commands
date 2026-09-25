package runtime

import (
	"errors"
	"fmt"
)

// Execution limit errors, worded as YAGPDB words them (common/templates/context_funcs.go).
var (
	ErrTooManyCalls    = errors.New("too many calls to this function")
	ErrTooManyAPICalls = errors.New("too many potential Discord API calls")
)

// yagError is an error whose text is YAGPDB's, which is what a {{catch}}'s .Error and a
// failed run's message show, with the emulator's explanation kept for warnings.
type yagError struct {
	err    error
	detail string
}

func (e *yagError) Error() string { return e.err.Error() }
func (e *yagError) Unwrap() error { return e.err }

// explain is an error as a warning puts it: a yagError's explanation, else its text.
func explain(err error) string {
	var ye *yagError
	if errors.As(err, &ye) {
		return ye.detail
	}
	return err.Error()
}

// discordError is an error the Discord API answers with. discordgo's RESTError reads
// "HTTP <status>, <response body>" (lib/discordgo/types.go); the body is Discord's JSON,
// written here as {"message": ..., "code": ...}. That spacing is Discord's usual, not
// captured from a live response, and a 50035 body also lists the fields at fault, which
// this leaves out.
type discordError struct {
	status  string // "404 Not Found"
	code    int
	message string
}

func (d discordError) Error() string {
	return fmt.Sprintf(`HTTP %s, {"message": %q, "code": %d}`, d.status, d.message, d.code)
}

var (
	errUnknownMessage  = discordError{"404 Not Found", 10008, "Unknown Message"}
	errUnknownEmoji    = discordError{"400 Bad Request", 10014, "Unknown Emoji"}
	errEditOthers      = discordError{"403 Forbidden", 50005, "Cannot edit a message authored by another user"}
	errEmptyMessage    = discordError{"400 Bad Request", 50006, "Cannot send an empty message"}
	errInvalidFormBody = discordError{"400 Bad Request", 50035, "Invalid Form Body"}
)
