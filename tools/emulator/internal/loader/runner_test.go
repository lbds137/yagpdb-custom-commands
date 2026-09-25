package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/runtime"
)

func TestSetupTemplatesRunFirstOnTheSameDatabase(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("seed.gohtml", `{{dbSet 0 "seeded" "yes"}}{{sendMessage nil "from setup"}}`)
	write("broken.gohtml", `{{index (cslice) 3}}`)

	tc := &TestCase{
		Name:           "reads the seed",
		TemplateSource: `{{(dbGet 0 "seeded").Value}}`,
		SetupTemplates: []string{"seed.gohtml"},
		Expected:       ExpectedResult{OutputEquals: str("yes")},
		Assertions:     Assertions{SentMessages: []MessageCheck{{ContentEquals: str("from test")}}},
	}
	tc.applyDefaults()
	r := NewRunner(RunnerConfig{BaseDir: dir})
	res := r.RunTest(tc)
	// The setup's message isn't the test's: the only message check fails, nothing else
	if res.Error != nil || len(res.Failures) != 1 || !strings.Contains(res.Failures[0], "no messages sent") {
		t.Errorf("want only the message check to fail: err=%v failures=%q", res.Error, res.Failures)
	}

	tc.SetupTemplates = []string{"broken.gohtml"}
	if res := r.RunTest(tc); res.Error == nil || !strings.Contains(res.Error.Error(), "setup template broken.gohtml") {
		t.Errorf("a failing setup template should fail the test and name itself: %v", res.Error)
	}
}

func TestArgsFollowTheHeaderTrigger(t *testing.T) {
	dir := t.TempDir()
	header := func(kind, trigger string) string {
		return "{{/*\n  Trigger type: `" + kind + "`\n  Trigger: `" + trigger + "`\n*/}}"
	}
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("kb.gohtml", header("Command", "kb")+`{{json .Args}}|{{.Cmd}}|{{json .CmdArgs}}`)
	write("link.gohtml", header("Regex", `\d{3}`)+`{{.Cmd}}|{{json .CmdArgs}}`)
	write("interval.gohtml", header("Minute interval", "")+`ran {{.Message}} {{.User}}`)
	write("none.gohtml", header("None", "")+`{{.Message.Author.ID}}`)
	write("plain.gohtml", `{{.Cmd}}`)
	write("message.gohtml", header("Command", "m")+`{{.Message.Content}}`)

	r := NewRunner(RunnerConfig{BaseDir: dir})
	tests := []struct {
		name, template string
		ctx            ContextDef
		out, err       string
	}{
		{"command", "kb.gohtml", ContextDef{Args: []string{"a b", "c"}}, `["-kb","a b","c"]|-kb|["a b","c"]`, ""},
		{"server prefix", "kb.gohtml", ContextDef{Guild: GuildDef{Prefix: "!"}}, `["!kb"]|!kb|[]`, ""},
		{"regex", "link.gohtml", ContextDef{MessageContent: "see 123 x"}, `see 123|["x"]`, ""},
		{"no header", "plain.gohtml", ContextDef{}, "-plain", ""},
		{"interval", "interval.gohtml", ContextDef{}, "ran <no value> <no value>", ""}, // no message or member
		{"none", "none.gohtml", ContextDef{}, "987654321098765432", ""},                // run by execCC: a caller's message
		{"both", "kb.gohtml", ContextDef{Args: []string{"x"}, MessageContent: "-kb x"}, "", "not both"},
		{"regex with args", "link.gohtml", ContextDef{Args: []string{"x"}}, "", "needs message_content"},
		{"no match", "link.gohtml", ContextDef{MessageContent: "no digits"}, "", "doesn't match"},
		{"interval with args", "interval.gohtml", ContextDef{Args: []string{"x"}}, "", "need a message trigger"},
		{"exec_data with args", "kb.gohtml", ContextDef{Args: []string{"x"}, ExecData: map[string]interface{}{"a": 1}}, "", "not exec_data"},
		{"reaction with args", "kb.gohtml", ContextDef{Args: []string{"x"}, Reaction: &ReactionDef{Emoji: "👍"}}, "", "not exec_data or reaction"},
		{"regex without a message", "link.gohtml", ContextDef{}, "", "needs message_content"},
		{"exec_data sets .Message", "message.gohtml", ContextDef{MessageContent: "hi", ExecData: map[string]interface{}{"a": 1}}, "hi", ""},
	}
	for _, tt := range tests {
		tc := &TestCase{Name: tt.name, Template: tt.template, Context: tt.ctx}
		tc.applyDefaults()
		res := r.RunTest(tc)
		if tt.err != "" {
			if res.Error == nil || !strings.Contains(res.Error.Error(), tt.err) {
				t.Errorf("%s: want an error containing %q, got %v", tt.name, tt.err, res.Error)
			}
			continue
		}
		if res.Error != nil || res.Output != tt.out {
			t.Errorf("%s: got %q, %v; want %q", tt.name, res.Output, res.Error, tt.out)
		}
	}
}

