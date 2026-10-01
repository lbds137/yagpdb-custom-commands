package runtime

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// A slash command or context menu invocation as YAGPDB handles it (vendor
// customcommands/handle_slashcommand.go and handle_contextmenu.go, c579722): the panel's
// option and subcommand rows, read from header lines, the panel's validation of them,
// and the handlers' data keys. The response machinery is the component click's
// (interactions.go).

// Header lines, one per panel row: "Slash subcommand: `get description`" and
// "Slash option: `[sub.]name type[!] description`" (type from slashFormType, `!` =
// required, `sub.` = the subcommand it belongs to).
var (
	headerSlashOption     = regexp.MustCompile("(?i:Slash option): `([^`]*)`")
	headerSlashSubcommand = regexp.MustCompile("(?i:Slash subcommand): `([^`]*)`")
)

// Panel limits and name rules (customcommands.go:179-190).
const (
	maxSlashCommandOptions     = 25
	maxSlashCommandDescription = 100
)

var (
	// slashCommandNameRegex is the panel's (customcommands.go:179): Discord's command and
	// option name pattern, lowercased.
	slashCommandNameRegex = regexp.MustCompile(`^[-_\p{L}\p{N}]{1,32}$`)
	// contextMenuNameRegex is the panel's (:184): spaces and capitals allowed.
	contextMenuNameRegex = regexp.MustCompile(`^[-_\p{L}\p{N} ]{1,32}$`)
)

// slashFormTypes are the panel's option type keys (slashFormType, customcommands.go:
// 278-302) and their Discord option types; a *_menu key is the same type with choices.
var slashFormTypes = map[string]types.ApplicationCommandOptionType{
	"string":       types.ApplicationCommandOptionString,
	"string_menu":  types.ApplicationCommandOptionString,
	"integer":      types.ApplicationCommandOptionInteger,
	"integer_menu": types.ApplicationCommandOptionInteger,
	"number":       types.ApplicationCommandOptionNumber,
	"number_menu":  types.ApplicationCommandOptionNumber,
	"boolean":      types.ApplicationCommandOptionBoolean,
	"user":         types.ApplicationCommandOptionUser,
	"channel":      types.ApplicationCommandOptionChannel,
	"role":         types.ApplicationCommandOptionRole,
	"mentionable":  types.ApplicationCommandOptionMentionable,
}

// slashFormTypeKeys lists the keys for error messages.
const slashFormTypeKeys = "string, string_menu, integer, integer_menu, number, number_menu, " +
	"boolean, user, channel, role, mentionable"

// SlashOption is one option row of the panel (SlashCommandOption, customcommands.go:
// 151-165, without the constraints the emulator doesn't validate: choices, min/max).
type SlashOption struct {
	Name        string
	FormType    string // the panel's type key (string, integer_menu, user...)
	Type        types.ApplicationCommandOptionType
	Description string
	Required    bool
}

// SlashSubcommand is one subcommand row with its options (SlashCommandSubcommand, :171-175).
type SlashSubcommand struct {
	Name        string
	Description string
	Options     []SlashOption
}

// SlashCommandDef is a slash command's stored definition (slashCommandData, :858-864):
// either top-level options or subcommands, each with its own options.
type SlashCommandDef struct {
	Options     []SlashOption
	Subcommands []SlashSubcommand
}

