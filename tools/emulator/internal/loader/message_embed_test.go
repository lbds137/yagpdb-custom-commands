package loader

import (
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

func TestTestMessageEmbeds(t *testing.T) {
	var ok MessageDef
	src := "id: 1\nembeds:\n  - { title: T, author: { name: A }, fields: [{ name: n, value: v }] }\n  - {}\n"
	if err := types.StrictYAML([]byte(src), &ok); err != nil {
		t.Fatal(err)
	}
	e := ok.Embeds[0].MessageEmbed
	if e.Title != "T" || e.Author.Name != "A" || e.Fields[0].Value != "v" {
		t.Errorf("got %+v", e)
	}

	for src, want := range map[string]string{
		"embeds:\n  - { titel: T }":                       `line 2: embeds: "titel" isn't one of cembed's keys`,
		"embeds:\n  - { color: red }":                     "line 2: embeds:",
		"embeds:\n  - x":                                  "line 2: embeds: write each embed as a mapping",
		"embeds:\n  - { author: { nmae: x } }":            `"nmae" isn't one of cembed's keys (in author)`,
		"embeds:\n  - { fields: [{ nme: n, value: v }] }": `"nme" isn't one of cembed's keys (in fields)`,
	} {
		var m MessageDef
		if err := types.StrictYAML([]byte(src), &m); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: want %q, got %v", src, want, err)
		}
	}

	// An empty embed isn't on the message, as discordgo drops it
	tc := &TestCase{Name: "e", TemplateSource: `{{len (getMessage nil 1).Embeds}}`}
	tc.Context.Messages = []MessageDef{ok}
	tc.applyDefaults()
	tc.Context.Messages[0].ChannelID = tc.Context.Channel.ID
	res := NewRunner(RunnerConfig{}).RunTest(tc)
	if res.Error != nil || strings.TrimSpace(res.Output) != "1" {
		t.Errorf("want 1 embed on the message, got %q %v %v", res.Output, res.Error, res.Failures)
	}
}
