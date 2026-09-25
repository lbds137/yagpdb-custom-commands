package runtime

import (
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// A message read back holds discordgo-shaped embeds; YAGPDB's send functions take those
// as they are (*discordgo.MessageEmbed, []*discordgo.MessageEmbed; CreateEmbed returns one
// unchanged), so a template can send a read embed again.
func embedMsgCtx() *ExecutionContext {
	ctx := newCtx(true, true)
	ctx.Messages = []types.CtxMessage{{ID: 7, ChannelID: ctx.ChannelID, Author: botUser,
		Embeds: []*types.MessageEmbed{{Title: "Quoted", Author: &types.MessageEmbedAuthor{Name: "Bot"}}}}}
	return ctx
}

func TestReadEmbedsCanBeSentAgain(t *testing.T) {
	read := `{{$m := getMessage nil 7}}{{$e := index $m.Embeds 0}}`
	for name, src := range map[string]string{
		"sendMessage an embed":        read + `{{sendMessage nil $e}}`,
		"sendMessage the embeds":      read + `{{sendMessage nil $m.Embeds}}`,
		"complexMessage":              read + `{{sendMessage nil (complexMessage "content" "x" "embed" $e)}}`,
		"complexMessage the embeds":   read + `{{sendMessage nil (complexMessage "embed" $m.Embeds)}}`,
		"cembed returns it unchanged": read + `{{sendMessage nil (cembed $e)}}`,
		"editMessage":                 read + `{{editMessage nil 7 $e}}{{sendMessage nil (index (getMessage nil 7).Embeds 0)}}`,
	} {
		ctx := embedMsgCtx()
		if _, err := run(t, ctx, src); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		last := ctx.SentMessages[len(ctx.SentMessages)-1]
		if e, ok := last.Embed.(types.Embed); !ok || e["title"] != "Quoted" {
			t.Errorf("%s: sent %#v", name, last.Embed)
		}
	}
}

// discordgo.MessageEmbed has no methods, so the emulator's conversions aren't template-visible
func TestReadEmbedsHaveNoExtraMethods(t *testing.T) {
	for _, src := range []string{`{{(index (getMessage nil 7).Embeds 0).Map}}`} {
		if _, err := run(t, embedMsgCtx(), src); err == nil {
			t.Errorf("%s should fail as in YAGPDB", src)
		}
	}
}

// discordgo drops empty embeds before sending, so a message read back has none
func TestEmptyEmbedsAreNotStored(t *testing.T) {
	ctx := newCtx(true, true)
	out, err := run(t, ctx, `{{$id := sendMessageRetID nil (complexMessage "content" "x" "embed" (cslice (cembed) (cembed "title" "T")))}}{{len (getMessage nil $id).Embeds}}`)
	if err != nil || !strings.HasSuffix(out, "1") {
		t.Errorf("want 1 stored embed: %q %v", out, err)
	}
}
