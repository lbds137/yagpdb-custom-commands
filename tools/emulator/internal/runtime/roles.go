package runtime

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// The role functions follow YAGPDB's (common/templates/context_funcs.go). Each comes in
// three forms that accept different role inputs: plain (ID, mention, name or role),
// ID (a number or numeric string) and Name (a name).

type roleInputType int

const (
	acceptRoleID roleInputType = 1 << iota
	acceptRoleMention
	acceptRoleName
	acceptRoleObject
	acceptAllRoleInput = acceptRoleID | acceptRoleMention | acceptRoleName | acceptRoleObject
)

// roleFuncs are the role functions, keyed by template name.
func (e *Engine) roleFuncs() map[string]interface{} {
	return map[string]interface{}{
		"getRole":     func(r interface{}) *types.CtxRole { return e.findRole(r, acceptAllRoleInput) },
		"getRoleID":   func(r interface{}) *types.CtxRole { return e.findRole(r, acceptRoleID) },
		"getRoleName": func(r string) *types.CtxRole { return e.findRole(r, acceptRoleName) },

		"mentionRole":     func(r interface{}) string { return e.mentionRole(r, acceptAllRoleInput) },
		"mentionRoleID":   func(r interface{}) string { return e.mentionRole(r, acceptRoleID) },
		"mentionRoleName": func(r string) string { return e.mentionRole(r, acceptRoleName) },

		"hasRole":     func(r interface{}) bool { return e.hasRole(r, acceptAllRoleInput) },
		"hasRoleID":   func(r interface{}) bool { return e.hasRole(r, acceptRoleID) },
		"hasRoleName": func(r string) bool { return e.hasRole(r, acceptRoleName) },

		"targetHasRole": func(t, r interface{}) (bool, error) {
			return e.targetHasRole(t, r, acceptAllRoleInput)
		},
		"targetHasRoleID": func(t, r interface{}) (bool, error) {
			return e.targetHasRole(t, r, acceptRoleID)
		},
		"targetHasRoleName": func(t interface{}, r string) (bool, error) {
			return e.targetHasRole(t, r, acceptRoleName)
		},

		"giveRole": func(t, r interface{}, opt ...interface{}) string {
			return e.giveRole(t, r, acceptAllRoleInput, opt...)
		},
		"giveRoleID": func(t, r interface{}, opt ...interface{}) string {
			return e.giveRole(t, r, acceptRoleID, opt...)
		},
		"giveRoleName": func(t interface{}, r string, opt ...interface{}) string {
			return e.giveRole(t, r, acceptRoleName, opt...)
		},

		"takeRole": func(t, r interface{}, opt ...interface{}) string {
			return e.takeRole(t, r, acceptAllRoleInput, opt...)
		},
		"takeRoleID": func(t, r interface{}, opt ...interface{}) string {
			return e.takeRole(t, r, acceptRoleID, opt...)
		},
		"takeRoleName": func(t interface{}, r string, opt ...interface{}) string {
			return e.takeRole(t, r, acceptRoleName, opt...)
		},

		"addRole": func(r interface{}, opt ...interface{}) (string, error) {
			return e.addRole(r, acceptAllRoleInput, opt...)
		},
		"addRoleID": func(r interface{}, opt ...interface{}) (string, error) {
			return e.addRole(r, acceptRoleID, opt...)
		},
		"addRoleName": func(r string, opt ...interface{}) (string, error) {
			return e.addRole(r, acceptRoleName, opt...)
		},

		"removeRole": func(r interface{}, opt ...interface{}) (string, error) {
			return e.removeRole(r, acceptAllRoleInput, opt...)
		},
		"removeRoleID": func(r interface{}, opt ...interface{}) (string, error) {
			return e.removeRole(r, acceptRoleID, opt...)
		},
		"removeRoleName": func(r string, opt ...interface{}) (string, error) {
			return e.removeRole(r, acceptRoleName, opt...)
		},
	}
}

