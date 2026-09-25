package runtime

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDelayedExecCCIsScheduledNotRun(t *testing.T) {
	ctx := newCtx(false, true)
	ctx.CommandIDMap = map[int64]string{5: "does-not-matter.gohtml"}
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
	if got := fmt.Sprintf("%T", runs[0].ExecData); got != "*yagstd.SDict" {
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
