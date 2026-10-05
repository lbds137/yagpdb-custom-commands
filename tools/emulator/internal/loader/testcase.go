// Package loader provides test case loading and parsing for the YAGPDB emulator.
package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/runtime"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// TestCase represents a single test definition.
type TestCase struct {
	Name           string     `yaml:"name"`
	Template       string     `yaml:"template"`        // Path to template file
	TemplateSource string     `yaml:"template_source"` // Inline template source
	Context        ContextDef `yaml:"context"`
	SetupDB        []DBEntry  `yaml:"setup_db"`
	// SetupTemplates run in order before the test, on the same database (e.g. a bootstrap)
	SetupTemplates []string         `yaml:"setup_templates"`
	CommandMap     map[int64]string `yaml:"command_map"` // Maps command IDs to template paths
	Expected       ExpectedResult   `yaml:"expected"`
	Assertions     Assertions       `yaml:"assertions"`
	Strict         bool             `yaml:"strict"`   // Fail on YAGPDB execution limits (like -strict)
	Snapshot       bool             `yaml:"snapshot"` // Compare results with the saved snapshot

	SourceFile string `yaml:"-"` // YAML file the test came from (for snapshots)
}

// TestClock is a test's `clock:`, a YAML timestamp (2026-01-02T15:04:05Z).
type TestClock time.Time

func (c *TestClock) UnmarshalYAML(node *yaml.Node) error {
	const example = "write a time like 2026-01-02T15:04:05Z"
	// node.Decode doesn't check fields, so a mapping would decode as the zero time
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: clock: %s", node.Line, example)
	}
	var t time.Time
	if err := node.Decode(&t); err != nil {
		return fmt.Errorf("line %d: clock: %v (%s)", node.Line, err, example)
	}
	*c = TestClock(t)
	return nil
}

// TestSeed is a test's `seed:`, a whole number.
type TestSeed int64

func (s *TestSeed) UnmarshalYAML(node *yaml.Node) error {
	var n int64
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: seed: write an integer, like 7", node.Line)
	}
	if node.Tag != "!!int" || node.Decode(&n) != nil {
		return fmt.Errorf("line %d: seed: %q isn't an integer that fits in 64 bits (write one like 7)", node.Line, node.Value)
	}
	*s = TestSeed(n)
	return nil
}

// ContextDef defines the execution context for a test.
type ContextDef struct {
	User    UserDef    `yaml:"user"`
	Channel ChannelDef `yaml:"channel"`
	Guild   GuildDef   `yaml:"guild"`
	// Args are the arguments after the trigger; the message is the trigger followed by them
	Args     []string               `yaml:"args"`
	ExecData map[string]interface{} `yaml:"exec_data"`
	Premium  *bool                  `yaml:"premium"` // Default true
	// Clock stops the run's clock at this time (currentTime, timestamps, database entry
	// times), so a snapshot can hold them; unset, it's the system clock
	Clock *TestClock `yaml:"clock"`
	// Seed seeds randInt, shuffle, adjective, noun and verb; unset, they're random
	Seed     *TestSeed    `yaml:"seed"`
	Reaction *ReactionDef `yaml:"reaction"` // Makes this a reaction-triggered run
	// Interaction makes this a run answering an interaction: a click on a button or menu
	// of a message in messages: (a Message Component trigger), a slash command (a Slash
	// Command trigger) or a context menu entry (a User/Message Context Menu trigger)
	Interaction *InteractionDef `yaml:"interaction"`
	Messages    []MessageDef    `yaml:"messages"` // Messages getMessage can find
	// MessageContent is the whole triggering message, trigger included (instead of args);
	// with exec_data or reaction, the message .Message is
	MessageContent string  `yaml:"message_content"`
	Members        []int64 `yaml:"members"` // If set, the only users getMember finds
	// MemberRoles gives other members' roles (the triggering user's are user.roles)
	MemberRoles map[int64][]int64 `yaml:"member_roles"`
	// MemberNicks gives members' nicknames, the triggering user's included
	MemberNicks map[int64]string `yaml:"member_nicks"`
	// MemberJoinedAgo gives how long before the run members joined, like "12h" (default 30 days)
	MemberJoinedAgo map[int64]Duration `yaml:"member_joined_ago"`
	// ExecResponses declares what exec/execAdmin return for a given command line (the line
	// an execs: assertion takes); a call whose line isn't a key returns "" and warns, since
	// the emulator can't run the bot command YAGPDB would
	ExecResponses map[string]string `yaml:"exec_responses"`
}

// MessageDef is an existing Discord message.
type MessageDef struct {
	ID        int64  `yaml:"id"`
	ChannelID int64  `yaml:"channel_id"`
	AuthorID  int64  `yaml:"author_id"`
	Content   string `yaml:"content"`
	// Embeds are written as cembed's keys (title, description, fields: [{name, value}],
	// author: {name}, image: {url}...); templates read them as discordgo's
	// (.Title, .Author.Name)
	Embeds []TestEmbed `yaml:"embeds"`
}

// TestEmbed is an embed of a test message, converted as cembed converts its dict.
type TestEmbed struct{ *types.MessageEmbed }

