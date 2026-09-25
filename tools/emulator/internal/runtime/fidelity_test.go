package runtime

import (
	"strings"
	"testing"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// Behaviors that come from running on YAGPDB's own template package and functions, or
// from modeling its database the way it serializes values.

func TestFidelityOutputs(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"dbGet returns sdicts as pointers, like msgpack decoding",
			`{{dbSet 0 "d" (sdict "a" 1)}}{{kindOf (dbGet 0 "d").Value}} {{kindOf (dbGet 0 "d").Value true}}`, "ptr map"},
		{"changing a value read with dbGet doesn't reach the database",
			`{{dbSet 0 "d" (sdict "a" 1)}}{{$v := (dbGet 0 "d").Value}}{{$v.Set "a" 2}}{{(dbGet 0 "d").Value.Get "a"}}`, "1"},
		{"changing a dict after dbSet doesn't reach the database",
			`{{$s := sdict "a" 1}}{{dbSet 0 "d" $s}}{{$s.Set "a" 2}}{{(dbGet 0 "d").Value.Get "a"}}`, "1"},
		{"a value read, changed and stored again is saved",
			`{{dbSet 0 "d" (sdict "a" 1)}}{{$v := (dbGet 0 "d").Value}}{{$v.Set "a" 3}}{{dbSet 0 "d" $v}}{{(dbGet 0 "d").Value.Get "a"}}`, "3"},
		{"nested dicts come back as pointers too",
			`{{dbSet 0 "d" (sdict "in" (sdict "x" 1))}}{{kindOf ((dbGet 0 "d").Value.Get "in")}}`, "ptr"},
		{"try catches an error returned by a function, with the error as dot",
			`{{try}}{{index (cslice) 5}}{{catch}}caught: {{.Error}}{{end}}`, "caught: index out of range: 5"}, // the function's own error, unprefixed
		{"while loops and break",
			`{{$i := 0}}{{while lt $i 10}}{{$i = add $i 1}}{{if eq $i 3}}{{break}}{{end}}{{end}}{{$i}}`, "3"},
		{"return from execTemplate",
			`{{define "sq"}}{{return (mult . .)}}{{end}}{{execTemplate "sq" 7}}`, "49"},
		{"in takes the list first",
			`{{in (cslice 1 2) 2}} {{in (cslice 1 2) 3}}`, "true false"},
		{"title lowercases the rest of each word (x/text cases)",
			`{{title "hELLO wORLD"}}`, "Hello World"},
		{"mod returns a float, as in YAGPDB",
			`{{kindOf (mod 7 2)}}`, "float64"},
		{"messages have discordgo's Link",
			`{{.Message.Link}}`, "https://discord.com/channels/1/123456789/0"},
		{"the guild has an owner who is a member (the triggering user by default)",
			`{{(userArg .Guild.OwnerID).Mention}}`, "<@987654321>"},
		{"and evaluates every argument",
			`{{$x := sdict}}{{if and false ($x.Set "k" 1)}}{{end}}{{$x.HasKey "k"}}`, "true"},
	}
	for _, c := range cases {
		out, err := run(t, newCtx(false, true), c.src)
		if err != nil || strings.TrimSpace(out) != c.want {
			t.Errorf("%s:\n  src:  %s\n  out:  %q\n  err:  %v\n  want: %q", c.name, c.src, out, err, c.want)
		}
	}
}

func TestFidelityErrors(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"eq across int and float is an error",
			`{{eq 1 1.0}}`, "incompatible types for comparison"},
		{"an 11th distinct regex is refused",
			`{{range seq 0 11}}{{reFind (print "a" .) "a1"}}{{end}}`, "too many unique regular expressions"},
		{"seq is capped at 10000",
			`{{seq 0 10001}}`, "sequence max length is 10000"},
		{"complexMessage rejects unknown keys",
			`{{complexMessage "contnet" "hi"}}`, `invalid key "contnet" passed to send message builder.`},
		{"sdict.HasKey takes a string",
			`{{(sdict "1" 1).HasKey 1}}`, "expected string; found 1"},
	}
	for _, c := range cases {
		_, err := run(t, newCtx(false, true), c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s:\n  src:  %s\n  err:  %v\n  want: %q", c.name, c.src, err, c.want)
		}
	}
}

func TestLimitErrorsAreCatchable(t *testing.T) {
	out, err := run(t, newCtx(true, false), `{{try}}{{range seq 0 11}}{{$e := dbGet 0 "k"}}{{end}}{{catch}}caught{{end}}`)
	if err != nil || strings.TrimSpace(out) != "caught" {
		t.Errorf("YAGPDB's limit errors can be caught: out=%q err=%v", out, err)
	}
}