func TestSuiteDefaultPrefix(t *testing.T) {
	tc := &TestCase{Name: "prefix", TemplateSource: `{{.Cmd}}|{{.ServerPrefix}}`}
	tc.mergeDefaults(ContextDef{Guild: GuildDef{Prefix: "!"}}, nil, nil)
	tc.applyDefaults()
	if res := NewRunner(RunnerConfig{}).RunTest(tc); res.Error != nil || res.Output != "!test|!" {
		t.Errorf("got %q, %v", res.Output, res.Error)
	}
}

func TestBadSuiteReportsItsOwnError(t *testing.T) {
	dir := t.TempDir()
	for name, want := range map[string]string{
		"days.yaml":     "72h, not 3d",
		"negative.yaml": "negative",
	} {
		value := map[string]string{"days.yaml": "3d", "negative.yaml": "-1h"}[name]
		src := "tests:\n  - name: x\n    template_source: \"hi\"\n    context:\n      member_joined_ago: { 5: " + value + " }\n"
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadTestFile(path); err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "line 5") {
			t.Errorf("%s: want an error naming line 5 and %q, got %v", name, want, err)
		}
	}
}

func TestUnknownKeysAreErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "typo.yaml")
	src := "tests:\n  - name: x\n    template_source: \"hi\"\n    assertions:\n      sent_mesages: []\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTestFile(path); err == nil || !strings.Contains(err.Error(), "sent_mesages") {
		t.Errorf("a misspelled assertion should be an error naming it, got %v", err)
	}
}

func TestSuiteDefaultsCantTriggerACommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "defaults.yaml")
	src := "defaults:\n  args: [x]\ntests:\n  - name: x\n    template_source: \"hi\"\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTestFile(path); err == nil || !strings.Contains(err.Error(), "set them per test") {
		t.Errorf("args in defaults should be an error, got %v", err)
	}
}

// YAGPDB runs no custom command for a bot's message, so a test's user can't be the bot,
// set per test or by the suite's defaults
func TestTheUserCantBeTheBot(t *testing.T) {
	for _, src := range []string{
		"tests:\n  - name: x\n    context: { user: { id: 1234567890 } }\n    template_source: \"hi\"\n",
		"defaults: { user: { id: 1234567890 } }\ntests:\n  - name: x\n    template_source: \"hi\"\n",
	} {
		path := filepath.Join(t.TempDir(), "bot.yaml")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadTestFile(path); err == nil || !strings.Contains(err.Error(), "is the bot's") {
			t.Errorf("%s: got %v", src, err)
		}
	}
}

func TestOversizedSetupValueIsAnError(t *testing.T) {
	tc := &TestCase{
		Name:           "big",
		TemplateSource: "hi",
		SetupDB:        []DBEntry{{Key: "big", Value: strings.Repeat("x", 100000)}},
	}
	tc.applyDefaults()
	if res := NewRunner(RunnerConfig{}).RunTest(tc); res.Error == nil || !strings.Contains(res.Error.Error(), "short write") {
		t.Errorf("want a setup_db error naming the short write, got %v", res.Error)
	}
}