func (e *TestEmbed) UnmarshalYAML(node *yaml.Node) error {
	var dict map[string]interface{}
	if err := node.Decode(&dict); err != nil {
		return fmt.Errorf("line %d: embeds: write each embed as a mapping of cembed's keys: %v", node.Line, err)
	}
	// cembed drops a key it doesn't know; in a test that is a typo quietly weakening it
	if err := checkEmbedKeys(dict, embedKeys, ""); err != nil {
		return fmt.Errorf("line %d: embeds: %v", node.Line, err)
	}
	embed, err := types.BuildEmbed(dict)
	if err != nil {
		return fmt.Errorf("line %d: embeds: %v", node.Line, err)
	}
	e.MessageEmbed = types.EmbedStruct(embed)
	return nil
}

// embedKeys are discordgo.MessageEmbed's JSON keys, and embedPartKeys its parts'.
var (
	embedKeys = map[string]bool{
		"url": true, "type": true, "title": true, "description": true, "timestamp": true,
		"color": true, "footer": true, "image": true, "thumbnail": true, "video": true,
		"provider": true, "author": true, "fields": true,
	}
	mediaKeys     = map[string]bool{"url": true, "proxy_url": true, "width": true, "height": true}
	embedPartKeys = map[string]map[string]bool{
		"footer":    {"text": true, "icon_url": true, "proxy_icon_url": true},
		"image":     mediaKeys,
		"thumbnail": mediaKeys,
		"video":     mediaKeys,
		"provider":  {"url": true, "name": true},
		"author":    {"url": true, "name": true, "icon_url": true, "proxy_icon_url": true},
		"fields":    {"name": true, "value": true, "inline": true},
	}
)

