package runtime

import (
	"strings"
	"testing"
	"time"
)

// sleep is YAGPDB's tmplSleep without the wait: at least a second, at most 60 per run,
// and the run's clock moves on by what was slept
func TestSleep(t *testing.T) {
	ctx := newCtx(true, true)
	ctx.FixClock(time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC))
	start := time.Now()
	out, err := run(t, ctx, `{{currentTime.Unix}} {{sleep 10}}{{currentTime.Unix}} {{sleep 50}}{{currentTime.Unix}}`+
		` {{dbSet 0 "k" 1}}{{(dbGet 0 "k").CreatedAt.Unix}}`)
	if err != nil || out != "981173106 981173116 981173166 981173166" {
		t.Errorf("got %q, %v", out, err)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("sleep waited: %s", time.Since(start))
	}

	for _, src := range []string{`{{sleep 61}}`, `{{sleep 0}}`, `{{sleep 0.5}}`, `{{sleep 30}}{{sleep 31}}`} {
		if _, err := run(t, newCtx(true, true), src); err == nil || !strings.Contains(err.Error(), "can sleep for max 60 seconds combined") {
			t.Errorf("%s: %v", src, err)
		}
	}
	// the refusal is a function error, so try catches it
	if out, err := run(t, newCtx(true, true), `{{try}}{{sleep 0}}{{catch}}caught{{end}}`); err != nil || out != "caught" {
		t.Errorf("got %q, %v", out, err)
	}
}

// An execCC child starts its own 60 seconds, on a clock its caller's sleeps moved on
func TestSleepInExecCCChild(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{sleep 60}}{{currentTime.Unix}}`)
	ctx := channelCtx()
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{1: "child.gohtml"}
	ctx.FixClock(time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC))
	if _, err := run(t, ctx, `{{sleep 60}}{{execCC 1 nil 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	if len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != "981173226" {
		t.Errorf("got %+v", ctx.SentMessages)
	}
}

// On the system clock too, the database's times and expiry move on with sleep
func TestSleepMovesTheDatabaseClock(t *testing.T) {
	ctx := newCtx(true, true)
	out, err := run(t, ctx, `{{$t := currentTime}}{{dbSetExpire 0 "e" 1 5}}{{sleep 10}}`+
		`{{if dbGet 0 "e"}}still there{{else}}expired{{end}} `+
		`{{dbSet 0 "k" 1}}{{((dbGet 0 "k").CreatedAt.Sub $t).Seconds | roundFloor}}`)
	if err != nil || out != "expired 10" {
		t.Errorf("got %q, %v", out, err)
	}
}

// A join time is a fact: an execCC child sees the same one after its caller slept
func TestSleepKeepsJoinTimesInExecCCChild(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{.Member.JoinedAt}}`)
	ctx := channelCtx()
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{1: "child.gohtml"}
	out, err := run(t, ctx, `{{.Member.JoinedAt}}{{sleep 10}}{{execCC 1 nil 0 nil}}`)
	if err != nil || len(ctx.SentMessages) != 1 || ctx.SentMessages[0].Content != out {
		t.Errorf("caller %q, child %+v, %v", out, ctx.SentMessages, err)
	}
}