// A setup_db key over 256 bytes is stored cut, as dbSet stores it, so dbGet (which cuts
// the same way) finds it
func TestLongSetupKeyIsCut(t *testing.T) {
	tc := &TestCase{
		Name:           "long key",
		TemplateSource: `{{(dbGet 0 (printf "%0300d" 0)).Value}} {{dbCount}}`,
		SetupDB:        []DBEntry{{Key: strings.Repeat("0", 300), Value: "found"}},
	}
	tc.applyDefaults()
	if res := NewRunner(RunnerConfig{}).RunTest(tc); res.Error != nil || res.Output != "found 1" {
		t.Errorf("got %q, %v", res.Output, res.Error)
	}
}

// A db_checks key over 256 bytes is looked up cut, as the command stored it
func TestLongDBCheckKeyIsCut(t *testing.T) {
	long := strings.Repeat("0", 300)
	tc := &TestCase{
		Name:           "long check key",
		TemplateSource: `{{dbSet 0 (printf "%0300d" 0) "v"}}`,
		Assertions:     Assertions{DBChecks: []DBCheck{{Key: long, ValueEquals: "v"}}},
	}
	tc.applyDefaults()
	if res := NewRunner(RunnerConfig{}).RunTest(tc); res.Error != nil || len(res.Failures) != 0 {
		t.Errorf("got %v, %q", res.Error, res.Failures)
	}
}

// A header setting the emulator can't read fails the test instead of defaulting
func TestBadHeaderSettingIsAnError(t *testing.T) {
	tc := &TestCase{Name: "bad header", TemplateSource: "{{/*\n  Show errors: `no`\n*/}}hi"}
	tc.applyDefaults()
	if res := NewRunner(RunnerConfig{}).RunTest(tc); res.Error == nil || !strings.Contains(res.Error.Error(), "isn't true or false") {
		t.Errorf("got %v", res.Error)
	}
}

// nth picks a later message, and an explicit "" in content_equals or output_equals asserts
// emptiness instead of checking nothing
func TestMessageAndOutputEquals(t *testing.T) {
	src := `{{sendMessage 9 "a"}}{{sendMessage 9 "b"}}{{sendMessage 8 "c"}}{{$id := sendMessageRetID 7 (complexMessage "content" "d" "embed" (cembed "title" "t"))}}{{editMessage 7 $id (complexMessageEdit "content" "" "embed" (cembed "title" "t"))}}`
	cases := []struct {
		name     string
		output   *string
		checks   []MessageCheck
		edits    []MessageCheck
		failures []string
	}{
		{"nth in a channel and overall", nil, []MessageCheck{
			{ChannelID: 9, Nth: 2, ContentEquals: str("b")}, {Nth: 3, ContentEquals: str("c")}}, nil, nil},
		{"no such message", nil, []MessageCheck{{ChannelID: 9, Nth: 3}}, nil,
			[]string{"sent message check 0: no message #3 sent in channel 9"}},
		{"empty content is checked", nil, []MessageCheck{{ContentEquals: str("")}}, nil,
			[]string{"message check 0: content mismatch"}},
		{"an emptied edit", nil, nil, []MessageCheck{{ChannelID: 7, ContentEquals: str("")}}, nil},
		{"an embed title", nil, []MessageCheck{{ChannelID: 7, EmbedTitle: "t"}}, nil, nil},
		{"a wrong embed title", nil, []MessageCheck{{ChannelID: 7, EmbedTitle: "u"}}, nil,
			[]string{"message check 0: embed title mismatch"}},
		{"an embed title on a message without one", nil, []MessageCheck{{EmbedTitle: "t"}}, nil,
			[]string{`message check 0: expected an embed titled "t" but the message has none`}},
		{"empty output passes", str(""), nil, nil, nil},
		{"output that isn't empty", str(""), nil, nil, nil},
	}
	for i, c := range cases {
		source := src
		if i == len(cases)-1 {
			source += "x"
		}
		tc := &TestCase{Name: c.name, TemplateSource: source,
			Expected:   ExpectedResult{OutputEquals: c.output},
			Assertions: Assertions{SentMessages: c.checks, EditedMessages: c.edits}}
		tc.applyDefaults()
		res := NewRunner(RunnerConfig{}).RunTest(tc)
		want := c.failures
		if i == len(cases)-1 {
			want = []string{"output mismatch"}
		}
		if res.Error != nil || len(res.Failures) != len(want) {
			t.Errorf("%s: %v, %q", c.name, res.Error, res.Failures)
			continue
		}
		for j, f := range want {
			if !strings.Contains(res.Failures[j], f) {
				t.Errorf("%s: failure %q, want %q", c.name, res.Failures[j], f)
			}
		}
	}
}

