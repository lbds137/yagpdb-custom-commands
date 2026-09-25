package runtime

import (
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// buildExecCmdLine is YAGPDB's: strings are quoted except switches, numbers and users
// aren't, and a nil or unsupported argument is an error
func TestBuildExecCmdLine(t *testing.T) {
	line, err := buildExecCmdLine("kick", int64(5), "a reason", "-switch", `\-dash`, 2.5,
		&types.DiscordUser{ID: 7}, types.DiscordUser{ID: 8}, []string{"x", "y"}, uint8(3))
	want := `kick 5 "a reason" -switch "-dash" 2.5E+00 <@7> <@8> x y 3 `
	if err != nil || line != want {
		t.Errorf("got %q, %v; want %q", line, err, want)
	}
	if _, err := buildExecCmdLine("kick", nil); err == nil || err.Error() != "Nil arg passed" {
		t.Errorf("nil: %v", err)
	}
	if _, err := buildExecCmdLine("kick", true); err == nil || !strings.HasPrefix(err.Error(), "Unknown type in exec") {
		t.Errorf("bool: %v", err)
	}
}

// An undeclared exec/execAdmin call is recorded, returns "" (the emulator can't run the
// bot command), and warns.
func TestExecIsRecordedNotRun(t *testing.T) {
	ctx := newCtx(true, true)
	out, err := run(t, ctx, `{{$r := exec "kick" 5 "spam"}}[{{$r}}]{{execAdmin "ban" 6}}`)
	if err != nil || out != "[]" {
		t.Fatalf("got %q, %v", out, err)
	}
	want := []Exec{{Line: `kick 5 "spam"`, ChannelID: ctx.ChannelID}, {Admin: true, Line: "ban 6", ChannelID: ctx.ChannelID}}
	if len(ctx.Execs) != 2 || ctx.Execs[0] != want[0] || ctx.Execs[1] != want[1] {
		t.Errorf("got %+v", ctx.Execs)
	}
	warnings := kinds(ctx, KindExec)
	if len(warnings) != 2 {
		t.Fatalf("got %d exec warnings, want 2: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], `exec kick 5 "spam" isn't run by the emulator`) {
		t.Errorf("exec warning: %q", warnings[0])
	}
	if !strings.Contains(warnings[1], `execAdmin ban 6 isn't run by the emulator`) {
		t.Errorf("execAdmin warning: %q", warnings[1])
	}
}

// A declared response in ExecResponses is returned, with no warning.
func TestExecReturnsDeclaredResponse(t *testing.T) {
	ctx := newCtx(true, true)
	ctx.ExecResponses = map[string]string{`kick 5 "spam"`: "Kicked"}
	out, err := run(t, ctx, `{{$r := exec "kick" 5 "spam"}}[{{$r}}]`)
	if err != nil || out != "[Kicked]" {
		t.Fatalf("got %q, %v", out, err)
	}
	if warnings := kinds(ctx, KindExec); len(warnings) != 0 {
		t.Errorf("got warnings %v, want none", warnings)
	}
}

// A declared response of "" is allowed and silences the warning.
func TestExecDeclaredEmptyResponseSilencesWarning(t *testing.T) {
	ctx := newCtx(true, true)
	ctx.ExecResponses = map[string]string{"clean 10": ""}
	out, err := run(t, ctx, `{{$r := exec "clean" 10}}[{{$r}}]`)
	if err != nil || out != "[]" {
		t.Fatalf("got %q, %v", out, err)
	}
	if warnings := kinds(ctx, KindExec); len(warnings) != 0 {
		t.Errorf("got warnings %v, want none", warnings)
	}
}

// An execCC child sees the parent's ExecResponses.
func TestExecCCChildInheritsExecResponses(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{exec "kick" 5 "spam"}}`)
	ctx := newCtx(true, true)
	ctx.ExecResponses = map[string]string{`kick 5 "spam"`: "Kicked"}
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	out, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`)
	if err != nil || out != "" {
		t.Fatalf("got %q, %v", out, err)
	}
	if len(ctx.Execs) != 1 || ctx.Execs[0].Line != `kick 5 "spam"` {
		t.Fatalf("got %+v", ctx.Execs)
	}
	if warnings := kinds(ctx, KindExec); len(warnings) != 0 {
		t.Errorf("got warnings %v, want none", warnings)
	}
}

// exec and execAdmin share YAGPDB's limit of 5; a refused call isn't recorded
func TestExecLimit(t *testing.T) {
	ctx := newCtx(true, true)
	_, err := run(t, ctx, `{{range seq 0 6}}{{exec "ping"}}{{end}}`)
	if err == nil || len(ctx.Execs) != 5 {
		t.Errorf("got %d execs, %v", len(ctx.Execs), err)
	}
}
