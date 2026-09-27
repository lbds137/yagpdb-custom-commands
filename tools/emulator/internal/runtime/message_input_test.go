package runtime

import (
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// YAGPDB c579722 unified message input: sendMessage and editMessage both take their
// message through parseMessageInput (context_funcs.go:32-70), and complexMessageEdit is
// complexMessage (context.go:113-114).

const sentWithEmbed = `{{$id := sendMessageRetID nil ` +
	`(complexMessage "content" "a" "embed" (cembed "title" "T"))}}`

// ToMessageEdit always sets the content (lib/discordgo/message.go:683-690), so an edit
// with only an embed sends "content": "" and clears the old text.
func TestEmbedOnlyEditClearsContent(t *testing.T) {
	ctx := newCtx(true, true)
	out, err := run(t, ctx, sentWithEmbed+
		`{{editMessage nil $id (cembed "title" "U")}}{{$m := getMessage nil $id}}`+
		`[{{$m.Content}}] {{(index $m.Embeds 0).Title}}`)
	if err != nil || out != "[] U" {
		t.Fatalf("got %q, %v", out, err)
	}
	if len(ctx.EditedMessages) != 1 || ctx.EditedMessages[0].Content != "" {
		t.Errorf("edits = %+v", ctx.EditedMessages)
	}
}

// The old "both content and embed cannot be null" check is gone: an edit with neither
// clears the content, and the message keeps its embed.
func TestEditWithNeitherContentNorEmbed(t *testing.T) {
	ctx := newCtx(true, true)
	out, err := run(t, ctx, sentWithEmbed+
		`{{editMessage nil $id (complexMessageEdit)}}{{$m := getMessage nil $id}}`+
		`[{{$m.Content}}] {{len $m.Embeds}}`)
	if err != nil || out != "[] 1" {
		t.Fatalf("got %q, %v", out, err)
	}
	if len(ctx.EditedMessages) != 1 || ctx.EditedMessages[0].Content != "" {
		t.Errorf("edits = %+v", ctx.EditedMessages)
	}
}

func TestEditMessageTakesComplexMessage(t *testing.T) {
	ctx := newCtx(true, true)
	out, err := run(t, ctx, sentWithEmbed+
		`{{editMessage nil $id (complexMessage "content" "b" "embed" (cembed "title" "U"))}}`+
		`{{$m := getMessage nil $id}}{{$m.Content}} {{(index $m.Embeds 0).Title}}`)
	if err != nil || out != "b U" {
		t.Fatalf("got %q, %v", out, err)
	}
}

// complexMessageEdit takes every complexMessage key, "file" and "reply" included
func TestComplexMessageEditTakesSendKeys(t *testing.T) {
	ctx := newCtx(true, true)
	ctx.Messages = []types.CtxMessage{{ID: 7, ChannelID: ctx.ChannelID, Author: botUser}}
	src := `{{sendMessage nil (complexMessageEdit "content" "c" "file" "data" "reply" 7)}}`
	_, err := run(t, ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.FileUploads) != 1 || ctx.FileUploads[0].Content != "data" {
		t.Errorf("uploads = %+v", ctx.FileUploads)
	}
	// "reply" is read, not only accepted
	built, err := NewEngine(newCtx(true, true)).complexMessage("reply", 7)
	if err != nil || built.ReplyTo != 7 {
		t.Errorf("complexMessageEdit \"reply\" 7: %+v, %v", built, err)
	}
}

// The builder's pairs are CreateComponentBuilder's (general.go:158-175), with its errors
func TestBuilderPairErrors(t *testing.T) {
	for src, want := range map[string]string{
		`{{complexMessageEdit "content" "a" "embed"}}`: "invalid dict call",
		`{{complexMessageEdit "content" "a" 1 "b"}}`:   "Only string keys supported in sdict",
	} {
		if _, err := run(t, newCtx(true, true), src); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want an error containing %q, got %v", src, want, err)
		}
	}
}

