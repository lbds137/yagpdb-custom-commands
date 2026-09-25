package runtime

import (
	template "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagtemplate"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/schema"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// q is a quoted template string of n characters.
func q(n int) string { return `"` + strings.Repeat("a", n) + `"` }

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

// msgCtx is a strict context whose channel has message 1, to react to
func msgCtx() *ExecutionContext {
	ctx := newCtx(true, true)
	ctx.Messages = []types.CtxMessage{{ID: 1, ChannelID: ctx.ChannelID}}
	return ctx
}

func TestReactionsCountPerEmoji(t *testing.T) {
	emoji := `"a" "b" "c" "d" "e" "f" "g" "h" "i" "j" "k" "l" "m" "n" "o" "p" "q" "r" "s" "t"`
	ctx := msgCtx()
	if _, err := run(t, ctx, `{{addReactions `+emoji+`}}`); err != nil {
		t.Fatalf("20 emoji are allowed: %v", err)
	}
	ctx = msgCtx()
	if _, err := run(t, ctx, `{{addReactions `+emoji+` "u"}}`); err == nil || !strings.Contains(err.Error(), ErrTooManyCalls.Error()) {
		t.Errorf("21 emoji in one call should fail, got %v", err)
	}
	ctx = msgCtx()
	if _, err := run(t, ctx, `{{addMessageReactions nil 1 (cslice `+emoji+` "u")}}`); err == nil || !strings.Contains(err.Error(), ErrTooManyCalls.Error()) {
		t.Errorf("a slice of 21 emoji should fail, got %v", err)
	}
}

func TestDeleteReactionsCounters(t *testing.T) {
	ctx := msgCtx()
	if _, err := run(t, ctx, `{{range seq 0 11}}{{deleteAllMessageReactions nil 1}}{{end}}`); err != nil {
		t.Errorf("without emoji each call is one API call (limit 100): %v", err)
	}
	ctx = msgCtx()
	if _, err := run(t, ctx, `{{range seq 0 101}}{{deleteAllMessageReactions nil 1}}{{end}}`); err == nil || !strings.Contains(err.Error(), ErrTooManyAPICalls.Error()) {
		t.Errorf("the 101st is over the API call limit: %v", err)
	}
	ctx = msgCtx()
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
	_, err := run(t, msgCtx(), `{{deleteAllMessageReactions nil 1 (cslice "a" "b" "c" "d" "e" "f" "g" "h" "i" "j" "k")}}`)
	if err == nil || !strings.Contains(err.Error(), "del_reaction_message") {
		t.Errorf("11 emoji in a slice should hit the limit, got %v", err)
	}
	if _, err := run(t, msgCtx(), `{{addReactions nil nil "a"}}`); err != nil {
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

func TestEmbedLimitsStrictFailsTheSend(t *testing.T) {
	cases := map[string]struct{ src, want string }{
		"title":       {`{{sendMessage nil (cembed "title" ` + q(257) + `)}}`, "embed 1 title is 257 characters (max 256)"},
		"description": {`{{sendMessage nil (cembed "description" ` + q(4097) + `)}}`, "description is 4097 characters (max 4096)"},
		"field value": {`{{sendMessage nil (cembed "fields" (cslice (sdict "name" "n" "value" ` + q(1025) + `)))}}`, "field 1 value is 1025 characters (max 1024)"},
		"empty value": {`{{sendMessage nil (cembed "fields" (cslice (sdict "name" "n" "value" "")))}}`, "field 1 value is empty"},
		"footer":      {`{{sendMessage nil (cembed "footer" (sdict "text" ` + q(2049) + `))}}`, "footer text is 2049 characters (max 2048)"},
		"author":      {`{{sendMessage nil (cembed "author" (sdict "name" ` + q(257) + `))}}`, "author name is 257 characters (max 256)"},
		"total": {`{{sendMessage nil (complexMessage "embed" (cslice (cembed "description" ` + q(4000) + `) (cembed "description" ` + q(2001) + `)))}}`,
			"the embeds total 6001 characters (max 6000)"},
		"content":     {`{{sendMessage nil ` + q(2001) + `}}`, "content is 2001 characters (max 2000)"},
		"blank name":  {`{{sendMessage nil (cembed "fields" (cslice (sdict "name" " " "value" "v")))}}`, "field 1 name is empty"},
		"empty":       {`{{sendMessage nil ""}}`, "the message is empty"},
		"empty embed": {`{{sendMessage nil (cembed)}}`, "the message is empty"},
		"retid":       {`{{sendMessageRetID nil (cembed "title" ` + q(257) + `)}}`, "title is 257 characters"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := newCtx(true, true)
			_, err := run(t, ctx, c.src)
			if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "HTTP 400") {
				t.Fatalf("want %q, got %v", c.want, err)
			}
			// The rejected message isn't recorded; the run's show_errors message is
			if len(ctx.SentMessages) != 1 || !strings.HasPrefix(ctx.SentMessages[0].Content, "\nAn error caused") {
				t.Errorf("a rejected message must not be recorded as sent: %+v", ctx.SentMessages)
			}
		})
	}
}

func TestEmbedFieldCountLimit(t *testing.T) {
	ctx := newCtx(true, true)
	src := `{{$f := cslice}}{{range seq 0 26}}{{$f = $f.Append (sdict "name" "n" "value" "v")}}{{end}}` +
		`{{sendMessage nil (cembed "fields" $f)}}`
	if _, err := run(t, ctx, src); err == nil || !strings.Contains(err.Error(), "has 26 fields (max 25)") {
		t.Fatalf("got %v", err)
	}
}

func TestEmbedLimitsWarnAndSendOutsideStrict(t *testing.T) {
	ctx := newCtx(false, true)
	out, err := run(t, ctx, `{{sendMessage nil (cembed "title" `+q(257)+`)}}{{sendMessage nil (cembed "title" `+q(257)+`)}}ok`)
	if err != nil || out != "ok" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if len(ctx.SentMessages) != 2 {
		t.Errorf("permissive mode still sends, got %d", len(ctx.SentMessages))
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 || !strings.Contains(w[0], "title is 257") {
		t.Errorf("want one warning for the repeated breach, got %q", w)
	}
}

func TestEmbedWithinLimitsAndUnsentEmbedsPass(t *testing.T) {
	ctx := newCtx(true, true)
	// cembed never fails on Discord's limits: only a send does, so a template may trim first
	src := `{{$e := cembed "title" ` + q(300) + `}}` +
		`{{sendMessage nil (cembed "title" ` + q(256) + ` "description" ` + q(4096) +
		` "fields" (cslice (sdict "name" "n" "value" ` + q(1024) + `)))}}ok`
	if out, err := run(t, ctx, src); err != nil || out != "ok" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if w := kinds(ctx, KindLimit); len(w) != 0 {
		t.Errorf("no warnings expected, got %q", w)
	}
}

func TestRejectedDMIsDroppedSilently(t *testing.T) {
	ctx := newCtx(true, true)
	// sendDM discards Discord's error: the DM isn't delivered and the command carries on
	out, err := run(t, ctx, `{{sendDM `+q(2001)+`}}ok`)
	if err != nil || out != "ok" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if len(ctx.SentMessages) != 0 {
		t.Errorf("the rejected DM must not be recorded")
	}
	if w := kinds(ctx, KindLimit); len(w) != 1 || !strings.Contains(w[0], "silently") {
		t.Errorf("want one silent-skip warning, got %q", w)
	}
}

func TestRejectedSendIsCatchable(t *testing.T) {
	ctx := newCtx(true, true)
	out, err := run(t, ctx, `{{try}}{{sendMessage nil ""}}{{catch}}caught{{end}}`)
	if err != nil || out != "caught" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestRejectedSendRecordsNoFile(t *testing.T) {
	ctx := newCtx(true, true)
	_, err := run(t, ctx, `{{sendMessage nil (complexMessage "content" `+q(2001)+` "file" "data")}}`)
	if err == nil || len(ctx.FileUploads) != 0 {
		t.Fatalf("want an error and no upload: err=%v uploads=%d", err, len(ctx.FileUploads))
	}
	// a file alone is not an empty message
	ctx = newCtx(true, true)
	if _, err := run(t, ctx, `{{sendMessage nil (complexMessage "file" "data")}}`); err != nil {
		t.Fatalf("file-only message: %v", err)
	}
}

func TestZeroWidthSpaceIsABlankFieldName(t *testing.T) {
	ctx := newCtx(true, true)
	if _, err := run(t, ctx, "{{sendMessage nil (cembed \"fields\" (cslice (sdict \"name\" \"\u200b\" \"value\" \"v\")))}}"); err != nil {
		t.Fatalf("a zero-width space name is accepted by Discord: %v", err)
	}
}

func TestAPILimitSkipsBeforeTheSendCheck(t *testing.T) {
	ctx := newCtx(true, true)
	// past the API call limit YAGPDB returns before sending, so Discord never sees the message
	src := `{{range seq 0 100}}{{sendMessage nil "x"}}{{end}}{{sendMessage nil ""}}ok`
	if out, err := run(t, ctx, src); err != nil || out != "ok" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestNonEmptyWithoutContent(t *testing.T) {
	for name, src := range map[string]string{
		// YAGPDB adds its server-info button to every DM
		"empty DM": `{{sendDM ""}}`,
		"buttons":  `{{sendMessage nil (complexMessage "buttons" (cslice (sdict "label" "a" "custom_id" "b")))}}`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx := newCtx(true, true)
			if _, err := run(t, ctx, src); err != nil || len(ctx.SentMessages) != 1 {
				t.Fatalf("err=%v sent=%d", err, len(ctx.SentMessages))
			}
			if w := kinds(ctx, KindLimit); len(w) != 0 {
				t.Errorf("no warnings expected, got %q", w)
			}
		})
	}
}

func TestEmptyMessageNamesDiscordsCode(t *testing.T) {
	_, err := run(t, newCtx(true, true), `{{sendMessage nil " "}}`)
	if err == nil || !strings.Contains(err.Error(), "50006 Cannot send an empty message") {
		t.Fatalf("got %v", err)
	}
}

func TestEditMessage(t *testing.T) {
	send := `{{$id := sendMessageRetID nil (complexMessage "content" "a" "embed" (cembed "title" "T"))}}`
	cases := []struct {
		name, src string
		strict    bool
		errWant   string // error with -strict ("" = none)
		out       string
	}{
		{"content only keeps the embed", send + `{{editMessage nil $id "b"}}{{$m := getMessage nil $id}}{{$m.Content}} {{len $m.Embeds}}`, true, "", "b 1"},
		{"an edit can clear the content", send + `{{editMessage nil $id (complexMessageEdit "content" "" "embed" (cembed "title" "U"))}}{{(getMessage nil $id).Content}}|`, true, "", "|"},
		// YAGPDB checks the edit alone, so this fails even though the message has an embed
		{"YAGPDB refuses blank content and no embed", send + `{{editMessage nil $id (complexMessageEdit "content" " ")}}`, false, "both content and embed cannot be null", ""},
		{"a number is printed as YAGPDB prints it", send + `{{editMessage nil $id 1.5}}{{(getMessage nil $id).Content}}`, true, "", "1.5"},
		{"an embed alone keeps the content", send + `{{editMessage nil $id (cembed "title" "U")}}{{(getMessage nil $id).Content}}`, true, "", "a"},
		{"components v2 skips YAGPDB's check", send + `{{editMessage nil $id (complexMessageEdit "content" "" "is_components_v2" true)}}`, false, "", ""},
		{"unknown message", `{{editMessage nil 42 "x"}}`, true, "10008 Unknown Message", ""},
		{"a message in another channel", send + `{{editMessage 99 $id "x"}}`, true, "10008 Unknown Message", ""},
		{"someone else's message", `{{editMessage nil 7 "x"}}`, true, "50005", ""},
		{"too long", send + `{{editMessage nil $id (printf "%2001s" "x")}}`, true, "HTTP 400", ""},
		{"unknown key", `{{complexMessageEdit "file" "x"}}`, false, `invalid key "file" passed to message edit builder`, ""},
	}
	for _, c := range cases {
		ctx := newCtx(c.strict, true)
		ctx.Messages = []types.CtxMessage{{ID: 7, ChannelID: ctx.ChannelID, Author: types.DiscordUser{ID: 5}}}
		out, err := run(t, ctx, c.src)
		if c.errWant != "" {
			if err == nil || !strings.Contains(err.Error(), c.errWant) {
				t.Errorf("%s: want an error containing %q, got %v", c.name, c.errWant, err)
			}
			continue
		}
		if err != nil || strings.TrimSpace(out) != c.out {
			t.Errorf("%s: got %q, %v; want %q", c.name, out, err, c.out)
		}
		if len(ctx.EditedMessages) != 1 {
			t.Errorf("%s: want one recorded edit, got %d", c.name, len(ctx.EditedMessages))
		} else if m := ctx.EditedMessages[0]; m.ID == 0 || !strings.HasPrefix(c.out, m.Content) {
			t.Errorf("%s: recorded %+v", c.name, m)
		}
	}

	// Without -strict a refused edit warns and changes nothing
	ctx := newCtx(false, true)
	if _, err := run(t, ctx, `{{editMessage nil 42 "x"}}`); err != nil || len(ctx.EditedMessages) != 0 || len(ctx.Diagnostics) == 0 {
		t.Errorf("want a warning and no edit: %v %d %q", err, len(ctx.EditedMessages), ctx.Diagnostics)
	}
}

func TestExecCCEditsAreRecorded(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{editMessage nil .ExecData.ID "edited"}}`)
	ctx := newCtx(true, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	if _, err := run(t, ctx, `{{$id := sendMessageRetID nil "a"}}{{execCC 7 nil 0 (sdict "ID" $id)}}`); err != nil {
		t.Fatal(err)
	}
	if len(ctx.EditedMessages) != 1 || ctx.EditedMessages[0].Content != "edited" {
		t.Errorf("edits %+v; diagnostics %q", ctx.EditedMessages, ctx.Diagnostics)
	}
}

func TestExecDataIsSetOnlyByExecCC(t *testing.T) {
	// Not run by execCC there is no .ExecData key, so a field of it is <no value>, not an error
	out, err := run(t, newCtx(false, true), `{{.ExecData.Title}} {{or .ExecData.Title "d"}} {{.StackDepth}}`)
	if err != nil || out != "<no value> d <no value>" {
		t.Errorf("got %q, %v", out, err)
	}

	// A run given data at the top level (a test's exec_data, like a delayed run) has it
	ctx := newCtx(false, true)
	ctx.ExecData = types.SDict{"Title": "t"}
	out, err = run(t, ctx, `{{.ExecData.Title}} {{.StackDepth}}`)
	if err != nil || out != "t <no value>" {
		t.Errorf("got %q, %v", out, err)
	}

	// An immediate execCC sets the key even when its data is nil, and a field of nil is an error
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{sendMessage nil (print "depth " .StackDepth)}}{{.ExecData.Title}}`)
	ctx = newCtx(false, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	if len(ctx.SentMessages) != 2 || ctx.SentMessages[0].Content != "depth 1" {
		t.Errorf("sent %+v", ctx.SentMessages)
	}
	want := "nil pointer evaluating interface {}.Title"
	if len(ctx.Diagnostics) != 1 || !strings.Contains(ctx.Diagnostics[0].Message, want) {
		t.Errorf("want the child to fail with %q, got %q", want, ctx.Diagnostics)
	}
}

// Query errors read as in production: the functions that select through sqlboiler wrap
// lib/pq's error twice, dbCount and dbRank don't
func TestDBQueryErrorsAsProductionWrapsThem(t *testing.T) {
	boiler := "models: failed to assign all query results to TemplatesUserDatabase slice: " +
		"bind failed to execute query: pq: "
	cases := []struct{ src, want string }{
		{`{{dbGetPattern 1 "%\\" 5 0}}`, "error calling dbGetPattern: " + boiler + "LIKE pattern must not end"},
		{`{{dbGetPatternReverse 1 "a" 5 -1}}`, "error calling dbGetPatternReverse: " + boiler + "OFFSET must not be negative"},
		{`{{dbTopEntries "%\\" 5 0}}`, "error calling dbTopEntries: " + boiler + "LIKE pattern must not end"},
		{`{{dbBottomEntries "%\\" 5 0}}`, "error calling dbBottomEntries: " + boiler + "LIKE pattern must not end"},
		{`{{dbDelMultiple (sdict) -1 0}}`, "error calling dbDelMultiple: " + boiler + "LIMIT must not be negative"},
		{`{{dbDelMultiple (sdict "pattern" "%\\") 5 0}}`, "error calling dbDelMultiple: " + boiler + "LIKE pattern must not end"},
		{`{{dbCount "%\\"}}`, "error calling dbCount: pq: LIKE pattern must not end"},
		{`{{dbRank (sdict "pattern" "%\\") 1 "a"}}`, "error calling dbRank: pq: LIKE pattern must not end"},
	}
	for _, c := range cases {
		ctx := newCtx(false, true)
		_, err := run(t, ctx, `{{dbSet 1 "a" 1}}`+c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %q", c.src, err, c.want)
		}
	}
}

// A pattern that isn't an exact match and ends with the escape may fail while Postgres
// plans the query, so it warns even when no key reaches the escape
func TestPatternPlanRiskWarns(t *testing.T) {
	cases := []struct {
		src  string
		warn bool
	}{
		{`{{dbCount "%\\"}}`, true},     // no rows: the mock can't fail, production may
		{`{{dbCount "a_\\"}}`, true},    // a _ is a wildcard too
		{`{{dbCount "a\\"}}`, false},    // an exact match: the planner runs no LIKE
		{`{{dbCount "a\\%\\"}}`, false}, // the % is escaped, so still exact
		{`{{dbCount "a%\\\\"}}`, false}, // the last backslash is escaped
	}
	for _, c := range cases {
		ctx := newCtx(false, true)
		if _, err := run(t, ctx, c.src); err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		warned := len(ctx.Diagnostics) == 1 && ctx.Diagnostics[0].Kind == KindDB
		if warned != c.warn || (!c.warn && len(ctx.Diagnostics) != 0) {
			t.Errorf("%s: diagnostics %q, want a [db] warning: %v", c.src, ctx.Diagnostics, c.warn)
		}
	}
}

func TestDBSetOverTheLimitFails(t *testing.T) {
	_, err := run(t, newCtx(false, true), `{{dbSet 0 "big" (printf "%100000s" "x")}}`)
	if err == nil || !strings.Contains(err.Error(), "short write") {
		t.Errorf("want YAGPDB's short write, got %v", err)
	}
	out, err := run(t, newCtx(false, true), `{{dbSet 0 "k" "x"}}{{(dbGet 0 "k").ValueSize}}`)
	if err != nil || out != "2" {
		t.Errorf("got %q, %v", out, err)
	}
}

func TestDBQueries(t *testing.T) {
	// user 1: a=5, b=7, c=5 (c after a, so c has the higher id); user 2: a=9
	setup := `{{dbSet 1 "a" 5}}{{dbSet 1 "b" 7}}{{dbSet 1 "c" 5}}{{dbSet 2 "a" 9}}`
	cases := []struct{ name, src, out, err string }{
		{"count all", `{{dbCount}}`, "4", ""},
		{"count a user", `{{dbCount 1}}`, "3", ""},
		{"count a pattern", `{{dbCount "a"}}`, "2", ""},
		{"count a query", `{{dbCount (sdict "userID" 1 "pattern" "a")}}`, "1", ""},
		{"count a bad query", `{{dbCount (sdict "user" 1)}}`, "", "Invalid Key: user passed to query constructor"},
		{"a float isn't a user", `{{dbCount (sdict "userID" 1.0)}}`, "", "Invalid UserID datatype"},
		{"rank", `{{dbRank (sdict) 2 "a"}} {{dbRank (sdict) 1 "b"}} {{dbRank (sdict) 1 "c"}} {{dbRank (sdict) 1 "a"}}`, "1 2 3 4", ""},
		{"rank ascending", `{{dbRank (sdict "reverse" true) 1 "a"}}`, "1", ""},
		{"rank within a user", `{{dbRank (sdict "userID" 1) 1 "a"}} {{dbRank (sdict "userID" 2) 1 "a"}}`, "3 0", ""},
		{"rank of a missing key", `{{dbRank (sdict) 1 "zzz"}}`, "0", ""},
		{"delete the lowest two", `{{dbDelMultiple (sdict "reverse" true) 2 0}} {{dbCount}} {{(dbGet 1 "b").Value}}`, "2 2 7", ""},
		{"negative skip", `{{dbDelMultiple (sdict) 1 -1}}`, "", "OFFSET must not be negative"},
		{"a trailing escape the match never reaches", `{{dbCount "a\\"}}`, "0", ""},
		{"a trailing escape the match reaches", `{{dbCount "%\\"}}`, "", "LIKE pattern must not end with escape character"},
		{"a failed delete deletes nothing", `{{try}}{{dbDelMultiple (sdict "pattern" "%\\") 5 0}}{{catch}}{{end}}{{dbCount}}`, "4", ""},
		{"negative skip in a pattern", `{{dbGetPattern 1 "%" 1 -1}}`, "", "OFFSET must not be negative"},
		{"negative skip in top entries", `{{dbTopEntries "%" 1 -1}}`, "", "OFFSET must not be negative"},
		{"a float user ID fails as in YAGPDB", `{{dbRank (sdict) (toFloat 1) "a"}}`, "", "wrong type for value; expected int64; got float64"},
		{"a missing rank is an int 0", `{{printf "%T %T" (dbRank (sdict) 1 "zzz") (dbRank (sdict) 1 "a")}}`, "int int64", ""},
		{"count by .User.ID", `{{dbSet .User.ID "x" 1}}{{dbCount .User.ID}}`, "1", ""},
		{"rank within a pattern", `{{dbRank (sdict "pattern" "a") 1 "a"}}`, "2", ""},
		{"amount 0 means 100", `{{dbDelMultiple (sdict) 0 0}}`, "4", ""},
		{"negative amount", `{{dbDelMultiple (sdict) -1 0}}`, "", "LIMIT must not be negative"},
		{"OFFSET is checked first", `{{dbDelMultiple (sdict) -1 -1}}`, "", "OFFSET must not be negative"},
		{"delete by pattern", `{{dbDelMultiple (sdict "pattern" "a") 5 0}} {{dbCount}}`, "2 2", ""},
		{"a cut keeps whole characters", `{{dbSet 0 (print (printf "%255s" "k") "é") 1}}{{len (index (dbGetPattern 0 "%" 1 0) 0).Key}}`, "255", ""},
		{"long keys are cut to 256 bytes", `{{dbSet 0 (printf "%300s" "k") 1}}{{len (index (dbGetPattern 0 "%" 1 0) 0).Key}}`, "256", ""},
	}
	for _, c := range cases {
		out, err := run(t, newCtx(false, true), setup+c.src)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%s: want an error containing %q, got %v", c.name, c.err, err)
			}
			continue
		}
		if err != nil || out != c.out {
			t.Errorf("%s: got %q, %v; want %q", c.name, out, err, c.out)
		}
	}
}