// checkEmbedKeys refuses a key of dict that isn't in known, and checks each part's keys.
func checkEmbedKeys(dict map[string]interface{}, known map[string]bool, where string) error {
	for key, val := range dict {
		if !known[key] {
			return fmt.Errorf("%q isn't one of cembed's keys%s", key, where)
		}
		part := embedPartKeys[key]
		if part == nil || where != "" {
			continue
		}
		items := []interface{}{val}
		if list, ok := val.([]interface{}); ok {
			items = list
		}
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				if err := checkEmbedKeys(m, part, " (in "+key+")"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ReactionDef describes the reaction that triggered a command.
type ReactionDef struct {
	Emoji     string `yaml:"emoji"`      // Unicode emoji, or a custom emoji's name
	EmojiID   int64  `yaml:"emoji_id"`   // Custom emoji ID (0 for Unicode emoji)
	MessageID int64  `yaml:"message_id"` // Message that was reacted to
	Added     *bool  `yaml:"added"`      // false for a removed reaction (default true)
}

// InteractionDef describes the interaction that triggered a command, by type: a
// component click (component), a slash command invocation (slash), a use of a user
// context menu entry (user_menu) or of a message context menu entry (message_menu), or a
// modal submission (modal).
type InteractionDef struct {
	// "component", "slash", "user_menu", "message_menu" or "modal"
	Type string `yaml:"type"`
	// CustomID is the clicked component's (or the submitted modal's) custom ID as the
	// command wrote it (the templates- prefix YAGPDB adds is implied; giving it is fine too)
	CustomID string `yaml:"custom_id"`
	// MessageID is the message clicked (component), the message the entry was used on
	// (message_menu) or the message whose component opened the modal (modal, optional),
	// in messages: (in the run's channel)
	MessageID int64 `yaml:"message_id"`
	// Fields are a modal's submitted text inputs, in the modal's order (.Values keeps
	// it): a mapping of custom ID to the text entered
	Fields ModalFields `yaml:"fields"`
	// Form is how a modal's fields come back: action_row (the default: a cmodal "fields"
	// modal) or label (a modal of clabels: modalBuilder, cmodal "components")
	Form   string   `yaml:"form"`
	Values []string `yaml:"values"` // a menu's chosen values (.Values)
	// Component is the clicked component's kind: button (the default), string_menu,
	// user_menu, role_menu, mentionable_menu or channel_menu
	Component string `yaml:"component"`
	// Subcommand is the slash subcommand chosen (a Slash subcommand header line), when
	// the command has them
	Subcommand string `yaml:"subcommand"`
	// Options are the slash options given, by name, typed by their header line: a string,
	// a whole number (integer), a number, true/false (boolean), or an ID (user, channel:
	// the run's or a guild.channels entry, role: a guild.roles entry, mentionable: a role
	// else a user); a required one can't be left out
	Options map[string]interface{} `yaml:"options"`
	// Target is the user the user context menu entry was used on (.TargetUser)
	Target int64 `yaml:"target"`
	// Delayed makes this the delayed half of a slash command (type: slash): a run that
	// execCC with a delay or scheduleUniqueCC scheduled, which needs exec_data. It gets
	// the stored `.Interaction` and `.ExecData` but not .IsSlashCommand, .SubCommand or
	// .Options (vendor customcommands/handle_timed.go:102-117), and no subcommand or
	// options: of every interaction + exec_data combination only this one is allowed.
	Delayed bool `yaml:"delayed"`
	// RespondedTo is the stored interaction's RespondedTo when the run was scheduled
	// (delayed only): true makes the run's response a followup, false (the default) its
	// first response
	RespondedTo bool `yaml:"responded_to"`
}

// componentKinds are InteractionDef.Component's values and their component types.
var componentKinds = map[string]types.ComponentType{
	"":                 types.ButtonComponent,
	"button":           types.ButtonComponent,
	"string_menu":      types.SelectMenuComponent,
	"user_menu":        types.UserSelectMenuComponent,
	"role_menu":        types.RoleSelectMenuComponent,
	"mentionable_menu": types.MentionableSelectMenuComponent,
	"channel_menu":     types.ChannelSelectMenuComponent,
}

// validate rejects an interaction the emulator can't run: an unmodelled type, a field
// another type takes, or a missing one.
func (d *InteractionDef) validate() error {
	if d.RespondedTo && !d.Delayed {
		return fmt.Errorf("interaction responded_to is only for a delayed run (delayed: true)")
	}
	if d.Delayed {
		if d.Type != "slash" {
			return fmt.Errorf("interaction delayed models a delayed slash command run: write type: slash")
		}
		// the restored interaction has no subcommand or options of its own to give: the
		// handler's keys aren't restored (handle_timed.go:102-117)
		if d.Subcommand != "" || d.Options != nil {
			return fmt.Errorf("interaction delayed takes no subcommand or options (a delayed run " +
				"has only .Interaction and .ExecData; give the data in exec_data)")
		}
		if err := d.onlyFields("slash delayed", "responded_to"); err != nil {
			return err
		}
		return nil
	}
	switch d.Type {
	case "component":
		if err := d.onlyFields("component", "custom_id, message_id, values, component"); err != nil {
			return err
		}
		kind, ok := componentKinds[d.Component]
		if !ok {
			return fmt.Errorf("interaction component %q isn't button, string_menu, user_menu, role_menu, mentionable_menu or channel_menu", d.Component)
		}
		if d.CustomID == "" {
			return fmt.Errorf("interaction needs the custom_id clicked")
		}
		if d.MessageID == 0 {
			return fmt.Errorf("interaction needs the message_id clicked (a messages: entry)")
		}
		if kind == types.ButtonComponent && len(d.Values) > 0 {
			return fmt.Errorf("interaction values need a menu component (component: string_menu ...); a button has none")
		}
	case "slash":
		if err := d.onlyFields("slash", "subcommand, options"); err != nil {
			return err
		}
		for name, v := range d.Options {
			switch v.(type) {
			case string, int, int64, float64, bool:
			default:
				return fmt.Errorf("interaction option %q: write a string, a number, true/false or an ID, not %T", name, v)
			}
		}
	case "user_menu":
		if err := d.onlyFields("user_menu", "target"); err != nil {
			return err
		}
		if d.Target == 0 {
			return fmt.Errorf("interaction needs the target user's ID (target:)")
		}
	case "message_menu":
		if err := d.onlyFields("message_menu", "message_id"); err != nil {
			return err
		}
		if d.MessageID == 0 {
			return fmt.Errorf("interaction needs the message_id the entry was used on (a messages: entry)")
		}
	case "modal":
		if err := d.onlyFields("modal", "custom_id, message_id, fields, form"); err != nil {
			return err
		}
		if d.CustomID == "" {
			return fmt.Errorf("interaction needs the custom_id of the modal submitted")
		}
		// A modal has 1 to 5 top-level components (Discord's limit, ModalBuilder's cap)
		if len(d.Fields) < 1 || len(d.Fields) > 5 {
			return fmt.Errorf("interaction fields: a modal submits 1 to 5 fields, not %d", len(d.Fields))
		}
		// Discord's custom IDs are 1+ characters and unique within the modal, as the
		// command wrote them (with the templates- prefix Discord sends back)
		seen := map[string]string{}
		for _, f := range d.Fields {
			full := runtime.WithTemplatePrefix(f.CustomID)
			if full == runtime.TemplateCustomIDPrefix {
				return fmt.Errorf("interaction fields: a custom ID can't be empty (%q)", f.CustomID)
			}
			if first, dup := seen[full]; dup {
				return fmt.Errorf("interaction fields: %q and %q are the same custom ID (%s)",
					first, f.CustomID, full)
			}
			seen[full] = f.CustomID
		}
		switch d.Form {
		case "", runtime.ModalFormActionRow, runtime.ModalFormLabel:
		default:
			return fmt.Errorf("interaction form %q isn't action_row or label", d.Form)
		}
	default:
		return fmt.Errorf("interaction type %q: write type: component, slash, user_menu, message_menu or modal", d.Type)
	}
	return nil
}

// ModalFields are a modal submission's fields in the order the test writes them, from a
// YAML mapping of custom ID to text (`fields: { rule_text: "new text", reason: "typo" }`);
// a plain map would lose the order .Values has.
type ModalFields []runtime.ModalField

// UnmarshalYAML reads the mapping in order: each key a field's custom ID, each value the
// text entered, a scalar. (yaml.v3 itself refuses a key given twice.)
func (f *ModalFields) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: interaction fields: write a mapping of custom ID to text "+
			"(fields: { rule_text: \"new text\" })", node.Line)
	}
	out := ModalFields{}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode || value.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: interaction fields: each entry is a custom ID and its text", key.Line)
		}
		out = append(out, runtime.ModalField{CustomID: key.Value, Value: value.Value})
	}
	*f = out
	return nil
}

