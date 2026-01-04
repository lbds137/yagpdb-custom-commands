// Package funcs provides Discord-related template functions for the YAGPDB emulator.
package funcs

import (
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// DiscordFuncs provides Discord-related template functions.
type DiscordFuncs struct {
	// Callbacks to the runtime context for recording side effects
	onSendMessage func(channelID int64, content string, embed interface{})
	onRoleChange  func(userID, roleID int64, action string)

	// State accessors
	getUserID     func() int64
	getChannelID  func() int64
	getUserRoles  func() []int64
	getMemberFunc func(userID int64) *types.CtxMember
	getChannelFunc func(channelID int64) *types.CtxChannel
}

// DiscordFuncsConfig configures the Discord functions.
type DiscordFuncsConfig struct {
	OnSendMessage  func(channelID int64, content string, embed interface{})
	OnRoleChange   func(userID, roleID int64, action string)
	GetUserID      func() int64
	GetChannelID   func() int64
	GetUserRoles   func() []int64
	GetMember      func(userID int64) *types.CtxMember
	GetChannel     func(channelID int64) *types.CtxChannel
}

// NewDiscordFuncs creates a new DiscordFuncs instance.
func NewDiscordFuncs(config DiscordFuncsConfig) *DiscordFuncs {
	return &DiscordFuncs{
		onSendMessage:  config.OnSendMessage,
		onRoleChange:   config.OnRoleChange,
		getUserID:      config.GetUserID,
		getChannelID:   config.GetChannelID,
		getUserRoles:   config.GetUserRoles,
		getMemberFunc:  config.GetMember,
		getChannelFunc: config.GetChannel,
	}
}

// SendMessage sends a message to a channel.
func (d *DiscordFuncs) SendMessage(args ...interface{}) string {
	var channelID int64
	var content string
	var embed interface{}

	if d.getChannelID != nil {
		channelID = d.getChannelID()
	}

	if len(args) >= 1 {
		if args[0] != nil {
			channelID = ToInt64(args[0])
		}
	}
	if len(args) >= 2 {
		switch v := args[1].(type) {
		case string:
			content = v
		case types.SDict:
			embed = v
		default:
			content = ToString(v)
		}
	}

	if d.onSendMessage != nil {
		d.onSendMessage(channelID, content, embed)
	}
	return ""
}

// SendDM sends a direct message.
func (d *DiscordFuncs) SendDM(msg interface{}) string {
	content := ToString(msg)
	if d.onSendMessage != nil {
		d.onSendMessage(0, content, nil) // 0 = DM
	}
	return ""
}

// EditMessage edits a message (mock - does nothing).
func (d *DiscordFuncs) EditMessage(channel, msgID, content interface{}) string {
	return ""
}

// GetMessage retrieves a message (mock - returns nil).
func (d *DiscordFuncs) GetMessage(channel, msgID interface{}) interface{} {
	return nil
}

// DeleteMessage deletes a message (mock - does nothing).
func (d *DiscordFuncs) DeleteMessage(args ...interface{}) string {
	return ""
}

// DeleteTrigger deletes the trigger message (mock - does nothing).
func (d *DiscordFuncs) DeleteTrigger(args ...interface{}) string {
	return ""
}

// DeleteResponse deletes the response message (mock - does nothing).
func (d *DiscordFuncs) DeleteResponse(args ...interface{}) string {
	return ""
}

// AddReactions adds reactions to a message (mock - does nothing).
func (d *DiscordFuncs) AddReactions(args ...interface{}) string {
	return ""
}

// AddMessageReactions adds reactions to a specific message (mock - does nothing).
func (d *DiscordFuncs) AddMessageReactions(args ...interface{}) string {
	return ""
}

// HasRole checks if the current user has a role.
func (d *DiscordFuncs) HasRole(roleInput interface{}) bool {
	roleID := ToInt64(roleInput)
	if d.getUserRoles == nil {
		return false
	}
	for _, r := range d.getUserRoles() {
		if r == roleID {
			return true
		}
	}
	return false
}

// HasRoleID is an alias for HasRole.
func (d *DiscordFuncs) HasRoleID(roleID interface{}) bool {
	return d.HasRole(roleID)
}

// TargetHasRole checks if a target user has a role.
func (d *DiscordFuncs) TargetHasRole(target, roleInput interface{}) (bool, error) {
	return false, nil
}

// AddRole adds a role to the current user.
func (d *DiscordFuncs) AddRole(roleInput interface{}, delay ...interface{}) string {
	roleID := ToInt64(roleInput)
	if d.onRoleChange != nil && d.getUserID != nil {
		d.onRoleChange(d.getUserID(), roleID, "add")
	}
	return ""
}

// GiveRole gives a role to a target user.
func (d *DiscordFuncs) GiveRole(target, roleInput interface{}, delay ...interface{}) string {
	userID := ToInt64(target)
	roleID := ToInt64(roleInput)
	if d.onRoleChange != nil {
		d.onRoleChange(userID, roleID, "add")
	}
	return ""
}

// RemoveRole removes a role from the current user.
func (d *DiscordFuncs) RemoveRole(roleInput interface{}, delay ...interface{}) string {
	roleID := ToInt64(roleInput)
	if d.onRoleChange != nil && d.getUserID != nil {
		d.onRoleChange(d.getUserID(), roleID, "remove")
	}
	return ""
}

// TakeRole takes a role from a target user.
func (d *DiscordFuncs) TakeRole(target, roleInput interface{}, delay ...interface{}) string {
	userID := ToInt64(target)
	roleID := ToInt64(roleInput)
	if d.onRoleChange != nil {
		d.onRoleChange(userID, roleID, "remove")
	}
	return ""
}

// SetRoles sets roles for a target user (mock - does nothing).
func (d *DiscordFuncs) SetRoles(target interface{}, roles interface{}) string {
	return ""
}

// GiveRoleID is an alias for GiveRole.
func (d *DiscordFuncs) GiveRoleID(target, roleID interface{}) string {
	return d.GiveRole(target, roleID)
}

// TakeRoleID is an alias for TakeRole.
func (d *DiscordFuncs) TakeRoleID(target, roleID interface{}) string {
	return d.TakeRole(target, roleID)
}

// AddRoleID is an alias for AddRole.
func (d *DiscordFuncs) AddRoleID(roleID interface{}, delay ...interface{}) string {
	return d.AddRole(roleID, delay...)
}

// RemoveRoleID is an alias for RemoveRole.
func (d *DiscordFuncs) RemoveRoleID(roleID interface{}, delay ...interface{}) string {
	return d.RemoveRole(roleID, delay...)
}

// GetMember gets a member by user ID.
func (d *DiscordFuncs) GetMember(userID interface{}) interface{} {
	if d.getMemberFunc != nil {
		return d.getMemberFunc(ToInt64(userID))
	}
	return types.CtxMember{
		User: types.DiscordUser{
			ID:       ToInt64(userID),
			Username: "MockUser",
		},
	}
}

// UserArg parses a user argument.
func (d *DiscordFuncs) UserArg(arg interface{}) interface{} {
	return types.DiscordUser{
		ID:       ToInt64(arg),
		Username: "MockUser",
	}
}

// GetTargetPermissionsIn gets permissions for a user in a channel.
func (d *DiscordFuncs) GetTargetPermissionsIn(userID, channelID interface{}) int64 {
	return 0
}

// GetChannel gets a channel by ID.
func (d *DiscordFuncs) GetChannel(channelID interface{}) interface{} {
	if d.getChannelFunc != nil {
		return d.getChannelFunc(ToInt64(channelID))
	}
	return types.CtxChannel{
		ID:   ToInt64(channelID),
		Name: "mock-channel",
	}
}

// Cembed creates an embed from key-value pairs.
func (d *DiscordFuncs) Cembed(args ...interface{}) (types.SDict, error) {
	result := make(types.SDict)
	for i := 0; i+1 < len(args); i += 2 {
		key := ToString(args[i])
		result[key] = args[i+1]
	}
	return result, nil
}

// ComplexMessage creates a complex message (same as cembed for now).
func (d *DiscordFuncs) ComplexMessage(args ...interface{}) (types.SDict, error) {
	return d.Cembed(args...)
}

// SendTemplate sends a template (mock - does nothing).
func (d *DiscordFuncs) SendTemplate(args ...interface{}) string {
	return ""
}
