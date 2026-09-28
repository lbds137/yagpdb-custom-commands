package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInteractionContextIsValidatedAtLoad checks the load-time rejections of an
// interaction: combined with another trigger, an unmodelled type, a bad component, and
// a suite default.
func TestInteractionContextIsValidatedAtLoad(t *testing.T) {
	dir := t.TempDir()
	load := func(name, src string) error {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadTestFile(path)
		return err
	}
	cases := []struct{ name, src, want string }{
		{"with args", `
tests:
  - name: x
    template_source: "{{ .CustomID }}"
    context:
      args: ["a"]
      interaction: { type: component, custom_id: "pg:1", message_id: 7 }
`, "can't be combined with args, message_content, reaction or exec_data"},
		{"with a reaction", `
tests:
  - name: x
    template_source: "{{ .CustomID }}"
    context:
      reaction: { emoji: "x", message_id: 7 }
      interaction: { type: component, custom_id: "pg:1", message_id: 7 }
`, "can't be combined with args, message_content, reaction or exec_data"},
		{"a modal isn't modelled yet", `
tests:
  - name: x
    template_source: "{{ .CustomID }}"
    context:
      interaction: { type: modal, custom_id: "edit:1", message_id: 7 }
`, `interaction type "modal" isn't modelled yet`},
		{"an unknown type", `
tests:
  - name: x
    template_source: "{{ .CustomID }}"
    context:
      interaction: { type: click, custom_id: "pg:1", message_id: 7 }
`, "write type: component"},
		{"an unknown component", `
tests:
  - name: x
    template_source: "{{ .CustomID }}"
    context:
      interaction: { type: component, custom_id: "pg:1", message_id: 7, component: dropdown }
`, "isn't button, string_menu"},
		{"a button has no values", `
tests:
  - name: x
    template_source: "{{ .CustomID }}"
    context:
      interaction: { type: component, custom_id: "pg:1", message_id: 7, values: ["a"] }
`, "values need a menu component"},
		{"no message", `
tests:
  - name: x
    template_source: "{{ .CustomID }}"
    context:
      interaction: { type: component, custom_id: "pg:1" }
`, "needs the message_id clicked"},
		{"a suite default", `
defaults:
  interaction: { type: component, custom_id: "pg:1", message_id: 7 }
tests:
  - name: x
    template_source: "{{ .CustomID }}"
`, "set them per test"},
	}
	for i, c := range cases {
		err := load("t"+string(rune('a'+i))+".yaml", c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want an error containing %q, got %v", c.name, c.want, err)
		}
	}
	// A single-test file is validated too
	if err := load("single.yaml", `
name: x
template_source: "{{ .CustomID }}"
context:
  args: ["a"]
  interaction: { type: component, custom_id: "pg:1", message_id: 7 }
`); err == nil || !strings.Contains(err.Error(), "can't be combined") {
		t.Errorf("single test case: %v", err)
	}
}