// onlyFields rejects a field of another interaction type, so a typo can't quietly
// weaken a test; allowed lists the type's own for the message.
func (d *InteractionDef) onlyFields(typ, allowed string) error {
	set := map[string]bool{
		"custom_id":  d.CustomID != "",
		"message_id": d.MessageID != 0,
		"values":     len(d.Values) > 0,
		"component":  d.Component != "",
		"subcommand": d.Subcommand != "",
		"options":    d.Options != nil,
		"target":     d.Target != 0,
		"fields":     d.Fields != nil,
		"form":       d.Form != "",
	}
	for _, f := range strings.Split(allowed, ", ") {
		delete(set, f)
	}
	for _, f := range []string{"custom_id", "message_id", "values", "component", "subcommand", "options", "target",
		"fields", "form"} {
		if set[f] {
			return fmt.Errorf("interaction %s isn't a type: %s field (it takes %s)", f, typ, allowed)
		}
	}
	return nil
}

// click is the interaction as the runtime takes it.
func (d *InteractionDef) click() runtime.ComponentClick {
	return runtime.ComponentClick{CustomID: d.CustomID, MessageID: d.MessageID,
		ComponentType: componentKinds[d.Component], Values: d.Values}
}

// slash is the invocation as the runtime takes it.
func (d *InteractionDef) slash() runtime.SlashInvocation {
	return runtime.SlashInvocation{Subcommand: d.Subcommand, Options: d.Options}
}

// modal is the submission as the runtime takes it.
func (d *InteractionDef) modal() runtime.ModalSubmission {
	return runtime.ModalSubmission{CustomID: d.CustomID, Fields: d.Fields, MessageID: d.MessageID,
		Form: d.Form}
}

// contextMenuTarget is the target as the runtime takes it.
func (d *InteractionDef) contextMenuTarget() runtime.ContextMenuTarget {
	return runtime.ContextMenuTarget{UserID: d.Target, MessageID: d.MessageID}
}

// UserDef defines user context.
type UserDef struct {
	ID            int64   `yaml:"id"`
	Username      string  `yaml:"username"`
	Discriminator string  `yaml:"discriminator"`
	Roles         []int64 `yaml:"roles"`
}

// ChannelDef defines channel context.
type ChannelDef struct {
	ID   int64  `yaml:"id"`
	Name string `yaml:"name"`
	// Type is Discord's channel type: 0 text (the default), 2 voice, 4 category,
	// 5 announcement, 15 forum
	Type     int    `yaml:"type"`
	ParentID int64  `yaml:"parent_id"` // the category it's in
	Position int    `yaml:"position"`  // .Guild.Channels is sorted by position
	Topic    string `yaml:"topic"`
	NSFW     bool   `yaml:"nsfw"`
	// AvailableTags are a forum channel's tags, which a createForumPost "tags" argument
	// can apply, by name or by ID (meaningful on type 15 channels only)
	AvailableTags []TagDef `yaml:"available_tags"`
	// BotCannotSend makes a send to this channel fail as Discord refuses it (403 Missing
	// Permissions), instead of being recorded
	BotCannotSend bool `yaml:"bot_cannot_send"`
	// PermissionOverwrites are the channel's allow/deny bits for roles and members, which
	// getTargetPermissionsIn applies as YAGPDB does (a thread uses its parent's)
	PermissionOverwrites []OverwriteDef `yaml:"permission_overwrites"`
}

// OverwriteDef is discordgo's PermissionOverwrite: id is a role's (the guild's ID is
// @everyone) or a member's, type is "role" (the default) or "member", allow and deny are
// permission bit masks (View Channel is 1024).
type OverwriteDef struct {
	ID    int64         `yaml:"id"`
	Type  OverwriteType `yaml:"type"`
	Allow int64         `yaml:"allow"`
	Deny  int64         `yaml:"deny"`
}

// OverwriteType is an overwrite's `type:`, "role" (the default) or "member", read as
// discordgo's PermissionOverwriteType value (0 or 1).
type OverwriteType int

func (o *OverwriteType) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		switch node.Value {
		case "role":
			*o = types.PermissionOverwriteTypeRole
			return nil
		case "member":
			*o = types.PermissionOverwriteTypeMember
			return nil
		}
	}
	return fmt.Errorf(`line %d: permission_overwrites type: write "role" or "member"`, node.Line)
}

// TagDef is a forum channel's tag, as a test declares it.
type TagDef struct {
	ID   int64  `yaml:"id"`
	Name string `yaml:"name"`
}

// GuildDef defines guild/server context.
type GuildDef struct {
	ID    int64     `yaml:"id"`
	Name  string    `yaml:"name"`
	Icon  string    `yaml:"icon"`  // .Guild.Icon: the icon hash, as Discord's guild "icon" (default none)
	Roles []RoleDef `yaml:"roles"` // If set, getRole and targetHasRole know only these
	// Channels, if set, are the server's other channels (the test's channel always exists):
	// channel arguments then accept only these, by ID or by name
	Channels []ChannelDef `yaml:"channels"`
	// OwnerID is .Guild.OwnerID (default: the triggering user)
	OwnerID int64  `yaml:"owner_id"`
	Prefix  string `yaml:"prefix"` // The command prefix (default: YAGPDB's "-")
	// BotMentionEveryone is whether the bot has Discord's "Mention @everyone, @here, and
	// All Roles" permission (default true). Without it @everyone and @here never ping, and
	// a role mention pings only a mentionable role.
	BotMentionEveryone *bool `yaml:"bot_mention_everyone"`
}

