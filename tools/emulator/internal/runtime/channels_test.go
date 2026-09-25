package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

func channelCtx() *ExecutionContext {
	ctx := newCtx(true, true)
	ctx.ChannelName = "general"
	ctx.Channels = map[int64]string{ctx.ChannelID: "general", 42: "Staff-Log"}
	ctx.ChannelOrder = []int64{ctx.ChannelID, 42}
	return ctx
}

// Channel arguments follow YAGPDB's baseChannelArg: nil is this channel, an int an ID, a
// string an ID or a name (any case), anything else no channel; the channel must exist
func TestChannelArguments(t *testing.T) {
	cases := []struct{ src, out, err string }{
		{`{{(getChannel nil).Name}} {{(getChannel 42).Name}} {{(getChannel "42").ID}} {{(getChannel "staff-log").ID}}`, "general Staff-Log 42 42", ""},
		{`{{getChannel 43}} {{getChannel 42.0}} {{getChannel "nope"}}`, "<nil> <nil> <nil>", ""},
		{`{{sendMessage "staff-log" "a"}}{{sendMessage 43 "b"}}{{sendMessage 42.0 "c"}}`, "", ""},
		{`{{$id := sendMessageRetID 42 "a"}}{{$none := sendMessageRetID 43 "b"}}[{{$none}}]`, "[]", ""},
		{`{{getMessage 43 1}}`, "<nil>", ""},
		{`{{editMessage 43 1 "x"}}`, "", "unknown channel"},
		{`{{execCC 7 43 0 nil}}`, "", "Unknown channel"},
		{`{{execCC 7 43 10 nil}}`, "", "Unknown channel"},
		{`{{scheduleUniqueCC 7 43 0 "k" nil}}`, "", "Unknown channel"},
	}
	for _, c := range cases {
		ctx := channelCtx()
		out, err := run(t, ctx, c.src)
		if out != c.out || (c.err == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s: got %q, %v; want %q, %q", c.src, out, err, c.out, c.err)
		}
	}
	// Only the message to a real channel was sent
	ctx := channelCtx()
	if _, err := run(t, ctx, `{{sendMessage "staff-log" "a"}}{{sendMessage 43 "b"}}{{sendMessage 42.0 "c"}}`); err != nil ||
		len(ctx.SentMessages) != 1 || ctx.SentMessages[0].ChannelID != 42 {
		t.Errorf("sent %+v, %v", ctx.SentMessages, err)
	}
}

// With no channels declared any channel ID is taken to exist, with one warning per ID;
// names match only the current channel
func TestUndeclaredChannelsWarn(t *testing.T) {
	ctx := newCtx(false, true)
	ctx.ChannelName = "general"
	out, err := run(t, ctx, `{{(getChannel 42).ID}} {{(getChannel 42).Name}}|{{(getChannel "general").ID}} {{getChannel "other"}}{{sendMessage 42 "a"}}`)
	want := fmt.Sprintf("42 |%d <nil>", ctx.ChannelID)
	if err != nil || out != want || len(ctx.Diagnostics) != 1 || ctx.Diagnostics[0].Kind != KindChannel {
		t.Errorf("got %q, %v, %q; want %q", out, err, ctx.Diagnostics, want)
	}
}

// parseArgs' channel argument is dcmd's: a mention or an ID of a channel the server has
func TestParseArgsChannel(t *testing.T) {
	ctx := channelCtx()
	if err := ctx.SetTriggerMessage(Trigger{Type: "Command", Text: "c"}, "-c <#42>"); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, ctx, `{{$a := parseArgs 1 "" (carg "channel" "c")}}{{($a.Get 0).Name}}`)
	if err != nil || out != "Staff-Log" {
		t.Errorf("got %q, %v", out, err)
	}
	ctx = channelCtx()
	if err := ctx.SetTriggerMessage(Trigger{Type: "Command", Text: "c"}, "-c 43"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, ctx, `{{parseArgs 1 "" (carg "channel" "c")}}`); err == nil || !strings.Contains(err.Error(), `Improper mention "43"`) {
		t.Errorf("unknown channel: %v", err)
	}
}

