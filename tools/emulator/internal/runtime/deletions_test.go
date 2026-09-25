package runtime

import (
	"slices"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

func deletions(ctx *ExecutionContext) []string {
	var out []string
	for _, d := range ctx.Deletions {
		out = append(out, d.String())
	}
	return out
}

// deleteTrigger and deleteMessage are YAGPDB's: 10 seconds by default, at most a day, a
// delay under 1 at once; an unknown channel deletes nothing
func TestDeleteTriggerAndMessage(t *testing.T) {
	ctx := channelCtx()
	ctx.MessageID = 55
	src := `{{deleteTrigger}}{{deleteTrigger 3}}{{deleteTrigger -5}}{{deleteTrigger 100000}}` +
		`{{deleteMessage nil 77}}{{deleteMessage "staff-log" 78 0}}{{deleteMessage 99 79}}`
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"trigger 55 in channel 123456789 after 10s", "trigger 55 in channel 123456789 after 3s",
		"trigger 55 in channel 123456789 after 0s", "trigger 55 in channel 123456789 after 24h0m0s",
		"message 77 in channel 123456789 after 10s", "message 78 in channel 42 after 0s",
	}
	if got := deletions(ctx); !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if _, err := run(t, channelCtx(), `{{deleteMessage nil}}`); err == nil || !strings.Contains(err.Error(), "wrong number of args for deleteMessage") {
		t.Errorf("deleteMessage takes a channel and a message: %v", err)
	}
}

// deleteTrigger deletes the run's message: the reacted-to message in a reaction run, the
// caller's in an execCC child; an interval run's stand-in (ID 0) deletes nothing
func TestDeleteTriggerByRunKind(t *testing.T) {
	ctx := channelCtx()
	ctx.NoMessage = true
	if _, err := run(t, ctx, `{{deleteTrigger 1}}`); err != nil || len(ctx.Deletions) != 0 {
		t.Errorf("interval: %v, %q", err, deletions(ctx))
	}

	ctx = channelCtx()
	ctx.Reaction = &types.CtxReaction{MessageID: 56}
	if _, err := run(t, ctx, `{{deleteTrigger 1}}`); err != nil || !slices.Equal(deletions(ctx), []string{"trigger 56 in channel 123456789 after 1s"}) {
		t.Errorf("reaction: %v, %q", err, deletions(ctx))
	}

	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", `{{deleteTrigger 2}}{{deleteResponse 5}}child`)
	writeFile(t, dir+"/middle.gohtml", `{{execCC 7 "staff-log" 0 nil}}`)
	ctx = channelCtx()
	ctx.MessageID = 55
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml", 8: "middle.gohtml"}
	if _, err := run(t, ctx, `{{execCC 8 nil 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	// the grandchild deletes the first caller's message, and its own response
	want := []string{"trigger 55 in channel 123456789 after 2s", "response 1100000000000000001 in channel 42 after 5s"}
	if got := deletions(ctx); !slices.Equal(got, want) || ctx.SentMessages[0].ID != 1100000000000000001 {
		t.Errorf("execCC: got %q, want %q (sent %+v)", got, want, ctx.SentMessages)
	}

	ctx = channelCtx()
	ctx.NoMessage = true
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`); err != nil || !slices.Equal(deletions(ctx), []string{"response 1100000000000000001 in channel 123456789 after 5s"}) {
		t.Errorf("interval execCC: %v, %q", err, deletions(ctx))
	}
}

// deleteResponse deletes the response only when one is sent: not for empty output, a delay
// under 1, or a failed run whose show_errors message goes out instead
func TestDeleteResponseOnlyOfASentResponse(t *testing.T) {
	cases := []struct{ src, want string }{
		{`{{deleteResponse}}hi`, "response in channel 123456789 after 10s"},
		{`{{deleteResponse 3}}  `, ""},
		{`{{deleteResponse 0}}hi`, ""},
		{`{{deleteResponse 3}}hi{{index (cslice) 5}}`, ""},
		{"{{/*\n\tShow errors: `false`\n*/}}{{deleteResponse 3}}hi{{index (cslice) 5}}", "response in channel 123456789 after 3s"},
		{"{{/*\n\tShow errors: `false`\n*/}}{{deleteResponse 0}}hi{{index (cslice) 5}}", ""},
		// over 2000 characters YAGPDB sends (and deletes) its notice instead
		{`{{deleteResponse 4}}{{printf "%03000d" 0}}`, "response in channel 123456789 after 4s"},
	}
	for _, c := range cases {
		ctx := channelCtx()
		run(t, ctx, c.src)
		if got := strings.Join(deletions(ctx), "|"); got != c.want {
			t.Errorf("%q: got %q, want %q", c.src, got, c.want)
		}
	}
}
