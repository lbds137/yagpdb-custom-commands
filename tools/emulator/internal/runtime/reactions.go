package runtime

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
	yagstd "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// Reaction functions, copied from YAGPDB (common/templates/context_funcs.go): the call
// counting, argument checks and early returns are YAGPDB's, and Discord's side is the
// emulator's record of the reactions (ctx.Reactions).

// ReactionChange is a reaction the bot added or removed.
type ReactionChange struct {
	// Action is "add", "remove" (one user's), "remove_emoji" (everyone's, of one emoji) or
	// "remove_all"
	Action    string
	ChannelID int64
	MessageID int64 // 0: the top-level response (addResponseReactions)
	UserID    int64 // "remove"
	Emoji     string
}

func (r ReactionChange) String() string {
	s := r.Action
	if r.Emoji != "" {
		s += " " + r.Emoji
	}
	if r.UserID != 0 {
		s += fmt.Sprintf(" of user %d", r.UserID)
	}
	if r.MessageID != 0 {
		return s + fmt.Sprintf(" on message %d in channel %d", r.MessageID, r.ChannelID)
	}
	return s + fmt.Sprintf(" on the response in channel %d", r.ChannelID)
}

// reactionCount counts one call against a limit: an error with -strict or inside
// {{try}}, otherwise a warning, as the other limited functions do.
func (e *Engine) reactionCount(fn string, l callLimit) error {
	if err := e.ctx.countCall(fn, l); err != nil {
		if e.ctx.Strict || e.ctx.caughtInTry(err) {
			return err
		}
		e.ctx.warnOnce(err.Error() + "; YAGPDB stops the command here")
	}
	return nil
}

// react is Discord's side of a reaction call on a message: it fails for a message that
// isn't known to exist (messageExists; 10008) or an emoji that reflect formatted from a non-string
// argument ("<int Value>", 10014), and otherwise records the change. ok is false when
// Discord refused.
func (e *Engine) react(fn string, change ReactionChange) (ok bool, err error) {
	if strings.HasPrefix(change.Emoji, "<") && strings.HasSuffix(change.Emoji, " Value>") {
		return false, e.ctx.discordRefuses(fn, "HTTP 400, 10014 Unknown Emoji", fmt.Sprintf("the emoji %s is not a string", change.Emoji))
	}
	if !e.ctx.messageExists(change.ChannelID, change.MessageID) {
		return false, e.ctx.discordRefuses(fn, "HTTP 404, 10008 Unknown Message",
			fmt.Sprintf("no message %d in channel %d (a test's messages, a sent one or the run's message)", change.MessageID, change.ChannelID))
	}
	e.ctx.Reactions = append(e.ctx.Reactions, change)
	return true, nil
}

func argInterface(v reflect.Value) interface{} {
	if v.IsValid() {
		return v.Interface()
	}
	return nil
}

// addReactions is tmplAddReactions: reactions on the run's message (the reacted-to one in
// a reaction run, the caller's in an execCC child). Its `c.Msg == nil` check never fires:
// Context.Execute gives a run without a message (an interval run) a stand-in with ID 0,
// so there each emoji is counted and Discord refuses it.
func (e *Engine) addReactions(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
		m := e.ctx.triggerMsg()

		for _, reaction := range args {
			if err := e.reactionCount("addReactions", limitReactTrig); err != nil {
				return reflect.Value{}, err
			}

			if _, err := e.react("addReactions", ReactionChange{Action: "add", ChannelID: m.ChannelID, MessageID: m.ID, Emoji: reaction.String()}); err != nil {
				return reflect.Value{}, err
			}
		}
		return reflect.ValueOf(""), nil
	}

	return yagstd.CallVariadic(f, true, values...)
}

// addResponseReactions is tmplAddResponseReactions: the reactions go on the response once
// it is sent (see recordResponseSent).
func (e *Engine) addResponseReactions(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
		for _, reaction := range args {
			if err := e.reactionCount("addResponseReactions", limitReactResp); err != nil {
				return reflect.Value{}, err
			}

			e.ctx.responseReactions = append(e.ctx.responseReactions, reaction.String())
		}
		return reflect.ValueOf(""), nil
	}

	return yagstd.CallVariadic(f, true, values...)
}

// addMessageReactions is tmplAddMessageReactions.
func (e *Engine) addMessageReactions(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
		if len(args) < 2 {
			return reflect.Value{}, errors.New("not enough arguments (need channel and message-id)")
		}

		cID := e.channelArg(argInterface(args[0]))
		if cID == 0 {
			return reflect.ValueOf(""), nil
		}

		var mID int64
		if args[1].IsValid() {
			mID = funcs.ToInt64(args[1].Interface())
		}

		for i, reaction := range args {
			if i < 2 {
				continue
			}

			if err := e.reactionCount("addMessageReactions", limitReactMsg); err != nil {
				return reflect.Value{}, err
			}

			if _, err := e.react("addMessageReactions", ReactionChange{Action: "add", ChannelID: cID, MessageID: mID, Emoji: reaction.String()}); err != nil {
				return reflect.Value{}, err
			}
		}
		return reflect.ValueOf(""), nil
	}

	return yagstd.CallVariadic(f, false, values...)
}

