package runtime

import (
	"fmt"
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

func TestAnAssumedRoleWarns(t *testing.T) {
	ctx := newCtx(false, true) // declares no guild roles
	out, err := run(t, ctx, `{{mentionRoleID 99}} {{(getRole 99).ID}} {{(getRole .Guild.ID).Name}}`)
	if err != nil || out != "<@&99> 99 @everyone" {
		t.Fatalf("got %q, %v", out, err)
	}
	if len(ctx.Diagnostics) != 1 || ctx.Diagnostics[0].Kind != KindRole ||
		!strings.Contains(ctx.Diagnostics[0].Message, "role 99 is assumed to exist") {
		t.Errorf("want one warning for role 99, got %q", ctx.Diagnostics)
	}

	ctx = roleCtx() // declared roles are looked up without a warning
	if _, err := run(t, ctx, `{{mentionRoleID 10}}{{mentionRoleID 99}}`); err != nil || len(ctx.Diagnostics) != 0 {
		t.Errorf("got %v, %q", err, ctx.Diagnostics)
	}
}

// .Guild.Roles is in YAGPDB's order (highest position first, the lower ID on a tie), which
// a name lookup follows, and has @everyone when the test declares no roles
func TestGuildRolesAreInYAGPDBOrder(t *testing.T) {
	src := `{{range .Guild.Roles}}{{.ID}}/{{.Name}} {{end}}{{(getRole "A").ID}}`
	for i := 0; i < 20; i++ {
		ctx := newCtx(false, true)
		ctx.AvailableRoles = map[int64]types.CtxRole{
			30: {ID: 30, Name: "a", Position: 1}, 20: {ID: 20, Name: "a"}, 10: {ID: 10, Name: "b", Position: 1},
		}
		if out, err := run(t, ctx, src); err != nil || out != "10/b 30/a 20/a 30" {
			t.Fatalf("got %q, %v", out, err)
		}
	}
	src = `{{range .Guild.Roles}}{{.ID}}/{{.Name}}{{end}}`
	ctx := newCtx(false, true)
	want := fmt.Sprintf("%d/@everyone", ctx.GuildID)
	if out, err := run(t, ctx, src); err != nil || out != want {
		t.Errorf("got %q, %v; want %q", out, err, want)
	}
	ctx = newCtx(false, true)
	ctx.GuildID = 0 // no guild, no @everyone
	if out, err := run(t, ctx, src); err != nil || out != "" {
		t.Errorf("got %q, %v", out, err)
	}
}

// Over the API-call limit getRole* fail with "too many calls to this function", as YAGPDB's
// getRole does; other API functions give the API-call error
func TestGetRoleOverTheLimit(t *testing.T) {
	strict := func() *ExecutionContext { ctx := roleCtx(); ctx.Strict = true; return ctx }
	for _, fn := range []string{`getRole 10`, `getRoleID 10`, `getRoleName "Staff"`} {
		_, err := run(t, strict(), `{{range seq 0 101}}{{`+fn+`}}{{end}}`)
		if err == nil || !strings.Contains(err.Error(), "too many calls to this function") {
			t.Errorf("%s: got %v", fn, err)
		}
	}
	_, err := run(t, strict(), `{{range seq 0 101}}{{targetHasRoleID 5 20}}{{end}}`)
	if err == nil || !strings.Contains(err.Error(), "too many potential Discord API calls") {
		t.Errorf("targetHasRoleID: got %v", err)
	}
}

// parseArgs' role argument is YAGPDB's RoleArg: a mention or ID matches a role's ID or, as
// text, its exact name, whichever role comes first in guild order
func TestParseArgsRole(t *testing.T) {
	cases := []struct{ arg, out, err string }{
		{"Staff", "10", ""},
		{"staff", "caught", ""}, // names are case-sensitive here
		{"<@&10>", "10", ""},
		{"<@&100", "10", ""}, // the mention's last character is cut, whatever it is
		{"20", "30", ""},     // role 30, named "20", outranks role 20
		{"99", "caught", ""}, // "Invalid role mention or id", which try catches
		// the bad mention's -1 is an int, and RoleArg's int64 assertion panics: no try
		// catches a panic
		{"<@&x>", "", "error calling parseArgs: interface conversion: interface {} is int, not int64"},
		{"<@&>", "", "error calling parseArgs: interface conversion: interface {} is int, not int64"},
	}
	for _, c := range cases {
		ctx := roleCtx()
		ctx.AvailableRoles[30] = types.CtxRole{ID: 30, Name: "20", Position: 5}
		if err := ctx.SetTriggerMessage(Trigger{Type: "Command", Text: "c"}, "-c "+c.arg); err != nil {
			t.Fatal(err)
		}
		out, err := run(t, ctx, `{{try}}{{((parseArgs 1 "" (carg "role" "r")).Get 0).ID}}{{catch}}caught{{end}}`)
		if c.err == "" && (err != nil || out != c.out) || c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%s: got %q, %v; want %q, %q", c.arg, out, err, c.out, c.err)
		}
	}
	ctx := roleCtx()
	if err := ctx.SetTriggerMessage(Trigger{Type: "Command", Text: "c"}, "-c 99"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, ctx, `{{parseArgs 1 "" (carg "role" "r")}}`); err == nil || !strings.Contains(err.Error(), "Invalid role mention or id") {
		t.Errorf("unknown role: %v", err)
	}
}

// With no roles declared, a role argument's ID is assumed to exist, with a warning, and
// @everyone is found by name
func TestParseArgsRoleAssumed(t *testing.T) {
	ctx := newCtx(false, true)
	if err := ctx.SetTriggerMessage(Trigger{Type: "Command", Text: "c"}, "-c <@&99> @everyone"); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, ctx, `{{$a := parseArgs 2 "" (carg "role" "r") (carg "role" "s")}}{{($a.Get 0).ID}} {{eq ($a.Get 1).ID .Guild.ID}}`)
	if err != nil || out != "99 true" {
		t.Fatalf("got %q, %v", out, err)
	}
	if w := kinds(ctx, KindRole); len(w) != 1 || !strings.Contains(w[0], "role 99 is assumed") {
		t.Errorf("want one warning for role 99, got %q", w)
	}
}
