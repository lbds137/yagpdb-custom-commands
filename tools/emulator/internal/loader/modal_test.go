package loader

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/runtime"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
)

// TestModalContextIsValidatedAtLoad checks the load-time rejections of a modal
// submission (a field of another type, a missing custom_id, a field count Discord never
// sends, a fields form that loses the order) and that fields keep the order written.
func TestModalContextIsValidatedAtLoad(t *testing.T) {
	dir := t.TempDir()
	load := func(name, src string) ([]*TestCase, error) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return LoadTestFile(path)
	}
	test := func(interaction string) string {
		return "tests:\n  - name: x\n    template_source: \"{{ .Cmd }}\"\n    context:\n      interaction: " +
			interaction + "\n"
	}
	cases := []struct{ name, interaction, want string }{
		{"no custom_id", `{ type: modal, fields: { a: "1" } }`, "needs the custom_id of the modal submitted"},
		{"a menu's values", `{ type: modal, custom_id: "m", fields: { a: "1" }, values: ["x"] }`,
			"interaction values isn't a type: modal field (it takes custom_id, message_id, fields, form)"},
		{"fields on a click", `{ type: component, custom_id: "x", message_id: 7, fields: { a: "1" } }`,
			"interaction fields isn't a type: component field"},
		{"six fields", `{ type: modal, custom_id: "m", fields: { a: "1", b: "2", c: "3", d: "4", e: "5", f: "6" } }`,
			"a modal submits 1 to 5 fields, not 6"},
		{"fields as a list", `{ type: modal, custom_id: "m", fields: ["a"] }`, "write a mapping of custom ID to text"},
		{"a field that isn't text", `{ type: modal, custom_id: "m", fields: { a: [1] } }`,
			"each entry is a custom ID and its text"},
		{"a field given twice", `{ type: modal, custom_id: "m", fields: { a: "1", a: "2" } }`, `mapping key "a" already defined`}, // yaml.v3's own check
		{"with args", "{ type: modal, custom_id: \"m\", fields: { a: \"1\" } }\n      args: [\"a\"]",
			"can't be combined with args"},
		{"an empty field ID", `{ type: modal, custom_id: "m", fields: { "": "x" } }`,
			"interaction fields: a custom ID can't be empty"},
		{"IDs that collide once prefixed", `{ type: modal, custom_id: "m", fields: { x: "1", templates-x: "2" } }`,
			`interaction fields: "x" and "templates-x" are the same custom ID (templates-x)`},
		{"the bare prefix", `{ type: modal, custom_id: "m", fields: { templates-: "x" } }`,
			"interaction fields: a custom ID can't be empty"},
		{"an unknown form", `{ type: modal, custom_id: "m", fields: { a: "1" }, form: grid }`,
			`interaction form "grid" isn't action_row or label`},
		{"a form on a click", `{ type: component, custom_id: "x", message_id: 7, form: label }`,
			"interaction form isn't a type: component field"},
	}
	for i, c := range cases {
		_, err := load("t"+string(rune('a'+i))+".yaml", test(c.interaction))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want an error containing %q, got %v", c.name, c.want, err)
		}
	}

	// The order written is the order submitted, whatever the IDs sort as
	tf, err := load("ordered.yaml", test(`{ type: modal, custom_id: "edit:rule:3", message_id: 7, `+
		`fields: { zeta: "last id first", alpha: "", mid: 42 } }`))
	if err != nil {
		t.Fatal(err)
	}
	got := tf[0].Context.Interaction.modal()
	want := runtime.ModalSubmission{CustomID: "edit:rule:3", MessageID: 7, Fields: []runtime.ModalField{
		{CustomID: "zeta", Value: "last id first"}, {CustomID: "alpha", Value: ""}, {CustomID: "mid", Value: "42"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("submission:\n got %+v\nwant %+v", got, want)
	}
	tf, err = load("label.yaml", test(`{ type: modal, custom_id: "m", fields: { a: "1" }, form: label }`))
	if err != nil || tf[0].Context.Interaction.modal().Form != runtime.ModalFormLabel {
		t.Errorf("form: label: %v", err)
	}
}

// A modal response has no message, so its snapshot entry has no message_id; the other
// kinds keep theirs, 0 or not
func TestSnapshotModalHasNoMessageID(t *testing.T) {
	ctx := runtime.NewExecutionContext(1, state.NewMockDB(1))
	ctx.InteractionResponses = []runtime.InteractionResponse{
		{Kind: runtime.ResponseModal, Title: "T", CustomID: "m", Fields: []string{"a"}},
		{Kind: runtime.ResponseMessage, MessageID: 5, Content: "x"},
	}
	data, err := yaml.Marshal(takeSnapshot("", ctx, ctx.DB))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "message_id:"); n != 1 || !strings.Contains(string(data), "message_id: 5") {
		t.Errorf("only the message has a message_id:\n%s", data)
	}
}

