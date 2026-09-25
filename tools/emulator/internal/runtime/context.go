// Package runtime provides the template execution runtime for the YAGPDB emulator.
package runtime

import (
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/schema"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// SentMessage represents a message that was "sent" during template execution.
type SentMessage struct {
	ChannelID int64
	Content   string
	Embed     interface{}
}

// RoleChange represents a role change that occurred during template execution.
type RoleChange struct {
	UserID int64
	RoleID int64
	Action string // "add" or "remove"
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
	Prefix    string

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

	// Messages that exist, for getMessage; Members, if set, are the only users in the server
	Messages []types.CtxMessage
	sentIDs  *int64 // see sentMessageIDs
	Members  []int64
	// MemberRoles are other members' roles; the triggering user's are UserRoles
	MemberRoles map[int64][]int64

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
	RoleChanges  []RoleChange
	FileUploads  []FileUpload

	// Warnings found during execution (limits, db calls in loops, schema mismatches)
	Diagnostics []Diagnostic

	// Per-run call counters, keyed like YAGPDB's Context.Counters
	Counters map[string]int
	warned   map[string]bool // limit warnings already recorded

	// CCID is the custom command's number, shown in YAGPDB's over-2k notice
	CCID int64

	StartTime time.Time

	// Available roles (for hasRole checks)
	AvailableRoles map[int64]types.CtxRole

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

	// Build member object
	member := types.CtxMember{
		User:     user,
		Roles:    ctx.UserRoles,
		JoinedAt: types.TemplateTime{Time: time.Now().Add(-24 * time.Hour)}, // Default: joined 24h ago
	}

	// Build channel object
	channel := types.CtxChannel{
		ID:      ctx.ChannelID,
		GuildID: ctx.GuildID,
		Name:    ctx.ChannelName,
	}

	// Build guild object with roles
	guildRoles := make([]types.CtxRole, 0, len(ctx.AvailableRoles))
	for _, role := range ctx.AvailableRoles {
		guildRoles = append(guildRoles, role)
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

	// Build message object
	message := types.CtxMessage{
		ID:        ctx.MessageID,
		ChannelID: ctx.ChannelID,
		GuildID:   ctx.GuildID,
		Author:    user,
		Content:   ctx.MessageContent,
		Timestamp: time.Now(),
	}

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

		// ExecData (from execCC) - use empty SDict if nil to prevent nil pointer errors
		"ExecData": func() interface{} {
			if ctx.ExecData == nil {
				return types.SDict{}
			}
			return ctx.ExecData
		}(),

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

	if ctx.Reaction != nil {
		data["Reaction"] = ctx.Reaction
		data["ReactionAdded"] = ctx.ReactionAdded
		// YAGPDB sets both to the message that was reacted to. The emulator only knows
		// its ID; author and content stay the defaults.
		reactionMessage := message
		reactionMessage.ID = ctx.Reaction.MessageID
		data["ReactionMessage"] = reactionMessage
		data["Message"] = reactionMessage
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
func (ctx *ExecutionContext) RecordSentMessage(channelID int64, content string, embed interface{}) int64 {
	ctx.SentMessages = append(ctx.SentMessages, SentMessage{
		ChannelID: channelID,
		Content:   content,
		Embed:     embed,
	})
	*ctx.sentMessageIDs()++
	id := firstSentMessageID + *ctx.sentIDs
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
func (ctx *ExecutionContext) RecordRoleChange(userID, roleID int64, action string) {
	ctx.RoleChanges = append(ctx.RoleChanges, RoleChange{
		UserID: userID,
		RoleID: roleID,
		Action: action,
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

// rolesOf returns a member's role IDs.
func (ctx *ExecutionContext) rolesOf(userID int64) []int64 {
	if userID == ctx.UserID {
		return ctx.UserRoles
	}
	return ctx.MemberRoles[userID]
}
