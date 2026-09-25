package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/schema"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
)

func newCtx(strict, premium bool) *ExecutionContext {
	ctx := NewExecutionContext(1, state.NewMockDB(1))
	ctx.Strict = strict
	if !premium {
		ctx.SetNonPremium()
	}
	return ctx
}

func run(t *testing.T, ctx *ExecutionContext, src string) (string, error) {
	t.Helper()
	return NewEngine(ctx).Execute(src)
}

func kinds(ctx *ExecutionContext, kind string) []string {
	var out []string
	for _, d := range ctx.Diagnostics {
		if d.Kind == kind {
			out = append(out, d.Message)
		}
	}
	return out
}

// eleven dbGet calls: one over the free limit of 10
const elevenDBGets = `{{dbGet 0 "a"}}{{dbGet 0 "a"}}{{dbGet 0 "a"}}{{dbGet 0 "a"}}{{dbGet 0 "a"}}` +
	`{{dbGet 0 "a"}}{{dbGet 0 "a"}}{{dbGet 0 "a"}}{{dbGet 0 "a"}}{{dbGet 0 "a"}}{{dbGet 0 "a"}}done`

func TestDBLimitStrictFree(t *testing.T) {
	ctx := newCtx(true, false)
	_, err := run(t, ctx, elevenDBGets)
	if !errors.Is(err, ErrTooManyCalls) {
		t.Fatalf("want ErrTooManyCalls, got %v", err)
	}
	if !strings.Contains(err.Error(), "dbGet: over the limit of 10 db_interactions") {
		t.Errorf("error should name the function and limit: %v", err)
	}
}

func TestDBLimitStrictPremiumAllows(t *testing.T) {
	ctx := newCtx(true, true)
	out, err := run(t, ctx, elevenDBGets)
	if err != nil || !strings.HasSuffix(out, "done") {
		t.Fatalf("premium allows 50: out=%q err=%v", out, err)
	}
}