// ReadSlashCommand reads a command's slash option and subcommand rows from its header
// and validates them as the panel does (validateSlashCommandData, customcommands.go:
// 641-711, and validateSlashOptionList, :715-777), with the panel's error texts. The
// trigger is the command's name (:610-612, :645-646); the defer mode is checked too
// (:614-615).
func ReadSlashCommand(source string) (SlashCommandDef, error) {
	var def SlashCommandDef
	t, ok := ReadTrigger(source)
	if !ok || !t.SlashTriggered() {
		return def, fmt.Errorf("Slash option and Slash subcommand lines need a Slash Command trigger")
	}
	name := strings.TrimSpace(t.Text)
	if t.Text != strings.ToLower(t.Text) {
		return def, fmt.Errorf("Slash command name must be lowercase")
	}
	if !slashCommandNameRegex.MatchString(strings.ToLower(name)) {
		return def, fmt.Errorf("Slash command name must be 1-32 characters and contain only letters, " +
			"numbers, dashes and underscores")
	}
	if ReadDeferMode(source) == DeferModeUpdate {
		return def, fmt.Errorf("\"Update message\" defer mode is not valid for slash commands")
	}

	header := headerComment(source)
	for _, m := range headerSlashSubcommand.FindAllStringSubmatch(header, -1) {
		sub, err := parseSlashSubcommandRow(m[1])
		if err != nil {
			return def, err
		}
		def.Subcommands = append(def.Subcommands, sub)
	}
	for _, m := range headerSlashOption.FindAllStringSubmatch(header, -1) {
		subName, opt, err := parseSlashOptionRow(m[1])
		if err != nil {
			return def, err
		}
		if subName == "" {
			if len(def.Subcommands) > 0 {
				// A command with subcommands has no top-level options (:657-658)
				return def, fmt.Errorf("Slash option `%s`: a command with subcommands has no "+
					"top-level options; write `<subcommand>.%s ...`", m[1], opt.Name)
			}
			def.Options = append(def.Options, opt)
			continue
		}
		placed := false
		for i := range def.Subcommands {
			if strings.EqualFold(def.Subcommands[i].Name, subName) {
				def.Subcommands[i].Options = append(def.Subcommands[i].Options, opt)
				placed = true
				break
			}
		}
		if !placed {
			return def, fmt.Errorf("Slash option `%s`: no Slash subcommand line names %q "+
				"(subcommand lines come first)", m[1], subName)
		}
	}
	return def, validateSlashCommandDef(def)
}

// parseSlashSubcommandRow reads "name description".
func parseSlashSubcommandRow(row string) (SlashSubcommand, error) {
	fields := strings.Fields(row)
	if len(fields) < 2 {
		return SlashSubcommand{}, fmt.Errorf("Slash subcommand `%s`: write `name description`", row)
	}
	return SlashSubcommand{Name: fields[0], Description: strings.Join(fields[1:], " ")}, nil
}

// parseSlashOptionRow reads "[sub.]name type[!] description": the subcommand it belongs
// to (or ""), and the option.
func parseSlashOptionRow(row string) (sub string, opt SlashOption, err error) {
	fields := strings.Fields(row)
	if len(fields) < 3 {
		return "", opt, fmt.Errorf("Slash option `%s`: write `[sub.]name type[!] description` "+
			"(type: %s; ! = required)", row, slashFormTypeKeys)
	}
	name := fields[0]
	if i := strings.Index(name, "."); i >= 0 {
		sub, name = name[:i], name[i+1:]
	}
	formType, required := strings.CutSuffix(fields[1], "!")
	discordType, ok := slashFormTypes[formType]
	if !ok {
		// the panel's dropdown can't hold a wrong type; in a header it's a typo
		return "", opt, fmt.Errorf("Invalid type for option %q: %q isn't one of %s", name, fields[1],
			slashFormTypeKeys)
	}
	opt = SlashOption{Name: name, FormType: formType, Type: discordType, Required: required,
		Description: strings.Join(fields[2:], " ")}
	return sub, opt, nil
}

// validateSlashCommandDef is validateSlashCommandData's subcommand and option checks
// (customcommands.go:657-686) with the panel's texts; the description, built-in name,
// uniqueness and per-guild count checks need the panel's state and are left out.
func validateSlashCommandDef(def SlashCommandDef) error {
	if len(def.Subcommands) > 0 {
		seenSubs := make(map[string]bool, len(def.Subcommands))
		for _, sub := range def.Subcommands {
			sname := strings.TrimSpace(sub.Name)
			if !slashCommandNameRegex.MatchString(sname) {
				return fmt.Errorf("Subcommand name %q must be 1-32 characters (letters, numbers, dashes, "+
					"underscores)", sub.Name)
			}
			if sname != strings.ToLower(sname) {
				return fmt.Errorf("Subcommand name %q must be lowercase", sub.Name)
			}
			if seenSubs[sname] {
				return fmt.Errorf("Duplicate subcommand name %q", sname)
			}
			seenSubs[sname] = true
			if l := utf8.RuneCountInString(sub.Description); l < 1 || l > maxSlashCommandDescription {
				return fmt.Errorf("Description for subcommand %q must be between 1 and %d characters",
					sname, maxSlashCommandDescription)
			}
			if err := validateSlashOptionList(sub.Options); err != nil {
				return fmt.Errorf("Subcommand %q: %w", sname, err)
			}
		}
		return nil
	}
	return validateSlashOptionList(def.Options)
}

