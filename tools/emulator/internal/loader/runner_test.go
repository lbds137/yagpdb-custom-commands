package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	write("interval.gohtml", header("Minute interval", "")+`ran`)
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
		{"interval", "interval.gohtml", ContextDef{}, "ran", ""},
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