// findRole is YAGPDB's FindRole: the role the input names, if accept allows that kind of
// input, or nil.
func (e *Engine) findRole(role interface{}, accept roleInputType) *types.CtxRole {
	switch t := role.(type) {
	case string:
		if (accept & acceptRoleID) != 0 {
			parsed, err := strconv.ParseInt(t, 10, 64)
			if err == nil {
				return e.guildRole(parsed)
			}
		}

		if (accept & acceptRoleMention) != 0 {
			if len(t) > 4 && strings.HasPrefix(t, "<@&") && strings.HasSuffix(t, ">") {
				parsedMention, err := strconv.ParseInt(t[3:len(t)-1], 10, 64)
				if err == nil {
					return e.guildRole(parsedMention)
				}
			}
		}

		if (accept & acceptRoleName) != 0 {
			// If it's the everyone role, we just use the guild ID
			if t == "@everyone" {
				return e.guildRole(e.ctx.GuildID)
			}

			// It's a name after all. YAGPDB takes the first match in the guild's role
			// order; the emulator's roles are a map, so it goes by ID for a stable result.
			var found *types.CtxRole
			for _, r := range e.ctx.AvailableRoles {
				if strings.EqualFold(r.Name, t) && (found == nil || r.ID < found.ID) {
					found = &r
				}
			}
			return found
		}
	case *types.CtxRole:
		if (accept & acceptRoleObject) != 0 {
			return t
		}
	case types.CtxRole:
		if (accept & acceptRoleObject) != 0 {
			return &t
		}
	default:
		if (accept & acceptRoleID) != 0 {
			int64Role := funcs.ToInt64(t)
			if int64Role == 0 {
				return nil
			}

			return e.guildRole(int64Role)
		}
	}
	return nil
}

// guildRole is the guild's role with that ID, or nil. When the test declares no roles,
// any role ID is taken to exist, so commands run without listing the server's roles.
func (e *Engine) guildRole(id int64) *types.CtxRole {
	if role, ok := e.ctx.AvailableRoles[id]; ok {
		return &role
	}
	if len(e.ctx.AvailableRoles) > 0 || id == 0 {
		return nil
	}
	return &types.CtxRole{ID: id, Name: "MockRole", Color: 0x7289DA}
}

// mentionRole is the role's mention, or "" for a role the guild doesn't have.
func (e *Engine) mentionRole(roleInput interface{}, accept roleInputType) string {
	role := e.findRole(roleInput, accept)
	if role == nil {
		return ""
	}
	// The response pings the roles mentionRole returned
	if !slices.Contains(e.ctx.mentionRoles, role.ID) {
		e.ctx.mentionRoles = append(e.ctx.mentionRoles, role.ID)
	}
	return fmt.Sprintf("<@&%d>", role.ID)
}

// hasRole reports whether the triggering member has the role; false for an unknown role.
func (e *Engine) hasRole(roleInput interface{}, accept roleInputType) bool {
	role := e.findRole(roleInput, accept)
	if role == nil {
		return false
	}
	return e.ctx.HasRole(role.ID)
}

// targetHasRole: an unknown target, a user who isn't a member, or a role the guild
// doesn't have is an error.
func (e *Engine) targetHasRole(target, roleInput interface{}, accept roleInputType) (bool, error) {
	id := targetUserID(target)
	if id == 0 {
		return false, fmt.Errorf("target %v not found", target)
	}
	if !e.ctx.isMember(id) {
		return false, fmt.Errorf("member not found in state")
	}
	role := e.findRole(roleInput, accept)
	if role == nil {
		return false, fmt.Errorf("role %v not found", roleInput)
	}
	return e.memberHasRole(id, role.ID), nil
}

func (e *Engine) memberHasRole(userID, roleID int64) bool {
	for _, r := range e.ctx.rolesOf(userID) {
		if r == roleID {
			return true
		}
	}
	return false
}

