package runtime

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// A failed child's error message quotes the lines around the error as YAGPDB does: tabs as
// four spaces, the common indent removed, long lines cut, backticks broken with a ZWS
func TestFailedChildMessageQuotesTheSource(t *testing.T) {
	src := "\t{{$a := 1}}\n\t\t{{.ExecData.X.Y}} `c`\n\t{{/* a comment long enough to be cut off */}}\n"
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", src)
	writeFile(t, dir+"/bad.gohtml", "{{if}}")
	ctx := newCtx(false, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml", 8: "bad.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}{{execCC 8 nil 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	if len(ctx.SentMessages) != 2 {
		t.Fatalf("sent %+v", ctx.SentMessages)
	}
	stop := "\nAn error caused the execution of the custom command template to stop:\n"
	got := ctx.SentMessages[0].Content
	wantStart := stop + "`Failed executing CC #7, line 2, row "
	wantEnd := "nil pointer evaluating interface {}.X`\n```" +
		"1    {{$a := 1}}\n2        {{.ExecData.X.Y}} `​c`​\n3    {{/* a comment long enough to ...\n```"
	if !strings.HasPrefix(got, wantStart) || !strings.HasSuffix(got, wantEnd) {
		t.Errorf("got %q\nwant %q ... %q", got, wantStart, wantEnd)
	}
	// An execution error the formatter can't place quotes the whole error, with YAGPDB's prefix
	if _, err := run(t, newCtx(true, true), `{{printf "%030000d" 1}}`); err == nil ||
		err.Error() != "Failed executing template: response grew too big (>25k)" {
		t.Errorf("got %v", err)
	}
	// Not an execution error (here a parse error): the error as it is, in backticks
	want := stop + "`Failed parsing template: template: CC #8:1: missing value for if`"
	if got := ctx.SentMessages[1].Content; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A failed child's error goes to the redirect-errors channel when its header sets one;
// with show_errors off its partial output is sent as a normal response instead
func TestFailedChildErrorSettings(t *testing.T) {
	head := func(lines string) string { return "{{/*\n  Trigger type: `None`\n" + lines + "*/}}" }
	dir := t.TempDir()
	writeFile(t, dir+"/redirect.gohtml", head("  Redirect errors: `77`\n")+`a{{.ExecData.X.Y}}`)
	writeFile(t, dir+"/quiet.gohtml", head("  Show errors: `false`\n")+`b <@5>{{.ExecData.X.Y}}`)
	writeFile(t, dir+"/dropped.gohtml", head("  Show errors: `false`\n")+`{{deleteResponse 0}}c{{.ExecData.X.Y}}`)
	ctx := roleCtx()
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "redirect.gohtml", 8: "quiet.gohtml", 9: "dropped.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 42 0 nil}}{{execCC 8 42 0 nil}}{{execCC 9 42 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range ctx.SentMessages {
		got = append(got, fmt.Sprintf("%d|%.20s|%s", m.ChannelID, m.Content, m.Pings))
	}
	want := []string{"77|a\nAn error caused th|nobody", "42|b <@5>|<@5>"}
	if !slices.Equal(got, want) {
		t.Errorf("sent %q, want %q", got, want)
	}
	if s := ReadErrorSettings("{{/* no header */}}"); s != (ErrorSettings{ShowErrors: true}) {
		t.Errorf("defaults: %+v", s)
	}
}

// A failed top-level run posts its show_errors message too, and its response pings no one;
// with show_errors off its output is the response (deleteResponse can still drop it)
func TestFailedRunErrorSettings(t *testing.T) {
	head := func(lines string) string { return "{{/*\n  Trigger type: `None`\n" + lines + "*/}}" }
	ctx := roleCtx()
	out, err := run(t, ctx, head("  redirect ERRORS: `77`\n")+`<@5> x{{index (cslice) 5}}`)
	if err == nil || out != "<@5> x" || ctx.ResponsePings.String() != "nobody" ||
		len(ctx.SentMessages) != 1 || ctx.SentMessages[0].ChannelID != 77 {
		t.Errorf("show_errors: %q, %v, %s, %+v", out, err, ctx.ResponsePings, ctx.SentMessages)
	}
	ctx = roleCtx()
	out, err = run(t, ctx, head("  Show errors: `False`\n")+`<@5> x{{index (cslice) 5}}`)
	if err == nil || out != "<@5> x" || ctx.ResponsePings.String() != "<@5>" || len(ctx.SentMessages) != 0 {
		t.Errorf("no show_errors: %q, %v, %s, %+v", out, err, ctx.ResponsePings, ctx.SentMessages)
	}
	ctx = roleCtx()
	out, _ = run(t, ctx, head("  Show errors: `false`\n")+`{{deleteResponse 0}}<@5> x{{index (cslice) 5}}`)
	if out != "" || ctx.ResponsePings.String() != "nobody" {
		t.Errorf("deleted: %q, %s", out, ctx.ResponsePings)
	}
}

// Without show_errors a failed child's output goes to its own channel, even with a redirect,
// and only if there is some; deleteResponse drops it only under a second
func TestFailedChildWithoutShowErrors(t *testing.T) {
	head := "{{/*\n  Trigger type: `None`\n  Show errors: `false`\n  Redirect errors: `77`\n*/}}"
	dir := t.TempDir()
	writeFile(t, dir+"/a.gohtml", head+`a{{.ExecData.X.Y}}`)
	writeFile(t, dir+"/b.gohtml", head+`{{deleteResponse 1}}b{{.ExecData.X.Y}}`)
	writeFile(t, dir+"/c.gohtml", head+`   {{.ExecData.X.Y}}`)
	ctx := roleCtx()
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "a.gohtml", 8: "b.gohtml", 9: "c.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 42 0 nil}}{{execCC 8 42 0 nil}}{{execCC 9 42 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range ctx.SentMessages {
		got = append(got, fmt.Sprintf("%d|%s", m.ChannelID, m.Content))
	}
	if want := []string{"42|a", "42|b"}; !slices.Equal(got, want) {
		t.Errorf("sent %q, want %q", got, want)
	}
}

// Settings are read from the leading header comment only, keys in any case, and a value
// the emulator can't read is an error rather than a default
func TestHeaderSettings(t *testing.T) {
	body := "{{/*\n  Trigger type: `None`\n*/}}{{/* Show errors: `false` */}}{{$s := \"Redirect errors: `99`\"}}"
	if s := ReadErrorSettings(body); s != (ErrorSettings{ShowErrors: true}) {
		t.Errorf("read from the body: %+v", s)
	}
	if tr, _ := ReadTrigger("{{/*\n  trigger TYPE: `Command`\n  TRIGGER: `x`\n  Case Sensitive: `TRUE`\n*/}}"); tr != (Trigger{Type: "Command", Text: "x", CaseSensitive: true}) {
		t.Errorf("keys in any case: %+v", tr)
	}
	for src, want := range map[string]string{
		"{{/*\n  Show errors: `no`\n*/}}":        "Show errors: `no` isn't true or false",
		"{{/*\n  Case sensitive: `true `\n*/}}":  "Case sensitive: `true ` isn't true or false",
		"{{/*\n  Redirect errors: `<#77>`\n*/}}": "Redirect errors: `<#77>` isn't a channel ID",
		"{{/*\n  Redirect errors: `-5`\n*/}}":    "Redirect errors: `-5` isn't a channel ID",
		"{{/*\n  Show errors: `true`\n*/}}":      "",
	} {
		err := ValidateHeader(src)
		if (want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), want)) {
			t.Errorf("%q: got %v, want %q", src, err, want)
		}
	}

	// An execCC target's header is checked too
	dir := t.TempDir()
	writeFile(t, dir+"/bad.gohtml", "{{/*\n  Show errors: `off`\n*/}}hi")
	ctx := roleCtx()
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "bad.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`); err == nil || !strings.Contains(err.Error(), "execCC 7 (bad.gohtml): header Show errors") {
		t.Errorf("got %v", err)
	}
}

