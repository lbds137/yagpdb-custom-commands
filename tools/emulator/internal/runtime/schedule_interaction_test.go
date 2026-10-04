package runtime

import (
	"strings"
	"testing"
)

// TestScheduledRunCarriesTheInteractionState checks that a delayed run records its
// scheduler's interaction as YAGPDB's stored CurrentFrame would hold it, for execCC with a
// delay and scheduleUniqueCC alike (customcommands/tmplextensions.go:256-282, :344).
func TestScheduledRunCarriesTheInteractionState(t *testing.T) {
	cases := []struct {
		name        string
		interaction func(ctx *ExecutionContext)
		src         string
		want        ScheduledInteraction
	}{
		{"no interaction", func(*ExecutionContext) {}, `{{execCC 5 nil 1 nil}}`, ScheduledNoInteraction},
		{"pending", func(c *ExecutionContext) { c.SetInteractionDelayed("x", false) },
			`{{execCC 5 nil 1 nil}}`, ScheduledInteractionPending},
		{"responded at schedule time", func(c *ExecutionContext) { c.SetInteractionDelayed("x", false) },
			`{{sendResponse nil "first"}}{{execCC 5 nil 1 nil}}`, ScheduledInteractionResponded},
		{"responded before the run", func(c *ExecutionContext) { c.SetInteractionDelayed("x", true) },
			`{{execCC 5 nil 1 nil}}`, ScheduledInteractionResponded},
		{"unique, pending", func(c *ExecutionContext) { c.SetInteractionDelayed("x", false) },
			`{{scheduleUniqueCC 5 nil 1 "k" nil}}`, ScheduledInteractionPending},
		{"unique, responded", func(c *ExecutionContext) { c.SetInteractionDelayed("x", true) },
			`{{scheduleUniqueCC 5 nil 1 "k" nil}}`, ScheduledInteractionResponded},
		{"unique, none", func(*ExecutionContext) {}, `{{scheduleUniqueCC 5 nil 1 "k" nil}}`, ScheduledNoInteraction},
	}
	for _, c := range cases {
		ctx := newCtx(false, true)
		c.interaction(ctx)
		if _, err := run(t, ctx, c.src); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		runs := ctx.ScheduledRuns()
		if len(runs) != 1 || runs[0].Interaction != c.want {
			t.Errorf("%s: want one run with interaction %q, got %+v", c.name, c.want, runs)
		}
	}
}

// TestDelayedInteractionRun checks the delayed half of a slash command: .Interaction is
// there, the handler's keys are not (handle_timed.go:102-117), and a response is a
// followup exactly when the stored interaction was responded to.
func TestDelayedInteractionRun(t *testing.T) {
	probe := `{{ if .Interaction.Token }}tok{{ end }}|{{ .IsSlashCommand }}|{{ .SubCommand }}|{{ .Options }}|{{ .InteractionData }}`
	ctx := newCtx(false, true)
	ctx.ExecData = map[string]interface{}{"Description": "x"}
	ctx.SetInteractionDelayed("Hebrew", true)
	out, err := run(t, ctx, probe)
	if err != nil {
		t.Fatal(err)
	}
	if out != "tok|<no value>|<no value>|<no value>|<no value>" {
		t.Errorf("a delayed run has .Interaction and no handler keys, got %q", out)
	}
	if ctx.Interaction.DataCommand.Name != "hebrew" {
		t.Errorf("name %q", ctx.Interaction.DataCommand.Name)
	}

	// responded: the run's response is a followup
	ctx = newCtx(false, true)
	ctx.ExecData = map[string]interface{}{}
	ctx.SetInteractionDelayed("hebrew", true)
	if _, err := run(t, ctx, `{{ sendResponse nil "later" }}`); err != nil {
		t.Fatal(err)
	}
	if got := ctx.InteractionResponses; len(got) != 1 || got[0].Kind != ResponseFollowup {
		t.Errorf("responded: %+v", got)
	}
	// not responded: it is the interaction's first response
	ctx = newCtx(false, true)
	ctx.ExecData = map[string]interface{}{}
	ctx.SetInteractionDelayed("hebrew", false)
	if _, err := run(t, ctx, `{{ sendResponse nil "first" }}`); err != nil {
		t.Fatal(err)
	}
	if got := ctx.InteractionResponses; len(got) != 1 || got[0].Kind != ResponseMessage {
		t.Errorf("pending: %+v", got)
	}
}

// TestDelayedInteractionMessage checks what handle_timed.go restores: .Message and the
// trigger message are the slash scheduler's blank message (ID 0, the invoker as author),
// and .Interaction is the inner *Interaction, not the CustomCommandInteraction wrapper.
func TestDelayedInteractionMessage(t *testing.T) {
	ctx := newCtx(false, true)
	ctx.ExecData = map[string]interface{}{}
	ctx.SetInteractionDelayed("hebrew", true)
	out, err := run(t, ctx, `{{ .Message.ID }}|{{ .Message.Author.ID }}|{{ printf "%T" .Interaction }}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "0|") || !strings.HasSuffix(out, "|*types.Interaction") {
		t.Errorf("delayed run .Message / .Interaction: got %q", out)
	}
	if got := ctx.triggerMsg(); got.ID != 0 || got.Author.ID != ctx.UserID {
		t.Errorf("delayed run trigger message %+v, want the blank slash message", got)
	}
}
