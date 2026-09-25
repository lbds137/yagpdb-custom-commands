package runtime

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

func TestSentMessagePings(t *testing.T) {
	const mentions = `<@5> <@!6> <@&10> <@&20> @everyone`
	cases := []struct{ src, want string }{
		// sendMessage pings users only; NoEscape pings everything
		{`{{sendMessage nil "` + mentions + `"}}`, "<@5> <@6>"},
		{`{{sendMessageNoEscape nil "` + mentions + `"}}`, "everyone <@5> <@6> <@&10> <@&20>"},
		{`{{sendMessage nil (cembed "description" "` + mentions + `")}}`, "nobody"},
		// complexMessage's allowed_mentions holds for sendMessage, not for NoEscape
		{`{{sendMessage nil (complexMessage "content" "` + mentions + `" "allowed_mentions" (sdict "roles" (cslice 20) "parse" (cslice "everyone")))}}`, "everyone <@&20>"},
		{`{{sendMessage nil (complexMessage "content" "` + mentions + `" "allowed_mentions" nil)}}`, "nobody"},
		{`{{sendMessageNoEscape nil (complexMessage "content" "` + mentions + `" "allowed_mentions" nil)}}`, "everyone <@5> <@6> <@&10> <@&20>"},
		{`{{sendMessage nil (complexMessage "content" "<@5> <@6> @here" "allowed_mentions" (sdict "users" (cslice "6")))}}`, "<@6>"},
		{`{{sendMessageNoEscape nil (cembed "description" "` + mentions + `")}}`, "nobody"},
		{`{{$id := sendMessageNoEscapeRetID nil "<@&10> @here"}}`, "everyone <@&10>"},
	}
	for _, c := range cases {
		ctx := roleCtx()
		if _, err := run(t, ctx, c.src); err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if got := ctx.SentMessages[0].Pings.String(); got != c.want {
			t.Errorf("%s: pings %s, want %s", c.src, got, c.want)
		}
	}
}

func TestNoEscapeRetIDReturnsTheMessage(t *testing.T) {
	ctx := roleCtx()
	out, err := run(t, ctx, `{{$id := sendMessageNoEscapeRetID nil "hi"}}{{(getMessage nil $id).Content}}`)
	if err != nil || out != "hi" {
		t.Errorf("got %q, %v", out, err)
	}
}

func TestResponsePings(t *testing.T) {
	cases := []struct{ src, want string }{
		// Typed mentions: only users ping
		{`<@5> <@&10> @everyone`, "<@5>"},
		// mentionRole and mentionEveryone make theirs ping, wherever they were printed
		{`{{mentionRoleID 10}} <@&20> {{$x := mentionEveryone}}@here`, "everyone <@&10>"},
		{`{{$r := mentionRoleName "Guest"}}<@&20>`, "<@&20>"},
		{`{{$x := mentionHere}}@everyone`, "everyone"},
	}
	for _, c := range cases {
		ctx := roleCtx()
		if _, err := run(t, ctx, c.src); err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if got := ctx.ResponsePings.String(); got != c.want {
			t.Errorf("%s: pings %s, want %s", c.src, got, c.want)
		}
	}
}

// A response over 2000 characters is replaced by a notice, which pings no one
func TestLongResponsePingsNoOne(t *testing.T) {
	for _, strict := range []bool{false, true} {
		ctx := newCtx(strict, true)
		if _, err := run(t, ctx, `<@5>{{range seq 0 2001}}x{{end}}`); err != nil {
			t.Fatal(err)
		}
		if !ctx.ResponsePings.Empty() {
			t.Errorf("strict %v: pings %s", strict, ctx.ResponsePings)
		}
	}
}

func TestAllowedMentionsErrorsAsYAGPDB(t *testing.T) {
	cases := []struct{ arg, want string }{
		{`(sdict "parse" (cslice "users") "users" (cslice 5))`, "cannot parse all users if only allowing a set of users"},
		{`(sdict "parse" (cslice "admins"))`, `invalid slice element in "Parse"`},
		{`(sdict "parse" "users")`, `accepts a slice only`},
		{`(sdict "roles" (cslice 0))`, `invalid ID passed`},
		{`(sdict "replied_user" 1)`, `accepts a bool only`},
		{`(sdict "nope" 1)`, `invalid key "nope"`},
	}
	for _, c := range cases {
		_, err := run(t, roleCtx(), `{{complexMessage "content" "x" "allowed_mentions" `+c.arg+`}}`)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %q", c.arg, err, c.want)
		}
	}
}