// A failed run's show_errors message is itself subject to Discord's 2000-character limit,
// since YAGPDB's ChannelMessageSend discards its own rejection error: non-strict records
// the (too-long) message anyway and warns of the rejection; strict drops it, also with a
// warning (checkSend's usual silent semantics).
func TestFailedRunShowErrorsMessageOverDiscordLimit(t *testing.T) {
	// out itself is 1990 runes (under YAGPDB's 2k response cap), but with the appended
	// error text the show_errors message is over Discord's 2000-character limit
	src := strings.Repeat("a", 1990) + `{{index (cslice) 5}}`

	ctx := newCtx(false, true)
	if _, err := run(t, ctx, src); err == nil {
		t.Fatal("want an error")
	}
	if len(ctx.SentMessages) != 1 {
		t.Fatalf("non-strict: want the message recorded, got %+v", ctx.SentMessages)
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 || !strings.Contains(w[0], "show_errors message") ||
		!strings.Contains(w[0], "content is 2201 characters (max 2000)") {
		t.Errorf("non-strict: want a warning naming the Discord rejection, got %q", w)
	}

	ctx = newCtx(true, true)
	if _, err := run(t, ctx, src); err == nil {
		t.Fatal("want an error")
	}
	if len(ctx.SentMessages) != 0 {
		t.Errorf("strict: want the message not recorded, got %+v", ctx.SentMessages)
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 || !strings.Contains(w[0], "show_errors message") ||
		!strings.Contains(w[0], "content is 2201 characters (max 2000)") {
		t.Errorf("strict: want a warning naming the Discord rejection, got %q", w)
	}
}

// Strict, with output over 2000 runes: YAGPDB's cap runs first (out becomes the "longer
// than 2k" notice), then the error is appended - as bot.go orders it, not the other way.
func TestFailedRunShowErrorsMessageStartsWithOverCapNotice(t *testing.T) {
	src := strings.Repeat("a", 2500) + `{{index (cslice) 5}}`
	ctx := newCtx(true, true)
	if _, err := run(t, ctx, src); err == nil {
		t.Fatal("want an error")
	}
	want := "Custom command (#0) response was longer than 2k (contact an admin on the server...)" +
		"\nAn error caused the execution of the custom command template to stop:\n"
	if len(ctx.SentMessages) != 1 || !strings.HasPrefix(ctx.SentMessages[0].Content, want) {
		t.Fatalf("got %+v, want prefix %q", ctx.SentMessages, want)
	}
}

// The same over-Discord-limit case, through an execCC child instead of a top-level run
func TestFailedChildShowErrorsMessageOverDiscordLimit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", strings.Repeat("a", 1990)+`{{index (cslice) 5}}`)
	ctx := newCtx(true, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	if len(ctx.SentMessages) != 0 {
		t.Errorf("strict: want the child's message not recorded, got %+v", ctx.SentMessages)
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 || !strings.Contains(w[0], "show_errors message") {
		t.Errorf("want a warning naming the Discord rejection, got %q", w)
	}
}
