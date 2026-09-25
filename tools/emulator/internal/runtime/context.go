// Package runtime provides the template execution runtime for the YAGPDB emulator.
package runtime

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/schema"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// SentMessage represents a message that was "sent" during template execution.
type SentMessage struct {
	ID        int64
	ChannelID int64
	Content   string
	Embed     interface{}
	Pings     Pings // who the message notifies
}

// RoleChange represents a role change that occurred during template execution.
type RoleChange struct {
	UserID int64
	RoleID int64
	Action string        // "add" or "remove"
	Delay  time.Duration // scheduled this far ahead (0 = now)
}

// FileUpload represents a file that was "uploaded" during template execution.
type FileUpload struct {
	ChannelID int64
	Filename  string
	Content   string
}

// ExecutionContext holds all state for a single template execution.
type ExecutionContext struct {
	// Guild/Server context
	GuildID   int64
	GuildName string
	OwnerID   int64 // The guild owner; 0 means the triggering user
	// BotCannotMentionEveryone: the bot lacks "Mention @everyone, @here, and All Roles", so
	// only mentionable roles ping and @everyone/@here don't
	BotCannotMentionEveryone bool
	Prefix                   string

	// Channel context
	ChannelID   int64
	ChannelName string

	// User context
	UserID        int64
	Username      string
	Discriminator string
	UserRoles     []int64

	// Message context
	MessageID      int64
	MessageContent string

	// Command arguments, set by SetTriggerMessage; triggered is whether a message ran the command
	Args        []interface{}
	CmdArgs     []interface{}
	StrippedMsg string
	Cmd         string
	triggered   bool

	// SourceName names the template in warnings (usually its file path)
	SourceName string

	// ExecData for execCC calls
	ExecData interface{}
	// NoMessage and NoMember are set for a scheduled (interval or cron) run, which YAGPDB
	// starts with neither: no .Message, and no .User, .Member or .BotUser. An execCC child
	// of such a run keeps NoMember and gets a blank .Message (see triggerMsg). A None
	// command only runs by execCC, so it always has its caller's message.
	NoMessage bool
	NoMember  bool
	// InheritedMessage is an execCC child's .Message: its caller's triggering message as
	// YAGPDB keeps it (see triggerMsg)
	InheritedMessage *types.CtxMessage
	// inheritedFromReaction: InheritedMessage is a reaction run's, whose author is the
	// reactor, not the message's real author
	inheritedFromReaction bool

	// Messages that exist, for getMessage; Members, if set, are the only users in the server
	Messages []types.CtxMessage
	sentIDs  *int64 // see sentMessageIDs
	Members  []int64
	// MemberRoles are other members' roles; the triggering user's are UserRoles
	MemberRoles map[int64][]int64
	// MemberNicks and MemberJoinedAgo give members' nicknames and how long before the run
	// they joined, the triggering user's included (default DefaultJoinedAgo)
	MemberNicks     map[int64]string
	MemberJoinedAgo map[int64]time.Duration

	// Reaction trigger: set for commands triggered by a reaction
	Reaction      *types.CtxReaction
	ReactionAdded bool

	// Premium mode
	IsPremium bool

	// Strict makes YAGPDB's execution limits fail the run, as they would in production.
	// Without it, a breached limit is recorded as a warning and execution continues.
	Strict bool

	// Mocked services
	DB *state.MockDB

	// Schema, when set, holds the expected types of database values (warnings on mismatch).
	Schema *schema.Schema

	// Side effects captured during execution
	SentMessages []SentMessage
	// EditedMessages are messages as editMessage left them, in the order they were edited
	EditedMessages []SentMessage
	RoleChanges    []RoleChange
	FileUploads    []FileUpload
	// ResponsePings are who the response (the template's output) notifies
	ResponsePings Pings

	scheduled *[]ScheduledRun // see ScheduledRuns

	// Set by mentionEveryone/mentionHere and mentionRole, so the response pings them
	mentionEveryone bool
	mentionRoles    []int64
	// deleteResponse's setting: YAGPDB sends no response it would delete at once (delay < 1)
	delResponse      bool
	delResponseDelay int

	// Warnings found during execution (limits, db calls in loops, schema mismatches)
	Diagnostics []Diagnostic

	// Per-run call counters, keyed like YAGPDB's Context.Counters
	Counters map[string]int
	warned   map[string]bool // limit warnings already recorded

	// CCID is the custom command's number (set for execCC children): it names the template,
	// as in YAGPDB's errors, and the over-2k notice
	CCID int64

	StartTime time.Time

	// Available roles (for hasRole checks)
	AvailableRoles map[int64]types.CtxRole
	// Channels are the server's channels by ID (name as value), when the test declares them;
	// empty, any channel ID is taken to exist (see channelArg)
	Channels map[int64]string
	// ChannelOrder is the declared channels' order, which stands in for their position
	// (YAGPDB looks names up in position order)
	ChannelOrder []int64

	// Command ID mapping (for execCC)
	CommandIDMap map[int64]string

	// execCC tracking
	ExecCCDepth     int    // Current nesting depth
	MaxExecCCDepth  int    // Maximum allowed depth (default 2)
	TemplateBaseDir string // Base directory for resolving template paths
}