// An execCC child's response is sent to its channel with its own pings, an empty one isn't,
// and a failed one sends its output and YAGPDB's error message, pinging no one (and warns)
func TestExecCCResponseIsSent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", "  {{.ExecData.Text}} <@5> <@&20> {{mentionRoleID 10}}\n")
	writeFile(t, dir+"/sends.gohtml", `{{sendMessage nil "sent"}} reply`)
	writeFile(t, dir+"/quiet.gohtml", `{{sendMessage nil "quiet"}}  `)
	writeFile(t, dir+"/fails.gohtml", `partial <@5>{{.ExecData.Missing.Field}}`)
	ctx := roleCtx()
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml", 8: "sends.gohtml", 9: "fails.gohtml", 10: "quiet.gohtml"}
	ctx.Channels = map[int64]string{ctx.ChannelID: ctx.ChannelName, 42: "forty-two", 43: "forty-three"}
	src := `{{execCC 7 42 0 (sdict "Text" "hi")}}{{execCC 8 nil 0 nil}}{{execCC 9 43 0 nil}}{{execCC 10 nil 0 nil}}`
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range ctx.SentMessages {
		got = append(got, fmt.Sprintf("%d|%s|%s", m.ChannelID, m.Content, m.Pings))
	}
	// A child's own sends come before its response
	here := ctx.ChannelID
	want := []string{"42|hi <@5> <@&20> <@&10>|<@5> <@&10>", fmt.Sprintf("%d|sent|nobody", here),
		fmt.Sprintf("%d|reply|nobody", here),
		"43|partial <@5>\nAn error caused the execution of the custom command template to stop:\n" +
			"`Failed executing CC #9, line 1, row 23: executing \"CC #9\" at <.ExecData.Missing.Field>: " +
			"nil pointer evaluating interface {}.Missing`\n```1    partial <@5>{{.ExecData.Missin...\n```|nobody",
		fmt.Sprintf("%d|quiet|nobody", here)}
	if !slices.Equal(got, want) {
		t.Errorf("sent %q, want %q", got, want)
	}
	if len(ctx.Diagnostics) != 1 || !strings.Contains(ctx.Diagnostics[0].Message, "execCC 9") {
		t.Errorf("want the failed child's warning, got %q", ctx.Diagnostics)
	}
	// The child runs after the caller in YAGPDB, so the caller can't getMessage its messages
	for _, m := range ctx.Messages {
		if m.Content == "reply" || m.Content == "sent" {
			t.Errorf("the caller sees the child's message %q", m.Content)
		}
	}
}