func TestExpectedErrorStillChecksWhatTheRunDid(t *testing.T) {
	tc := &TestCase{
		Name:           "fails late",
		TemplateSource: `before{{dbSet 0 "k" "v"}}{{index (cslice) 5}}`,
		Expected:       ExpectedResult{ErrorContains: "index out of range", OutputEquals: str("before")},
		Assertions:     Assertions{DBChecks: []DBCheck{{Key: "k", ValueEquals: "v"}}},
	}
	tc.applyDefaults()
	r := NewRunner(RunnerConfig{})
	if res := r.RunTest(tc); !res.Passed {
		t.Errorf("want a pass, got %q %v", res.Failures, res.Error)
	}
	tc.Expected.OutputEquals = str("after")
	if res := r.RunTest(tc); res.Passed {
		t.Error("a wrong output assertion must fail even when the expected error matched")
	}
}

func TestRoleChangeAssertions(t *testing.T) {
	tc := &TestCase{
		Name:           "roles",
		TemplateSource: `{{giveRoleID 5 10 "90"}}`,
		Assertions:     Assertions{RoleChanges: []RoleCheck{{UserID: 5, RoleID: 10, Action: "add", Delay: Duration(90 * time.Second)}}},
	}
	tc.applyDefaults()
	r := NewRunner(RunnerConfig{})
	if res := r.RunTest(tc); !res.Passed {
		t.Errorf("a matching delay passes: %q %v", res.Failures, res.Error)
	}
	tc.Assertions.RoleChanges[0].Delay = Duration(time.Minute)
	if res := r.RunTest(tc); res.Passed {
		t.Error("a different delay fails")
	}
	tc.Assertions = Assertions{NoRoleChanges: true}
	if res := r.RunTest(tc); res.Passed {
		t.Error("no_role_changes fails when a role changed")
	}
	tc.TemplateSource = "nothing"
	if res := r.RunTest(tc); !res.Passed {
		t.Errorf("no_role_changes passes when nothing changed: %q", res.Failures)
	}
}

func str(s string) *string { return &s }

// A deletions check lists every deletion in order; unset fields match anything, and an
// explicit 0s delay means at once
func TestCheckDeletions(t *testing.T) {
	got := []runtime.Deletion{
		{Of: "trigger", ChannelID: 9, MessageID: 5, Delay: 0},
		{Of: "response", ChannelID: 9, Delay: 10 * time.Second},
	}
	zero, ten := Duration(0), Duration(10*time.Second)
	cases := []struct {
		checks []DeletionCheck
		fail   string
	}{
		{[]DeletionCheck{{Of: "trigger", Delay: &zero}, {Delay: &ten}}, ""},
		{[]DeletionCheck{{}, {Of: "response", ChannelID: 9}}, ""},
		{[]DeletionCheck{{Of: "trigger"}}, "expected 1 deletions, got 2"},
		{[]DeletionCheck{{Delay: &ten}, {}}, "deletion 0 doesn't match"},
		{[]DeletionCheck{{MessageID: 6}, {}}, "deletion 0 doesn't match"},
		{[]DeletionCheck{{}, {Of: "message"}}, "deletion 1 doesn't match"},
		{[]DeletionCheck{{}, {ChannelID: 8}}, "deletion 1 doesn't match"},
		{[]DeletionCheck{{Of: "reply"}, {}}, `of is "reply"`},
	}
	for i, c := range cases {
		failures := strings.Join(checkDeletions(got, &c.checks), "\n")
		if (c.fail == "") != (failures == "") || !strings.Contains(failures, c.fail) {
			t.Errorf("case %d: failures %q, want %q", i, failures, c.fail)
		}
	}
	if f := checkDeletions(nil, &[]DeletionCheck{}); f != nil {
		t.Errorf("[] with no deletions: %q", f)
	}
	if f := checkDeletions(got, nil); f != nil {
		t.Errorf("no check: %q", f)
	}
}