// NewExecutionContext creates a new execution context with default values.
func NewExecutionContext(guildID int64, db *state.MockDB) *ExecutionContext {
	return &ExecutionContext{
		GuildID:        guildID,
		GuildName:      "Test Server",
		Prefix:         DefaultPrefix,
		ChannelID:      123456789,
		ChannelName:    "test-channel",
		UserID:         987654321,
		MessageID:      234567890, // every Discord message has an ID
		Username:       "TestUser",
		Discriminator:  "0001",
		UserRoles:      []int64{},
		Args:           []interface{}{},
		CmdArgs:        []interface{}{},
		DB:             db,
		IsPremium:      true,
		Counters:       make(map[string]int),
		StartTime:      time.Now(),
		AvailableRoles: make(map[int64]types.CtxRole),
		Channels:       make(map[int64]string),
		CommandIDMap:   make(map[int64]string),
		MaxExecCCDepth: 2, // YAGPDB default
	}
}

// SetNonPremium configures the context for non-premium limits.
func (ctx *ExecutionContext) SetNonPremium() {
	ctx.IsPremium = false
}

// BuildTemplateData creates the data map passed to template execution (the "dot").
func (ctx *ExecutionContext) BuildTemplateData() map[string]interface{} {
	// Build user object
	user := types.DiscordUser{
		ID:            ctx.UserID,
		Username:      ctx.Username,
		Discriminator: ctx.Discriminator,
	}

	member := ctx.member(ctx.UserID)

	// Build channel object
	channel := types.CtxChannel{
		ID:      ctx.ChannelID,
		GuildID: ctx.GuildID,
		Name:    ctx.ChannelName,
	}

	// With no roles declared, @everyone, as getRole has it
	guildRoles := ctx.sortedRoles()
	if len(guildRoles) == 0 && ctx.GuildID != 0 {
		guildRoles = append(guildRoles, types.CtxRole{ID: ctx.GuildID, Name: "@everyone"})
	}
	ownerID := ctx.OwnerID
	if ownerID == 0 {
		ownerID = ctx.UserID
	}
	guild := types.CtxGuild{
		ID:      ctx.GuildID,
		Name:    ctx.GuildName,
		OwnerID: ownerID,
		Roles:   guildRoles,
	}

	message := ctx.message()

	// Build permissions map
	permissions := map[string]int64{
		"Administrator":         0x8,
		"ManageServer":          0x20,
		"ManageRoles":           0x10000000,
		"ManageChannels":        0x10,
		"KickMembers":           0x2,
		"BanMembers":            0x4,
		"ManageMessages":        0x2000,
		"MentionEveryone":       0x20000,
		"ManageNicknames":       0x8000000,
		"ManageWebhooks":        0x20000000,
		"ManageEmojis":          0x40000000,
		"ViewAuditLog":          0x80,
		"SendMessages":          0x800,
		"EmbedLinks":            0x4000,
		"AttachFiles":           0x8000,
		"ReadMessageHistory":    0x10000,
		"UseExternalEmojis":     0x40000,
		"Connect":               0x100000,
		"Speak":                 0x200000,
		"MuteMembers":           0x400000,
		"DeafenMembers":         0x800000,
		"MoveMembers":           0x1000000,
		"UseVAD":                0x2000000,
		"CreateInstantInvite":   0x1,
		"ChangeNickname":        0x4000000,
		"AddReactions":          0x40,
		"ViewChannel":           0x400,
		"SendTTSMessages":       0x1000,
		"PrioritySpeaker":       0x100,
		"Stream":                0x200,
		"UseSlashCommands":      0x80000000,
		"RequestToSpeak":        0x100000000,
		"ManageThreads":         0x400000000,
		"CreatePublicThreads":   0x800000000,
		"CreatePrivateThreads":  0x1000000000,
		"UseExternalStickers":   0x2000000000,
		"SendMessagesInThreads": 0x4000000000,
		"UseEmbeddedActivities": 0x8000000000,
		"ModerateMembers":       0x10000000000,
	}

	data := map[string]interface{}{
		// User/Member
		"User":   user,
		"user":   user, // YAGPDB supports both cases
		"Member": member,

		// Guild/Server
		"Guild":        guild,
		"Server":       guild,
		"ServerPrefix": ctx.Prefix,

		// Channel
		"Channel": channel,
		"channel": channel,

		// Message
		"Message": message,

		// Bot user (simplified)
		"BotUser": botUser,

		// Permissions
		"Permissions": permissions,

		// Premium status
		"IsPremium": ctx.IsPremium,

		// Time constants
		"DiscordEpoch": time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC),
		"UnixEpoch":    time.Unix(0, 0),
		"TimeHour":     time.Hour,
		"TimeMinute":   time.Minute,
		"TimeSecond":   time.Second,

		// Nil constant
		"nil": nil,
	}

	// An immediate execCC sets ExecData even when it is nil, so .ExecData.Key errors there;
	// other runs only have it when there is data (delayed runs: DelayedRunCCData.UserData)
	if ctx.ExecCCDepth > 0 || ctx.ExecData != nil {
		data["ExecData"] = ctx.ExecData
	}
	if ctx.ExecCCDepth > 0 {
		data["StackDepth"] = ctx.ExecCCDepth
	}

	switch {
	case ctx.InheritedMessage != nil:
		data["Message"] = *ctx.InheritedMessage
	case ctx.NoMessage:
		delete(data, "Message")
	}
	if ctx.NoMember { // setupBaseData sets these only when there is a member
		for _, k := range []string{"User", "user", "Member", "BotUser"} {
			delete(data, k)
		}
	}
	if ctx.Reaction != nil {
		data["Reaction"] = ctx.Reaction
		data["ReactionAdded"] = ctx.ReactionAdded
		// YAGPDB sets both to the message that was reacted to
		data["ReactionMessage"] = ctx.reactedMessage(message)
		data["Message"] = data["ReactionMessage"]
	}
	// Only a message trigger sets the arguments; YAGPDB leaves them unset otherwise
	if ctx.triggered {
		data["Args"] = ctx.Args
		data["CmdArgs"] = ctx.CmdArgs
		data["StrippedMsg"] = ctx.StrippedMsg
		data["Cmd"] = ctx.Cmd
	}

	return data
}