// RoleDef is a role in the guild.
type RoleDef struct {
	ID       int64  `yaml:"id"`
	Name     string `yaml:"name"`
	Color    int    `yaml:"color"`
	Position int    `yaml:"position"` // Higher is above; roleAbove compares these
	// Mentionable lets anyone ping the role; see GuildDef.BotMentionEveryone
	Mentionable bool `yaml:"mentionable"`
	// Managed is a bot's or integration's role: .Managed, which no member can be given
	Managed bool `yaml:"managed"`
	// Permissions is the role's permission bit mask (the guild's ID is @everyone's role);
	// getTargetPermissionsIn sums them
	Permissions int64 `yaml:"permissions"`
}

// DBEntry represents a database entry for setup.
type DBEntry struct {
	UserID int64       `yaml:"user_id"`
	Key    string      `yaml:"key"`
	Value  interface{} `yaml:"value"`
}

// ExpectedResult defines expected output.
type ExpectedResult struct {
	OutputEquals    *string `yaml:"output_equals"`    // Exact match ("" asserts no output)
	OutputContains  string  `yaml:"output_contains"`  // Substring match
	OutputMatches   string  `yaml:"output_matches"`   // Regex match
	ErrorContains   string  `yaml:"error_contains"`   // Expected error
	WarningContains string  `yaml:"warning_contains"` // Expected diagnostic
	// NoTrigger asserts the test's message_content does NOT match the template's header
	// trigger: the test passes without executing the template, or fails if the message
	// does match. It can't be combined with any other expected: field or with assertions:,
	// since those need the template to have run.
	NoTrigger bool `yaml:"no_trigger"`
}

// Assertions defines post-execution checks.
type Assertions struct {
	DBChecks []DBCheck `yaml:"db_checks"`
	// SentMessages are checked one by one, matching the nth (default: first) message in
	// each check's channel (or any channel); absent skips the check, `[]` asserts the run
	// sent no messages at all
	SentMessages *[]MessageCheck `yaml:"sent_messages"`
	// EditedMessages check messages as editMessage left them, one by one, matching the nth
	// (default: first) edit in each check's channel (or any channel); absent skips the
	// check, `[]` asserts the run edited no messages at all
	EditedMessages *[]MessageCheck `yaml:"edited_messages"`
	RoleChanges    []RoleCheck     `yaml:"role_changes"`
	// NoRoleChanges asserts the run changed no roles (a give of a role the member has, say)
	NoRoleChanges bool `yaml:"no_role_changes"`
	// ResponsePings is exactly who the response (the template's output) notifies
	ResponsePings *PingsCheck `yaml:"response_pings"`
	// ScheduledRuns are exactly the runs execCC with a delay and scheduleUniqueCC left
	// scheduled, in order (`[]` for none)
	ScheduledRuns *[]ScheduledRunCheck `yaml:"scheduled_runs"`
	// Deletions are exactly the message deletions the run asked for (deleteTrigger,
	// deleteMessage, a deleteResponse whose response was sent), in order (`[]` for none)
	Deletions *[]DeletionCheck `yaml:"deletions"`
	// Reactions are exactly the reactions the run added and removed, in order (`[]` for
	// none)
	Reactions *[]ReactionCheck `yaml:"reactions"`
	// Execs are exactly the bot commands the run executed with exec and execAdmin, in
	// order (`[]` for none)
	Execs *[]ExecCheck `yaml:"execs"`
	// InteractionResponses are exactly the run's answers to its interaction, in order
	// (`[]` for none): the response, followups, a deferred response's edit, an update
	// of the component's message, a modal opened
	InteractionResponses *[]InteractionResponseCheck `yaml:"interaction_responses"`
}

// InteractionResponseCheck matches an interaction response; unset fields match anything.
type InteractionResponseCheck struct {
	Kind      string `yaml:"kind"` // "message", "followup", "deferred_edit", "update" or "modal"
	Ephemeral *bool  `yaml:"ephemeral"`
	// The message's checks, as a sent_messages entry has them
	ContentContains    string `yaml:"content_contains"`
	EmbedTitle         string `yaml:"embed_title"`
	EmbedContains      string `yaml:"embed_contains"`
	ComponentsContains string `yaml:"components_contains"`
	// A modal's checks (kind: modal): its title and custom ID exactly, and exactly its
	// fields' custom IDs in order, all without the templates- prefix
	Title    *string   `yaml:"title"`
	CustomID *string   `yaml:"custom_id"`
	Fields   *[]string `yaml:"fields"`
}

// ExecCheck matches an exec or execAdmin call; unset fields match anything.
type ExecCheck struct {
	Line      string `yaml:"line"` // the command line, as YAGPDB builds it: kick 5 "reason"
	Admin     *bool  `yaml:"admin"`
	ChannelID int64  `yaml:"channel_id"`
}