// validateSlashOptionList is the panel's (customcommands.go:715-741) for the checks a
// header row can fail.
func validateSlashOptionList(options []SlashOption) error {
	if len(options) > maxSlashCommandOptions {
		return fmt.Errorf("can have at most %d options", maxSlashCommandOptions)
	}
	seenOptions := make(map[string]bool, len(options))
	for _, opt := range options {
		oname := strings.TrimSpace(opt.Name)
		if !slashCommandNameRegex.MatchString(oname) {
			return fmt.Errorf("Option name %q must be 1-32 characters (letters, numbers, dashes, "+
				"underscores)", opt.Name)
		}
		if oname != strings.ToLower(oname) {
			return fmt.Errorf("Option name %q must be lowercase", opt.Name)
		}
		if seenOptions[oname] {
			return fmt.Errorf("Duplicate option name %q", oname)
		}
		seenOptions[oname] = true
		if l := utf8.RuneCountInString(opt.Description); l < 1 || l > maxSlashCommandDescription {
			return fmt.Errorf("Description for option %q must be between 1 and %d characters", oname,
				maxSlashCommandDescription)
		}
	}
	return nil
}

// validateContextMenuHeader is the panel's check of a context menu command's name
// (validateContextMenuData, customcommands.go:553-557) and defer mode (:538-540).
func validateContextMenuHeader(source string, t Trigger) error {
	if !contextMenuNameRegex.MatchString(strings.TrimSpace(t.Text)) {
		return fmt.Errorf("Context menu command name must be 1-32 characters and contain only letters, " +
			"numbers, spaces, dashes and underscores")
	}
	if ReadDeferMode(source) == DeferModeUpdate {
		return fmt.Errorf("\"Update message\" defer mode is not valid for context menu commands")
	}
	return nil
}

// SlashInvocation is a slash command as a test invokes it: the subcommand chosen (""
// without one) and the options given, by name, as the test wrote them (the header's
// types resolve them: see SetInteractionSlash).
type SlashInvocation struct {
	Subcommand string
	Options    map[string]interface{}
}

// SlashTrigger is what the invocation gave the slash handler (handle_slashcommand.go:
// 116-164): the command's name, the subcommand, the options by name and the ordered
// arguments (the name first).
type SlashTrigger struct {
	CommandName string
	SubCommand  string
	Options     types.SDict
	Args        []interface{}
}

// setData sets the handler's data keys (handle_slashcommand.go:118-164).
func (s *SlashTrigger) setData(data map[string]interface{}) {
	data["IsSlashCommand"] = true
	data["CommandName"] = s.CommandName
	data["Cmd"] = s.CommandName
	data["SubCommand"] = s.SubCommand
	data["Options"] = s.Options
	data["Args"] = s.Args
	data["CmdArgs"] = s.Args[1:]
}

// slashMessage is a slash run's .Message (handle_slashcommand.go:166-173): application
// command interactions carry no source message, so the handler builds a blank one with
// the invoker as member and author (ID 0).
func (ctx *ExecutionContext) slashMessage() types.CtxMessage {
	member := ctx.member(ctx.UserID)
	return types.CtxMessage{GuildID: ctx.GuildID, ChannelID: ctx.ChannelID, Member: &member,
		Author: member.User}
}

// applicationCommandInteractionID is the ID of a slash or context menu interaction: any
// ID clear of the messages tests declare and the emulator sends (Discord's are snowflakes).
const applicationCommandInteractionID = 1_000_000_000_000_000_000