// HasRole checks if the current user has a specific role.
func (ctx *ExecutionContext) HasRole(roleID int64) bool {
	for _, r := range ctx.UserRoles {
		if r == roleID {
			return true
		}
	}
	return false
}

// RecordSentMessage records a message sent during execution. Messages the bot sends to a
// channel can be fetched with getMessage, as on Discord; it returns their ID.
func (ctx *ExecutionContext) RecordSentMessage(channelID int64, content string, embed interface{}, pings Pings) int64 {
	*ctx.sentMessageIDs()++
	id := firstSentMessageID + *ctx.sentIDs
	ctx.SentMessages = append(ctx.SentMessages, SentMessage{
		ID:        id,
		ChannelID: channelID,
		Content:   content,
		Embed:     embed,
		Pings:     pings,
	})
	msg := types.CtxMessage{
		ID:        id,
		ChannelID: channelID,
		GuildID:   ctx.GuildID,
		Author:    botUser,
		Content:   content,
		Timestamp: time.Now(),
	}
	if embed != nil {
		msg.Embeds = []interface{}{embed}
	}
	if channelID != 0 { // a DM isn't in the server's channels
		ctx.Messages = append(ctx.Messages, msg)
	}
	return id
}

// Sent messages get IDs from here up, clear of the IDs tests declare.
const firstSentMessageID = 1_100_000_000_000_000_000

// sentMessageIDs is the count of messages sent so far, shared with execCC children so every
// message in a run gets its own ID.
func (ctx *ExecutionContext) sentMessageIDs() *int64 {
	if ctx.sentIDs == nil {
		ctx.sentIDs = new(int64)
	}
	return ctx.sentIDs
}

var botUser = types.DiscordUser{ID: 1234567890, Username: "YAGPDB.xyz", Bot: true}

// RecordRoleChange records a role change during execution.
func (ctx *ExecutionContext) RecordRoleChange(userID, roleID int64, action string, delay time.Duration) {
	ctx.RoleChanges = append(ctx.RoleChanges, RoleChange{
		UserID: userID,
		RoleID: roleID,
		Action: action,
		Delay:  delay,
	})
}

// RecordFileUpload records a file that was "uploaded" during execution.
func (ctx *ExecutionContext) RecordFileUpload(channelID int64, filename, content string) {
	ctx.FileUploads = append(ctx.FileUploads, FileUpload{
		ChannelID: channelID,
		Filename:  filename,
		Content:   content,
	})
}