// deleteMessageReaction is tmplDelMessageReaction: one user's reactions.
func (e *Engine) deleteMessageReaction(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
		if len(args) < 4 {
			return reflect.Value{}, errors.New("not enough arguments (need channelID, messageID, userID, emoji)")
		}

		cID := e.channelArg(argInterface(args[0]))
		if cID == 0 {
			return reflect.ValueOf("non-existing channel"), nil
		}

		var mID, uID int64

		if args[1].IsValid() {
			mID = funcs.ToInt64(args[1].Interface())
		}

		if args[2].IsValid() {
			uID = targetUserID(args[2].Interface())
		}

		if uID == 0 {
			return reflect.ValueOf("non-existing user"), nil
		}

		for _, reaction := range args[3:] {

			if err := e.reactionCount("deleteMessageReaction", limitDelReactMsg); err != nil {
				return reflect.Value{}, err
			}

			if _, err := e.react("deleteMessageReaction", ReactionChange{Action: "remove", ChannelID: cID, MessageID: mID, UserID: uID, Emoji: reaction.String()}); err != nil {
				return reflect.Value{}, err
			}
		}
		return reflect.ValueOf(""), nil
	}

	return yagstd.CallVariadic(f, false, values...)
}

// deleteAllMessageReactions is tmplDelAllMessageReactions: the given emoji's reactions, or
// every reaction (where YAGPDB ignores Discord's error: an unknown message only warns).
func (e *Engine) deleteAllMessageReactions(values ...reflect.Value) (reflect.Value, error) {
	f := func(args []reflect.Value) (reflect.Value, error) {
		if len(args) < 2 {
			return reflect.Value{}, errors.New("not enough arguments (need channelID, messageID, emojis[optional])")
		}

		cID := e.channelArg(argInterface(args[0]))
		if cID == 0 {
			return reflect.ValueOf("non-existing channel"), nil
		}

		var mID int64
		if args[1].IsValid() {
			mID = funcs.ToInt64(args[1].Interface())
		}

		if len(args) > 2 {
			for _, emoji := range args[2:] {
				if err := e.reactionCount("deleteAllMessageReactions", limitDelReactMsg); err != nil {
					return reflect.Value{}, err
				}

				if _, err := e.react("deleteAllMessageReactions", ReactionChange{Action: "remove_emoji", ChannelID: cID, MessageID: mID, Emoji: emoji.String()}); err != nil {
					return reflect.Value{}, err
				}
			}
			return reflect.ValueOf(""), nil
		}

		if err := e.reactionCount("deleteAllMessageReactions", limitAPI); err != nil {
			return reflect.Value{}, err
		}
		if !e.ctx.messageExists(cID, mID) {
			e.ctx.Warn(KindMessage, "deleteAllMessageReactions: no message %d in channel %d (a test's messages, a sent one or the run's message); YAGPDB ignores Discord's error", mID, cID)
		} else {
			e.ctx.Reactions = append(e.ctx.Reactions, ReactionChange{Action: "remove_all", ChannelID: cID, MessageID: mID})
		}
		return reflect.ValueOf(""), nil
	}

	return yagstd.CallVariadic(f, false, values...)
}

// messageExists reports whether a message is known to exist: knownMessage's, or the run's
// own message (the reacted-to one in a reaction run, whose author knownMessage can't vouch
// for, or one an execCC caller passed on). The stand-in message of a run without one
// (ID 0) doesn't exist.
func (ctx *ExecutionContext) messageExists(channelID, id int64) bool {
	if ctx.knownMessage(channelID, id) != nil {
		return true
	}
	m := ctx.triggerMsg()
	return id != 0 && m.ID == id && m.ChannelID == channelID
}

// recordResponseReactions adds addResponseReactions' reactions to a sent response
// (messageID 0 for the top-level one). SendResponse adds them from a goroutine that
// ignores Discord's errors, so a bad emoji is dropped without a word.
func (ctx *ExecutionContext) recordResponseReactions(channelID, messageID int64) {
	for _, emoji := range ctx.responseReactions {
		if strings.HasPrefix(emoji, "<") && strings.HasSuffix(emoji, " Value>") {
			continue
		}
		ctx.Reactions = append(ctx.Reactions, ReactionChange{Action: "add", ChannelID: channelID, MessageID: messageID, Emoji: emoji})
	}
}