// Over 2000 characters, a child's response is YAGPDB's notice with the child's number
func TestExecCCLongResponseNamesTheChild(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/long.gohtml", `<@5>{{printf "%2001s" "x"}}`)
	ctx := newCtx(true, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "long.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	want := "Custom command (#7) response was longer than 2k (contact an admin on the server...)"
	if len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != want ||
		ctx.SentMessages[0].Pings.String() != "nobody" {
		t.Errorf("sent %+v", ctx.SentMessages)
	}
}

// A response deleteResponse would delete at once (delay under 1) isn't sent; a failed run's
// error message still carries the output
func TestDeleteResponseAtOnceSendsNothing(t *testing.T) {
	cases := []struct{ src, out, pings string }{
		{`{{deleteResponse 0}}hi <@5>`, "", "nobody"},
		{`{{deleteResponse -5}}hi`, "", "nobody"},
		{`{{deleteResponse 0.5}}hi`, "", "nobody"}, // cut to 0
		{`{{deleteResponse}}hi <@5>`, "hi <@5>", "<@5>"},
		{`{{deleteResponse 1}}hi`, "hi", "nobody"},
		{`{{deleteResponse 90000}}hi`, "hi", "nobody"},
	}
	for _, c := range cases {
		ctx := roleCtx()
		out, err := run(t, ctx, c.src)
		if err != nil || out != c.out || ctx.ResponsePings.String() != c.pings {
			t.Errorf("%s: got %q (%s), %v; want %q (%s)", c.src, out, ctx.ResponsePings, err, c.out, c.pings)
		}
	}

	// The output YAGPDB drops gets a note, not the over-2k warning (the notice isn't sent)
	ctx := roleCtx()
	if out, err := run(t, ctx, `{{deleteResponse 0}}{{range seq 0 2100}}x{{end}}`); err != nil || out != "" ||
		len(ctx.Diagnostics) != 1 || ctx.Diagnostics[0].Kind != KindResponse {
		t.Errorf("got %q, %v, %q", out, err, ctx.Diagnostics)
	}
	ctx = roleCtx()
	if _, err := run(t, ctx, `{{deleteResponse 0}}  `); err != nil || len(ctx.Diagnostics) != 0 {
		t.Errorf("nothing dropped, no note: %v, %q", err, ctx.Diagnostics)
	}

	dir := t.TempDir()
	writeFile(t, dir+"/quiet.gohtml", `{{deleteResponse 0}}reply`)
	writeFile(t, dir+"/fails.gohtml", `{{deleteResponse 0}}partial{{.ExecData.X.Y}}`)
	ctx = roleCtx()
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "quiet.gohtml", 8: "fails.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}{{execCC 8 nil 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	if len(ctx.SentMessages) != 1 || !strings.HasPrefix(ctx.SentMessages[0].Content, "partial\nAn error caused") {
		t.Errorf("sent %+v", ctx.SentMessages)
	}
}

// .Message by what started the run, and what an execCC child inherits (YAGPDB's ctx.Msg)
func TestMessageByTrigger(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{sendMessage nil (print "child " .Message.Author.ID " [" .Message.Content "] " .Message.ChannelID)}}{{execCC 8 nil 0 nil}}`)
	writeFile(t, dir+"/grandchild.gohtml", `{{sendMessage nil (print "grandchild " .Message.Author.ID " [" .Message.Content "]")}}`)
	withChildren := func(ctx *ExecutionContext) *ExecutionContext {
		ctx.TemplateBaseDir = dir
		ctx.CommandIDMap = map[int64]string{7: "child.gohtml", 8: "grandchild.gohtml"}
		return ctx
	}
	sent := func(ctx *ExecutionContext) string {
		var s []string
		for _, m := range ctx.SentMessages {
			s = append(s, m.Content)
		}
		return strings.Join(s, " | ")
	}
	src := `{{.Message.Author.ID}} [{{.Message.Content}}]{{execCC 7 99 0 nil}}`

	// A message run: the child gets the same message
	ctx := withChildren(roleCtx())
	ctx.MessageContent = "-cmd a"
	out, err := run(t, ctx, src)
	u := ctx.UserID
	want := fmt.Sprintf("child %d [-cmd a] %d | grandchild %d [-cmd a]", u, ctx.ChannelID, u)
	if err != nil || out != fmt.Sprintf("%d [-cmd a]", u) || sent(ctx) != want {
		t.Errorf("message run: %q, %v; sent %q", out, err, sent(ctx))
	}

	// An interval run has no .Message and no member; its child gets a blank message from
	// the bot (ID 0, this guild, the caller's channel), and no member either
	ctx = withChildren(roleCtx())
	ctx.NoMessage, ctx.NoMember = true, true
	writeFile(t, dir+"/blank.gohtml", `{{sendMessage nil (print "blank " .Message.ID " " .Message.GuildID " " .User " " .Member)}}`)
	ctx.CommandIDMap[9] = "blank.gohtml"
	out, err = run(t, ctx, `{{.Message}} {{.Message.Content}} {{.User}} {{.user}} {{.Member}} {{.BotUser}}{{execCC 7 99 0 nil}}{{execCC 9 99 0 nil}}`)
	want = fmt.Sprintf("child %d [] %d | grandchild %d [] | blank 0 %d <nil> <nil>", // print gets nil
		botUser.ID, ctx.ChannelID, botUser.ID, ctx.GuildID)
	if err != nil || out != strings.Repeat("<no value> ", 5)+"<no value>" || sent(ctx) != want {
		t.Errorf("interval run: %q, %v; sent %q", out, err, sent(ctx))
	}

	// A reaction run's .Message is the reacted-to message; its child gets it with the
	// reactor as author
	ctx = withChildren(roleCtx())
	ctx.Reaction = &types.CtxReaction{MessageID: 55, ChannelID: ctx.ChannelID}
	ctx.Messages = []types.CtxMessage{
		{ID: 55, ChannelID: ctx.ChannelID + 1, Author: types.DiscordUser{ID: 6}, Content: "elsewhere"},
		{ID: 55, ChannelID: ctx.ChannelID, Author: types.DiscordUser{ID: 5}, Content: "hi"},
	}
	out, err = run(t, ctx, src+`{{.ReactionMessage.ID}}`)
	want = fmt.Sprintf("child %d [hi] %d | grandchild %d [hi]", u, ctx.ChannelID, u)
	if err != nil || out != "5 [hi]55" || sent(ctx) != want {
		t.Errorf("reaction run: %q, %v; sent %q", out, err, sent(ctx))
	}
}