func TestDBLimitPermissiveWarnsOnce(t *testing.T) {
	ctx := newCtx(false, false)
	out, err := run(t, ctx, elevenDBGets+`{{dbGet 0 "a"}}`)
	if err != nil || !strings.Contains(out, "done") {
		t.Fatalf("permissive mode should run on: out=%q err=%v", out, err)
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 {
		t.Fatalf("want exactly one limit warning, got %q", w)
	}
}

func TestSilentLimitSkipsCallInStrictMode(t *testing.T) {
	ctx := newCtx(true, true)
	// send_dm allows one DM per run; YAGPDB drops the second without an error
	out, err := run(t, ctx, `{{sendDM "one"}}{{sendDM "two"}}ok`)
	if err != nil || out != "ok" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if len(ctx.SentMessages) != 1 {
		t.Errorf("second DM should be dropped, sent %d", len(ctx.SentMessages))
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 || !strings.Contains(w[0], "silently") {
		t.Errorf("want a warning about the dropped call, got %q", w)
	}
}

func TestSilentLimitRunsCallInPermissiveMode(t *testing.T) {
	ctx := newCtx(false, true)
	if _, err := run(t, ctx, `{{sendDM "one"}}{{sendDM "two"}}`); err != nil {
		t.Fatal(err)
	}
	if len(ctx.SentMessages) != 2 {
		t.Errorf("permissive mode should still send, sent %d", len(ctx.SentMessages))
	}
	if len(kinds(ctx, KindLimit)) != 1 {
		t.Errorf("want one warning, got %q", ctx.Diagnostics)
	}
}

func TestVariadicFunctionStillWorksWhenWrapped(t *testing.T) {
	ctx := newCtx(true, true)
	if _, err := run(t, ctx, `{{sendMessage nil "hi"}}{{sendMessage 42 "there"}}`); err != nil {
		t.Fatal(err)
	}
	if len(ctx.SentMessages) != 2 || ctx.SentMessages[1].ChannelID != 42 {
		t.Errorf("unexpected messages %+v", ctx.SentMessages)
	}
}

func TestSourceLengthLimit(t *testing.T) {
	long := "{{/*" + strings.Repeat("x", 10001) + "*/}}ok"

	ctx := newCtx(true, false)
	if _, err := run(t, ctx, long); err == nil || !strings.Contains(err.Error(), "refuses to save") {
		t.Errorf("free: want length error, got %v", err)
	}
	ctx = newCtx(true, true)
	if _, err := run(t, ctx, long); err != nil {
		t.Errorf("premium allows 20000: %v", err)
	}
}

func TestResponseOver2000Replaced(t *testing.T) {
	src := `{{range seq 0 2001}}x{{end}}`

	ctx := newCtx(true, true)
	ctx.Cmd = "big"
	out, err := run(t, ctx, src)
	if err != nil || !strings.HasPrefix(out, "Template output for big was longer than 2k") {
		t.Errorf("strict: out=%q err=%v", out, err)
	}

	ctx = newCtx(false, true)
	out, _ = run(t, ctx, src)
	if len(out) != 2001 || len(kinds(ctx, KindLimit)) != 1 {
		t.Errorf("permissive: len=%d warnings=%q", len(out), ctx.Diagnostics)
	}
}

func TestOutputOver25kFailsInStrictMode(t *testing.T) {
	ctx := newCtx(true, true)
	_, err := run(t, ctx, `{{range seq 0 25001}}x{{end}}`)
	if err == nil || !strings.Contains(err.Error(), "response grew too big") {
		t.Errorf("want output error, got %v", err)
	}
}

func TestLoopDBWarningLineNumbers(t *testing.T) {
	src := "{{try}}\n{{$x := 1}}\n{{catch}}\n{{/* two\nlines */}}\n{{end}}\n" + // lines 1-6
		"{{$a := dbGet 0 \"k\"}}\n" + // line 7: outside the loop
		"{{range $k := cslice \"a\" \"b\"}}\n" + // line 8
		"  {{$e := dbGet 0 $k}}\n" + // line 9
		"  {{if (dbGet 0 \"x\")}}{{end}}\n" + // line 10
		"{{end}}"
	ctx := newCtx(false, true)
	ctx.SourceName = "cmd.gohtml"
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	w := kinds(ctx, KindLoopDB)
	if len(w) != 2 {
		t.Fatalf("want 2 loop warnings, got %q", w)
	}
	if !strings.HasPrefix(w[0], "cmd.gohtml:9: dbGet") || !strings.HasPrefix(w[1], "cmd.gohtml:10: dbGet") {
		t.Errorf("wrong locations: %q", w)
	}
}

func TestTryCatchFillerPrintsNothing(t *testing.T) {
	out, err := run(t, newCtx(false, true), "a{{try}}b{{catch}}\n\nc{{end}}d")
	if err != nil || out != "abd" {
		t.Errorf("out=%q err=%v", out, err)
	}
}

func TestSchemaWarning(t *testing.T) {
	ctx := newCtx(false, true)
	zero := int64(0)
	ctx.Schema = &schema.Schema{Entries: []schema.Rule{{UserID: &zero, Key: "Global", Type: schema.TypeDict}}}
	if _, err := run(t, ctx, `{{dbSet 0 "Global" "oops"}}{{dbSet 0 "Global" (sdict "a" 1)}}{{dbSet 5 "Global" "fine"}}`); err != nil {
		t.Fatal(err)
	}
	w := kinds(ctx, KindSchema)
	if len(w) != 1 || !strings.Contains(w[0], `dbSet: user 0, key "Global": schema expects dict, got string`) {
		t.Errorf("got %q", w)
	}
}

func TestExecCCChildFailureIsReported(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{sendDM "a"}}{{parseArgs 1 "needs an arg"}}`)
	ctx := newCtx(false, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 (sdict)}}ok`); err != nil {
		t.Fatal(err)
	}
	w := kinds(ctx, KindExecCC)
	if len(w) != 1 || !strings.Contains(w[0], "execCC 7 (child.gohtml) failed") {
		t.Errorf("got %q", ctx.Diagnostics)
	}
}