// An empty "embed" slice builds non-nil, empty embeds (general.go:295), which an edit sends
// as "embeds": [] and so clears the message's embeds; nil embeds leave them alone.
func TestEditEmbedsEmptyClearsNilKeeps(t *testing.T) {
	ctx := newCtx(true, true)
	out, err := run(t, ctx, sentWithEmbed+
		`{{editMessage nil $id (complexMessageEdit "content" "x" "embed" (cslice))}}`+
		`{{len (getMessage nil $id).Embeds}}`+
		`{{$id2 := sendMessageRetID nil (complexMessage "content" "a" "embed" (cembed "title" "T"))}}`+
		`{{editMessage nil $id2 (complexMessageEdit "content" "y" "embed" nil)}}`+
		`{{len (getMessage nil $id2).Embeds}}`)
	if err != nil || out != "01" {
		t.Fatalf("got %q, %v", out, err)
	}
	if len(ctx.EditedMessages) != 2 || len(ctx.EditedMessages[0].Embeds) != 0 {
		t.Errorf("edits = %+v", ctx.EditedMessages)
	}
}

// ToMessageEdit (lib/discordgo/message.go:683-690) drops the file: an edit uploads
// nothing, and the file doesn't count as content.
func TestEditDropsFile(t *testing.T) {
	ctx := newCtx(true, true)
	out, err := run(t, ctx, sentWithEmbed+
		`{{editMessage nil $id (complexMessageEdit "content" "b" "file" "data")}}`+
		`{{(getMessage nil $id).Content}}`)
	if err != nil || out != "b" || len(ctx.FileUploads) != 0 {
		t.Fatalf("got %q, %v; uploads %+v", out, err, ctx.FileUploads)
	}

	// a message left with a file alone is empty, which Discord refuses
	ctx = newCtx(true, true)
	_, err = run(t, ctx, `{{$id := sendMessageRetID nil "a"}}`+
		`{{editMessage nil $id (complexMessageEdit "file" "data")}}`)
	if err == nil || !strings.Contains(err.Error(), `"code": 50006`) {
		t.Errorf("file-only edit: got %v", err)
	}
	if len(ctx.FileUploads) != 0 || len(ctx.EditedMessages) != 0 {
		t.Errorf("uploads %+v, edits %+v", ctx.FileUploads, ctx.EditedMessages)
	}
}

// general.go ~295: each non-nil "embed" key starts the embeds over, so the last one wins
func TestRepeatedEmbedKeyReplaces(t *testing.T) {
	ctx := newCtx(true, true)
	src := `{{sendMessage nil (complexMessage "embed" (cembed "title" "A")` +
		` "embed" (cslice (cembed "title" "B") (cembed "title" "C")) "embed" nil)}}`
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	embeds := ctx.SentMessages[0].Embeds
	title := func(i int) interface{} { return embeds[i].(types.Embed)["title"] }
	if len(embeds) != 2 || title(0) != "B" || title(1) != "C" {
		t.Errorf("embeds = %#v", embeds)
	}
}

// parseMessageInput drops nil embeds, unless the message is a Components V2 one
func TestParseMessageInputDropsNilEmbeds(t *testing.T) {
	msg := parseMessageInput([]*types.MessageEmbed{nil, {Title: "x"}})
	if len(msg.Embeds) != 1 || msg.Embeds[0].(types.Embed)["title"] != "x" {
		t.Errorf("slice: embeds = %#v", msg.Embeds)
	}
	// every embed dropped leaves nil (an edit then keeps the message's embeds)
	if msg := parseMessageInput((*types.MessageEmbed)(nil)); msg.Embeds != nil {
		t.Errorf("nil embed: embeds = %#v", msg.Embeds)
	}
	v2 := parseMessageInput(&types.MessageSend{ComponentsV2: true, Embeds: []interface{}{nil}})
	if len(v2.Embeds) != 1 {
		t.Errorf("components v2: embeds = %#v", v2.Embeds)
	}

	// through sendMessage: the nil embed isn't sent
	ctx := newCtx(true, true)
	engine := NewEngine(ctx)
	embeds := []*types.MessageEmbed{nil, {Title: "y"}}
	if _, err := engine.send("sendMessage", true, nil, embeds); err != nil {
		t.Fatal(err)
	}
	if len(ctx.SentMessages) != 1 || len(ctx.SentMessages[0].Embeds) != 1 {
		t.Errorf("sent = %+v", ctx.SentMessages)
	}
}

// MessageState has Pinned (lib/dstate/interface.go:425); the emulator models no pins
func TestMessagePinned(t *testing.T) {
	ctx := newCtx(true, true)
	ctx.Messages = []types.CtxMessage{{ID: 7, ChannelID: ctx.ChannelID, Author: botUser}}
	out, err := run(t, ctx, `{{.Message.Pinned}} {{(getMessage nil 7).Pinned}}`)
	if err != nil || out != "false false" {
		t.Errorf("got %q, %v", out, err)
	}
}
