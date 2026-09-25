package runtime

import (
	template "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagtemplate"
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
	if err == nil || !strings.Contains(err.Error(), ErrTooManyCalls.Error()) {
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
	if err != nil || !strings.HasPrefix(out, "Custom command (#0) response was longer than 2k") {
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
	_, err := run(t, ctx, `{{range seq 0 5001}}xxxxx{{end}}`)
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
	writeFile(t, dir+"/child.gohtml", `{{sendDM "a"}}{{index (cslice) 3}}`)
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

func TestReactionsCountPerEmoji(t *testing.T) {
	emoji := `"a" "b" "c" "d" "e" "f" "g" "h" "i" "j" "k" "l" "m" "n" "o" "p" "q" "r" "s" "t"`
	ctx := newCtx(true, true)
	if _, err := run(t, ctx, `{{addReactions `+emoji+`}}`); err != nil {
		t.Fatalf("20 emoji are allowed: %v", err)
	}
	ctx = newCtx(true, true)
	if _, err := run(t, ctx, `{{addReactions `+emoji+` "u"}}`); err == nil || !strings.Contains(err.Error(), ErrTooManyCalls.Error()) {
		t.Errorf("21 emoji in one call should fail, got %v", err)
	}
	ctx = newCtx(true, true)
	if _, err := run(t, ctx, `{{addMessageReactions nil 1 (cslice `+emoji+` "u")}}`); err == nil || !strings.Contains(err.Error(), ErrTooManyCalls.Error()) {
		t.Errorf("a slice of 21 emoji should fail, got %v", err)
	}
}

func TestDeleteReactionsCounters(t *testing.T) {
	ctx := newCtx(true, true)
	if _, err := run(t, ctx, `{{range seq 0 11}}{{deleteAllMessageReactions nil 1}}{{end}}`); err != nil {
		t.Errorf("without emoji each call is one API call (limit 100): %v", err)
	}
	ctx = newCtx(true, true)
	_, err := run(t, ctx, `{{deleteAllMessageReactions nil 1 "a" "b" "c" "d" "e" "f" "g" "h" "i" "j" "k"}}`)
	if err == nil || !strings.Contains(err.Error(), "del_reaction_message") {
		t.Errorf("11 emoji should pass the del_reaction_message limit of 10, got %v", err)
	}
}

func TestSetRolesOncePerUser(t *testing.T) {
	ctx := newCtx(true, true)
	if _, err := run(t, ctx, `{{setRoles 1 (cslice)}}{{setRoles 2 (cslice)}}`); err != nil {
		t.Fatalf("different users are fine: %v", err)
	}
	if _, err := run(t, newCtx(true, true), `{{setRoles 1 (cslice)}}{{setRoles 1 (cslice)}}`); err == nil ||
		!strings.Contains(err.Error(), "max 1 / user") {
		t.Errorf("same user twice should fail, got %v", err)
	}
}

func TestExecLimitSharedByExecAndExecAdmin(t *testing.T) {
	_, err := run(t, newCtx(true, true), `{{exec "a"}}{{execAdmin "a"}}{{exec "a"}}{{exec "a"}}{{exec "a"}}{{execAdmin "a"}}`)
	if err == nil || !strings.Contains(err.Error(), "Max number of commands executed") {
		t.Errorf("the sixth exec should fail, got %v", err)
	}
}

func TestEachBreachWarnsOnce(t *testing.T) {
	ctx := newCtx(false, true)
	if _, err := run(t, ctx, `{{range seq 0 105}}{{sendMessage nil "x"}}{{end}}{{getMember 1}}{{getMember 1}}after`); err != nil {
		t.Fatal(err)
	}
	w := kinds(ctx, KindLimit)
	if len(w) != 2 || !strings.Contains(w[0], "sendMessage") || !strings.Contains(w[1], "getMember") ||
		!strings.Contains(w[1], "stops the command") {
		t.Errorf("want one sendMessage and one getMember warning, got %q", w)
	}
}

func TestLoopDBWarningForms(t *testing.T) {
	src := `{{define "lookup"}}{{$e := dbGet 0 .}}{{end}}` +
		`{{define "loop"}}{{template "loop" .}}{{end}}` + "\n" + // recursive: must not hang
		"{{range $i := seq 0 3}}\n" + // line 2
		"{{$x := (dbGet 0 \"k\").Value}}\n" + // line 3: chain
		"{{template \"lookup\" \"k\"}}\n" + // line 4: template that calls dbGet
		"{{template \"loop\" 1}}\n" + // line 5: template without db calls
		"{{end}}"
	ctx := newCtx(false, true)
	engine := NewEngine(ctx)
	// Execute would recurse forever on "loop"; the check runs at parse time
	tmpl := template.Must(template.New("t").Funcs(engine.BuildFuncMap()).Parse(src))
	var got []string
	for _, f := range findLoopDBCalls(tmpl) {
		got = append(got, f.Message(""))
	}
	if len(got) != 2 || !strings.HasPrefix(got[0], "line 3: dbGet") ||
		!strings.HasPrefix(got[1], `line 4: template "lookup" (which calls dbGet)`) {
		t.Errorf("got %q", got)
	}
}

func TestEqMatchesYAGPDB(t *testing.T) {
	out, err := run(t, newCtx(false, true), `{{eq (toInt64 5) 5}} {{ne (toInt64 5) 5}} {{eq 3 1 2 3}} {{eq "a" "b"}} {{ne 1 2 1}}`)
	if err != nil || out != "true false true false false" {
		t.Errorf("out=%q err=%v", out, err)
	}
	for src, want := range map[string]string{
		`{{eq 1 1.0}}`:              "incompatible types for comparison",
		`{{eq nil 1}}`:              "invalid type for comparison",
		`{{eq (sdict) 1}}`:          "invalid type for comparison",
		`{{eq 1}}`:                  "missing argument for comparison",
		`{{eq (dbIncr 0 "n" 1) 1}}`: "incompatible types", // stored numbers are float64
		`{{dbSet 0 "n" 5}}{{eq (dbGet 0 "n").Value 5}}`: "incompatible types",
	} {
		if _, err := run(t, newCtx(false, true), src); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %q, got %v", src, want, err)
		}
	}
}

func TestReturnAndExecTemplate(t *testing.T) {
	src := `{{define "double"}}x{{return (mult . 2)}}never{{end}}` +
		`a{{$v := execTemplate "double" 21}}b{{$v}}{{return}}after`
	out, err := run(t, newCtx(false, true), src)
	if err != nil || out != "axb42" {
		t.Errorf("out=%q err=%v", out, err)
	}
	if _, err := run(t, newCtx(false, true), `{{execTemplate "missing"}}`); err == nil ||
		!strings.Contains(err.Error(), `template "missing" not defined`) {
		t.Errorf("got %v", err)
	}
}

func TestIndexAndLenMatchYAGPDB(t *testing.T) {
	for src, want := range map[string]string{
		`{{index (split "a" "/") 1}}`: "index out of range: 1",
		`{{index nil 0}}`:             "index of untyped nil",
		`{{len 5}}`:                   "len of type int",
	} {
		if _, err := run(t, newCtx(false, true), src); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %q, got %v", src, want, err)
		}
	}
	out, err := run(t, newCtx(false, true), `{{len (split "a/b" "/")}} {{index (sdict "k" 1) "missing"}} {{dbSet 0 "d" (sdict "a" 1)}}{{len (dbGet 0 "d").Value}}`)
	if err != nil || out != "2 <no value> 1" {
		t.Errorf("out=%q err=%v", out, err)
	}
}

func TestSetRolesTargets(t *testing.T) {
	if _, err := run(t, newCtx(true, true), `{{setRoles "<@111>" (cslice)}}{{setRoles "<@!222>" (cslice)}}{{setRoles (userArg 333) (cslice)}}`); err != nil {
		t.Errorf("different users by mention and user object are fine: %v", err)
	}
	if _, err := run(t, newCtx(true, true), `{{setRoles "<@111>" (cslice)}}{{setRoles 111 (cslice)}}`); err == nil {
		t.Error("the same user by mention and by ID should hit the limit")
	}
}

func TestDeleteReactionsFlattensSlices(t *testing.T) {
	_, err := run(t, newCtx(true, true), `{{deleteAllMessageReactions nil 1 (cslice "a" "b" "c" "d" "e" "f" "g" "h" "i" "j" "k")}}`)
	if err == nil || !strings.Contains(err.Error(), "del_reaction_message") {
		t.Errorf("11 emoji in a slice should hit the limit, got %v", err)
	}
	if _, err := run(t, newCtx(true, true), `{{addReactions nil nil "a"}}`); err != nil {
		t.Errorf("nil emoji are skipped: %v", err)
	}
}

func TestDBIncrKeepsExpiryAndReadsStrings(t *testing.T) {
	ctx := newCtx(false, true)
	out, err := run(t, ctx, `{{dbSetExpire 0 "c" "40" 3600}}{{dbIncr 0 "c" 2}}`)
	if err != nil || out != "42" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if e := ctx.DB.Get(0, "c"); e == nil || e.ExpiresAt.IsZero() {
		t.Errorf("dbIncr should keep the expiry: %+v", e)
	}
}

func TestMultiLineTryTagKeepsLinesInsideTheBody(t *testing.T) {
	_, err := run(t, newCtx(false, true), "{{ try\n}}\n{{ nope }}\n{{ catch }}{{ end }}")
	if err == nil || !strings.Contains(err.Error(), "yagtest:3:") {
		t.Errorf("error should point at line 3, got %v", err)
	}
}

func TestExecTemplateInLoopIsFlagged(t *testing.T) {
	ctx := newCtx(false, true)
	src := `{{define "d"}}{{return dbCount}}{{end}}` + "\n{{range seq 0 2}}{{$n := execTemplate \"d\"}}{{end}}"
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	if w := kinds(ctx, KindLoopDB); len(w) != 1 || !strings.Contains(w[0], `line 2: template "d" (which calls dbCount)`) {
		t.Errorf("got %q", w)
	}
}

func TestMultiLineTryTagKeepsLines(t *testing.T) {
	src := "{{ try\n}}\n{{$x := 1}}\n{{ catch }}{{ end }}\n{{range seq 0 2}}{{dbGet 0 \"k\"}}{{end}}" // loop on line 5
	ctx := newCtx(false, true)
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	if w := kinds(ctx, KindLoopDB); len(w) != 1 || !strings.HasPrefix(w[0], "line 5:") {
		t.Errorf("got %q", w)
	}
}