func TestOutputLimitFollowsYAGPDBsWriter(t *testing.T) {
	cases := []struct {
		name, src string
		out       string // "" = don't check
		breach    bool
	}{
		{"leading whitespace doesn't count", `{{printf "%30000s" "x"}}`, "x", false},
		{"a whitespace-only overflow is dropped", `{{printf "%-25010s" (printf "%025000d" 0)}}`, "", false},
		{"real overflow", `{{printf "%025001d" 0}}`, "", true},
	}
	for _, c := range cases {
		for _, strict := range []bool{true, false} {
			ctx := newCtx(strict, true)
			out, err := run(t, ctx, c.src)
			breached := err != nil
			for _, w := range kinds(ctx, KindLimit) {
				breached = breached || strings.Contains(w, "grew too big")
			}
			if breached != c.breach {
				t.Errorf("%s (strict %v): breach %v, want %v (err %v, %q)", c.name, strict, breached, c.breach, err, ctx.Diagnostics)
			}
			if strict && c.breach && (err == nil || !strings.Contains(err.Error(), "response grew too big (>25k)")) {
				t.Errorf("%s: want YAGPDB's error, got %v", c.name, err)
			}
			if !strict && err != nil {
				t.Errorf("%s: without -strict a breach is a warning, got %v", c.name, err)
			}
			if c.out != "" && strings.TrimSpace(out) != c.out {
				t.Errorf("%s (strict %v): output %q", c.name, strict, out)
			}
		}
	}
}

func TestOutputBreachBeforeAnErrorIsReported(t *testing.T) {
	ctx := newCtx(false, true)
	_, err := run(t, ctx, `{{printf "%025001d" 0}}{{index (cslice) 5}}`)
	if err == nil || !strings.Contains(err.Error(), "index out of range") {
		t.Fatalf("want the later error, got %v", err)
	}
	// The 25k warning, then the 2k one for the partial response
	if w := kinds(ctx, KindLimit); len(w) != 2 || !strings.Contains(w[0], "grew too big (>25k)") || !strings.Contains(w[1], "over 2000") {
		t.Errorf("want the 25k warning too, got %q", ctx.Diagnostics)
	}
}

func TestFailedRunKeepsItsOutput(t *testing.T) {
	out, err := run(t, newCtx(true, true), "  hello \n{{index (cslice) 5}}")
	if err == nil || out != "hello" {
		t.Errorf("YAGPDB sends the trimmed output printed before the error: %q, %v", out, err)
	}
	out, _ = run(t, newCtx(true, true), `{{printf "%02001d" 0}}{{index (cslice) 5}}`)
	if !strings.Contains(out, "response was longer than 2k") {
		t.Errorf("an over-2k partial response becomes the notice: %.40q", out)
	}
}
