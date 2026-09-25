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
}

// exec and execAdmin share YAGPDB's limit of 5; a refused call isn't recorded
func TestExecLimit(t *testing.T) {
	ctx := newCtx(true, true)
	_, err := run(t, ctx, `{{range seq 0 6}}{{exec "ping"}}{{end}}`)
	if err == nil || len(ctx.Execs) != 5 {
		t.Errorf("got %d execs, %v", len(ctx.Execs), err)
	}
}
