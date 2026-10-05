package runtime

import (
	"errors"
	"fmt"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
)

// pinMessage and unpinMessage are YAGPDB's tmplPinMessage (vendor common/templates/
// context_funcs.go): the message_pins counter (2 per run, shared by both functions, taken
// by withLimits as limitedFuncs lists it), ChannelArgNoDM ("unknown channel" when it
// resolves to nothing), then Discord's pin or unpin. Discord's side is the emulator's
// record (ctx.Pins) plus its refusals: a message that isn't known to exist is 10008, and
// a pin in a channel of a test's pins_full is 30003 (Maximum number of pins reached).

// PinChange is a pin or unpin the run made.
type PinChange struct {
	Action    string // "pin" or "unpin"
	ChannelID int64
	MessageID int64
}

func (p PinChange) String() string {
	return fmt.Sprintf("%s message %d in channel %d", p.Action, p.MessageID, p.ChannelID)
}

// errMaxPins is Discord's refusal of a pin in a channel whose pin list is full. The
// message text is the emulator's best statement of Discord's, not captured live.
var errMaxPins = discordError{"400 Bad Request", 30003, "Maximum number of pins reached"}

func (e *Engine) pinMessage(channel, msgID interface{}) (string, error) {
	return e.pin(false, channel, msgID)
}

func (e *Engine) unpinMessage(channel, msgID interface{}) (string, error) {
	return e.pin(true, channel, msgID)
}

func (e *Engine) pin(unpin bool, channel, msgID interface{}) (string, error) {
	fn, action := "pinMessage", "pin"
	if unpin {
		fn, action = "unpinMessage", "unpin"
	}
	cID := e.channelArg(channel) // ChannelArgNoDM
	if cID == 0 {
		return "", errors.New("unknown channel")
	}
	mID := funcs.ToInt64(msgID)
	if !e.ctx.messageExists(cID, mID) && !e.ctx.isForumStarter(cID, mID) {
		return "", e.ctx.discordRefuses(fn, errUnknownMessage,
			fmt.Sprintf("no message %d in channel %d (a test's messages, a sent one or the run's message)", mID, cID))
	}
	if !unpin && e.ctx.PinsFull[cID] {
		return "", e.ctx.discordRefuses(fn, errMaxPins,
			fmt.Sprintf("channel %d's pin list is full (a test's pins_full)", cID))
	}
	e.ctx.Pins = append(e.ctx.Pins, PinChange{Action: action, ChannelID: cID, MessageID: mID})
	return "", nil
}

// isForumStarter is whether the message is the first message of a forum post the run
// created: Discord gives it the thread's own ID.
func (ctx *ExecutionContext) isForumStarter(channelID, id int64) bool {
	return id != 0 && id == channelID && ctx.forumPosts[id]
}