// A test whose deletions differ from its deletions check fails
func TestDeletionsCheckFailsTheTest(t *testing.T) {
	r := NewRunner(RunnerConfig{})
	tc := &TestCase{Name: "del", TemplateSource: `{{deleteTrigger 5}}`,
		Assertions: Assertions{Deletions: &[]DeletionCheck{}}}
	tc.applyDefaults()
	res := r.RunTest(tc)
	if res.Error != nil || len(res.Failures) != 1 || !strings.Contains(res.Failures[0], "expected 0 deletions, got 1") {
		t.Errorf("got %v, %q", res.Error, res.Failures)
	}
}

// A reactions check lists every reaction change in order; unset fields match anything
func TestCheckReactions(t *testing.T) {
	got := []runtime.ReactionChange{
		{Action: "remove_all", ChannelID: 9, MessageID: 5},
		{Action: "add", ChannelID: 9, MessageID: 5, Emoji: "👋"},
		{Action: "remove", ChannelID: 9, MessageID: 5, UserID: 3, Emoji: "a"},
	}
	cases := []struct {
		checks []ReactionCheck
		fail   string
	}{
		{[]ReactionCheck{{Action: "remove_all"}, {Emoji: "👋", MessageID: 5}, {UserID: 3, ChannelID: 9}}, ""},
		{[]ReactionCheck{{}, {}}, "expected 2 reaction changes, got 3"},
		{[]ReactionCheck{{Action: "add"}, {}, {}}, "reaction change 0 doesn't match"},
		{[]ReactionCheck{{}, {Emoji: "👍"}, {}}, "reaction change 1 doesn't match"},
		{[]ReactionCheck{{}, {}, {UserID: 4}}, "reaction change 2 doesn't match"},
		{[]ReactionCheck{{ChannelID: 8}, {}, {}}, "reaction change 0 doesn't match"},
		{[]ReactionCheck{{MessageID: 6}, {}, {}}, "reaction change 0 doesn't match"},
		{[]ReactionCheck{{Action: "react"}, {}, {}}, `action is "react"`},
		{[]ReactionCheck{{}, {Response: true}, {}}, "reaction change 1 doesn't match"},
	}
	for i, c := range cases {
		failures := strings.Join(checkReactions(got, &c.checks), "\n")
		if (c.fail == "") != (failures == "") || !strings.Contains(failures, c.fail) {
			t.Errorf("case %d: failures %q, want %q", i, failures, c.fail)
		}
	}
	if f := checkReactions([]runtime.ReactionChange{{Action: "add", ChannelID: 9, Emoji: "a"}}, &[]ReactionCheck{{Response: true}}); f != nil {
		t.Errorf("a reaction on the response: %q", f)
	}
	if f := checkReactions(got, nil); f != nil {
		t.Errorf("no check: %q", f)
	}
	r := NewRunner(RunnerConfig{})
	tc := &TestCase{Name: "react", TemplateSource: `{{addReactions "a"}}`,
		Assertions: Assertions{Reactions: &[]ReactionCheck{}}}
	tc.applyDefaults()
	if res := r.RunTest(tc); res.Error != nil || len(res.Failures) != 1 || !strings.Contains(res.Failures[0], "expected 0 reaction changes, got 1") {
		t.Errorf("a wrong reactions check fails the test: %v, %q", res.Error, res.Failures)
	}
}

// A failed execCC child fails its test, unless the test expects it with warning_contains
func TestFailedChildFailsTheTest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.gohtml"), []byte(`{{index (cslice) 5}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewRunner(RunnerConfig{})
	for _, want := range []string{"", "index out of range"} {
		tc := &TestCase{Name: "child", TemplateSource: `{{execCC 9 nil 0 nil}}ok`, CommandMap: map[int64]string{9: filepath.Join(dir, "broken.gohtml")}}
		tc.Expected.WarningContains = want
		tc.applyDefaults()
		res := r.RunTest(tc)
		failed := len(res.Failures) == 1 && strings.Contains(res.Failures[0], "an execCC child failed")
		if res.Error != nil || failed != (want == "") || (want != "" && len(res.Failures) != 0) {
			t.Errorf("warning_contains %q: %v, %q", want, res.Error, res.Failures)
		}
	}
}

