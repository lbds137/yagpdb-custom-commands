package funcs

import (
	"strings"
	"testing"
	"time"
)

func mustCarg(t *testing.T, typ, name string, opts ...interface{}) *ArgDef {
	t.Helper()
	def, err := Carg(typ, name, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return def
}

func TestParseArgsLikeDcmd(t *testing.T) {
	str := mustCarg(t, "string", "s")
	num := mustCarg(t, "int", "n")
	cases := []struct {
		name, msg string
		defs      []*ArgDef
		required  int
		want      []interface{}
		err       string
	}{
		{"the last argument takes the rest", "hello big world", []*ArgDef{str}, 1, []interface{}{"hello big world"}, ""},
		{"quotes group words", `"a b" c`, []*ArgDef{str, str}, 2, []interface{}{"a b", "c"}, ""},
		{"the rest keeps its quotes", `a "b c" d`, []*ArgDef{str, str}, 2, []interface{}{"a", `"b c" d`}, ""},
		{"ints are ints", "5", []*ArgDef{num}, 1, []interface{}{5}, ""},
		{"optional arguments may be left out", "5", []*ArgDef{num, str}, 1, []interface{}{5, nil}, ""},
		{"too few", "", []*ArgDef{num}, 1, nil, "Not enough arguments passed\nfailed"},
		{"not a number", "abc", []*ArgDef{num}, 1, nil, `"abc" is not a whole number` + "\nfailed"},
		{"out of range", "50", []*ArgDef{mustCarg(t, "int", "n", 1, 10)}, 1, nil, "n is too big (has to be within 1 - 10)"},
		{"below range", "0", []*ArgDef{mustCarg(t, "int", "n", 1, 10)}, 1, nil, "n is too small"},
		{"float range", "2.5", []*ArgDef{mustCarg(t, "float", "f", 0, 1)}, 1, nil, "f is too big (has to be within 0.000000 - 1.000000)"},
		{"userid takes mentions", "<@!42>", []*ArgDef{mustCarg(t, "userid", "u")}, 1, []interface{}{int64(42)}, ""},
		{"int bounds keep int64 precision", "9007199254740993", []*ArgDef{mustCarg(t, "int", "n", int64(9007199254740993), int64(9007199254740993))}, 1, []interface{}{9007199254740993}, ""},
		{"duration bounds", "3d", []*ArgDef{mustCarg(t, "duration", "d", 0, 2*24*time.Hour)}, 1, nil, "d is too big, has to be smaller than 2 days"},
		{"duration range", "1m", []*ArgDef{mustCarg(t, "duration", "d", time.Hour, 25*time.Hour)}, 1, nil, "d is too small (has to be within `1 hour` and `1 day and 1 hour`)"},
		{"durations parse", "1h30m", []*ArgDef{mustCarg(t, "duration", "d")}, 1, []interface{}{90 * time.Minute}, ""},
		{"member needs an ID or mention", "bob", []*ArgDef{mustCarg(t, "member", "m")}, 1, nil, "Invalid mention or id"},
		{"userid rejects words", "bob", []*ArgDef{mustCarg(t, "userid", "u")}, 1, nil, `Improper mention "bob"`},
	}
	for _, c := range cases {
		pa, err := ParseArgs(c.msg, c.required, "failed", c.defs, Lookups{})
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%s: err = %v, want %q", c.name, err, c.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		for i, w := range c.want {
			if got := pa.Get(i); got != w {
				t.Errorf("%s: Get %d = %#v, want %#v", c.name, i, got, w)
			}
		}
	}
}

func TestParseArgsUsageLine(t *testing.T) {
	_, err := ParseArgs("", 1, "", []*ArgDef{mustCarg(t, "int", "n"), mustCarg(t, "string", "s")}, Lookups{})
	if err == nil || !strings.HasSuffix(err.Error(), "Usage: `<n:Whole number> [s:Text]`") {
		t.Errorf("err = %v", err)
	}
}

func TestCargRejectsUnknownTypes(t *testing.T) {
	if _, err := Carg("number", "n"); err == nil || err.Error() != "Unknown type" {
		t.Errorf("err = %v", err)
	}
}

func TestJoinArgsRoundTrips(t *testing.T) {
	args := []interface{}{"plain", "two words", `say "hi"`, "", `back\slash`, 7, "x\"y`z", `"`, "`", `\`}
	split := SplitArgs(JoinArgs(args))
	if len(split) != len(args) {
		t.Fatalf("split into %d: %q", len(split), JoinArgs(args))
	}
	for i, a := range args {
		if split[i].Str != ToString(a) {
			t.Errorf("arg %d: %q, want %q", i, split[i].Str, ToString(a))
		}
	}
}