// ReactionCheck matches a reaction change; unset fields match anything.
type ReactionCheck struct {
	Action    string `yaml:"action"` // "add", "remove", "remove_emoji" or "remove_all"
	Emoji     string `yaml:"emoji"`
	ChannelID int64  `yaml:"channel_id"`
	MessageID int64  `yaml:"message_id"`
	UserID    int64  `yaml:"user_id"` // whose reaction "remove" removed
	// Response requires the reaction to be on the command's response (addResponseReactions)
	Response bool `yaml:"response"`
}

// DeletionCheck matches a deletion; unset fields match anything.
type DeletionCheck struct {
	Of        string    `yaml:"of"` // "trigger", "message" or "response"
	ChannelID int64     `yaml:"channel_id"`
	MessageID int64     `yaml:"message_id"`
	Delay     *Duration `yaml:"delay"` // 0s = at once
}

// ScheduledRunCheck matches a scheduled run; unset fields match anything.
type ScheduledRunCheck struct {
	CCID             int64    `yaml:"cc_id"`
	ChannelID        int64    `yaml:"channel_id"`
	Delay            Duration `yaml:"delay"`
	Key              *string  `yaml:"key"`                // scheduleUniqueCC's key
	ExecDataContains string   `yaml:"exec_data_contains"` // substring of the data as JSON
	// Interaction is the state of the scheduling run's interaction when it scheduled the
	// run: none (it had no interaction), pending (one it hadn't responded to) or
	// responded (one it had responded to or deferred, so the delayed run's response is a
	// followup); unset = any. Both execCC with a delay and scheduleUniqueCC store it
	// (vendor customcommands/tmplextensions.go:256-282, :344).
	Interaction string `yaml:"interaction"`
}

// PingsCheck is exactly who a message notifies; unset fields expect no one.
type PingsCheck struct {
	Everyone bool    `yaml:"everyone"` // @everyone or @here
	Users    []int64 `yaml:"users"`
	Roles    []int64 `yaml:"roles"`
}

// DBCheck defines a database assertion.
type DBCheck struct {
	UserID        int64       `yaml:"user_id"`
	Key           string      `yaml:"key"`
	ValueEquals   interface{} `yaml:"value_equals"`
	ValueContains string      `yaml:"value_contains"`
	NotExists     bool        `yaml:"not_exists"`
}

// MessageCheck defines a sent message assertion.
type MessageCheck struct {
	ChannelID       int64   `yaml:"channel_id"`
	Nth             int     `yaml:"nth"`            // which message in the channel (or of all): 1 = first, the default
	ContentEquals   *string `yaml:"content_equals"` // "" asserts empty content
	ContentContains string  `yaml:"content_contains"`
	HasEmbed        bool    `yaml:"has_embed"`
	EmbedTitle      string  `yaml:"embed_title"`    // matches any of the message's embeds
	EmbedContains   string  `yaml:"embed_contains"` // substring of any of the message's embeds as JSON (title, fields, ...)
	// ComponentsContains is a substring of any of the message's action rows as JSON (a
	// custom_id, a label, ...)
	ComponentsContains string `yaml:"components_contains"`
	// Pings is exactly who the message notifies (edits notify no one)
	Pings *PingsCheck `yaml:"pings"`
	// SentAfterSeconds is exactly how many seconds into the run's sleeps the message was
	// sent (whole seconds); unset checks nothing
	SentAfterSeconds *int `yaml:"sent_after_seconds"`
	// Ephemeral is whether the message is an interaction response only the clicker
	// sees; unset checks nothing
	Ephemeral *bool `yaml:"ephemeral"`
}

// RoleCheck defines a role change assertion.
type RoleCheck struct {
	UserID int64    `yaml:"user_id"`
	RoleID int64    `yaml:"role_id"`
	Action string   `yaml:"action"` // "add" or "remove"
	Delay  Duration `yaml:"delay"`  // if set, the change is scheduled this far ahead
}

// TestSuite represents a collection of test cases.
type TestSuite struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Tests       []TestCase `yaml:"tests"`
	Defaults    ContextDef `yaml:"defaults"` // Default context values
	SetupDB     []DBEntry  `yaml:"setup_db"` // Shared database setup
	// SetupTemplates run before every test in the suite, ahead of the test's own
	SetupTemplates []string         `yaml:"setup_templates"`
	CommandMap     map[int64]string `yaml:"command_map"` // Shared command ID mapping
}

// LoadTestCase loads a single test case from a YAML file.
func LoadTestCase(filename string) (*TestCase, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading test file: %w", err)
	}

	var tc TestCase
	if err := types.StrictYAML(data, &tc); err != nil {
		return nil, fmt.Errorf("parsing test YAML: %w", err)
	}

	// Apply defaults
	tc.applyDefaults()
	tc.SourceFile = filename

	if err := tc.validateContext(); err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	if err := tc.validateNoTrigger(); err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}

	return &tc, nil
}

