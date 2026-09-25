package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		Expected:       ExpectedResult{OutputEquals: "yes"},
		Assertions:     Assertions{SentMessages: []MessageCheck{{ContentEquals: "from test"}}},
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

func TestExpectedErrorStillChecksWhatTheRunDid(t *testing.T) {
	tc := &TestCase{
		Name:           "fails late",
		TemplateSource: `before{{dbSet 0 "k" "v"}}{{index (cslice) 5}}`,
		Expected:       ExpectedResult{ErrorContains: "index out of range", OutputEquals: "before"},
		Assertions:     Assertions{DBChecks: []DBCheck{{Key: "k", ValueEquals: "v"}}},
	}
	tc.applyDefaults()
	r := NewRunner(RunnerConfig{})
	if res := r.RunTest(tc); !res.Passed {
		t.Errorf("want a pass, got %q %v", res.Failures, res.Error)
	}
	tc.Expected.OutputEquals = "after"
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