// giveRole does nothing, silently, for an unknown target or role, a member who already
// has the role, or a user who isn't a member (Discord refuses, and YAGPDB ignores it). A
// delay over a second schedules the change without those checks.
func (e *Engine) giveRole(target, roleInput interface{}, accept roleInputType, optionalArgs ...interface{}) string {
	var delay time.Duration
	if len(optionalArgs) > 0 {
		delay = validateDurationDelay(optionalArgs[0])
	}

	targetID := targetUserID(target)
	if targetID == 0 {
		return ""
	}

	role := e.findRole(roleInput, accept)
	if role == nil {
		return ""
	}

	if delay > time.Second {
		e.ctx.RecordRoleChange(targetID, role.ID, "add", delay)
		return ""
	}
	if !e.ctx.isMember(targetID) || e.memberHasRole(targetID, role.ID) {
		return ""
	}
	e.ctx.RecordRoleChange(targetID, role.ID, "add", 0)
	return ""
}

// takeRole is giveRole's opposite: nothing happens unless the member has the role.
func (e *Engine) takeRole(target, roleInput interface{}, accept roleInputType, optionalArgs ...interface{}) string {
	var delay time.Duration
	if len(optionalArgs) > 0 {
		delay = validateDurationDelay(optionalArgs[0])
	}

	targetID := targetUserID(target)
	if targetID == 0 {
		return ""
	}

	role := e.findRole(roleInput, accept)
	if role == nil {
		return ""
	}

	if delay > time.Second {
		e.ctx.RecordRoleChange(targetID, role.ID, "remove", delay)
		return ""
	}
	if !e.ctx.isMember(targetID) || !e.memberHasRole(targetID, role.ID) {
		return ""
	}
	e.ctx.RecordRoleChange(targetID, role.ID, "remove", 0)
	return ""
}

// addRole gives the triggering member the role; an unknown role is an error.
func (e *Engine) addRole(roleInput interface{}, accept roleInputType, optionalArgs ...interface{}) (string, error) {
	var delay time.Duration
	if len(optionalArgs) > 0 {
		delay = validateDurationDelay(optionalArgs[0])
	}

	role := e.findRole(roleInput, accept)
	if role == nil {
		return "", fmt.Errorf("role %v not found", roleInput)
	}

	if delay > time.Second {
		e.ctx.RecordRoleChange(e.ctx.UserID, role.ID, "add", delay)
	} else if !e.ctx.HasRole(role.ID) { // AddRoleDS: already has the role
		e.ctx.RecordRoleChange(e.ctx.UserID, role.ID, "add", 0)
	}
	return "", nil
}

// removeRole takes the role from the triggering member; an unknown role is an error.
func (e *Engine) removeRole(roleInput interface{}, accept roleInputType, optionalArgs ...interface{}) (string, error) {
	var delay time.Duration
	if len(optionalArgs) > 0 {
		delay = validateDurationDelay(optionalArgs[0])
	}

	role := e.findRole(roleInput, accept)
	if role == nil {
		return "", fmt.Errorf("role %v not found", roleInput)
	}

	if delay > time.Second {
		e.ctx.RecordRoleChange(e.ctx.UserID, role.ID, "remove", delay)
	} else if e.ctx.HasRole(role.ID) { // RemoveRoleDS: never had the role
		e.ctx.RecordRoleChange(e.ctx.UserID, role.ID, "remove", 0)
	}
	return "", nil
}

// validateDurationDelay is YAGPDB's: a number is seconds, a string is a number of
// seconds or a duration.
func validateDurationDelay(in interface{}) time.Duration {
	switch t := in.(type) {
	case int, int64:
		return time.Second * yagstd.ToDuration(t)
	case string:
		conv := yagstd.ToInt64(t)
		if conv != 0 {
			return time.Second * yagstd.ToDuration(conv)
		}

		return yagstd.ToDuration(t)
	default:
		return yagstd.ToDuration(t)
	}
}