func TestOperationLimit(t *testing.T) {
	src := `{{range seq 0 1100}}{{range seq 0 1000}}{{end}}{{end}}done`

	ctx := newCtx(true, false)
	if _, err := run(t, ctx, src); err == nil || !strings.Contains(err.Error(), "exceeded max operations") {
		t.Errorf("strict, free: want the operation limit error, got %v", err)
	}

	ctx = newCtx(false, false)
	out, err := run(t, ctx, src)
	if err != nil || !strings.HasSuffix(out, "done") {
		t.Fatalf("permissive: out=%q err=%v", out, err)
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 || !strings.Contains(w[0], "1000000 operations") {
		t.Errorf("permissive: want one operation warning, got %q", w)
	}

	if _, err := run(t, newCtx(true, true), src); err != nil {
		t.Errorf("premium allows 2.5M operations: %v", err)
	}
}

func TestComplexMessageSplitsContentAndEmbed(t *testing.T) {
	ctx := newCtx(false, true)
	if _, err := run(t, ctx, `{{sendMessage nil (complexMessage "content" "hi" "embed" (cembed "title" "T") "file" "data" "filename" "log")}}`); err != nil {
		t.Fatal(err)
	}
	m := ctx.SentMessages[0]
	if m.Content != "hi" {
		t.Errorf("content = %q", m.Content)
	}
	if e, ok := m.Embed.(types.Embed); !ok || e["title"] != "T" {
		t.Errorf("embed = %#v", m.Embed)
	}
	if len(ctx.FileUploads) != 1 || ctx.FileUploads[0].Filename != "log.txt" || ctx.FileUploads[0].Content != "data" {
		t.Errorf("file = %+v", ctx.FileUploads)
	}
}

func TestLoopCheckSeesWhile(t *testing.T) {
	ctx := newCtx(false, true)
	src := "{{$i := 0}}\n{{while lt $i (toInt (dbGet 0 \"max\"))}}{{$i = add $i 1}}{{end}}"
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	if w := kinds(ctx, KindLoopDB); len(w) != 1 || !strings.Contains(w[0], "line 2: dbGet inside a loop") {
		t.Errorf("a db call in a while condition runs every iteration: %q", w)
	}
}

func TestRunawayLoopsStopOutsideStrictMode(t *testing.T) {
	for _, src := range []string{
		`{{$i := 0}}{{while lt $i 5}}{{end}}`, // no output: stopped by the operation cap
		`{{while true}}xxxxxxxxxx{{end}}`,     // output: stopped by the output cap
	} {
		done := make(chan error, 1)
		go func() { _, err := run(t, newCtx(false, true), src); done <- err }()
		select {
		case err := <-done:
			if err == nil {
				t.Errorf("%s: a runaway loop should end with an error", src)
			}
		case <-time.After(20 * time.Second):
			t.Fatalf("%s: still running after 20s", src)
		}
	}
}

func TestStoredValuesComeBackAsMsgpackTypes(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"nested ints are int64",
			`{{dbSet 0 "s" (sdict "n" 5)}}{{kindOf ((dbGet 0 "s").Value.Get "n")}}`, "int64"},
		{"dict int keys become int64, so an int literal misses",
			`{{dbSet 0 "d" (dict 1 "one")}}{{index (dbGet 0 "d").Value 1}}`, "<no value>"},
		{"a map from jsonToSdict's contents stays a plain map",
			`{{dbSet 0 "j" (jsonToSdict "{\"a\":{\"b\":1}}")}}{{kindOf ((dbGet 0 "j").Value.Get "a")}}`, "map"},
		{"durations come back as int64",
			`{{dbSet 0 "t" (sdict "d" (toDuration "1h"))}}{{kindOf ((dbGet 0 "t").Value.Get "d")}}`, "int64"},
	}
	for _, c := range cases {
		out, err := run(t, newCtx(false, true), c.src)
		if err != nil || strings.TrimSpace(out) != c.want {
			t.Errorf("%s:\n  out: %q err: %v\n  want: %q", c.name, out, err, c.want)
		}
	}
}

func TestEmbedsAreValidatedLikeDiscord(t *testing.T) {
	for _, src := range []string{
		`{{cembed "description" 5}}`,
		`{{cembed "color" "red"}}`,
		`{{cembed "fields" (cslice (sdict "name" "n" "value" 3))}}`,
		`{{complexMessage "embed" (sdict "title" 1)}}`,
		`{{complexMessage "channel" 1}}`, // only valid inside "forward"
	} {
		if _, err := run(t, newCtx(false, true), src); err == nil {
			t.Errorf("%s: want an error, as in YAGPDB", src)
		}
	}
	if _, err := run(t, newCtx(false, true), `{{complexMessage (sdict "content" "hi" "is_components_v2" false)}}`); err != nil {
		t.Errorf("one sdict and is_components_v2 are accepted: %v", err)
	}
}