// isMember reports whether a user is in the server: anyone, unless the test lists members.
func (ctx *ExecutionContext) isMember(userID int64) bool {
	if userID == ctx.UserID || ctx.Members == nil {
		return true
	}
	for _, m := range ctx.Members {
		if m == userID {
			return true
		}
	}
	return false
}

// DefaultJoinedAgo is how long before now a member joined, unless a test says otherwise.
const DefaultJoinedAgo = 30 * 24 * time.Hour

// member is what YAGPDB's getMember gives for a server member.
func (ctx *ExecutionContext) member(userID int64) types.CtxMember {
	user := types.DiscordUser{ID: userID, Username: "MockUser"}
	if userID == ctx.UserID {
		user = types.DiscordUser{ID: userID, Username: ctx.Username, Discriminator: ctx.Discriminator}
	}
	ago, ok := ctx.MemberJoinedAgo[userID]
	if !ok {
		ago = DefaultJoinedAgo
	}
	return types.CtxMember{
		User:     user,
		Nick:     ctx.MemberNicks[userID],
		Roles:    ctx.rolesOf(userID),
		JoinedAt: types.NewTimestamp(ctx.StartTime.Add(-ago)),
	}
}

// rolesOf returns a member's role IDs.
func (ctx *ExecutionContext) rolesOf(userID int64) []int64 {
	if userID == ctx.UserID {
		return ctx.UserRoles
	}
	return ctx.MemberRoles[userID]
}

// sortedRoles is the guild's roles in YAGPDB's order: its state tracker sorts them as
// dstate.Roles (IsRoleAbove), highest position first and the lower ID on a tie.
func (ctx *ExecutionContext) sortedRoles() []types.CtxRole {
	roles := make([]types.CtxRole, 0, len(ctx.AvailableRoles))
	for _, role := range ctx.AvailableRoles {
		roles = append(roles, role)
	}
	slices.SortFunc(roles, func(a, b types.CtxRole) int {
		return cmp.Or(cmp.Compare(b.Position, a.Position), cmp.Compare(a.ID, b.ID))
	})
	return roles
}

// message is the triggering message as the test gives it: the user's, with the test's
// content.
func (ctx *ExecutionContext) message() types.CtxMessage {
	return types.CtxMessage{
		ID:        ctx.MessageID,
		ChannelID: ctx.ChannelID,
		GuildID:   ctx.GuildID,
		Author:    types.DiscordUser{ID: ctx.UserID, Username: ctx.Username, Discriminator: ctx.Discriminator},
		Content:   ctx.MessageContent,
		Timestamp: ctx.StartTime,
	}
}

// reactedMessage is the message a reaction run reacted to: the test's message with that ID
// in this channel, else the triggering message's defaults with the ID.
func (ctx *ExecutionContext) reactedMessage(defaults types.CtxMessage) types.CtxMessage {
	for _, m := range ctx.Messages {
		if m.ID == ctx.Reaction.MessageID && m.ChannelID == ctx.ChannelID {
			return m
		}
	}
	defaults.ID = ctx.Reaction.MessageID
	return defaults
}

// triggerMsg is YAGPDB's ctx.Msg for this run, which an execCC child gets as .Message: a
// reaction run's is the reacted-to message with the reactor as author (bot.go
// ExecuteCustomCommandFromReaction), and a run with no message gets a blank one from the
// bot, which Context.Execute makes before the template runs.
func (ctx *ExecutionContext) triggerMsg() types.CtxMessage {
	message := ctx.message()
	switch {
	case ctx.InheritedMessage != nil:
		return *ctx.InheritedMessage
	case ctx.Reaction != nil:
		reacted := ctx.reactedMessage(message)
		reacted.Author = message.Author
		return reacted
	case ctx.NoMessage:
		return types.CtxMessage{ChannelID: ctx.ChannelID, GuildID: ctx.GuildID, Author: botUser}
	}
	return message
}

// channelNamed is the ID of the first channel, in position order, with that name (any
// case), or 0; no channel has an empty name. Without declared channels only the current
// one has a name.
func (ctx *ExecutionContext) channelNamed(name string) int64 {
	if name == "" {
		return 0
	}
	if len(ctx.Channels) == 0 {
		if strings.EqualFold(name, ctx.ChannelName) {
			return ctx.ChannelID
		}
		return 0
	}
	for _, id := range ctx.ChannelOrder {
		if strings.EqualFold(name, ctx.Channels[id]) {
			return id
		}
	}
	return 0
}

// channelName is the name of the channel with that ID: the current one's, or a declared one's.
func (ctx *ExecutionContext) channelName(id int64) string {
	if id == ctx.ChannelID {
		return ctx.ChannelName
	}
	return ctx.Channels[id]
}
