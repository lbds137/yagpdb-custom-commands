package runtime

import (
	"slices"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

func reactions(ctx *ExecutionContext) []string {
	var out []string
	for _, r := range ctx.Reactions {
		out = append(out, r.String())
	}
	return out
}

// reactCtx is a strict context with message 77 in its channel (123456789) and channel 42
func reactCtx() *ExecutionContext {
	ctx := channelCtx()
	ctx.MessageID = 55
	ctx.Messages = []types.CtxMessage{{ID: 77, ChannelID: ctx.ChannelID}}
	return ctx
}

func TestMessageReactions(t *testing.T) {
	cases := []struct{ src, out, err string }{
		{`{{addMessageReactions nil 77 "👍" (cslice "a" "b")}}`, "", ""},
		{`{{deleteMessageReaction nil 77 5 "👍"}}{{deleteMessageReaction nil 77 "<@66>" "a" "b"}}`, "", ""},
		{`{{deleteAllMessageReactions nil 77}}{{deleteAllMessageReactions nil 77 "a"}}`, "", ""},
		// YAGPDB's own checks and early returns
		{`{{addMessageReactions nil}}`, "", "not enough arguments (need channel and message-id)"},
		{`{{deleteMessageReaction nil 77 5}}`, "", "not enough arguments (need channelID, messageID, userID, emoji)"},
		{`{{deleteAllMessageReactions nil}}`, "", "not enough arguments (need channelID, messageID, emojis[optional])"},
		{`{{addMessageReactions 99 77 "a"}}|{{deleteMessageReaction 99 77 5 "a"}}|{{deleteAllMessageReactions 99 77}}`, "|non-existing channel|non-existing channel", ""},
		// TargetUserID strips only a mention longer than 4 characters
		{`{{deleteMessageReaction nil 77 "nobody" "a"}}|{{deleteMessageReaction nil 77 "<@6>" "a"}}`, "non-existing user|non-existing user", ""},
		// Discord's refusals: an unknown message, an emoji that isn't a string
		{`{{addMessageReactions nil 78 "a"}}`, "", "10008 Unknown Message"},
		{`{{addMessageReactions nil 77 5}}`, "", "10014 Unknown Emoji): the emoji <int Value> is not a string"},
		{`{{addMessageReactions nil 77 nil}}`, "", "the emoji <invalid Value> is not a string"},
		{`{{deleteMessageReaction nil 78 5 "a"}}`, "", "10008 Unknown Message"},
		{`{{deleteAllMessageReactions nil 78 "a"}}`, "", "10008 Unknown Message"},
	}
	want := [][]string{
		{"add 👍 on message 77 in channel 123456789", "add a on message 77 in channel 123456789", "add b on message 77 in channel 123456789"},
		{"remove 👍 of user 5 on message 77 in channel 123456789", "remove a of user 66 on message 77 in channel 123456789", "remove b of user 66 on message 77 in channel 123456789"},
		{"remove_all on message 77 in channel 123456789", "remove_emoji a on message 77 in channel 123456789"},
	}
	for i, c := range cases {
		ctx := reactCtx()
		ctx.Channels = map[int64]string{ctx.ChannelID: "general"} // 99 is unknown
		out, err := run(t, ctx, c.src)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%s: want error %q, got %q, %v", c.src, c.err, out, err)
			}
			continue
		}
		if err != nil || out != c.out {
			t.Errorf("%s: got %q, %v; want %q", c.src, out, err, c.out)
		}
		if i < len(want) && !slices.Equal(reactions(ctx), want[i]) {
			t.Errorf("%s: reactions %q, want %q", c.src, reactions(ctx), want[i])
		}
		if i >= len(want) && len(ctx.Reactions) != 0 {
			t.Errorf("%s: reactions %q, want none", c.src, reactions(ctx))
		}
	}

	// an unknown channel returns before counting
	ctx := reactCtx()
	if _, err := run(t, ctx, `{{addMessageReactions 99 77 "a"}}`); err != nil || ctx.Counters["add_reaction_message"] != 0 {
		t.Errorf("unknown channel: %v, counters %v", err, ctx.Counters)
	}
	// removing every reaction from an unknown message: YAGPDB ignores Discord's error
	ctx = reactCtx()
	if _, err := run(t, ctx, `{{deleteAllMessageReactions nil 78}}`); err != nil || len(ctx.Reactions) != 0 || len(kinds(ctx, KindMessage)) != 1 {
		t.Errorf("remove_all of an unknown message: %v, %q, %q", err, reactions(ctx), ctx.Diagnostics)
	}
	// YAGPDB counts as it goes: 20 reactions are added before the 21st fails
	ctx = reactCtx()
	if _, err := run(t, ctx, `{{addMessageReactions nil 77 (seq 0 21)}}`); err == nil || len(ctx.Reactions) != 0 {
		// seq gives ints: the first is refused as an emoji
		t.Errorf("ints: %v, %q", err, reactions(ctx))
	}
	ctx = reactCtx()
	if _, err := run(t, ctx, `{{$e := cslice}}{{range seq 0 21}}{{$e = $e.Append (str .)}}{{end}}{{addMessageReactions nil 77 $e}}`); err == nil || !strings.Contains(err.Error(), "add_reaction_message") || len(ctx.Reactions) != 20 {
		t.Errorf("21 emoji: %v, %d reactions", err, len(ctx.Reactions))
	}
}

