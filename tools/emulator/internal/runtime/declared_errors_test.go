package runtime

import (
	"strings"
	"testing"
)

// A command_status id fails execCC and scheduleUniqueCC with YAGPDB's own error text, and
// wins over the id's command_map entry.
func TestCommandStatusErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `ran`)
	for status, want := range map[string]string{
		"missing":        "Couldn't find custom command",
		"group_disabled": "custom command group is disabled",
		"disabled":       "custom command is disabled",
	} {
		for _, call := range []string{`{{execCC 7 nil 0 nil}}`, `{{scheduleUniqueCC 7 nil 60 "k" nil}}`} {
			ctx := newCtx(true, true)
			ctx.TemplateBaseDir = dir
			ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
			ctx.CommandStatus = map[int64]string{7: status}
			out, err := run(t, ctx, call)
			if err == nil || !strings.Contains(err.Error(), want) || out != "" {
				t.Errorf("%s %s: got %q, %v; want error %q", status, call, out, err, want)
			}
		}
	}
	// an id without a status is unaffected
	ctx := newCtx(true, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml", 8: "child.gohtml"}
	ctx.CommandStatus = map[int64]string{8: "disabled"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`); err != nil || len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != "ran" {
		t.Errorf("unaffected id: %v, %+v", err, ctx.SentMessages)
	}
}

// The status reaches an execCC child, where try catches the error.
func TestCommandStatusReachesChild(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{try}}{{execCC 8 nil 0 nil}}{{catch}}child caught: {{.Error}}{{end}}`)
	ctx := newCtx(true, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	ctx.CommandStatus = map[int64]string{8: "disabled"}
	_, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`)
	if err != nil || len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != "child caught: custom command is disabled" {
		t.Fatalf("got %v, %+v", err, ctx.SentMessages)
	}
}

func TestValidCommandStatus(t *testing.T) {
	for _, s := range []string{"missing", "disabled", "group_disabled"} {
		if !ValidCommandStatus(s) {
			t.Errorf("%s should be valid", s)
		}
	}
	if ValidCommandStatus("gone") {
		t.Error("gone should be invalid")
	}
}

// A declared exec error returns "exec/execadmin, run: <message>", is still recorded, and
// reaches an execCC child.
func TestExecErrorText(t *testing.T) {
	ctx := newCtx(true, true)
	ctx.ExecErrors = map[string]string{`kick 5 "spam"`: "Missing Permissions"}
	out, err := run(t, ctx, `{{exec "kick" 5 "spam"}}`)
	if err == nil || !strings.Contains(err.Error(), "exec/execadmin, run: Missing Permissions") || out != "" {
		t.Fatalf("got %q, %v", out, err)
	}
	if len(ctx.Execs) != 1 || ctx.Execs[0].Line != `kick 5 "spam"` {
		t.Errorf("the failed exec should still be recorded: %+v", ctx.Execs)
	}
	if warnings := kinds(ctx, KindExec); len(warnings) != 0 {
		t.Errorf("got warnings %v, want none", warnings)
	}

	// execAdmin too, caught by try, and through an execCC child
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{try}}{{execAdmin "kick" 5 "spam"}}{{catch}}[{{.Error}}]{{end}}`)
	ctx = newCtx(true, true)
	ctx.ExecErrors = map[string]string{`kick 5 "spam"`: "Missing Permissions"}
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	_, err = run(t, ctx, `{{execCC 7 nil 0 nil}}`)
	if err != nil || len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != "[exec/execadmin, run: Missing Permissions]" {
		t.Errorf("child: %v, %+v", err, ctx.SentMessages)
	}
}