// SetInteractionSlash makes the run answer a slash command invocation of the trigger t,
// whose options def the header declares: the subcommand must be one the header names
// (and given exactly when the command has subcommands), every required option must be
// given, and each value must fit its type as Discord would send it (structs.go:
// 1708-1750): a string, an integer, a number, true/false, or the ID of a user, a declared
// channel or a declared role (mentionable: a role's, else a user's). The handler's values
// (handle_slashcommand.go:181-238) then come from what Discord resolved. The panel's defer
// mode is applied as deferResponseToCCs does before the run (handle_component.go:161-190).
func (ctx *ExecutionContext) SetInteractionSlash(t Trigger, def SlashCommandDef, inv SlashInvocation,
	mode DeferMode) error {
	if !t.SlashTriggered() {
		return fmt.Errorf("a slash invocation needs a Slash Command trigger; the template's is %q", t.Type)
	}
	name := strings.ToLower(strings.TrimSpace(t.Text)) // registered lowercased (:275, :305)

	// The subcommand chosen: the first option, of type SUB_COMMAND, with the leaf options
	// nested under it (:130-140)
	defs := def.Options
	var provided []*types.ApplicationCommandInteractionDataOption
	subName := ""
	switch {
	case len(def.Subcommands) > 0 && inv.Subcommand == "":
		return fmt.Errorf("the slash command %q has subcommands; give interaction subcommand: "+
			"(%s)", name, subcommandNames(def))
	case len(def.Subcommands) > 0:
		found := false
		for _, s := range def.Subcommands {
			if strings.EqualFold(s.Name, inv.Subcommand) {
				subName, defs, found = s.Name, s.Options, true
				break
			}
		}
		if !found {
			return fmt.Errorf("interaction subcommand %q isn't one the header declares (%s)",
				inv.Subcommand, subcommandNames(def))
		}
	case inv.Subcommand != "":
		return fmt.Errorf("interaction subcommand %q: the slash command %q has no subcommands",
			inv.Subcommand, name)
	}

	// Option names are matched lowercased, as the handler keys what Discord sent (:143-145)
	resolved := &types.ApplicationCommandInteractionDataResolved{}
	given := make(map[string]interface{}, len(inv.Options))
	for optName, raw := range inv.Options {
		lower := strings.ToLower(optName)
		if _, dup := given[lower]; dup {
			return fmt.Errorf("interaction option %q is given twice (names are matched in any case)", lower)
		}
		given[lower] = raw
		known := false
		for _, d := range defs {
			if strings.ToLower(d.Name) == lower {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("interaction option %q isn't one the header declares%s", optName,
				forSubcommand(subName))
		}
	}
	for _, d := range defs {
		raw, ok := given[strings.ToLower(d.Name)]
		if !ok {
			if d.Required {
				return fmt.Errorf("interaction options: the required option %q (%s) wasn't given%s",
					d.Name, d.FormType, forSubcommand(subName))
			}
			continue
		}
		value, err := ctx.slashOptionValue(d, raw, resolved)
		if err != nil {
			return fmt.Errorf("interaction option %q: %w", d.Name, err)
		}
		provided = append(provided, &types.ApplicationCommandInteractionDataOption{
			Name: d.Name, Type: d.Type, Value: value})
	}

	data := types.ApplicationCommandInteractionData{
		ID:          applicationCommandInteractionID - 1,
		Name:        name,
		CommandType: types.ChatApplicationCommand,
		Resolved:    resolved,
		GuildID:     ctx.GuildID,
		Options:     provided,
	}
	if subName != "" {
		data.Options = []*types.ApplicationCommandInteractionDataOption{{Name: subName,
			Type: types.ApplicationCommandOptionSubCommand, Options: provided}}
	}

	member := ctx.member(ctx.UserID)
	ctx.Interaction = &types.CustomCommandInteraction{Interaction: &types.Interaction{
		ID:          applicationCommandInteractionID,
		Type:        types.InteractionApplicationCommand,
		Data:        data,
		DataCommand: &data,
		GuildID:     ctx.GuildID,
		ChannelID:   ctx.ChannelID,
		Member:      &member,
		Token:       interactionToken(applicationCommandInteractionID),
		Version:     1,
	}}

	// .Options by name and .Args in definition order, absent optionals skipped (:143-164)
	options := types.SDict{}
	args := make([]interface{}, 0, len(defs)+1)
	args = append(args, data.Name)
	for _, d := range defs {
		var o *types.ApplicationCommandInteractionDataOption
		for _, p := range provided {
			if strings.EqualFold(p.Name, d.Name) {
				o = p
				break
			}
		}
		if o == nil {
			// optional option not provided by the user
			continue
		}
		val := resolveSlashOptionValue(o, &data)
		options[d.Name] = val
		args = append(args, val)
	}
	ctx.Slash = &SlashTrigger{CommandName: data.Name, SubCommand: subName, Options: options, Args: args}

	ctx.deferMode = mode
	if mode != DeferModeNone {
		ctx.Interaction.RespondedTo = true
		ctx.Interaction.Deferred = true
	}
	return nil
}

func subcommandNames(def SlashCommandDef) string {
	names := make([]string, len(def.Subcommands))
	for i, s := range def.Subcommands {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}

func forSubcommand(sub string) string {
	if sub == "" {
		return ""
	}
	return fmt.Sprintf(" for subcommand %q", sub)
}

// slashOptionValue is the option's value as Discord sends it for a test's raw value
// (structs.go:1708-1750), resolving a snowflake option's target into resolved as
// Discord does (a user as the emulator's getMember gives it, a declared channel, a
// declared role). A value of the wrong shape, or a snowflake nothing declares, is an
// error: Discord only ever sends what the option type allows.
func (ctx *ExecutionContext) slashOptionValue(d SlashOption, raw interface{},
	resolved *types.ApplicationCommandInteractionDataResolved) (interface{}, error) {
	switch d.Type {
	case types.ApplicationCommandOptionString:
		if s, ok := raw.(string); ok {
			return s, nil
		}
		return nil, fmt.Errorf("%s takes a string, not %s", d.FormType, describeRaw(raw))
	case types.ApplicationCommandOptionInteger:
		if n, ok := rawInt(raw); ok {
			return n, nil
		}
		return nil, fmt.Errorf("%s takes a whole number, not %s", d.FormType, describeRaw(raw))
	case types.ApplicationCommandOptionNumber:
		switch v := raw.(type) {
		case float64:
			return v, nil
		case float32:
			return float64(v), nil
		}
		if n, ok := rawInt(raw); ok {
			return float64(n), nil
		}
		return nil, fmt.Errorf("%s takes a number, not %s", d.FormType, describeRaw(raw))
	case types.ApplicationCommandOptionBoolean:
		if b, ok := raw.(bool); ok {
			return b, nil
		}
		return nil, fmt.Errorf("%s takes true or false, not %s", d.FormType, describeRaw(raw))
	}

	// The snowflake types: the ID, resolved as Discord resolves it
	id, ok := rawInt(raw)
	if !ok || id <= 0 {
		return nil, fmt.Errorf("%s takes an ID, not %s", d.FormType, describeRaw(raw))
	}
	switch d.Type {
	case types.ApplicationCommandOptionUser:
		ctx.resolveUser(resolved, id)
	case types.ApplicationCommandOptionChannel:
		ch, ok := ctx.declaredChannel(id)
		if !ok {
			return nil, fmt.Errorf("channel %d isn't the run's channel or one in guild.channels", id)
		}
		if resolved.Channels == nil {
			resolved.Channels = map[int64]*types.CtxChannel{}
		}
		resolved.Channels[id] = &ch
	case types.ApplicationCommandOptionRole:
		role, ok := ctx.AvailableRoles[id]
		if !ok {
			return nil, fmt.Errorf("role %d isn't in guild.roles", id)
		}
		if resolved.Roles == nil {
			resolved.Roles = map[int64]*types.CtxRole{}
		}
		resolved.Roles[id] = &role
	case types.ApplicationCommandOptionMentionable:
		// a role when the ID is one, else a user (resolveSlashOptionValue :222-231)
		if role, ok := ctx.AvailableRoles[id]; ok {
			if resolved.Roles == nil {
				resolved.Roles = map[int64]*types.CtxRole{}
			}
			resolved.Roles[id] = &role
		} else {
			ctx.resolveUser(resolved, id)
		}
	}
	return id, nil
}

// resolveUser puts a user (and, for a member, their member) into resolved, as Discord
// sends both for a user option's target.
func (ctx *ExecutionContext) resolveUser(resolved *types.ApplicationCommandInteractionDataResolved,
	id int64) {
	member := ctx.member(id)
	user := member.User
	if resolved.Users == nil {
		resolved.Users = map[int64]*types.DiscordUser{}
		resolved.Members = map[int64]*types.CtxMember{}
	}
	resolved.Users[id] = &user
	if ctx.isMember(id) {
		resolved.Members[id] = &member
	}
}

// declaredChannel is a channel a slash option can name: the run's own, or one of
// guild.channels (a thread included); the declared name and details.
func (ctx *ExecutionContext) declaredChannel(id int64) (types.CtxChannel, bool) {
	if id == ctx.ChannelID {
		return ctx.channelState(id), true
	}
	if _, ok := ctx.ChannelDetails[id]; ok {
		return ctx.channelState(id), true
	}
	return types.CtxChannel{}, false
}

// rawInt is a YAML integer (or a float with no fraction, which YAML never produces for a
// written integer) as int64.
func rawInt(raw interface{}) (int64, bool) {
	switch v := raw.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case uint64:
		return int64(v), true
	}
	return 0, false
}

// describeRaw names a test's value in an error.
func describeRaw(raw interface{}) string {
	switch v := raw.(type) {
	case string:
		return fmt.Sprintf("the string %q", v)
	case bool:
		return fmt.Sprintf("%v", v)
	case nil:
		return "nothing"
	}
	return fmt.Sprintf("%v (%T)", raw, raw)
}

// resolveSlashOptionValue is YAGPDB's (handle_slashcommand.go:181-238): the value a
// template sees for an option, snowflake types resolved against what Discord sent (a
// bare {ID} when it didn't resolve one; the emulator always has).
func resolveSlashOptionValue(o *types.ApplicationCommandInteractionDataOption,
	data *types.ApplicationCommandInteractionData) interface{} {
	switch o.Type {
	case types.ApplicationCommandOptionString:
		if v, ok := o.Value.(string); ok {
			return v
		}
	case types.ApplicationCommandOptionInteger:
		if v, ok := o.Value.(int64); ok {
			return v
		}
	case types.ApplicationCommandOptionNumber:
		if v, ok := o.Value.(float64); ok {
			return v
		}
	case types.ApplicationCommandOptionBoolean:
		if v, ok := o.Value.(bool); ok {
			return v
		}
	case types.ApplicationCommandOptionUser:
		id, _ := o.Value.(int64)
		return resolveSlashUser(data, id)
	case types.ApplicationCommandOptionChannel:
		id, _ := o.Value.(int64)
		if data.Resolved != nil {
			if c, ok := data.Resolved.Channels[id]; ok {
				return c
			}
		}
		return &types.CtxChannel{ID: id}
	case types.ApplicationCommandOptionRole:
		id, _ := o.Value.(int64)
		if data.Resolved != nil {
			if r, ok := data.Resolved.Roles[id]; ok {
				return r
			}
		}
		return &types.CtxRole{ID: id}
	case types.ApplicationCommandOptionMentionable:
		id, _ := o.Value.(int64)
		if data.Resolved != nil {
			if r, ok := data.Resolved.Roles[id]; ok {
				return r
			}
		}
		return resolveSlashUser(data, id)
	}

	return o.Value
}

// resolveSlashUser is YAGPDB's (handle_slashcommand.go:240-247).
func resolveSlashUser(data *types.ApplicationCommandInteractionData, id int64) *types.DiscordUser {
	if data.Resolved != nil {
		if u, ok := data.Resolved.Users[id]; ok {
			return u
		}
	}
	return &types.DiscordUser{ID: id}
}

// ContextMenuTarget is what a context menu entry was used on, as a test declares it: a
// user (the user menu) or a message of the run's channel, declared in messages: (the
// message menu).
type ContextMenuTarget struct {
	UserID    int64
	MessageID int64
}

// ContextMenuTrigger is what the click gave the context menu handler
// (handle_contextmenu.go:121-159): the entry's name, "user" or "message", the invoker
// as .Author, the target's user and member (nil when they aren't a member), and, for the
// message menu, the clicked message. TargetMember is the MemberState shape, as
// bot.GetMember returns it.
type ContextMenuTrigger struct {
	CommandName  string
	CommandType  string
	Author       *types.DiscordUser
	TargetUser   *types.DiscordUser
	TargetMember *types.CtxTargetMember
	Message      *types.CtxMessage
}

// setData sets the handler's data keys (handle_contextmenu.go:130-155). .User/.Member
// aren't set: the handler passes a nil member (:123; context.go:366-369), so the run has
// NoMember. TargetMember is the pointer bot.GetMember returns: nil for a non-member.
func (c *ContextMenuTrigger) setData(data map[string]interface{}) {
	data["IsContextMenuCommand"] = true
	data["CommandName"] = c.CommandName
	data["Cmd"] = c.CommandName
	data["Author"] = c.Author
	data["CommandType"] = c.CommandType
	if c.TargetUser != nil {
		data["TargetUser"] = c.TargetUser
		data["TargetMember"] = c.TargetMember
	}
	if c.Message != nil {
		data["Message"] = *c.Message
	}
}

// SetInteractionContextMenu makes the run answer a use of the context menu entry t (a
// User Context Menu or Message Context Menu trigger; the entry's name is the trigger,
// matched trimmed, any case: handle_contextmenu.go:82) on target. The run has no member
// (.User/.Member unset, sendDM disabled: :118-120) and, for the user menu, no .Message
// (the handler sets none; an execCC child gets the bot's blank one, context.go:427-433).
// The message menu's target must be a message the test declares in the run's channel;
// it becomes .Message, and its author the target. The panel's defer mode is applied as
// deferResponseToCCs does before the run (handle_component.go:161-190).
func (ctx *ExecutionContext) SetInteractionContextMenu(t Trigger, target ContextMenuTarget,
	mode DeferMode) error {
	cmdType, ok := t.ContextMenuTriggered()
	if !ok {
		return fmt.Errorf("a context menu use needs a User Context Menu or Message Context Menu trigger; "+
			"the template's is %q", t.Type)
	}
	name := strings.TrimSpace(t.Text) // registered trimmed (:171)
	invoker := ctx.member(ctx.UserID)
	author := invoker.User
	trigger := &ContextMenuTrigger{CommandName: name, Author: &author}
	data := types.ApplicationCommandInteractionData{
		ID:          applicationCommandInteractionID - 1,
		Name:        name,
		CommandType: cmdType,
		Resolved:    &types.ApplicationCommandInteractionDataResolved{},
		GuildID:     ctx.GuildID,
	}

	switch cmdType {
	case types.UserApplicationCommand:
		if target.UserID == 0 {
			return fmt.Errorf("a user context menu use needs its target user (interaction target:)")
		}
		trigger.CommandType = "user"
		data.TargetID = target.UserID
		ctx.resolveUser(data.Resolved, target.UserID)
		trigger.TargetUser = resolveSlashUser(&data, target.UserID)
		trigger.TargetMember = ctx.targetMember(target.UserID)
		ctx.NoMessage = true
	case types.MessageApplicationCommand:
		var msg *types.CtxMessage
		for i := range ctx.Messages {
			if m := &ctx.Messages[i]; m.ID == target.MessageID && m.ChannelID == ctx.ChannelID {
				msg = m
				break
			}
		}
		if msg == nil {
			return fmt.Errorf("interaction message_id %d isn't a message in channel %d: declare it in "+
				"messages: with that channel_id", target.MessageID, ctx.ChannelID)
		}
		trigger.CommandType = "message"
		data.TargetID = target.MessageID
		clicked := *msg // its own copy, as the resolved data is Discord's
		clicked.GuildID = ctx.GuildID
		data.Resolved.Messages = map[int64]*types.CtxMessage{clicked.ID: &clicked}
		trigger.Message = &clicked
		targetUser := clicked.Author
		trigger.TargetUser = &targetUser
		trigger.TargetMember = ctx.targetMember(targetUser.ID)
	}

	ctx.Interaction = &types.CustomCommandInteraction{Interaction: &types.Interaction{
		ID:          applicationCommandInteractionID,
		Type:        types.InteractionApplicationCommand,
		Data:        data,
		DataCommand: &data,
		GuildID:     ctx.GuildID,
		ChannelID:   ctx.ChannelID,
		Member:      &invoker,
		Token:       interactionToken(applicationCommandInteractionID),
		Version:     1,
	}}
	ctx.ContextMenu = trigger
	ctx.NoMember = true

	ctx.deferMode = mode
	if mode != DeferModeNone {
		ctx.Interaction.RespondedTo = true
		ctx.Interaction.Deferred = true
	}
	return nil
}

// targetMember is bot.GetMember's result for a context menu target: the member as a
// dstate.MemberState (its member fields under .Member, always set for a found member —
// the fetch only returns members with Member != nil, bot/memberfetcher.go:26), or nil
// when the user isn't in the server.
func (ctx *ExecutionContext) targetMember(userID int64) *types.CtxTargetMember {
	if !ctx.isMember(userID) {
		return nil
	}
	m := ctx.member(userID)
	return &types.CtxTargetMember{
		User:    m.User,
		GuildID: ctx.GuildID,
		Member: &types.CtxMemberFields{
			JoinedAt: m.JoinedAt,
			Roles:    m.Roles,
			Nick:     m.Nick,
		},
	}
}