func TestFixedClockAndSeed(t *testing.T) {
	src := `{{currentTime.Unix}} {{.Message.Timestamp.Parse.Unix}} ` +
		`{{humanizeTimeSinceDays (currentTime.Add -172800000000000)}} ` +
		`{{dbSet 0 "k" 1}}{{(dbGet 0 "k").CreatedAt.Unix}} ` +
		`{{randInt 1000000}} {{shuffle (seq 0 10)}} {{adjective}} {{noun}} {{verb}}`
	clock := TestClock(time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC))
	run := func(seed int64) string {
		tc := &TestCase{Name: "fixed", TemplateSource: src, Strict: true}
		tc.Context.Clock = &clock
		s := TestSeed(seed)
		tc.Context.Seed = &s
		tc.applyDefaults()
		res := NewRunner(RunnerConfig{}).RunTest(tc)
		if res.Error != nil || !res.Passed {
			t.Fatalf("run failed: %v %q", res.Error, res.Failures)
		}
		return res.Output
	}

	out := run(7)
	want := "981173106 981173106 2 days 981173106 "
	if !strings.HasPrefix(out, want) {
		t.Errorf("clock: got %q, want the prefix %q", out, want)
	}
	if again := run(7); again != out {
		t.Errorf("the same seed should repeat the run:\n%q\n%q", out, again)
	}
	if other := run(8); other == out {
		t.Errorf("another seed gave the same values: %q", other)
	}
}

func TestClockAndSeedFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.yaml")
	os.WriteFile(path, []byte("defaults:\n  clock: 2001-02-03T04:05:06Z\n  seed: 7\n"+
		"tests:\n  - name: one\n    template_source: '{{currentTime.Unix}}'\n"+
		"  - name: two\n    template_source: '{{currentTime.Unix}}'\n    context: { clock: 2002-02-03T04:05:06Z }\n"), 0o644)
	suite, err := LoadTestSuite(path)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRunner(RunnerConfig{BaseDir: dir})
	for i, want := range []string{"981173106", "1012709106"} {
		tc := &suite.Tests[i]
		if res := r.RunTest(tc); res.Output != want || tc.Context.Seed == nil || *tc.Context.Seed != 7 {
			t.Errorf("%s: output %q (want %q), seed %v", tc.Name, res.Output, want, tc.Context.Seed)
		}
	}
}

// exec_responses in YAML reaches the run, and a suite's defaults apply when a test sets
// none.
func TestExecResponsesFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.yaml")
	os.WriteFile(path, []byte("defaults:\n  exec_responses: { 'ban 6': Banned }\n"+
		"tests:\n"+
		"  - name: per-test\n    template_source: '{{exec \"kick\" 5}}'\n"+
		"    context: { exec_responses: { 'kick 5': Kicked } }\n"+
		"  - name: from-defaults\n    template_source: '{{execAdmin \"ban\" 6}}'\n"), 0o644)
	suite, err := LoadTestSuite(path)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRunner(RunnerConfig{BaseDir: dir})
	for i, want := range []string{"Kicked", "Banned"} {
		tc := &suite.Tests[i]
		if res := r.RunTest(tc); res.Error != nil || res.Output != want {
			t.Errorf("%s: got %q, %v (want %q)", tc.Name, res.Output, res.Error, want)
		}
	}
}

// A declared channel that is the test's channel has its declared name everywhere
func TestTestChannelTakesItsDeclaredName(t *testing.T) {
	tc := &TestCase{Name: "n", TemplateSource: `{{.Channel.Name}} {{(getChannel nil).Name}}`}
	tc.Context.Channel = ChannelDef{ID: 99, Name: "here"}
	tc.Context.Guild.Channels = []ChannelDef{{ID: 99, Name: "other"}}
	tc.applyDefaults()
	if res := NewRunner(RunnerConfig{}).RunTest(tc); res.Error != nil || res.Output != "other other" {
		t.Errorf("got %q, %v", res.Output, res.Error)
	}
}
