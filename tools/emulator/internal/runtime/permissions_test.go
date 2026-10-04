package runtime

import (
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// A server for the permission math: guild 1 (@everyone is role 1), owner 900, channels 10
// (text) and 11 (a thread of 10), role 20 "Mod", role 21 "Admin", members 100 (Mod),
// 101 (Admin), 102 (no roles).
func permCtx() *ExecutionContext {
	ctx := newCtx(false, true)
	ctx.OwnerID = 900
	ctx.UserID = 100
	ctx.UserRoles = []int64{20}
	ctx.AvailableRoles = map[int64]types.CtxRole{
		1:  {ID: 1, Name: "@everyone", Permissions: permViewChannel},
		20: {ID: 20, Name: "Mod"},
		21: {ID: 21, Name: "Admin", Permissions: permAdministrator},
	}
	ctx.Channels = map[int64]string{ctx.ChannelID: "here", 10: "text"}
	ctx.ChannelOrder = []int64{ctx.ChannelID, 10}
	ctx.Threads = map[int64]string{11: "thread"}
	ctx.ThreadOrder = []int64{11}
	ctx.ChannelDetails = map[int64]types.CtxChannel{
		10: {ID: 10, Name: "text"},
		11: {ID: 11, Name: "thread", Type: channelTypeGuildPublicThread, ParentID: 10},
	}
	ctx.Members = []int64{100, 101, 102, 900}
	ctx.MemberRoles = map[int64][]int64{101: {21}}
	return ctx
}

// Each case is taken from vendor lib/dstate/permissions.go (CalculateBasePermissions and
// ApplyChannelPermissions) and interface.go (GetMemberPermissions)
func TestGetTargetPermissionsIn(t *testing.T) {
	deny := func(id, bits int64, typ int) types.PermissionOverwrite {
		return types.PermissionOverwrite{ID: id, Type: typ, Deny: bits}
	}
	allow := func(id, bits int64, typ int) types.PermissionOverwrite {
		return types.PermissionOverwrite{ID: id, Type: typ, Allow: bits}
	}
	const role, member = types.PermissionOverwriteTypeRole, types.PermissionOverwriteTypeMember
	cases := []struct {
		name       string
		src        string
		overwrites []types.PermissionOverwrite // on channel 10
		want       string
	}{
		{"the owner has every permission", `{{getTargetPermissionsIn 900 10}}`,
			[]types.PermissionOverwrite{deny(1, permViewChannel, role)}, "9223372036854775807"},
		{"an Administrator role has every permission, overwrites included", `{{getTargetPermissionsIn 101 10}}`,
			[]types.PermissionOverwrite{deny(1, permViewChannel, role)}, "9223372036854775807"},
		{"@everyone's bits apply to a member with no roles", `{{getTargetPermissionsIn 102 10}}`,
			nil, "1024"},
		{"an @everyone deny removes the bit", `{{getTargetPermissionsIn 102 10}}`,
			[]types.PermissionOverwrite{deny(1, permViewChannel, role)}, "0"},
		{"a role allow beats an @everyone deny", `{{getTargetPermissionsIn 100 10}}`,
			[]types.PermissionOverwrite{deny(1, permViewChannel, role), allow(20, permViewChannel, role)}, "1024"},
		{"a role allow doesn't reach a member without the role", `{{getTargetPermissionsIn 102 10}}`,
			[]types.PermissionOverwrite{deny(1, permViewChannel, role), allow(20, permViewChannel, role)}, "0"},
		{"a member overwrite beats a role's", `{{getTargetPermissionsIn 100 10}}`,
			[]types.PermissionOverwrite{allow(20, permViewChannel, role), deny(100, permViewChannel, member)}, "0"},
		{"a member's id matches only a member overwrite", `{{getTargetPermissionsIn 100 10}}`,
			[]types.PermissionOverwrite{deny(100, permViewChannel, role)}, "1024"},
		{"a thread uses its parent's overwrites", `{{getTargetPermissionsIn 102 11}}`,
			[]types.PermissionOverwrite{deny(1, permViewChannel, role)}, "0"},
		{"a user not in the server is the Unknown Member error", `{{getTargetPermissionsIn 555 10}}`,
			nil, "Unknown Member"},
		{"an unknown user is 0", `{{getTargetPermissionsIn 0 10}}`, nil, "0"},
		{"a channel the server lacks is 0", `{{getTargetPermissionsIn 100 77}}`, nil, "0"},
	}
	for _, c := range cases {
		ctx := permCtx()
		ctx.ChannelOverwrites = map[int64][]types.PermissionOverwrite{10: c.overwrites}
		out, err := run(t, ctx, c.src)
		if c.want == "Unknown Member" {
			if err == nil || !strings.Contains(err.Error(), "Unknown Member") {
				t.Errorf("%s: got %q, %v; want the Unknown Member error", c.name, out, err)
			}
			continue
		}
		if err != nil || out != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.name, out, err, c.want)
		}
	}
}

// GetMemberPermissions errors for a thread whose parent the server lacks (the permissions
// are still computed), and the owner default is the triggering user
func TestGetTargetPermissionsInEdges(t *testing.T) {
	ctx := permCtx()
	ctx.ChannelDetails[11] = types.CtxChannel{ID: 11, Name: "thread", Type: channelTypeGuildPublicThread, ParentID: 99}
	if out, err := run(t, ctx, `{{getTargetPermissionsIn 102 11}}`); err == nil || out != "" ||
		!strings.Contains(err.Error(), "Channel not found: 11") {
		t.Errorf("orphaned thread: got %q, %v", out, err)
	}

	ctx = permCtx()
	ctx.OwnerID = 0 // the triggering user (100) owns the server
	ctx.ChannelOverwrites = map[int64][]types.PermissionOverwrite{
		10: {{ID: 1, Type: types.PermissionOverwriteTypeRole, Deny: permViewChannel}},
	}
	if out, err := run(t, ctx, `{{getTargetPermissionsIn 100 10}} {{getTargetPermissionsIn 102 10}}`); err != nil ||
		out != "9223372036854775807 0" {
		t.Errorf("default owner: got %q, %v", out, err)
	}
}

// An overwrite can't grant or deny the bits dstate.ChannelPermsMask filters out
func TestChannelPermsMask(t *testing.T) {
	ctx := permCtx()
	ctx.ChannelOverwrites = map[int64][]types.PermissionOverwrite{
		10: {{ID: 1, Type: types.PermissionOverwriteTypeRole, Allow: permManageRoles | permViewChannel}},
	}
	out, err := run(t, ctx, `{{getTargetPermissionsIn 102 10}}`)
	if err != nil || out != "1024" {
		t.Errorf("got %q, %v; want 1024", out, err)
	}
}