// validateContext rejects a context that says two things about how the command was
// triggered: an interaction can't come with args, message_content, a reaction or
// exec_data, and must be one the emulator models.
func (tc *TestCase) validateContext() error {
	c := tc.Context
	if c.Interaction == nil {
		return nil
	}
	// exec_data comes with an interaction only as the delayed half of a slash command
	delayed := c.Interaction.Delayed
	if delayed && c.ExecData == nil {
		return fmt.Errorf("test %q: a delayed interaction run needs exec_data (the data it was scheduled with)", tc.Name)
	}
	if len(c.Args) > 0 || c.MessageContent != "" || c.Reaction != nil || (c.ExecData != nil && !delayed) {
		return fmt.Errorf("test %q: interaction can't be combined with args, message_content, reaction or exec_data"+
			" (exec_data only with delayed: true)", tc.Name)
	}
	if err := c.Interaction.validate(); err != nil {
		return fmt.Errorf("test %q: %w", tc.Name, err)
	}
	return nil
}

// validateNoTrigger rejects expected.no_trigger combined with any other expected: field or
// with assertions:, since a no_trigger test passes or fails without ever running the
// template, so nothing else could be checked.
func (tc *TestCase) validateNoTrigger() error {
	if !tc.Expected.NoTrigger {
		return nil
	}
	e := tc.Expected
	if e.OutputEquals != nil || e.OutputContains != "" || e.OutputMatches != "" ||
		e.ErrorContains != "" || e.WarningContains != "" {
		return fmt.Errorf("test %q: no_trigger can't be combined with another expected: field (the template never runs)", tc.Name)
	}
	if !reflect.DeepEqual(tc.Assertions, Assertions{}) {
		return fmt.Errorf("test %q: no_trigger can't be combined with assertions: (the template never runs)", tc.Name)
	}
	if tc.Snapshot {
		return fmt.Errorf("test %q: no_trigger can't be combined with snapshot: true (the template never runs)", tc.Name)
	}
	if tc.Context.ExecData != nil || tc.Context.Reaction != nil {
		return fmt.Errorf("test %q: no_trigger can't be combined with an exec_data or reaction context (those bypass the header trigger check entirely)", tc.Name)
	}
	return nil
}

// LoadTestSuite loads a test suite from a YAML file.
func LoadTestSuite(filename string) (*TestSuite, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading test suite file: %w", err)
	}

	var ts TestSuite
	if err := types.StrictYAML(data, &ts); err != nil {
		return nil, fmt.Errorf("parsing test suite YAML: %w", err)
	}

	// A suite's defaults can't trigger or feed a command; each test says that for itself
	d := ts.Defaults
	if len(d.Args) > 0 || d.ExecData != nil || d.MessageContent != "" || d.Reaction != nil || d.Interaction != nil {
		return nil, fmt.Errorf("%s: defaults can't set args, exec_data, message_content, reaction or interaction; set them per test", filename)
	}

	// Apply defaults to all tests
	for i := range ts.Tests {
		ts.Tests[i].mergeDefaults(ts.Defaults, ts.SetupDB, ts.CommandMap)
		if len(ts.SetupTemplates) > 0 {
			ts.Tests[i].SetupTemplates = append(append([]string{}, ts.SetupTemplates...), ts.Tests[i].SetupTemplates...)
		}
		ts.Tests[i].SourceFile = filename
		// YAGPDB runs no custom command for a bot's message (customcommands/bot.go)
		if ts.Tests[i].Context.User.ID == runtime.BotUserID {
			return nil, fmt.Errorf("%s: test %q: user.id %d is the bot's; YAGPDB runs no custom command for a bot's message",
				filename, ts.Tests[i].Name, runtime.BotUserID)
		}
		if err := ts.Tests[i].validateContext(); err != nil {
			return nil, fmt.Errorf("%s: %w", filename, err)
		}
		if err := ts.Tests[i].validateNoTrigger(); err != nil {
			return nil, fmt.Errorf("%s: %w", filename, err)
		}
	}

	return &ts, nil
}

// LoadTestFile loads a test file: a suite if it has a top-level tests: key, otherwise a
// single test case. A suite that doesn't parse is an error, not a single test case.
func LoadTestFile(filename string) ([]*TestCase, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading test file: %w", err)
	}
	var keys map[string]interface{}
	if err := yaml.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filename, err)
	}
	if _, isSuite := keys["tests"]; !isSuite {
		tc, err := LoadTestCase(filename)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", filename, err)
		}
		return []*TestCase{tc}, nil
	}
	ts, err := LoadTestSuite(filename)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", filename, err)
	}
	tests := make([]*TestCase, len(ts.Tests))
	for i := range ts.Tests {
		tests[i] = &ts.Tests[i]
	}
	return tests, nil
}

// LoadTestsFromDir loads all test files from a directory.
func LoadTestsFromDir(dir string) ([]*TestCase, error) {
	var tests []*TestCase

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == "__snapshots__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Load .yaml and .yml files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		loaded, err := LoadTestFile(path)
		tests = append(tests, loaded...)
		return err
	})

	if err != nil {
		return nil, err
	}

	return tests, nil
}