// addReactions reacts to the run's message: the trigger, the reacted-to message, the
// caller's in an execCC child; an interval run's stand-in (ID 0) is counted and refused;
// nil emoji are skipped
func TestAddReactions(t *testing.T) {
	ctx := reactCtx()
	if _, err := run(t, ctx, `{{addReactions "a" nil (cslice "b")}}`); err != nil ||
		!slices.Equal(reactions(ctx), []string{"add a on message 55 in channel 123456789", "add b on message 55 in channel 123456789"}) {
		t.Errorf("trigger: %v, %q", err, reactions(ctx))
	}
	ctx = reactCtx()
	ctx.NoMessage = true
	if _, err := run(t, ctx, `{{addReactions "a"}}`); err == nil || !strings.Contains(err.Error(), "10008 Unknown Message") ||
		len(ctx.Reactions) != 0 || ctx.Counters["add_reaction_trigger"] != 1 {
		t.Errorf("interval: %v, %q, %v", err, reactions(ctx), ctx.Counters)
	}
	// a reaction run's reacted-to message exists, though the test doesn't declare it
	ctx = reactCtx()
	ctx.Reaction = &types.CtxReaction{MessageID: 56}
	src := `{{addReactions "a"}}{{deleteMessageReaction nil .Reaction.MessageID .User.ID "b"}}{{addMessageReactions nil 56 "c"}}`
	if _, err := run(t, ctx, src); err != nil || !slices.Equal(reactions(ctx), []string{"add a on message 56 in channel 123456789",
		"remove b of user 987654321 on message 56 in channel 123456789", "add c on message 56 in channel 123456789"}) {
		t.Errorf("reaction run: %v, %q", err, reactions(ctx))
	}
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{addReactions "a"}}`)
	ctx = reactCtx()
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 42 0 nil}}`); err != nil || !slices.Equal(reactions(ctx), []string{"add a on message 55 in channel 123456789"}) {
		t.Errorf("execCC child: %v, %q", err, reactions(ctx))
	}
	if _, err := run(t, reactCtx(), `{{addReactions 5}}`); err == nil || !strings.Contains(err.Error(), "10014 Unknown Emoji") {
		t.Errorf("an int emoji: %v", err)
	}
}

// addResponseReactions reacts to the response once it is sent, dropping bad emoji as
// SendResponse's goroutine does
func TestAddResponseReactions(t *testing.T) {
	cases := []struct{ src, want string }{
		{`{{addResponseReactions "a" 5 (cslice "b")}}hi`, "add a on the response in channel 123456789|add b on the response in channel 123456789"},
		{`{{addResponseReactions "a"}}  `, ""},
		{`{{addResponseReactions "a"}}{{deleteResponse 0}}hi`, ""},
		{`{{addResponseReactions "a"}}hi{{index (cslice) 5}}`, ""}, // the show_errors message instead
	}
	for _, c := range cases {
		ctx := reactCtx()
		run(t, ctx, c.src)
		if got := strings.Join(reactions(ctx), "|"); got != c.want {
			t.Errorf("%s: got %q, want %q", c.src, got, c.want)
		}
	}
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{addResponseReactions "a"}}child`)
	ctx := reactCtx()
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 42 0 nil}}`); err != nil ||
		!slices.Equal(reactions(ctx), []string{"add a on message 1100000000000000001 in channel 42"}) {
		t.Errorf("execCC child: %v, %q", err, reactions(ctx))
	}
	ctx = reactCtx()
	if _, err := run(t, ctx, `{{addResponseReactions (seq 0 21)}}x`); err == nil || !strings.Contains(err.Error(), "add_reaction_response") {
		t.Errorf("21 response reactions: %v", err)
	}
}
