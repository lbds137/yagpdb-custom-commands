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
