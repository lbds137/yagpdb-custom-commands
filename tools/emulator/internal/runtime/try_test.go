package runtime

import (
	"strings"
	"testing"
)

// Outside strict mode YAGPDB's errors are warnings and the run goes on, except inside
// {{try}}: there YAGPDB's {{catch}} gets the error, so the emulator returns it too.

const elevenDBGetsInTry = `{{try}}` + elevenDBGets + `{{catch}}caught: {{.Error}}{{end}}`

func TestCallLimitInsideTryGoesToCatch(t *testing.T) {
	ctx := newCtx(false, false)
	out, err := run(t, ctx, elevenDBGetsInTry)
	if err != nil {
		t.Fatal(err)
	}
	// What the try printed before the error stays, as in YAGPDB
	// The catch sees YAGPDB's text alone: .Error is "too many calls to this function"
	if !strings.HasSuffix(out, "caught: "+ErrTooManyCalls.Error()) || strings.Contains(out, "done") {
		t.Errorf("the catch should get the limit error and the rest of the try not run: %q", out)
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 || !strings.Contains(w[0], "inside {{try}}, so its {{catch}} runs") {
		t.Errorf("want one warning naming the catch, got %q", w)
	}
}

func TestCallLimitOutsideTryStillWarns(t *testing.T) {
	ctx := newCtx(false, false)
	out, err := run(t, ctx, elevenDBGets)
	if err != nil || !strings.HasSuffix(out, "done") {
		t.Fatalf("outside try the run goes on: out=%q err=%v", out, err)
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 || !strings.Contains(w[0], "YAGPDB stops the command here") {
		t.Errorf("want the usual warning, got %q", w)
	}
}

// The catch itself isn't inside the try: a breach there is a warning again
func TestCallLimitInsideCatchWarns(t *testing.T) {
	ctx := newCtx(false, false)
	out, err := run(t, ctx, `{{try}}{{index (cslice) 5}}{{catch}}`+elevenDBGets+`{{end}}`)
	if err != nil || !strings.HasSuffix(out, "done") {
		t.Fatalf("the catch should run to the end: out=%q err=%v", out, err)
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 || !strings.Contains(w[0], "YAGPDB stops the command here") {
		t.Errorf("want the usual warning, got %q", w)
	}
}

// A template called from inside the try is inside it too
func TestCallLimitInTemplateCalledFromTry(t *testing.T) {
	ctx := newCtx(false, false)
	src := `{{define "gets"}}` + elevenDBGets + `{{end}}{{try}}{{template "gets"}}{{catch}}caught{{end}}`
	out, err := run(t, ctx, src)
	if err != nil || !strings.HasSuffix(out, "caught") || strings.Contains(out, "done") {
		t.Fatalf("want the catch to run: out=%q err=%v", out, err)
	}
}

func TestDiscordRefusalInsideTryGoesToCatch(t *testing.T) {
	ctx := msgCtx()
	ctx.Strict = false
	out, err := run(t, ctx, `{{try}}{{editMessage nil 424242 "x"}}after{{catch}}caught: {{.Error}}{{end}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "caught: ") || !strings.HasSuffix(out, "caught: "+errUnknownMessage.Error()) || strings.Contains(out, "after") {
		t.Errorf("the catch should get Discord's refusal: %q", out)
	}
	if len(ctx.EditedMessages) != 0 {
		t.Errorf("a refused edit isn't recorded: %v", ctx.EditedMessages)
	}
}

func TestRejectedSendInsideTryGoesToCatch(t *testing.T) {
	ctx := newCtx(false, true)
	out, err := run(t, ctx, `{{try}}{{sendMessage nil ""}}after{{catch}}caught{{end}}`)
	if err != nil || strings.TrimSpace(out) != "caught" {
		t.Fatalf("want the catch to run: out=%q err=%v", out, err)
	}
	if len(ctx.SentMessages) != 0 {
		t.Errorf("a refused message isn't sent: %v", ctx.SentMessages)
	}
}

func TestReactionLimitInsideTryGoesToCatch(t *testing.T) {
	emoji := `"a" "b" "c" "d" "e" "f" "g" "h" "i" "j" "k" "l" "m" "n" "o" "p" "q" "r" "s" "t" "u"`
	ctx := msgCtx()
	ctx.Strict = false
	out, err := run(t, ctx, `{{try}}{{addReactions `+emoji+`}}after{{catch}}caught{{end}}`)
	if err != nil || !strings.HasSuffix(out, "caught") || strings.Contains(out, "after") {
		t.Fatalf("21 emoji inside try should reach the catch: out=%q err=%v", out, err)
	}
	// YAGPDB reacts one emoji at a time, so the 20 before the limit are on the message
	if len(ctx.Reactions) != 20 {
		t.Errorf("want 20 reactions recorded, got %d", len(ctx.Reactions))
	}
}

// YAGPDB discards sendDM's errors and skips its calls over the limit silently, so even
// inside {{try}} they never reach the catch
func TestSilentCallsInsideTryDontGoToCatch(t *testing.T) {
	for name, src := range map[string]string{
		"a DM Discord rejects": `{{try}}{{sendDM ` + q(2001) + `}}after{{catch}}caught{{end}}`,
		"a DM over the limit":  `{{try}}{{sendDM "a"}}{{sendDM "b"}}after{{catch}}caught{{end}}`,
	} {
		ctx := newCtx(false, true)
		out, err := run(t, ctx, src)
		if err != nil || strings.TrimSpace(out) != "after" {
			t.Errorf("%s: the try should run on: out=%q err=%v", name, out, err)
		}
		if len(ctx.SentMessages) == 0 {
			t.Errorf("%s: outside strict mode the DM is still recorded", name)
		}
		for _, w := range kinds(ctx, KindLimit) {
			if strings.Contains(w, "inside {{try}}") {
				t.Errorf("%s: a silent call isn't caught: %q", name, w)
			}
		}
	}
}

// -strict returns the error anyway; the catch warning is only for non-strict runs (the
// explanation of the error is still a warning)
func TestStrictTryWarnsOnlyTheExplanation(t *testing.T) {
	ctx := newCtx(true, false)
	out, err := run(t, ctx, elevenDBGetsInTry)
	if err != nil || !strings.Contains(out, "caught: ") {
		t.Fatalf("want the catch to run: out=%q err=%v", out, err)
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 || strings.Contains(w[0], "inside {{try}}") {
		t.Errorf("want only the explanation with -strict, got %q", w)
	}
}

// discordgo's RESTError text, "HTTP <status>, <body>", with Discord's JSON body
func TestDiscordErrorText(t *testing.T) {
	for err, want := range map[discordError]string{
		errUnknownMessage:  `HTTP 404 Not Found, {"message": "Unknown Message", "code": 10008}`,
		errUnknownEmoji:    `HTTP 400 Bad Request, {"message": "Unknown Emoji", "code": 10014}`,
		errEditOthers:      `HTTP 403 Forbidden, {"message": "Cannot edit a message authored by another user", "code": 50005}`,
		errEmptyMessage:    `HTTP 400 Bad Request, {"message": "Cannot send an empty message", "code": 50006}`,
		errInvalidFormBody: `HTTP 400 Bad Request, {"message": "Invalid Form Body", "code": 50035}`,
	} {
		if err.Error() != want {
			t.Errorf("got %s, want %s", err.Error(), want)
		}
	}
}

// The error holds YAGPDB's text only; the emulator's explanation is a warning
func TestExplanationStaysOutOfTheError(t *testing.T) {
	for src, detail := range map[string]string{
		elevenDBGets:                     "over the limit",
		`{{editMessage nil 424242 "x"}}`: "Discord refuses",
		`{{sendMessage nil ""}}`:         "Discord rejects",
	} {
		ctx := msgCtx() // strict
		ctx.SetNonPremium()
		_, err := run(t, ctx, src)
		if err == nil || strings.Contains(err.Error(), detail) || !strings.Contains(explained(ctx, err), detail) {
			t.Errorf("%s: want %q in a warning, not the error: %v / %q", src, detail, err, kinds(ctx, KindLimit))
		}
	}
}