// TestModalNeedsAModalTrigger checks the run-time pairing: a submission needs a Modal
// Submission template, such a template needs a submission, a message_id must be
// declared, and a custom ID the trigger doesn't match is a trigger mismatch.
func TestModalNeedsAModalTrigger(t *testing.T) {
	r := NewRunner(RunnerConfig{BaseDir: t.TempDir()})
	const modal = "{{/* Trigger type: `Modal Submission`\nTrigger: `^edit:` */}}{{ .CustomID }}"
	submit := &InteractionDef{Type: "modal", CustomID: "edit:1",
		Fields: ModalFields{{CustomID: "text", Value: "x"}}}
	runCase := func(name, src string, in *InteractionDef, messages []MessageDef) *TestResult {
		tc := &TestCase{Name: name, TemplateSource: src, Context: ContextDef{Interaction: in, Messages: messages}}
		tc.applyDefaults()
		return r.RunTest(tc)
	}
	for _, c := range []struct {
		name, src string
		in        *InteractionDef
		want      string
	}{
		{"modal on a Component template", "{{/* Trigger type: `Message Component`\nTrigger: `^edit:` */}}x",
			submit, "a modal interaction needs a Modal Submission trigger"},
		{"click on a Modal template", modal, &InteractionDef{Type: "component", CustomID: "edit:1", MessageID: 7},
			"a component interaction needs a Message Component trigger"},
		{"modal template without an interaction", modal, nil, "give context.interaction { type: modal"},
		{"an undeclared source message", modal, &InteractionDef{Type: "modal", CustomID: "edit:1", MessageID: 8,
			Fields: submit.Fields}, "isn't a message in channel"},
	} {
		res := runCase(c.name, c.src, c.in, nil)
		if res.Error == nil || !strings.Contains(res.Error.Error(), c.want) {
			t.Errorf("%s: want an error containing %q, got %v (failures %q)", c.name, c.want, res.Error, res.Failures)
		}
	}
	res := runCase("a submission that runs", modal, submit, nil)
	if res.Error != nil || len(res.Failures) > 0 || strings.TrimSpace(res.Output) != "edit:1" {
		t.Errorf("err=%v failures=%q output=%q", res.Error, res.Failures, res.Output)
	}

	// The modal response check: title, custom_id and fields exactly
	const opener = "{{/* Trigger type: `Message Component`\nTrigger: `^open$` */}}" +
		`{{ sendModal (cmodal "title" "Edit" "custom_id" "edit:1" "fields" (cslice (sdict "custom_id" "a") (sdict "custom_id" "b"))) }}`
	click := &InteractionDef{Type: "component", CustomID: "open", MessageID: 7}
	messages := []MessageDef{{ID: 7, ChannelID: 123456789012345678, AuthorID: 1}}
	tc := &TestCase{Name: "opens a modal", TemplateSource: opener,
		Context: ContextDef{Interaction: click, Messages: messages},
		Assertions: Assertions{InteractionResponses: &[]InteractionResponseCheck{
			{Kind: "modal", Title: str("Edit"), CustomID: str("edit:1"), Fields: &[]string{"a", "b"}}}}}
	tc.applyDefaults()
	if res := r.RunTest(tc); !res.Passed {
		t.Errorf("err=%v failures=%q", res.Error, res.Failures)
	}
	tc.Assertions.InteractionResponses = &[]InteractionResponseCheck{{Kind: "modal", Fields: &[]string{"b", "a"}}}
	if res := r.RunTest(tc); len(res.Failures) != 1 || !strings.Contains(res.Failures[0], "the modal doesn't match") {
		t.Errorf("fields out of order: failures=%q", res.Failures)
	}
	tc.Assertions.InteractionResponses = &[]InteractionResponseCheck{{Kind: "modal", CustomID: str("templates-edit:1")}}
	if res := r.RunTest(tc); len(res.Failures) != 1 {
		t.Errorf("the custom_id is checked without its prefix: failures=%q", res.Failures)
	}
}
