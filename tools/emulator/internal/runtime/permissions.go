package runtime

import (
	"fmt"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// Discord permission bits the calculation names (vendor lib/discordgo/permission.go).
const (
	permKickMembers    int64 = 1 << 1
	permBanMembers     int64 = 1 << 2
	permAdministrator  int64 = 1 << 3
	permManageGuild    int64 = 1 << 5
	permViewChannel    int64 = 1 << 10
	permChangeNickname int64 = 1 << 26
	permManageRoles    int64 = 1 << 28
	// discordgo.PermissionAll, dstate.AllPermissions
	permAllPermissions int64 = int64(^uint64(0) >> 1)
	// dstate.ChannelPermsMask (vendor lib/dstate/permissions.go): the bits an overwrite
	// can't change
	permChannelPermMask = ^(permAdministrator | permManageGuild | permChangeNickname |
		permManageRoles | permKickMembers | permBanMembers)
)

// errUnknownMember is the REST error bot.GetMember returns for a user who isn't in the
// server (vendor bot/memberfetcher.go: the state misses, so the Discord call answers).
var errUnknownMember = discordError{"404 Not Found", 10007, "Unknown Member"}

// calculateBasePermissions is dstate.CalculateBasePermissions (vendor
// lib/dstate/permissions.go): the owner has everything; else @everyone's (the role whose
// ID is the guild's) and the member's roles' bits, Administrator being everything.
func (ctx *ExecutionContext) calculateBasePermissions(memberID int64, roles []int64) int64 {
	if ctx.guildOwnerID() == memberID {
		return permAllPermissions
	}
	var perms int64
	if everyone, ok := ctx.AvailableRoles[ctx.GuildID]; ok {
		perms |= everyone.Permissions
	}
	for _, roleID := range roles {
		if role, ok := ctx.AvailableRoles[roleID]; ok {
			perms |= role.Permissions
		}
	}
	if perms&permAdministrator == permAdministrator {
		return permAllPermissions
	}
	return perms
}

// guildOwnerID is .Guild.OwnerID: the test's owner, or the triggering user.
func (ctx *ExecutionContext) guildOwnerID() int64 {
	if ctx.OwnerID == 0 {
		return ctx.UserID
	}
	return ctx.OwnerID
}

// applyChannelPermissions is dstate.ApplyChannelPermissions (vendor
// lib/dstate/permissions.go): the @everyone overwrite, then the member's roles' overwrites
// (denies and allows gathered, applied together), then the member's own.
func applyChannelPermissions(perms, guildID int64, overwrites []types.PermissionOverwrite, memberID int64, roles []int64) int64 {
	if len(overwrites) == 0 {
		return perms
	}
	// an admin or owner is not subject to overwrites
	if perms == permAllPermissions {
		return perms
	}
	for _, ow := range overwrites {
		if guildID == ow.ID {
			perms &= ^(ow.Deny & permChannelPermMask)
			perms |= ow.Allow & permChannelPermMask
			break
		}
	}
	var denies, allows int64
	for _, ow := range overwrites {
		for _, roleID := range roles {
			if ow.Type == types.PermissionOverwriteTypeRole && roleID == ow.ID {
				denies |= ow.Deny & permChannelPermMask
				allows |= ow.Allow & permChannelPermMask
				break
			}
		}
	}
	perms &= ^denies
	perms |= allows
	for _, ow := range overwrites {
		if ow.Type == types.PermissionOverwriteTypeMember && ow.ID == memberID {
			perms &= ^(ow.Deny & permChannelPermMask)
			perms |= ow.Allow & permChannelPermMask
			break
		}
	}
	return perms
}

// memberPermissions is dstate.GuildSet.GetMemberPermissions (vendor
// lib/dstate/interface.go): a thread uses its parent's overwrites; a channel (or a
// thread's parent) the server doesn't have is an error, with the permissions still
// computed from the roles alone. Without declared channels every channel is assumed to
// exist (see channelArg) and has no overwrites.
func (ctx *ExecutionContext) memberPermissions(channelID, memberID int64, roles []int64) (int64, error) {
	var overwrites []types.PermissionOverwrite
	var err error
	if len(ctx.Channels) > 0 || len(ctx.Threads) > 0 {
		lookup := channelID
		if d, ok := ctx.ChannelDetails[channelID]; ok && IsThreadChannelType(d.Type) {
			lookup = d.ParentID
		}
		if _, ok := ctx.Channels[lookup]; ok {
			overwrites = ctx.ChannelOverwrites[lookup]
		} else if channelID != 0 {
			err = fmt.Errorf("Channel not found: %d", channelID)
		}
	}
	perms := ctx.calculateBasePermissions(memberID, roles)
	perms = applyChannelPermissions(perms, ctx.GuildID, overwrites, memberID, roles)
	return perms, err
}
