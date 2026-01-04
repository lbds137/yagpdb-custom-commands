// Package runtime provides the template execution runtime for the YAGPDB emulator.
package runtime

import (
	"strings"
	"time"

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

	// Command arguments
	Args       []interface{}
	CmdArgs    []interface{}
	StrippedMsg string
	Cmd         string

	// ExecData for execCC calls
	ExecData interface{}

	// Premium mode
	IsPremium bool

	// Mocked services
	DB *state.MockDB

	// Side effects captured during execution
	Output       strings.Builder
	SentMessages []SentMessage
	RoleChanges  []RoleChange
	FileUploads  []FileUpload
	Errors       []error

	// Execution limits
	MaxOps      int
	CurrentOps  int
	MaxOutput   int
	StartTime   time.Time
	MaxDuration time.Duration

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
		MaxOps:         2500000, // Premium limit
		MaxOutput:      25000,
		StartTime:      time.Now(),
		MaxDuration:    10 * time.Second,
		AvailableRoles: make(map[int64]types.CtxRole),
		CommandIDMap:   make(map[int64]string),
		MaxExecCCDepth: 2, // YAGPDB default
	}
}

// SetNonPremium configures the context for non-premium limits.
func (ctx *ExecutionContext) SetNonPremium() {
	ctx.IsPremium = false
	ctx.MaxOps = 1000000
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
	guild := types.CtxGuild{
		ID:    ctx.GuildID,
		Name:  ctx.GuildName,
		Roles: guildRoles,
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
		"Administrator":           0x8,
		"ManageServer":            0x20,
		"ManageRoles":             0x10000000,
		"ManageChannels":          0x10,
		"KickMembers":             0x2,
		"BanMembers":              0x4,
		"ManageMessages":          0x2000,
		"MentionEveryone":         0x20000,
		"ManageNicknames":         0x8000000,
		"ManageWebhooks":          0x20000000,
		"ManageEmojis":            0x40000000,
		"ViewAuditLog":            0x80,
		"SendMessages":            0x800,
		"EmbedLinks":              0x4000,
		"AttachFiles":             0x8000,
		"ReadMessageHistory":      0x10000,
		"UseExternalEmojis":       0x40000,
		"Connect":                 0x100000,
		"Speak":                   0x200000,
		"MuteMembers":             0x400000,
		"DeafenMembers":           0x800000,
		"MoveMembers":             0x1000000,
		"UseVAD":                  0x2000000,
		"CreateInstantInvite":     0x1,
		"ChangeNickname":          0x4000000,
		"AddReactions":            0x40,
		"ViewChannel":             0x400,
		"SendTTSMessages":         0x1000,
		"PrioritySpeaker":         0x100,
		"Stream":                  0x200,
		"UseSlashCommands":        0x80000000,
		"RequestToSpeak":          0x100000000,
		"ManageThreads":           0x400000000,
		"CreatePublicThreads":     0x800000000,
		"CreatePrivateThreads":    0x1000000000,
		"UseExternalStickers":     0x2000000000,
		"SendMessagesInThreads":   0x4000000000,
		"UseEmbeddedActivities":   0x8000000000,
		"ModerateMembers":         0x10000000000,
	}

	return map[string]interface{}{
		// User/Member
		"User":   user,
		"user":   user, // YAGPDB supports both cases
		"Member": member,

		// Guild/Server
		"Guild":  guild,
		"Server": guild,

		// Channel
		"Channel": channel,
		"channel": channel,

		// Message
		"Message": message,

		// Command arguments
		"Args":        ctx.Args,
		"CmdArgs":     ctx.CmdArgs,
		"StrippedMsg": ctx.StrippedMsg,
		"Cmd":         ctx.Cmd,

		// ExecData (from execCC) - use empty SDict if nil to prevent nil pointer errors
		"ExecData": func() interface{} {
			if ctx.ExecData == nil {
				return types.SDict{}
			}
			return ctx.ExecData
		}(),

		// Bot user (simplified)
		"BotUser": types.DiscordUser{
			ID:       1234567890,
			Username: "YAGPDB.xyz",
			Bot:      true,
		},

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

// HasRoleID is an alias for HasRole.
func (ctx *ExecutionContext) HasRoleID(roleID int64) bool {
	return ctx.HasRole(roleID)
}

// RecordSentMessage records a message that was "sent" during execution.
func (ctx *ExecutionContext) RecordSentMessage(channelID int64, content string, embed interface{}) {
	ctx.SentMessages = append(ctx.SentMessages, SentMessage{
		ChannelID: channelID,
		Content:   content,
		Embed:     embed,
	})
}

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

// CheckLimits verifies the execution is within limits.
func (ctx *ExecutionContext) CheckLimits() error {
	if ctx.CurrentOps > ctx.MaxOps {
		return ErrTooManyOps
	}
	if time.Since(ctx.StartTime) > ctx.MaxDuration {
		return ErrTimeout
	}
	if ctx.Output.Len() > ctx.MaxOutput {
		return ErrOutputTooLarge
	}
	return nil
}

// IncrementOps increments the operation counter and checks limits.
func (ctx *ExecutionContext) IncrementOps(n int) error {
	ctx.CurrentOps += n
	return ctx.CheckLimits()
}
