package runtime

import (
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