// TestInteractionNeedsAComponentTrigger checks the run-time pairing: a click needs a
// Message Component template, and such a template needs a click.
func TestInteractionNeedsAComponentTrigger(t *testing.T) {
	r := NewRunner(RunnerConfig{BaseDir: t.TempDir()})
	click := &InteractionDef{Type: "component", CustomID: "pg:1", MessageID: 7}
	messages := []MessageDef{{ID: 7, ChannelID: 123456789012345678, AuthorID: 1}}

	tc := &TestCase{Name: "click on a Command template", TemplateSource: "{{/* Trigger type: `Command`\nTrigger: `pg` */}}x",
		Context: ContextDef{Interaction: click, Messages: messages}}
	tc.applyDefaults()
	if res := r.RunTest(tc); res.Error == nil || !strings.Contains(res.Error.Error(), "needs a Message Component trigger") {
		t.Errorf("got %v", res.Error)
	}

	tc = &TestCase{Name: "component template without a click", TemplateSource: "{{/* Trigger type: `Message Component`\nTrigger: `^pg:` */}}x"}
	tc.applyDefaults()
	if res := r.RunTest(tc); res.Error == nil || !strings.Contains(res.Error.Error(), "give context.interaction") {
		t.Errorf("got %v", res.Error)
	}

	tc = &TestCase{Name: "message in another channel", TemplateSource: "{{/* Trigger type: `Message Component`\nTrigger: `^pg:` */}}x",
		Context: ContextDef{Interaction: click, Messages: []MessageDef{{ID: 7, ChannelID: 999, AuthorID: 1}}}}
	tc.applyDefaults()
	if res := r.RunTest(tc); res.Error == nil || !strings.Contains(res.Error.Error(), "isn't a message in channel") {
		t.Errorf("got %v", res.Error)
	}

	tc = &TestCase{Name: "a click that runs", TemplateSource: "{{/* Trigger type: `Message Component`\nTrigger: `^pg:` */}}{{ .CustomID }}",
		Context:  ContextDef{Interaction: click, Messages: messages},
		Expected: ExpectedResult{OutputEquals: str("pg:1")},
		Assertions: Assertions{InteractionResponses: &[]InteractionResponseCheck{
			{Kind: "message", Ephemeral: boolPtr(false), ContentContains: "pg:1"}}}}
	tc.applyDefaults()
	if res := r.RunTest(tc); !res.Passed {
		t.Errorf("err=%v failures=%q", res.Error, res.Failures)
	}
	// The assertion catches a wrong kind and a wrong count
	tc.Assertions.InteractionResponses = &[]InteractionResponseCheck{{Kind: "update"}}
	if res := r.RunTest(tc); len(res.Failures) != 1 || !strings.Contains(res.Failures[0], "doesn't match (kind \"update\"") {
		t.Errorf("failures=%q", res.Failures)
	}
	tc.Assertions.InteractionResponses = &[]InteractionResponseCheck{}
	if res := r.RunTest(tc); len(res.Failures) != 1 || !strings.Contains(res.Failures[0], "expected 0 interaction responses, got 1") {
		t.Errorf("failures=%q", res.Failures)
	}
	tc.Assertions.InteractionResponses = &[]InteractionResponseCheck{{Kind: "reply"}}
	if res := r.RunTest(tc); len(res.Failures) != 1 || !strings.Contains(res.Failures[0], "it takes message, followup, deferred_edit or update") {
		t.Errorf("failures=%q", res.Failures)
	}
}

func boolPtr(b bool) *bool { return &b }

