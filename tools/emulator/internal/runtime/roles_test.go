package runtime

import (
	"strings"
	"testing"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

func roleCtx() *ExecutionContext {
	ctx := newCtx(false, true)
	ctx.AvailableRoles = map[int64]types.CtxRole{
		10: {ID: 10, Name: "Staff"}, 20: {ID: 20, Name: "Guest"},
	}
	ctx.UserRoles = []int64{10}
	ctx.Members = []int64{ctx.UserID, 5}
	ctx.MemberRoles = map[int64][]int64{5: {20}}
	return ctx
}

func TestRoleInputsFollowFindRole(t *testing.T) {
	cases := []struct{ src, want string }{
		{`{{(getRole "staff").ID}} {{(getRole "<@&20>").ID}} {{(getRole 10).ID}} {{(getRole "10").ID}}`, "10 20 10 10"},
		{`{{getRoleID "Staff"}} {{getRoleID "<@&10>"}} {{(getRoleID "10").ID}}`, "<nil> <nil> 10"},
		{`{{getRoleName "10"}} {{(getRoleName "GUEST").ID}}`, "<nil> 20"},
		{`{{mentionRoleID 10}}|{{mentionRoleID 99}}|{{mentionRoleName "Guest"}}|{{mentionRole "<@&10>"}}`, "<@&10>||<@&20>|<@&10>"},
		{`{{hasRoleID 10}} {{hasRoleID 20}} {{hasRoleName "staff"}} {{hasRoleID 99}}`, "true false true false"},
		{`{{targetHasRoleID 5 20}} {{targetHasRoleName 5 "Staff"}}`, "true false"},
	}
	for _, c := range cases {
		out, err := run(t, roleCtx(), c.src)
		if err != nil || out != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.src, out, err, c.want)
		}
	}
	if _, err := run(t, roleCtx(), `{{targetHasRoleID 5 "Staff"}}`); err == nil || !strings.Contains(err.Error(), "role Staff not found") {
		t.Errorf("an ID-only lookup of a name is an unknown role: %v", err)
	}
	if _, err := run(t, roleCtx(), `{{addRoleID 99}}`); err == nil || !strings.Contains(err.Error(), "role 99 not found") {
		t.Errorf("addRole of an unknown role errors: %v", err)
	}
}

func TestRoleChangesHappenOnlyWhenYAGPDBMakesThem(t *testing.T) {
	cases := []struct {
		name, src string
		want      []RoleChange
	}{
		{"give a new role", `{{giveRoleID 5 10}}`, []RoleChange{{UserID: 5, RoleID: 10, Action: "add"}}},
		{"give a role they have", `{{giveRoleID 5 20}}`, nil},
		{"give to a non-member", `{{giveRoleID 6 10}}`, nil},
		{"give an unknown role", `{{giveRoleID 5 99}}`, nil},
		{"take a role they have", `{{takeRoleName 5 "guest"}}`, []RoleChange{{UserID: 5, RoleID: 20, Action: "remove"}}},
		{"take a role they lack", `{{takeRoleID 5 10}}`, nil},
		{"a delay schedules it anyway", `{{giveRoleID 5 20 "90"}}`, []RoleChange{{UserID: 5, RoleID: 20, Action: "add", Delay: 90 * time.Second}}},
		{"a duration string", `{{takeRoleID 5 10 "2m"}}`, []RoleChange{{UserID: 5, RoleID: 10, Action: "remove", Delay: 2 * time.Minute}}},
		{"add to self", `{{addRoleID 20}}{{addRoleID 10}}`, []RoleChange{{UserID: 987654321, RoleID: 20, Action: "add"}}},
		{"remove from self", `{{removeRoleID 20}}{{removeRoleName "staff"}}`, []RoleChange{{UserID: 987654321, RoleID: 10, Action: "remove"}}},
	}
	for _, c := range cases {
		ctx := roleCtx()
		if _, err := run(t, ctx, c.src); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(ctx.RoleChanges) != len(c.want) {
			t.Errorf("%s: changes %+v, want %+v", c.name, ctx.RoleChanges, c.want)
			continue
		}
		for i := range c.want {
			if ctx.RoleChanges[i] != c.want[i] {
				t.Errorf("%s: change %+v, want %+v", c.name, ctx.RoleChanges[i], c.want[i])
			}
		}
	}
}
