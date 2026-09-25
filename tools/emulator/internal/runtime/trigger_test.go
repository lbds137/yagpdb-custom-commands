package runtime

import (
	"reflect"
	"testing"
)

func TestReadTrigger(t *testing.T) {
	src := "{{- /*\n  Trigger type: `Command`\n  Trigger: `db`\n*/ -}}"
	if got, ok := ReadTrigger(src); !ok || got != (Trigger{Type: "Command", Text: "db"}) {
		t.Errorf("got %+v, %v", got, ok)
	}
	if _, ok := ReadTrigger("{{/* no header */}}"); ok {
		t.Error("a template without a header has no trigger")
	}
}

func TestCheckMatch(t *testing.T) {
	db := Trigger{Type: "Command", Text: "db"}
	link := Trigger{Type: "Regex", Text: `https://discord\.com/channels/\d+/\d+/\d+`}
	tests := []struct {
		name     string
		t        Trigger
		msg      string
		match    bool
		stripped string
		args     []string
	}{
		{"command with arguments", db, `-db get "a b"`, true, `get "a b"`, []string{"-db", "get", "a b"}},
		{"case-insensitive", db, "-DB get", true, "get", []string{"-DB", "get"}},
		{"no arguments", db, "-db", true, "", []string{"-db"}},
		{"bot mention instead of the prefix", db, "<@1234567890> db x", true, "x", []string{"<@1234567890> db", "x"}},
		{"trigger must end the word", db, "-dbx", false, "", nil},
		{"trigger must start the message", db, "hi -db", false, "", nil},
		{"regex anywhere", link, "look https://discord.com/channels/1/2/3 here", true, " here",
			[]string{"look https://discord.com/channels/1/2/3", "here"}},
		{"nickname mention", db, "<@!1234567890>db x", true, "x", []string{"<@!1234567890>db", "x"}},
		{"starts with", Trigger{Type: "Starts with", Text: "hey"}, "heya you", true, "a you", []string{"hey", "a", "you"}},
		{"contains", Trigger{Type: "Contains", Text: "cat"}, "a cat b", true, " b", []string{"a cat", "b"}},
		{"exact match", Trigger{Type: "Exact match", Text: "hi"}, "hi there", false, "", nil},
		{"exact match matches", Trigger{Type: "Exact match", Text: "hi"}, "HI", true, "", []string{"HI"}},
		{"no message trigger", Trigger{Type: "Reaction"}, "-db", false, "", nil},
	}
	for _, tt := range tests {
		match, stripped, args := CheckMatch(DefaultPrefix, tt.t, tt.msg)
		if match != tt.match || stripped != tt.stripped || !reflect.DeepEqual(args, tt.args) {
			t.Errorf("%s: got %v %q %q", tt.name, match, stripped, args)
		}
	}
}

func TestSetTriggerMessageBuildsArgsLikeYAGPDB(t *testing.T) {
	ctx := newCtx(false, true)
	db := Trigger{Type: "Command", Text: "db"}
	msg := TriggerMessage(DefaultPrefix, db, []string{"set", "Some Key", `{"a": "b"}`})
	if err := ctx.SetTriggerMessage(db, msg); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, ctx, `{{json .Args}}|{{.Cmd}}|{{json .CmdArgs}}|{{.StrippedMsg}}|{{.Message.Content}}|{{.ServerPrefix}}`)
	if err != nil {
		t.Fatal(err)
	}
	want := `["-db","set","Some Key","{\"a\": \"b\"}"]|-db|["set","Some Key","{\"a\": \"b\"}"]|` +
		`set "Some Key" "{\"a\": \"b\"}"|-db set "Some Key" "{\"a\": \"b\"}"|-`
	if out != want {
		t.Errorf("got  %s\nwant %s", out, want)
	}

	if err := ctx.SetTriggerMessage(db, "-kb x"); err == nil {
		t.Error("a message that doesn't trigger the command should be an error")
	}
}

func TestPrefixIsQuoted(t *testing.T) {
	if match, _, _ := CheckMatch(".", Trigger{Type: "Command", Text: "db"}, "xdb"); match {
		t.Error("a prefix is text, not a regex")
	}
	if match, _, _ := CheckMatch(".", Trigger{Type: "Command", Text: "db"}, ".db"); !match {
		t.Error("the prefix should match itself")
	}
}

func TestExecCCChildHasTheMessageButNoArguments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{sendMessage nil (print .Message.Content "|" .ServerPrefix "|" .CmdArgs "|" .Cmd)}}`)
	ctx := newCtx(false, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	ctx.Prefix = "!"
	if err := ctx.SetTriggerMessage(Trigger{Type: "Command", Text: "t"}, "!t a"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	if len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != "!t a|!|<nil>|<nil>" {
		t.Errorf("sent %+v", ctx.SentMessages)
	}
}

func TestParseArgsNeedsATriggerMessage(t *testing.T) {
	// An interval or a top-level exec run has no message: parseArgs gives nothing, no error
	out, err := run(t, newCtx(false, true), `{{$a := parseArgs 1 "" (carg "string" "x")}}{{$a.IsSet 0}}`)
	if err != nil || out != "false" {
		t.Errorf("got %q, %v", out, err)
	}
}

func TestScheduledTriggers(t *testing.T) {
	for typ, want := range map[string]bool{
		"Minute interval": true, "Hourly interval": true, "Cron": true,
		"None": false, "Command": false, "Reaction": false, "Join message": false,
	} {
		if got := (Trigger{Type: typ}).Scheduled(); got != want {
			t.Errorf("%s: %v", typ, got)
		}
	}
}
