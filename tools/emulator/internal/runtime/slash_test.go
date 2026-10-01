package runtime

import (
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

const probeHeader = "{{/*\n  Trigger type: `Slash Command`\n  Trigger: `probe`\n" +
	"  Slash option: `text string! the text`\n" +
	"  Slash option: `count integer how many`\n" +
	"  Slash option: `who user! a member`\n" +
	"  Slash option: `where channel a channel`\n" +
	"  Slash option: `rank role a role`\n" +
	"  Slash option: `whom mentionable a role or a user`\n*/}}"

const kvHeader = "{{/*\n  Trigger type: `Slash Command`\n  Trigger: `kv`\n" +
	"  Slash subcommand: `get read a key`\n  Slash subcommand: `set write a key`\n" +
	"  Slash option: `get.key string! the key`\n  Slash option: `set.key string! the key`\n" +
	"  Slash option: `set.ttl integer seconds`\n*/}}"

func TestReadSlashCommand(t *testing.T) {
	def, err := ReadSlashCommand(probeHeader)
	if err != nil {
		t.Fatal(err)
	}
	if len(def.Options) != 6 || len(def.Subcommands) != 0 {
		t.Fatalf("got %+v", def)
	}
	want := []SlashOption{
		{"text", "string", types.ApplicationCommandOptionString, "the text", true},
		{"count", "integer", types.ApplicationCommandOptionInteger, "how many", false},
		{"who", "user", types.ApplicationCommandOptionUser, "a member", true},
	}
	for i, w := range want {
		if def.Options[i] != w {
			t.Errorf("option %d: got %+v, want %+v", i, def.Options[i], w)
		}
	}

	def, err = ReadSlashCommand(kvHeader)
	if err != nil {
		t.Fatal(err)
	}
	if len(def.Options) != 0 || len(def.Subcommands) != 2 || def.Subcommands[0].Name != "get" ||
		def.Subcommands[0].Description != "read a key" || len(def.Subcommands[1].Options) != 2 ||
		def.Subcommands[1].Options[1].Name != "ttl" || def.Subcommands[1].Options[1].Required {
		t.Errorf("subcommands: %+v", def.Subcommands)
	}
}

// TestSlashHeaderValidation checks the panel's rejections (customcommands.go:610-615,
// 641-741) with its texts, and the header's own shape errors.
func TestSlashHeaderValidation(t *testing.T) {
	slash := func(name, rows string) string {
		return "{{/*\n  Trigger type: `Slash Command`\n  Trigger: `" + name + "`\n" + rows + "*/}}"
	}
	cases := []struct{ name, src, want string }{
		{"uppercase name", slash("Probe", ""), "Slash command name must be lowercase"},
		{"name with a space", slash("my cmd", ""), "Slash command name must be 1-32 characters"},
		{"update defer", slash("x", "  Defer mode: `Update Message Response`\n"),
			`"Update message" defer mode is not valid for slash commands`},
		{"bad option row", slash("x", "  Slash option: `key string`\n"), "write `[sub.]name type[!] description`"},
		{"unknown type", slash("x", "  Slash option: `key text! the key`\n"),
			`Invalid type for option "key": "text!" isn't one of string, string_menu`},
		{"uppercase option", slash("x", "  Slash option: `Key string the key`\n"), `Option name "Key" must be lowercase`},
		{"bad option name", slash("x", "  Slash option: `ke$y string the key`\n"),
			`Option name "ke$y" must be 1-32 characters`},
		{"duplicate option", slash("x", "  Slash option: `key string a`\n  Slash option: `key string b`\n"),
			`Duplicate option name "key"`},
		{"long description", slash("x", "  Slash option: `key string "+strings.Repeat("d", 101)+"`\n"),
			`Description for option "key" must be between 1 and 100 characters`},
		{"too many options", slash("x", manyOptions(26)), "can have at most 25 options"},
		{"bad subcommand row", slash("x", "  Slash subcommand: `get`\n"), "write `name description`"},
		{"uppercase subcommand", slash("x", "  Slash subcommand: `Get read`\n"), `Subcommand name "Get" must be lowercase`},
		{"duplicate subcommand", slash("x", "  Slash subcommand: `get a`\n  Slash subcommand: `get b`\n"),
			`Duplicate subcommand name "get"`},
		{"subcommand option error is prefixed", slash("x", "  Slash subcommand: `get a`\n  Slash option: `get.Key string k`\n"),
			`Subcommand "get": Option name "Key" must be lowercase`},
		{"top-level option with subcommands", slash("x", "  Slash subcommand: `get a`\n  Slash option: `key string k`\n"),
			"a command with subcommands has no top-level options"},
		{"option of an unknown subcommand", slash("x", "  Slash subcommand: `get a`\n  Slash option: `set.key string k`\n"),
			`no Slash subcommand line names "set"`},
		{"option rows on a message trigger", "{{/*\n  Trigger type: `Command`\n  Trigger: `x`\n  Slash option: `key string k`\n*/}}",
			"need a Slash Command trigger"},
		{"context menu update defer", "{{/*\n  Trigger type: `User Context Menu`\n  Trigger: `View`\n  Defer mode: `Update Message Response`\n*/}}",
			`"Update message" defer mode is not valid for context menu commands`},
		{"context menu bad name", "{{/*\n  Trigger type: `Message Context Menu`\n  Trigger: `Quote!`\n*/}}",
			"Context menu command name must be 1-32 characters"},
	}
	for _, c := range cases {
		err := ValidateHeader(c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want an error containing %q, got %v", c.name, c.want, err)
		}
	}
	for name, src := range map[string]string{
		"a valid slash header":     probeHeader,
		"subcommands":              kvHeader,
		"a context menu entry":     "{{/*\n  Trigger type: `User Context Menu`\n  Trigger: `View avatar`\n*/}}",
		"unicode option name":      slash("x", "  Slash option: `מפתח string the key`\n"),
		"an ephemeral slash defer": slash("x", "  Defer mode: `Ephemeral Message Response`\n"),
	} {
		if err := ValidateHeader(src); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func manyOptions(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("  Slash option: `o")
		b.WriteString(strings.Repeat("x", i))
		b.WriteString(" string an option`\n")
	}
	return b.String()
}

// slashCtx is a run of the probe command with the given options.
func slashCtx(t *testing.T, header string, inv SlashInvocation, mode DeferMode) (*ExecutionContext, error) {
	t.Helper()
	ctx := newCtx(false, true)
	ctx.UserID = 42
	ctx.Members = []int64{42, 5}
	ctx.AvailableRoles[111] = types.CtxRole{ID: 111, Name: "Staff"}
	ctx.ChannelDetails = map[int64]types.CtxChannel{9: {ID: 9, Name: "log"}}
	def, err := ReadSlashCommand(header)
	if err != nil {
		t.Fatal(err)
	}
	trigger, _ := ReadTrigger(header)
	return ctx, ctx.SetInteractionSlash(trigger, def, inv, mode)
}

func TestSetInteractionSlash(t *testing.T) {
	all := map[string]interface{}{"text": "hi", "count": 3, "who": 5, "where": 9, "rank": 111, "whom": 5}
	ctx, err := slashCtx(t, probeHeader, SlashInvocation{Options: all}, DeferModeNone)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Interaction.Type != types.InteractionApplicationCommand || ctx.Interaction.Message != nil ||
		ctx.Interaction.Member.User.ID != 42 || ctx.Interaction.RespondedTo {
		t.Errorf("interaction: %+v", ctx.Interaction.Interaction)
	}
	data := ctx.BuildTemplateData()
	if data["IsSlashCommand"] != true || data["Cmd"] != "probe" || data["CommandName"] != "probe" || data["SubCommand"] != "" {
		t.Errorf("keys: %v %v %v %q", data["IsSlashCommand"], data["Cmd"], data["CommandName"], data["SubCommand"])
	}
	opts := data["Options"].(types.SDict)
	if opts["text"] != "hi" || opts["count"] != int64(3) {
		t.Errorf("scalar options: %#v", opts)
	}
	if u, ok := opts["who"].(*types.DiscordUser); !ok || u.ID != 5 {
		t.Errorf("user option: %#v", opts["who"])
	}
	if c, ok := opts["where"].(*types.CtxChannel); !ok || c.ID != 9 || c.Name != "log" {
		t.Errorf("channel option: %#v", opts["where"])
	}
	if r, ok := opts["rank"].(*types.CtxRole); !ok || r.ID != 111 {
		t.Errorf("role option: %#v", opts["rank"])
	}
	if u, ok := opts["whom"].(*types.DiscordUser); !ok || u.ID != 5 {
		t.Errorf("a mentionable that isn't a role is a user: %#v", opts["whom"])
	}
	args := data["Args"].([]interface{})
	if len(args) != 7 || args[0] != "probe" || len(data["CmdArgs"].([]interface{})) != 6 {
		t.Errorf("args: %v", args)
	}
	msg := data["Message"].(types.CtxMessage)
	if msg.ID != 0 || msg.Author.ID != 42 || msg.Member == nil || msg.ChannelID != ctx.ChannelID {
		t.Errorf(".Message is blank with the invoker: %+v", msg)
	}
	if d, ok := data["InteractionData"].(*types.ApplicationCommandInteractionData); !ok || d != ctx.Interaction.DataCommand ||
		d.Name != "probe" {
		t.Errorf("InteractionData is .Interaction.DataCommand: %T %v", data["InteractionData"], ctx.Interaction.DataCommand)
	}

	// Option names match in any case (the handler lowercases them, :143-145); a user
	// option naming a non-member is a user without a resolved member
	ctx, err = slashCtx(t, probeHeader, SlashInvocation{Options: map[string]interface{}{"TEXT": "hi", "Who": 6}}, DeferModeNone)
	if err != nil {
		t.Fatal(err)
	}
	if u, ok := ctx.Slash.Options["who"].(*types.DiscordUser); !ok || u.ID != 6 || ctx.Slash.Options["text"] != "hi" {
		t.Errorf("any-case option names: %#v", ctx.Slash.Options)
	}
	if res := ctx.Interaction.DataCommand.Resolved; res.Users[6] == nil || res.Members[6] != nil {
		t.Errorf("a non-member is resolved as a user only: %+v", res)
	}
	if _, err := slashCtx(t, probeHeader, SlashInvocation{Options: map[string]interface{}{"text": "a", "Text": "b", "who": 5}}, DeferModeNone); err == nil ||
		!strings.Contains(err.Error(), `option "text" is given twice`) {
		t.Errorf("two spellings of one option: %v", err)
	}
	if data["User"].(types.DiscordUser).ID != 42 {
		t.Error(".User is the invoker")
	}

	// Absent optionals are skipped in CmdArgs; a mentionable role resolves to the role
	ctx, err = slashCtx(t, probeHeader, SlashInvocation{Options: map[string]interface{}{"text": "hi", "who": 5, "whom": 111}}, DeferModeEphemeral)
	if err != nil {
		t.Fatal(err)
	}
	data = ctx.BuildTemplateData()
	if got := data["CmdArgs"].([]interface{}); len(got) != 3 {
		t.Errorf("CmdArgs with absent optionals: %v", got)
	}
	if _, ok := data["Options"].(types.SDict)["count"]; ok {
		t.Error("an absent option isn't in .Options")
	}
	if r, ok := data["Options"].(types.SDict)["whom"].(*types.CtxRole); !ok || r.Name != "Staff" {
		t.Errorf("a mentionable role: %#v", data["Options"].(types.SDict)["whom"])
	}
	if !ctx.Interaction.RespondedTo || !ctx.Interaction.Deferred {
		t.Error("a defer mode responds before the run")
	}

	// Subcommands
	ctx, err = slashCtx(t, kvHeader, SlashInvocation{Subcommand: "SET", Options: map[string]interface{}{"key": "k", "ttl": 5}}, DeferModeNone)
	if err != nil {
		t.Fatal(err)
	}
	data = ctx.BuildTemplateData()
	if data["SubCommand"] != "set" || len(data["CmdArgs"].([]interface{})) != 2 {
		t.Errorf("subcommand: %v %v", data["SubCommand"], data["CmdArgs"])
	}
	in := ctx.Interaction.Data.(types.ApplicationCommandInteractionData)
	if len(in.Options) != 1 || in.Options[0].Type != types.ApplicationCommandOptionSubCommand || len(in.Options[0].Options) != 2 {
		t.Errorf("the subcommand is the first option with the leaf options nested: %+v", in.Options)
	}
}

func TestSetInteractionSlashErrors(t *testing.T) {
	base := map[string]interface{}{"text": "hi", "who": 5}
	with := func(k string, v interface{}) map[string]interface{} {
		m := map[string]interface{}{}
		for kk, vv := range base {
			m[kk] = vv
		}
		m[k] = v
		return m
	}
	cases := []struct {
		name   string
		header string
		inv    SlashInvocation
		want   string
	}{
		{"missing required", probeHeader, SlashInvocation{Options: map[string]interface{}{"text": "hi"}}, `the required option "who" (user) wasn't given`},
		{"unknown option", probeHeader, SlashInvocation{Options: with("nope", 1)}, `option "nope" isn't one the header declares`},
		{"string given a number", probeHeader, SlashInvocation{Options: with("text", 3)}, `option "text": string takes a string, not 3 (int)`},
		{"integer given a string", probeHeader, SlashInvocation{Options: with("count", "3")}, `integer takes a whole number, not the string "3"`},
		{"integer given a float", probeHeader, SlashInvocation{Options: with("count", 1.5)}, "integer takes a whole number"},
		{"user given a string", probeHeader, SlashInvocation{Options: with("who", "5")}, "user takes an ID"},
		{"undeclared channel", probeHeader, SlashInvocation{Options: with("where", 77)}, "channel 77 isn't the run's channel or one in guild.channels"},
		{"undeclared role", probeHeader, SlashInvocation{Options: with("rank", 77)}, "role 77 isn't in guild.roles"},
		{"subcommand on a plain command", probeHeader, SlashInvocation{Subcommand: "get", Options: base}, `has no subcommands`},
		{"no subcommand given", kvHeader, SlashInvocation{Options: map[string]interface{}{"key": "k"}}, "give interaction subcommand: (get, set)"},
		{"unknown subcommand", kvHeader, SlashInvocation{Subcommand: "del"}, `subcommand "del" isn't one the header declares (get, set)`},
		{"required option of the subcommand", kvHeader, SlashInvocation{Subcommand: "get"}, `"key" (string) wasn't given for subcommand "get"`},
		{"option of another subcommand", kvHeader, SlashInvocation{Subcommand: "get", Options: map[string]interface{}{"key": "k", "ttl": 1}}, `option "ttl" isn't one the header declares for subcommand "get"`},
	}
	for _, c := range cases {
		_, err := slashCtx(t, c.header, c.inv, DeferModeNone)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want an error containing %q, got %v", c.name, c.want, err)
		}
	}
	// The run's own channel is declared without guild.channels; a number option takes an
	// integer or a float
	ctx := newCtx(false, true)
	def, _ := ReadSlashCommand(probeHeader)
	trigger, _ := ReadTrigger(probeHeader)
	if err := ctx.SetInteractionSlash(trigger, def, SlashInvocation{Options: with("where", int(ctx.ChannelID))}, DeferModeNone); err != nil {
		t.Errorf("the run's channel: %v", err)
	}
	if err := ctx.SetInteractionSlash(Trigger{Type: "Command", Text: "x"}, def, SlashInvocation{}, DeferModeNone); err == nil ||
		!strings.Contains(err.Error(), "needs a Slash Command trigger") {
		t.Errorf("wrong trigger: %v", err)
	}
	num := "{{/*\n  Trigger type: `Slash Command`\n  Trigger: `n`\n  Slash option: `ratio number! r`\n*/}}"
	def, _ = ReadSlashCommand(num)
	trigger, _ = ReadTrigger(num)
	for _, raw := range []interface{}{2, 0.5} {
		if err := ctx.SetInteractionSlash(trigger, def, SlashInvocation{Options: map[string]interface{}{"ratio": raw}}, DeferModeNone); err != nil {
			t.Errorf("number %v: %v", raw, err)
		} else if _, ok := ctx.Slash.Options["ratio"].(float64); !ok {
			t.Errorf("number %v is a float64: %#v", raw, ctx.Slash.Options["ratio"])
		}
	}
}

func TestSetInteractionContextMenu(t *testing.T) {
	newMenuCtx := func() *ExecutionContext {
		ctx := newCtx(false, true)
		ctx.UserID = 42
		ctx.Members = []int64{42, 5}
		ctx.Messages = append(ctx.Messages, types.CtxMessage{ID: 7, ChannelID: ctx.ChannelID,
			Author: types.DiscordUser{ID: 6, Username: "Gone"}, Content: "quote me"})
		return ctx
	}
	user := Trigger{Type: "User Context Menu", Text: " View avatar "}
	ctx := newMenuCtx()
	if err := ctx.SetInteractionContextMenu(user, ContextMenuTarget{UserID: 5}, DeferModeNone); err != nil {
		t.Fatal(err)
	}
	if !ctx.NoMember || !ctx.NoMessage || ctx.Interaction.Type != types.InteractionApplicationCommand {
		t.Errorf("a user menu run has no member and no message: %+v", ctx.Interaction.Interaction)
	}
	data := ctx.BuildTemplateData()
	for _, k := range []string{"User", "user", "Member", "Message"} {
		if _, ok := data[k]; ok {
			t.Errorf(".%s is set", k)
		}
	}
	if data["IsContextMenuCommand"] != true || data["CommandType"] != "user" || data["CommandName"] != "View avatar" ||
		data["Cmd"] != "View avatar" {
		t.Errorf("keys: %v %v %q", data["IsContextMenuCommand"], data["CommandType"], data["CommandName"])
	}
	if data["Author"].(*types.DiscordUser).ID != 42 || data["TargetUser"].(*types.DiscordUser).ID != 5 ||
		data["TargetMember"].(*types.CtxTargetMember).User.ID != 5 {
		t.Errorf("author/target: %+v %+v %+v", data["Author"], data["TargetUser"], data["TargetMember"])
	}
	if in := ctx.Interaction.Data.(types.ApplicationCommandInteractionData); in.CommandType != types.UserApplicationCommand ||
		in.TargetID != 5 || in.Name != "View avatar" {
		t.Errorf("data: %+v", in)
	}
	if msg := ctx.triggerMsg(); msg.ID != 0 || msg.Author.ID != botUser.ID || msg.Member == nil || msg.Member.User.ID != botUser.ID {
		t.Errorf("an execCC child gets the bot's blank message, the bot as member: %+v", msg)
	}
	if ctx.Interaction.DataCommand == nil || ctx.Interaction.DataCommand.Name != "View avatar" {
		t.Errorf("DataCommand: %+v", ctx.Interaction.DataCommand)
	}

	// A target who left: TargetMember is a nil pointer, as bot.GetMember returns
	ctx = newMenuCtx()
	if err := ctx.SetInteractionContextMenu(user, ContextMenuTarget{UserID: 6}, DeferModeNone); err != nil {
		t.Fatal(err)
	}
	if m, ok := ctx.BuildTemplateData()["TargetMember"].(*types.CtxTargetMember); !ok || m != nil {
		t.Errorf("TargetMember for a non-member: %#v", ctx.BuildTemplateData()["TargetMember"])
	}

	// The message menu: .Message is the clicked message, its author the target
	msgMenu := Trigger{Type: "Message Context Menu", Text: "Quote Message"}
	ctx = newMenuCtx()
	if err := ctx.SetInteractionContextMenu(msgMenu, ContextMenuTarget{MessageID: 7}, DeferModeMessage); err != nil {
		t.Fatal(err)
	}
	data = ctx.BuildTemplateData()
	if msg := data["Message"].(types.CtxMessage); msg.ID != 7 || msg.GuildID != ctx.GuildID || msg.Author.ID != 6 {
		t.Errorf(".Message: %+v", msg)
	}
	if data["CommandType"] != "message" || data["TargetUser"].(*types.DiscordUser).Username != "Gone" ||
		data["TargetMember"].(*types.CtxTargetMember) != nil {
		t.Errorf("message target: %v %+v %v", data["CommandType"], data["TargetUser"], data["TargetMember"])
	}
	if ctx.NoMessage || !ctx.NoMember || !ctx.Interaction.Deferred {
		t.Error("a message menu run has the message, no member, and a deferral responds")
	}
	if msg := ctx.triggerMsg(); msg.ID != 7 {
		t.Errorf("an execCC child gets the clicked message: %+v", msg)
	}

	// Errors
	ctx = newMenuCtx()
	if err := ctx.SetInteractionContextMenu(msgMenu, ContextMenuTarget{MessageID: 8}, DeferModeNone); err == nil ||
		!strings.Contains(err.Error(), "isn't a message in channel") {
		t.Errorf("undeclared message: %v", err)
	}
	if err := ctx.SetInteractionContextMenu(user, ContextMenuTarget{}, DeferModeNone); err == nil ||
		!strings.Contains(err.Error(), "needs its target user") {
		t.Errorf("no target: %v", err)
	}
	if err := ctx.SetInteractionContextMenu(Trigger{Type: "Slash Command", Text: "x"}, ContextMenuTarget{UserID: 5}, DeferModeNone); err == nil ||
		!strings.Contains(err.Error(), "needs a User Context Menu or Message Context Menu trigger") {
		t.Errorf("wrong trigger: %v", err)
	}
}

// TestContextMenuRunSendsNoDM: tmplSendDM returns "" with a nil member
// (context_funcs.go:75), which a context menu run has (handle_contextmenu.go:118-123).
func TestContextMenuRunSendsNoDM(t *testing.T) {
	ctx := newCtx(false, true)
	if err := ctx.SetInteractionContextMenu(Trigger{Type: "User Context Menu", Text: "DM me"},
		ContextMenuTarget{UserID: 5}, DeferModeNone); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, ctx, `{{sendDM "hi"}}ok`)
	if err != nil || out != "ok" {
		t.Fatalf("got %q, %v", out, err)
	}
	if len(ctx.SentMessages) != 1 || ctx.SentMessages[0].ChannelID != ctx.ChannelID {
		t.Errorf("only the response was sent, no DM: %+v", ctx.SentMessages)
	}
	if w := kinds(ctx, KindResponse); len(w) == 0 || !strings.Contains(w[0], "sends no DM") {
		t.Errorf("warnings: %q", w)
	}
}

// TestTriggerKinds checks the trigger type predicates.
func TestTriggerKinds(t *testing.T) {
	if !(Trigger{Type: "slash command"}).SlashTriggered() || (Trigger{Type: "Command"}).SlashTriggered() {
		t.Error("SlashTriggered")
	}
	if kind, ok := (Trigger{Type: "Message Context Menu"}).ContextMenuTriggered(); !ok || kind != types.MessageApplicationCommand {
		t.Error("ContextMenuTriggered message")
	}
	if kind, ok := (Trigger{Type: "user context menu"}).ContextMenuTriggered(); !ok || kind != types.UserApplicationCommand {
		t.Error("ContextMenuTriggered user")
	}
	if (Trigger{Type: "Reaction"}).InteractionTriggered() || !(Trigger{Type: "Component"}).InteractionTriggered() {
		t.Error("InteractionTriggered")
	}
	if name, ok := (Trigger{Type: "Slash Command"}).disallowedExecCCType(); ok {
		t.Errorf("execCC into a slash command is allowed (tmplextensions.go:202-206): %q", name)
	}
}
