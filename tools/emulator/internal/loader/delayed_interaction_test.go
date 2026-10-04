package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDelayedInteractionIsValidatedAtLoad checks that exec_data comes with an interaction
// only in the explicit delayed shape, and that the shape's own fields are checked.
func TestDelayedInteractionIsValidatedAtLoad(t *testing.T) {
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
		{"exec_data without delayed", `
tests:
  - name: x
    template_source: "{{ .ExecData }}"
    context:
      exec_data: { a: 1 }
      interaction: { type: slash }
`, "can't be combined with args, message_content, reaction or exec_data"},
		{"delayed without exec_data", `
tests:
  - name: x
    template_source: "x"
    context:
      interaction: { type: slash, delayed: true }
`, "needs exec_data"},
		{"delayed with a component", `
tests:
  - name: x
    template_source: "x"
    context:
      exec_data: { a: 1 }
      interaction: { type: component, delayed: true, custom_id: "a", message_id: 7 }
`, "write type: slash"},
		{"delayed with options", `
tests:
  - name: x
    template_source: "x"
    context:
      exec_data: { a: 1 }
      interaction: { type: slash, delayed: true, options: { a: "b" } }
`, "takes no subcommand or options"},
		{"responded_to alone", `
tests:
  - name: x
    template_source: "x"
    context:
      interaction: { type: slash, responded_to: true }
`, "only for a delayed run"},
		{"delayed with args", `
tests:
  - name: x
    template_source: "x"
    context:
      args: ["a"]
      exec_data: { a: 1 }
      interaction: { type: slash, delayed: true }
`, "can't be combined with args"},
	}
	for i, c := range cases {
		err := load("d"+string(rune('a'+i))+".yaml", c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want an error containing %q, got %v", c.name, c.want, err)
		}
	}
	if err := load("ok.yaml", `
tests:
  - name: x
    template_source: "x"
    context:
      exec_data: { a: 1 }
      interaction: { type: slash, delayed: true, responded_to: true }
`); err != nil {
		t.Errorf("the delayed shape should load: %v", err)
	}
}

// TestDelayedInteractionRuns runs the delayed shape end to end: the response is a
// followup when responded_to, the first response when not, and the scheduled_runs
// interaction field matches the state recorded at schedule time.
func TestDelayedInteractionRuns(t *testing.T) {
	r := NewRunner(RunnerConfig{BaseDir: t.TempDir()})
	src := "{{/* Trigger type: `Slash Command`\nTrigger: `probe` */}}{{ sendResponse nil .ExecData.Msg }}"
	tc := &TestCase{Name: "delayed", TemplateSource: src,
		Context: ContextDef{ExecData: map[string]interface{}{"Msg": "later"},
			Interaction: &InteractionDef{Type: "slash", Delayed: true, RespondedTo: true}},
		Assertions: Assertions{InteractionResponses: &[]InteractionResponseCheck{
			{Kind: "followup", ContentContains: "later"}}}}
	tc.applyDefaults()
	if res := r.RunTest(tc); !res.Passed {
		t.Errorf("responded: err=%v failures=%q", res.Error, res.Failures)
	}
	tc.Context.Interaction.RespondedTo = false
	if res := r.RunTest(tc); len(res.Failures) != 1 || !strings.Contains(res.Failures[0], `doesn't match (kind "followup"`) {
		t.Errorf("pending should be a first response: err=%v failures=%q", res.Error, res.Failures)
	}

	// scheduled_runs: interaction
	sched := "{{/* Trigger type: `Slash Command`\nTrigger: `probe` */}}{{ sendResponse nil \"a\" }}{{ execCC 5 nil 1 nil }}"
	tc = &TestCase{Name: "schedules", TemplateSource: sched,
		Context:    ContextDef{Interaction: &InteractionDef{Type: "slash"}},
		Assertions: Assertions{ScheduledRuns: &[]ScheduledRunCheck{{CCID: 5, Interaction: "responded"}}}}
	tc.applyDefaults()
	if res := r.RunTest(tc); !res.Passed {
		t.Errorf("schedules: err=%v failures=%q", res.Error, res.Failures)
	}
	tc.Assertions.ScheduledRuns = &[]ScheduledRunCheck{{CCID: 5, Interaction: "pending"}}
	if res := r.RunTest(tc); len(res.Failures) != 1 || !strings.Contains(res.Failures[0], "interaction pending") {
		t.Errorf("wrong state should fail: failures=%q", res.Failures)
	}
	tc.Assertions.ScheduledRuns = &[]ScheduledRunCheck{{Interaction: "sometimes"}}
	if res := r.RunTest(tc); len(res.Failures) != 1 || !strings.Contains(res.Failures[0], "takes none, pending or responded") {
		t.Errorf("bad value should fail: failures=%q", res.Failures)
	}
}