// TestApplicationCommandContextIsValidatedAtLoad checks the load-time rejections of a
// slash or context menu interaction: a field of another type, a missing one, a value
// that isn't a scalar.
func TestApplicationCommandContextIsValidatedAtLoad(t *testing.T) {
	dir := t.TempDir()
	load := func(name, src string) error {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadTestFile(path)
		return err
	}
	test := func(interaction string) string {
		return "tests:\n  - name: x\n    template_source: \"{{ .Cmd }}\"\n    context:\n      interaction: " + interaction + "\n"
	}
	cases := []struct{ name, interaction, want string }{
		{"slash with a custom_id", `{ type: slash, custom_id: "x" }`, "interaction custom_id isn't a type: slash field (it takes subcommand, options)"},
		{"slash with a list option", `{ type: slash, options: { who: [1, 2] } }`, `interaction option "who": write a string, a number, true/false or an ID`},
		{"user menu without a target", `{ type: user_menu }`, "needs the target user's ID (target:)"},
		{"user menu with a message", `{ type: user_menu, target: 5, message_id: 7 }`, "interaction message_id isn't a type: user_menu field"},
		{"message menu without a message", `{ type: message_menu }`, "needs the message_id the entry was used on"},
		{"message menu with options", `{ type: message_menu, message_id: 7, options: { a: 1 } }`, "interaction options isn't a type: message_menu field"},
		{"component with options", `{ type: component, custom_id: "x", message_id: 7, options: {} }`, "interaction options isn't a type: component field"},
		{"slash with args", "{ type: slash }\n      args: [\"a\"]", "can't be combined with args"},
	}
	for i, c := range cases {
		err := load("t"+string(rune('a'+i))+".yaml", test(c.interaction))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want an error containing %q, got %v", c.name, c.want, err)
		}
	}
	for name, interaction := range map[string]string{
		"a bare slash":   `{ type: slash }`,
		"typed options":  `{ type: slash, subcommand: get, options: { key: "k", n: 1, f: 1.5, b: true } }`,
		"a user menu":    `{ type: user_menu, target: 5 }`,
		"a message menu": `{ type: message_menu, message_id: 7 }`,
	} {
		if err := load(strings.ReplaceAll(name, " ", "_")+".yaml", test(interaction)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestApplicationCommandNeedsItsTrigger checks the run-time pairing and the header
// resolution: the interaction type and the trigger type must agree, a required option
// or a mismatched value is an error before the run, and a bad header fails the test.
func TestApplicationCommandNeedsItsTrigger(t *testing.T) {
	r := NewRunner(RunnerConfig{BaseDir: t.TempDir()})
	const probe = "{{/* Trigger type: `Slash Command`\nTrigger: `probe`\nSlash option: `who user! a member`\n" +
		"Slash option: `n integer a number` */}}{{ .Options.who.ID }} {{ len .CmdArgs }}"
	runCase := func(name, src string, in *InteractionDef) *TestResult {
		tc := &TestCase{Name: name, TemplateSource: src, Context: ContextDef{Interaction: in,
			Messages: []MessageDef{{ID: 7, ChannelID: 123456789012345678, AuthorID: 5}}}}
		tc.applyDefaults()
		return r.RunTest(tc)
	}
	errCases := []struct {
		name, src string
		in        *InteractionDef
		want      string
	}{
		{"slash on a Command template", "{{/* Trigger type: `Command`\nTrigger: `probe` */}}x", &InteractionDef{Type: "slash"}, "a slash interaction needs a Slash Command trigger"},
		{"click on a slash template", probe, &InteractionDef{Type: "component", CustomID: "x", MessageID: 7}, "a component interaction needs a Message Component trigger"},
		{"user menu on a message menu template", "{{/* Trigger type: `Message Context Menu`\nTrigger: `Quote` */}}x", &InteractionDef{Type: "user_menu", Target: 5}, "a user_menu interaction needs a User Context Menu trigger"},
		{"message menu on a slash template", probe, &InteractionDef{Type: "message_menu", MessageID: 7}, "a message_menu interaction needs a Message Context Menu trigger"},
		{"slash template without an interaction", probe, nil, "give context.interaction { type: slash"},
		{"menu template without an interaction", "{{/* Trigger type: `User Context Menu`\nTrigger: `View` */}}x", nil, "runs from the context menu: give context.interaction"},
		{"missing required option", probe, &InteractionDef{Type: "slash", Options: map[string]interface{}{"n": 1}}, `the required option "who" (user) wasn't given`},
		{"type mismatch", probe, &InteractionDef{Type: "slash", Options: map[string]interface{}{"who": 5, "n": "one"}}, `option "n": integer takes a whole number, not the string "one"`},
		{"unknown option", probe, &InteractionDef{Type: "slash", Options: map[string]interface{}{"who": 5, "nope": 1}}, `option "nope" isn't one the header declares`},
		{"bad header", "{{/* Trigger type: `Slash Command`\nTrigger: `Probe` */}}x", &InteractionDef{Type: "slash"}, "Slash command name must be lowercase"},
	}
	for _, c := range errCases {
		res := runCase(c.name, c.src, c.in)
		if res.Error == nil || !strings.Contains(res.Error.Error(), c.want) {
			t.Errorf("%s: want an error containing %q, got %v (failures %q)", c.name, c.want, res.Error, res.Failures)
		}
	}
	res := runCase("a slash that runs", probe, &InteractionDef{Type: "slash", Options: map[string]interface{}{"who": 5}})
	if res.Error != nil || len(res.Failures) > 0 || strings.TrimSpace(res.Output) != "5 1" {
		t.Errorf("err=%v failures=%q output=%q", res.Error, res.Failures, res.Output)
	}
}