func TestEmbedIsACopyOfTheDict(t *testing.T) {
	ctx := newCtx(false, true)
	if _, err := run(t, ctx, `{{$e := sdict "title" "first"}}{{sendMessage nil (cembed $e)}}{{$e.Set "title" "second"}}{{sendMessage nil (cembed $e)}}`); err != nil {
		t.Fatal(err)
	}
	if got := ctx.SentMessages[0].Embed.(types.Embed)["title"]; got != "first" {
		t.Errorf("the first message changed with the dict: %v", got)
	}
}

func TestMessageContentUsesYAGPDBToString(t *testing.T) {
	ctx := newCtx(false, true)
	if _, err := run(t, ctx, `{{sendMessage nil 1.5}}{{sendMessage nil true}}{{sendMessage nil (sdict "a" 1)}}`); err != nil {
		t.Fatal(err)
	}
	got := []string{ctx.SentMessages[0].Content, ctx.SentMessages[1].Content, ctx.SentMessages[2].Content}
	if got[0] != "1.5E+00" || got[1] != "" || got[2] != "" || ctx.SentMessages[2].Embed != nil {
		t.Errorf("contents = %q (a plain sdict is not an embed)", got)
	}
}

func TestSentMessagesCanBeFetched(t *testing.T) {
	ctx := newCtx(false, true)
	src := `{{$a := sendMessageRetID nil "first"}}{{$b := sendMessageRetID 42 (cembed "title" "T")}}` +
		`{{ne $a $b}} {{(getMessage nil $a).Content}} {{(getMessage 42 $b).Author.Bot}} ` +
		`{{len (getMessage 42 $b).Embeds}} {{(getMessage 42 $b).Link}}` +
		`{{if getMessage nil $b}} (nil is the current channel, not 42){{end}}`
	out, err := run(t, ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	want := "true first true 1 https://discord.com/channels/1/42/1100000000000000002"
	if strings.TrimSpace(out) != want {
		t.Errorf("out = %q, want %q", out, want)
	}
	if out, _ := run(t, newCtx(false, true), `{{sendDM "hi"}}{{if getMessage nil 1100000000000000001}}found{{end}}`); out != "" {
		t.Errorf("a DM isn't a server message: %q", out)
	}
}

func TestSentMessageIDsAreUniqueAcrossExecCC(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{sendMessage nil (print "from child, owner " .Guild.OwnerID)}}`)
	ctx := newCtx(false, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	ctx.OwnerID = 55
	out, err := run(t, ctx, `{{$a := sendMessageRetID nil "a"}}{{execCC 7 nil 0 (sdict)}}`+
		`{{$b := sendMessageRetID nil "b"}}{{sub $b $a}} {{(getMessage nil $b).Content}}`)
	if err != nil {
		t.Fatal(err)
	}
	// the child's message took the ID between them
	if strings.TrimSpace(out) != "2 b" {
		t.Errorf("out = %q", out)
	}
	if got := ctx.SentMessages[1].Content; got != "from child, owner 55" {
		t.Errorf("the child sees the same guild owner: %q", got)
	}
}

func TestParseArgsOnlyParsesMessageTriggers(t *testing.T) {
	dir := t.TempDir()
	// Like gematria: parseArgs for the trigger, .ExecData when another command runs it
	writeFile(t, dir+"/child.gohtml", `{{$a := parseArgs 1 "Usage: [text]" (carg "string" "text")}}`+
		`{{sendMessage nil (or .ExecData.Text ($a.Get 0))}}`)
	ctx := newCtx(false, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	ctx.CmdArgs = []interface{}{"two words", "more"}
	out, err := run(t, ctx, `{{(parseArgs 1 "" (carg "string" "s")).Get 0}}|{{.StrippedMsg}}{{execCC 7 nil 0 (sdict "Text" "from exec data")}}`)
	if err != nil {
		t.Fatal(err)
	}
	// the last argument takes the rest of the message, quotes and all, as in dcmd
	if out != `"two words" more|"two words" more` {
		t.Errorf("out = %q", out)
	}
	if len(kinds(ctx, KindExecCC)) != 0 || len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != "from exec data" {
		t.Errorf("the execCC run parses nothing and fails nothing: %q %+v", ctx.Diagnostics, ctx.SentMessages)
	}
}
