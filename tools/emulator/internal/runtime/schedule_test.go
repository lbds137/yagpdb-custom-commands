package runtime

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDelayedExecCCIsScheduledNotRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{sendMessage nil "ran"}}`) // would send if it ran now
	ctx := newCtx(false, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{5: "child.gohtml"}
	_, err := run(t, ctx, `{{execCC 5 77 90 (sdict "n" 1 "list" (cslice 1 "a") "d" (dict 1 2) "m" (sdict "x" 2.5))}}`+
		`{{execCC 5 nil "30" nil}}`)
	if err != nil {
		t.Fatal(err)
	}
	runs := ctx.ScheduledRuns()
	if len(runs) != 2 || runs[0].CCID != 5 || runs[0].ChannelID != 77 || runs[0].Delay != 90*time.Second ||
		runs[1].ChannelID != ctx.ChannelID || runs[1].Delay != 30*time.Second || runs[1].ExecData != nil {
		t.Fatalf("runs: %+v", runs)
	}
	// Types as msgpack gives them back: the registered sdict/dict/cslice as pointers
	if got := fmt.Sprintf("%T", runs[0].ExecData); got != "*templates.SDict" { // YAGPDB's own type name, which %T prints
		t.Errorf("ExecData type %s", got)
	}
	if len(ctx.SentMessages) != 0 {
		t.Errorf("a scheduled run shouldn't run now: %+v", ctx.SentMessages)
	}
}

func TestExecCCDataCap(t *testing.T) {
	ctx := newCtx(false, true)
	_, err := run(t, ctx, `{{$s := ""}}{{range seq 0 1000}}{{$s = print $s "`+strings.Repeat("x", 1000)+`"}}{{end}}{{execCC 5 nil 10 $s}}`)
	if err == nil || !strings.Contains(err.Error(), "ExecData is too big") {
		t.Errorf("got %v", err)
	}
	// scheduleUniqueCC has no cap
	ctx = newCtx(false, true)
	if _, err := run(t, ctx, `{{$s := ""}}{{range seq 0 1000}}{{$s = print $s "`+strings.Repeat("x", 1000)+`"}}{{end}}{{scheduleUniqueCC 5 nil 10 "k" $s}}`); err != nil {
		t.Errorf("scheduleUniqueCC: %v", err)
	}
}

func TestScheduleUniqueCCReplacesAndCancels(t *testing.T) {
	ctx := newCtx(false, true)
	_, err := run(t, ctx, `{{scheduleUniqueCC 5 nil 60 "u1" "first"}}{{scheduleUniqueCC 5 nil 60 1 "other"}}`+
		`{{scheduleUniqueCC 6 nil 60 "u1" "cc6"}}{{scheduleUniqueCC 5 nil 120 "u1" "second"}}`+
		`{{scheduleUniqueCC 5 nil 0 "u2" "no delay"}}{{cancelScheduledUniqueCC 5 "1"}}`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range ctx.ScheduledRuns() {
		got = append(got, fmt.Sprintf("%d/%s/%v/%s", r.CCID, *r.Key, r.ExecData, r.Delay))
	}
	if want := "6/u1/cc6/1m0s 5/u1/second/2m0s"; strings.Join(got, " ") != want {
		t.Errorf("got %v, want %s", got, want)
	}
}

func TestImmediateExecCCDepthIsAnError(t *testing.T) {
	ctx := newCtx(false, true)
	ctx.ExecCCDepth = 2
	if _, err := run(t, ctx, `{{execCC 5 nil 0 nil}}`); err == nil || !strings.Contains(err.Error(), "Max nested immediate execCC calls reached (2)") {
		t.Errorf("got %v", err)
	}
	// A delayed run starts a fresh stack, so depth doesn't limit it
	if _, err := run(t, ctx, `{{execCC 5 nil 5 nil}}`); err != nil {
		t.Errorf("delayed: %v", err)
	}
}

// execCC, scheduleUniqueCC and cancelScheduledUniqueCC take the command as an int, as in
// YAGPDB: the template engine converts other ints but refuses a float or a string
func TestCCIDIsAnInt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{sendMessage nil "ran"}}`)
	for _, src := range []string{`{{execCC 5 nil 0 nil}}`, `{{execCC (toInt64 5) nil 0 nil}}`, `{{execCC (toInt "5") nil 0 nil}}`, `{{execCC 5.0 nil 0 nil}}`} {
		ctx := newCtx(false, true)
		ctx.TemplateBaseDir = dir
		ctx.CommandIDMap = map[int64]string{5: "child.gohtml"}
		if _, err := run(t, ctx, src); err != nil || len(ctx.SentMessages) != 1 {
			t.Errorf("%s: %v, %+v", src, err, ctx.SentMessages)
		}
	}
	for _, c := range []struct{ src, want string }{
		// a value read from the database, say
		{`{{$id := "5"}}{{execCC $id nil 0 nil}}`, "wrong type for value; expected int; got string"},
		{`{{$id := toFloat 5}}{{execCC $id nil 0 nil}}`, "wrong type for value; expected int; got float64"},
		{`{{$id := "5"}}{{scheduleUniqueCC $id nil 10 "k" nil}}`, "expected int; got string"},
		{`{{$id := 5.5}}{{cancelScheduledUniqueCC $id "k"}}`, "expected int; got float64"},
		// a constant is checked as it's parsed
		{`{{execCC "5" nil 0 nil}}`, `expected integer; found "5"`},
	} {
		if _, err := run(t, newCtx(false, true), c.src); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.src, err)
		}
	}
}