// applyDefaults sets default values for unset fields.
func (tc *TestCase) applyDefaults() {
	if tc.Context.User.ID == 0 {
		tc.Context.User.ID = 987654321098765432
	}
	if tc.Context.User.Username == "" {
		tc.Context.User.Username = "TestUser"
	}
	if tc.Context.User.Discriminator == "" {
		tc.Context.User.Discriminator = "0001"
	}
	if tc.Context.Channel.ID == 0 {
		tc.Context.Channel.ID = 123456789012345678
	}
	if tc.Context.Channel.Name == "" {
		tc.Context.Channel.Name = "test-channel"
	}
	if tc.Context.Guild.ID == 0 {
		tc.Context.Guild.ID = 111222333444555666
	}
	if tc.Context.Guild.Name == "" {
		tc.Context.Guild.Name = "Test Server"
	}
}

// mergeDefaults merges suite defaults into a test case.
func (tc *TestCase) mergeDefaults(defaults ContextDef, sharedDB []DBEntry, sharedCommandMap map[int64]string) {
	// Merge user defaults
	if tc.Context.User.ID == 0 {
		tc.Context.User.ID = defaults.User.ID
	}
	if tc.Context.User.Username == "" {
		tc.Context.User.Username = defaults.User.Username
	}
	if tc.Context.User.Discriminator == "" {
		tc.Context.User.Discriminator = defaults.User.Discriminator
	}
	if tc.Context.User.Roles == nil && defaults.User.Roles != nil {
		tc.Context.User.Roles = defaults.User.Roles
	}

	// Merge channel defaults
	if tc.Context.Channel.ID == 0 {
		tc.Context.Channel.ID = defaults.Channel.ID
	}
	if tc.Context.Channel.Name == "" {
		tc.Context.Channel.Name = defaults.Channel.Name
	}

	if tc.Context.Premium == nil {
		tc.Context.Premium = defaults.Premium
	}
	if tc.Context.Clock == nil {
		tc.Context.Clock = defaults.Clock
	}
	if tc.Context.Seed == nil {
		tc.Context.Seed = defaults.Seed
	}
	if tc.Context.Messages == nil {
		tc.Context.Messages = defaults.Messages
	}
	if tc.Context.Members == nil {
		tc.Context.Members = defaults.Members
	}
	if tc.Context.MemberRoles == nil {
		tc.Context.MemberRoles = defaults.MemberRoles
	}
	if tc.Context.MemberNicks == nil {
		tc.Context.MemberNicks = defaults.MemberNicks
	}
	if tc.Context.MemberJoinedAgo == nil {
		tc.Context.MemberJoinedAgo = defaults.MemberJoinedAgo
	}
	if tc.Context.ExecResponses == nil {
		tc.Context.ExecResponses = defaults.ExecResponses
	}
	if tc.Context.Guild.Roles == nil {
		tc.Context.Guild.Roles = defaults.Guild.Roles
	}
	if tc.Context.Guild.Channels == nil {
		tc.Context.Guild.Channels = defaults.Guild.Channels
	}

	// Merge guild defaults
	if tc.Context.Guild.ID == 0 {
		tc.Context.Guild.ID = defaults.Guild.ID
	}
	if tc.Context.Guild.Name == "" {
		tc.Context.Guild.Name = defaults.Guild.Name
	}
	if tc.Context.Guild.Icon == "" {
		tc.Context.Guild.Icon = defaults.Guild.Icon
	}
	if tc.Context.Guild.Prefix == "" {
		tc.Context.Guild.Prefix = defaults.Guild.Prefix
	}
	if tc.Context.Guild.OwnerID == 0 {
		tc.Context.Guild.OwnerID = defaults.Guild.OwnerID
	}
	if tc.Context.Guild.BotMentionEveryone == nil {
		tc.Context.Guild.BotMentionEveryone = defaults.Guild.BotMentionEveryone
	}

	// Prepend shared DB entries
	if len(sharedDB) > 0 {
		tc.SetupDB = append(append([]DBEntry{}, sharedDB...), tc.SetupDB...)
	}

	// Merge command map (suite-level + test-level, test overrides suite)
	if len(sharedCommandMap) > 0 {
		if tc.CommandMap == nil {
			tc.CommandMap = make(map[int64]string)
		}
		for k, v := range sharedCommandMap {
			if _, exists := tc.CommandMap[k]; !exists {
				tc.CommandMap[k] = v
			}
		}
	}

	// Apply standard defaults
	tc.applyDefaults()
}

// GetTemplateSource returns the template source, loading from file if needed.
func (tc *TestCase) GetTemplateSource(baseDir string) (string, error) {
	if tc.TemplateSource != "" {
		return tc.TemplateSource, nil
	}

	if tc.Template == "" {
		return "", fmt.Errorf("no template specified")
	}

	// Resolve template path
	templatePath := tc.Template
	if !filepath.IsAbs(templatePath) {
		templatePath = filepath.Join(baseDir, templatePath)
	}

	data, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("reading template %s: %w", templatePath, err)
	}

	return string(data), nil
}

// Duration is a Go duration in YAML, like "12h" or "90m".
type Duration time.Duration

// UnmarshalYAML parses a duration string.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	parsed, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("line %d: %w (use h, m or s: 72h, not 3d)", node.Line, err)
	}
	if parsed < 0 {
		return fmt.Errorf("line %d: %q is negative, a join time in the future", node.Line, node.Value)
	}
	*d = Duration(parsed)
	return nil
}