// An execCC child knows the server's channels and is in its target channel
func TestExecCCChildChannels(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{sendMessage nil (print .Channel.Name " " (getChannel "general").ID)}}`)
	ctx := channelCtx()
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 "staff-log" 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("Staff-Log %d", ctx.ChannelID)
	if len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != want || ctx.SentMessages[0].ChannelID != 42 {
		t.Errorf("sent %+v, want %q in 42", ctx.SentMessages, want)
	}
}

// dcmd cuts a mention's "<#" and its last character, whatever that is
func TestParseArgsChannelMentionCut(t *testing.T) {
	ctx := channelCtx()
	if err := ctx.SetTriggerMessage(Trigger{Type: "Command", Text: "c"}, "-c <#42x"); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, ctx, `{{((parseArgs 1 "" (carg "channel" "c")).Get 0).ID}}`); err != nil || out != "42" {
		t.Errorf("got %q, %v", out, err)
	}
}

// An edit sets the message's EditedTimestamp, as Discord does
func TestEditSetsEditedTimestamp(t *testing.T) {
	out, err := run(t, channelCtx(), `{{$id := sendMessageRetID nil "a"}}{{(getMessage nil $id).EditedTimestamp.IsZero}} {{editMessage nil $id "b"}}{{(getMessage nil $id).EditedTimestamp.IsZero}}`)
	if err != nil || out != "true false" {
		t.Errorf("got %q, %v", out, err)
	}
}

// Over the API-call limit sendMessageRetID returns "" like YAGPDB, not nil; getMessage
// finds a message by its channel's name
func TestRetIDOverTheLimitAndGetMessageByName(t *testing.T) {
	out, err := run(t, channelCtx(), `{{range seq 0 100}}{{sendMessage nil "x"}}{{end}}{{printf "%T" (sendMessageRetID nil "y")}}`)
	if err != nil || out != "string" {
		t.Errorf("got %q, %v", out, err)
	}
	out, err = run(t, channelCtx(), `{{$id := sendMessageRetID 42 "a"}}{{(getMessage "staff-log" $id).Content}} {{getMessage 42.0 $id}} {{getChannel ""}}`)
	if err != nil || out != "a <nil> <nil>" {
		t.Errorf("got %q, %v", out, err)
	}
}

// No channel has an empty name, even an unnamed test channel; of two channels with the
// same name the first in declared (position) order wins
func TestChannelNameEdgeCases(t *testing.T) {
	ctx := channelCtx()
	ctx.ChannelName, ctx.Channels[ctx.ChannelID] = "", ""
	ctx.Channels[7], ctx.Channels[42] = "dup", "dup"
	ctx.ChannelOrder = []int64{42, 7, ctx.ChannelID}
	out, err := run(t, ctx, `{{getChannel ""}} {{(getChannel "DUP").ID}}`)
	if err != nil || out != "<nil> 42" {
		t.Errorf("got %q, %v", out, err)
	}
}

// getMessage finds the triggering message, as YAGPDB (which asks Discord) does, and an
// execCC child finds its caller's; editMessage refuses it, as someone else's
func TestGetMessageFindsTheTrigger(t *testing.T) {
	ctx := channelCtx()
	ctx.MessageID = 55
	if err := ctx.SetTriggerMessage(Trigger{Type: "Command", Text: "c"}, "-c hi"); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, ctx, `{{$m := getMessage nil .Message.ID}}{{$m.Content}}|{{eq $m.Author.ID .User.ID}}|{{getMessage 42 .Message.ID}}|{{getMessage nil 0}}`)
	if err != nil || out != "-c hi|true|<nil>|<nil>" {
		t.Errorf("got %q, %v", out, err)
	}

	ctx = channelCtx() // strict
	if _, err := run(t, ctx, `{{editMessage nil .Message.ID "x"}}`); err == nil || !strings.Contains(err.Error(), "50005 Cannot edit a message authored by another user") {
		t.Errorf("editing the trigger: %v", err)
	}

	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{(getMessage nil 55).Content}}`)
	ctx = channelCtx()
	ctx.MessageID = 55
	ctx.MessageContent = "caller"
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`); err != nil || len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != "caller" {
		t.Errorf("execCC child: %v, %+v", err, ctx.SentMessages)
	}
}

// Without a triggering message (an interval run) or with one only reacted to, getMessage
// finds only what the test declares
func TestGetMessageWithoutATrigger(t *testing.T) {
	ctx := channelCtx()
	ctx.MessageID = 55
	ctx.NoMessage = true
	if out, err := run(t, ctx, `{{getMessage nil 55}}|{{getMessage nil 0}}`); err != nil || out != "<nil>|<nil>" {
		t.Errorf("interval: %q, %v", out, err)
	}
	// an interval run's execCC child has a blank .Message, with no ID
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{getMessage nil 0}}`)
	ctx = channelCtx()
	ctx.NoMessage = true
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`); err != nil || len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != "<nil>" {
		t.Errorf("interval execCC child: %v, %+v", err, ctx.SentMessages)
	}
	ctx = channelCtx()
	ctx.MessageID = 55
	ctx.Reaction = &types.CtxReaction{MessageID: 56}
	if out, err := run(t, ctx, `{{getMessage nil 55}}|{{getMessage nil 56}}`); err != nil || out != "<nil>|<nil>" {
		t.Errorf("reaction: %q, %v", out, err)
	}
	// nor does a reaction run's execCC child, whose .Message has the reactor as author
	writeFile(t, dir+"/child.gohtml", `{{getMessage nil .Message.ID}}`)
	ctx = channelCtx()
	ctx.Reaction = &types.CtxReaction{MessageID: 56}
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`); err != nil || len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != "<nil>" {
		t.Errorf("reaction execCC child: %v, %+v", err, ctx.SentMessages)
	}
}
