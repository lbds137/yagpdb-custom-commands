// Package state provides Discord state management for the YAGPDB emulator.
package state

import (
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// MockDiscord holds mocked Discord state.
type MockDiscord struct {
	GuildID int64

	// Channels by ID
	Channels map[int64]*types.CtxChannel

	// Roles by ID
	Roles map[int64]*types.CtxRole

	// Members by user ID
	Members map[int64]*types.CtxMember

	// Messages by ID (for getMessage, editMessage, etc.)
	Messages map[int64]*types.CtxMessage

	// Current user's roles (for hasRole checks)
	CurrentUserRoles []int64
}

// NewMockDiscord creates a new mock Discord state.
func NewMockDiscord(guildID int64) *MockDiscord {
	return &MockDiscord{
		GuildID:          guildID,
		Channels:         make(map[int64]*types.CtxChannel),
		Roles:            make(map[int64]*types.CtxRole),
		Members:          make(map[int64]*types.CtxMember),
		Messages:         make(map[int64]*types.CtxMessage),
		CurrentUserRoles: []int64{},
	}
}

// AddChannel adds a channel to the mock state.
func (d *MockDiscord) AddChannel(id int64, name string) {
	d.Channels[id] = &types.CtxChannel{
		ID:      id,
		GuildID: d.GuildID,
		Name:    name,
	}
}

// GetChannel returns a channel by ID.
func (d *MockDiscord) GetChannel(id int64) *types.CtxChannel {
	if ch, ok := d.Channels[id]; ok {
		return ch
	}
	// Return a mock channel if not found
	return &types.CtxChannel{
		ID:      id,
		GuildID: d.GuildID,
		Name:    "unknown-channel",
	}
}

// AddRole adds a role to the mock state.
func (d *MockDiscord) AddRole(id int64, name string, color int, position int) {
	d.Roles[id] = &types.CtxRole{
		ID:       id,
		Name:     name,
		Color:    color,
		Position: position,
	}
}

// GetRole returns a role by ID.
func (d *MockDiscord) GetRole(id int64) *types.CtxRole {
	if r, ok := d.Roles[id]; ok {
		return r
	}
	return nil
}

// GetRoleByName returns a role by name.
func (d *MockDiscord) GetRoleByName(name string) *types.CtxRole {
	for _, r := range d.Roles {
		if r.Name == name {
			return r
		}
	}
	return nil
}

// AddMember adds a member to the mock state.
func (d *MockDiscord) AddMember(userID int64, username string, roles []int64) {
	d.Members[userID] = &types.CtxMember{
		User: types.DiscordUser{
			ID:       userID,
			Username: username,
		},
		Roles: roles,
	}
}

// GetMember returns a member by user ID.
func (d *MockDiscord) GetMember(userID int64) *types.CtxMember {
	if m, ok := d.Members[userID]; ok {
		return m
	}
	// Return a mock member if not found
	return &types.CtxMember{
		User: types.DiscordUser{
			ID:       userID,
			Username: "UnknownUser",
		},
		Roles: []int64{},
	}
}

// SetCurrentUserRoles sets the roles for the current user (for hasRole checks).
func (d *MockDiscord) SetCurrentUserRoles(roles []int64) {
	d.CurrentUserRoles = roles
}

// HasRole checks if the current user has a specific role.
func (d *MockDiscord) HasRole(roleID int64) bool {
	for _, r := range d.CurrentUserRoles {
		if r == roleID {
			return true
		}
	}
	return false
}

// HasRoleByName checks if the current user has a role by name.
func (d *MockDiscord) HasRoleByName(name string) bool {
	role := d.GetRoleByName(name)
	if role == nil {
		return false
	}
	return d.HasRole(role.ID)
}

// MemberHasRole checks if a specific member has a role.
func (d *MockDiscord) MemberHasRole(userID, roleID int64) bool {
	member := d.GetMember(userID)
	if member == nil {
		return false
	}
	for _, r := range member.Roles {
		if r == roleID {
			return true
		}
	}
	return false
}

// AddMessage adds a message to the mock state.
func (d *MockDiscord) AddMessage(msg *types.CtxMessage) {
	d.Messages[msg.ID] = msg
}

// GetMessage returns a message by ID.
func (d *MockDiscord) GetMessage(id int64) *types.CtxMessage {
	if msg, ok := d.Messages[id]; ok {
		return msg
	}
	return nil
}

// GiveRole adds a role to a member.
func (d *MockDiscord) GiveRole(userID, roleID int64) {
	member := d.Members[userID]
	if member == nil {
		member = &types.CtxMember{
			User: types.DiscordUser{
				ID:       userID,
				Username: "UnknownUser",
			},
			Roles: []int64{},
		}
		d.Members[userID] = member
	}

	// Check if already has role
	for _, r := range member.Roles {
		if r == roleID {
			return
		}
	}
	member.Roles = append(member.Roles, roleID)
}

// TakeRole removes a role from a member.
func (d *MockDiscord) TakeRole(userID, roleID int64) {
	member := d.Members[userID]
	if member == nil {
		return
	}

	newRoles := make([]int64, 0, len(member.Roles))
	for _, r := range member.Roles {
		if r != roleID {
			newRoles = append(newRoles, r)
		}
	}
	member.Roles = newRoles
}
