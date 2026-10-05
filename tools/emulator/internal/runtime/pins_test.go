package runtime

import (
	"slices"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

func pinChanges(ctx *ExecutionContext) []string {
	var out []string
	for _, p := range ctx.Pins {
		out = append(out, p.String())
	}
	return out
}

// pinCtx is a strict context with message 77 in its channel (123456789) and channel 42
func pinCtx() *ExecutionContext {
	ctx := channelCtx()
	ctx.Messages = []types.CtxMessage{{ID: 77, ChannelID: ctx.ChannelID}, {ID: 78, ChannelID: 42}}
	return ctx
}

func TestPinMessage(t *testing.T) {
	cases := []struct {
		src  string
		pins []string
		err  string
	}{
		{`{{pinMessage nil 77}}`, []string{"pin message 77 in channel 123456789"}, ""},
		{`{{pinMessage 42 78}}{{unpinMessage 42 78}}`,
			[]string{"pin message 78 in channel 42", "unpin message 78 in channel 42"}, ""},
		// the channel resolves through ChannelArgNoDM, a name included
		{`{{pinMessage "staff-log" 78}}`, []string{"pin message 78 in channel 42"}, ""},
		// tmplPinMessage's own errors: no such channel, over the message_pins counter of 2
		{`{{pinMessage 43 77}}`, nil, "unknown channel"},
		{`{{pinMessage nil 77}}{{pinMessage nil 77}}{{unpinMessage nil 77}}`, nil, "too many calls to this function"},
		// the counter is checked before the channel resolves (vendor order)
		{`{{pinMessage nil 77}}{{pinMessage nil 77}}{{pinMessage 43 77}}`, nil, "too many calls to this function"},
		// Discord's refusals: a message that doesn't exist, a full pin list
		{`{{pinMessage nil 99}}`, nil, `"code": 10008`},
		{`{{pinMessage 42 77}}`, nil, `"code": 10008`},
		{`{{pinMessage 42 78}}`, nil, `"code": 30003`},
	}
	for _, c := range cases {
		ctx := pinCtx()
		if strings.Contains(c.err, "30003") {
			ctx.PinsFull = map[int64]bool{42: true}
		}
		out, err := run(t, ctx, c.src)
		if c.err != "" {
			if err == nil || !strings.Contains(explained(ctx, err), c.err) {
				t.Errorf("%s: want error %q, got %q, %v", c.src, c.err, out, err)
			}
			continue
		}
		if err != nil || out != "" {
			t.Errorf("%s: got %q, %v", c.src, out, err)
		}
		if !slices.Equal(pinChanges(ctx), c.pins) {
			t.Errorf("%s: pins %q, want %q", c.src, pinChanges(ctx), c.pins)
		}
	}
}

// A full pin list refuses a pin, not an unpin; inside {{try}} the refusal is an error the
// template catches, and nothing is recorded
func TestPinMessageRefusedInTry(t *testing.T) {
	ctx := pinCtx()
	ctx.Strict = false
	ctx.PinsFull = map[int64]bool{ctx.ChannelID: true}
	out, err := run(t, ctx, `{{try}}{{pinMessage nil 77}}ok{{catch}}caught {{.Error}}{{end}}|{{unpinMessage nil 77}}`)
	if err != nil || !strings.HasPrefix(out, "caught HTTP 400 Bad Request, {\"message\": \"Maximum number of pins reached\", \"code\": 30003}") {
		t.Fatalf("got %q, %v", out, err)
	}
	if got := pinChanges(ctx); !slices.Equal(got, []string{"unpin message 77 in channel 123456789"}) {
		t.Errorf("pins %q", got)
	}
}

// createForumPost's first message has the thread's own ID, as in Discord, so a post's
// starter message can be pinned in the new thread
func TestPinForumStarter(t *testing.T) {
	ctx := pinCtx()
	ctx.Channels[50] = "forum"
	ctx.ChannelDetails = map[int64]types.CtxChannel{50: {ID: 50, Name: "forum", Type: 15}}
	out, err := run(t, ctx, `{{$p := createForumPost 50 "T" (complexMessage "content" "hi")}}{{pinMessage $p.ID $p.ID}}{{pinMessage $p.ID 5}}`)
	if err == nil || !strings.Contains(explained(ctx, err), `"code": 10008`) {
		t.Fatalf("a message that isn't the starter must be refused: %q, %v", out, err)
	}
	if got := pinChanges(ctx); !slices.Equal(got, []string{"pin message 1200000000000000000 in channel 1200000000000000000"}) {
		t.Errorf("pins %q", got)
	}
}
