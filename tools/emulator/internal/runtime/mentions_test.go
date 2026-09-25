package runtime

import (
	"fmt"
	"slices"
	"strings"
	"testing"
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

// An execCC child's response is sent to its channel with its own pings; an empty or failed
// one sends nothing (the failure is a warning)
func TestExecCCResponseIsSent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", "  {{.ExecData.Text}} <@5> <@&20> {{mentionRoleID 10}}\n")
	writeFile(t, dir+"/sends.gohtml", `{{sendMessage nil "sent"}} reply`)
	writeFile(t, dir+"/quiet.gohtml", `{{sendMessage nil "quiet"}}  `)
	writeFile(t, dir+"/fails.gohtml", `partial{{.ExecData.Missing.Field}}`)
	ctx := roleCtx()
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml", 8: "sends.gohtml", 9: "fails.gohtml", 10: "quiet.gohtml"}
	src := `{{execCC 7 42 0 (sdict "Text" "hi")}}{{execCC 8 nil 0 nil}}{{execCC 9 nil 0 nil}}{{execCC 10 nil 0 nil}}`
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
		fmt.Sprintf("%d|reply|nobody", here), fmt.Sprintf("%d|quiet|nobody", here)}
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