// tmplRunCC's order: the command is looked up (an interval or cron command refused) before
// the channel; an unmapped command warns when run now, and a delayed one is scheduled
func TestExecCCLookupOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/timed.gohtml", "{{/*\n\tTrigger type: `Interval`\n*/}}hi")
	writeFile(t, dir+"/cron.gohtml", "{{/*\n\tTrigger type: `Crontab`\n*/}}hi") // YAGPDB's name
	cases := []struct{ src, err string }{
		{`{{execCC 5 99 0 nil}}`, "interval and cron type custom commands cannot be used with execCC"},
		{`{{execCC 6 99 10 nil}}`, "interval and cron type custom commands cannot be used with execCC"},
		{`{{scheduleUniqueCC 5 99 10 "k" nil}}`, "interval and cron type custom commands cannot be used with scheduleUniqueCC"},
		{`{{execCC 7 nil 0 nil}}`, "execCC 7: can't read its command_map template"},
		{`{{scheduleUniqueCC 7 nil 10 "k" nil}}`, "scheduleUniqueCC 7: can't read its command_map template"},
		{`{{execCC 8 99 0 nil}}`, "Unknown channel"}, // unmapped: then the channel
	}
	for _, c := range cases {
		ctx := channelCtx() // 99 isn't one of its channels
		ctx.TemplateBaseDir = dir
		ctx.CommandIDMap = map[int64]string{5: "timed.gohtml", 6: "cron.gohtml", 7: "missing.gohtml"}
		if _, err := run(t, ctx, c.src); err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: %v, want %q", c.src, err, c.err)
		}
	}

	ctx := channelCtx()
	if _, err := run(t, ctx, `{{execCC 8 nil 0 nil}}{{execCC 8 nil 10 nil}}`); err != nil {
		t.Fatal(err)
	}
	if w := kinds(ctx, KindExecCC); len(w) != 1 || !strings.Contains(w[0], "execCC 8 isn't in the test's command_map") {
		t.Errorf("want one warning, for the immediate run: %q", w)
	}
	if runs := ctx.ScheduledRuns(); len(runs) != 1 || runs[0].CCID != 8 {
		t.Errorf("the delayed run of an unmapped command is scheduled: %+v", runs)
	}
}
