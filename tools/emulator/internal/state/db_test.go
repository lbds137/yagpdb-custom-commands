package state

import (
	"fmt"
	"testing"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

func TestIncrFollowsYAGPDBUpsert(t *testing.T) {
	db := NewMockDB(1)

	if v, _ := db.Incr(1, "n", 2); v != 2 {
		t.Errorf("new entry: got %v, want 2", v)
	}
	if v, _ := db.Incr(1, "n", 3); v != 5 {
		t.Errorf("existing number: got %v, want 5", v)
	}
	if got := db.Get(1, "n").Value; got != 5.0 {
		t.Errorf("a stored number reads as value_num: got %#v", got)
	}

	// A string's value_num is its parsed number; incrementing leaves the string itself
	db.Set(1, "s", "5")
	if v, _ := db.Incr(1, "s", 1); v != 6 {
		t.Errorf("string: dbIncr returns value_num 6, got %v", v)
	}
	if got := db.Get(1, "s").Value; got != "5" {
		t.Errorf("string: dbGet still returns the raw value, got %#v", got)
	}

	// A dict counts as 0 and stays a dict
	db.Set(1, "d", types.SDict{"a": 1})
	if v, _ := db.Incr(1, "d", 1); v != 1 {
		t.Errorf("dict: got %v, want 1", v)
	}
	if _, ok := db.Get(1, "d").Value.(types.SDict); !ok {
		t.Errorf("dict: raw value should stay a dict, got %#v", db.Get(1, "d").Value)
	}
}

func TestIncrKeepsExpiryAndRestartsExpired(t *testing.T) {
	db := NewMockDB(1)
	db.SetWithExpiry(1, "live", 10, 3600)
	db.Incr(1, "live", 1)
	if e := db.Get(1, "live"); e == nil || e.ExpiresAt.IsZero() || e.Value != 11.0 {
		t.Errorf("live entry keeps its expiry: %+v", e)
	}

	db.SetWithExpiry(1, "old", 10, 3600)
	db.entries[makeKey(1, "old")].ExpiresAt = time.Now().Add(-time.Minute)
	if v, _ := db.Incr(1, "old", 4); v != 4 {
		t.Errorf("expired entry restarts at the amount: got %v", v)
	}
	if e := db.Get(1, "old"); e == nil || !e.ExpiresAt.IsZero() {
		t.Errorf("expired entry loses its expiry: %+v", e)
	}
}

func TestTopEntriesUseValueNum(t *testing.T) {
	db := NewMockDB(1)
	db.Set(1, "xp", 10)
	db.Set(2, "xp", "50") // a numeric string ranks by its number
	db.Set(3, "xp", 30)
	var got []int64
	entries, err := db.TopEntries("xp", 10, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		got = append(got, e.UserID)
	}
	if len(got) != 3 || got[0] != 2 || got[1] != 3 || got[2] != 1 {
		t.Errorf("order = %v, want [2 3 1]", got)
	}
}

func TestMatchPatternIsLike(t *testing.T) {
	cases := []struct {
		s, pattern string
		want       bool
	}{
		{"score_ann", "score_%", true},
		{"scoreXann", "score_%", true}, // _ is any single character
		{"score_ann", `score\_%`, true},
		{"scoreXann", `score\_%`, false},
		{"a-middle-z", "a%z", true},
		{"has middle here", "%middle%", true},
		{"Score", "score", false}, // case-sensitive
		{"abc", "ab", false},      // whole key
		{"a.c", "a.c", true},      // regex metacharacters are literal
		{"abc", "a.c", false},
		{"héllo", "h_llo", true}, // _ is one character, not one byte
		{"héllo", "h__llo", false},
		{"", "%", true},
		{"", "", true},
		{"a%b", `a\%b`, true},
		{"axb", `a\%b`, false},
		{"é", "%__", false}, // the _ after a % steps by character
		{"a", "a%", true},   // trailing %'s match the end of the text
	}
	for _, c := range cases {
		if got, err := matchPattern(c.s, c.pattern); got != c.want || err != nil {
			t.Errorf("matchPattern(%q, %q) = %v, %v; want %v", c.s, c.pattern, got, err, c.want)
		}
	}
}

// Postgres raises the trailing-escape error only when matching reaches the escape
// with text left (like_match.c)
func TestLikeTrailingEscape(t *testing.T) {
	cases := []struct {
		s, pattern string
		fails      bool
	}{
		{"abcd", `abc\`, true},
		{"x", `%\`, true},
		{"abc", `abc\`, false}, // the text ends first: no match, no error
		{"xyz", `abc\`, false}, // a mismatch before the escape
		{"", `%\`, false},
		{"ab", `\`, true},
	}
	for _, c := range cases {
		got, err := matchPattern(c.s, c.pattern)
		if got || (err != nil) != c.fails || (err != nil && err != ErrLikeEscape) {
			t.Errorf("matchPattern(%q, %q) = %v, %v; want an error: %v", c.s, c.pattern, got, err, c.fails)
		}
	}

	// A failing statement changes nothing
	db := NewMockDB(1)
	db.Set(1, "abcd", 1)
	if _, err := db.DelMultiple(nil, ptr(`abc\`), false, 100, 0); err != ErrLikeEscape {
		t.Errorf("got %v", err)
	}
	if db.Get(1, "abcd") == nil {
		t.Error("the failed delete removed the entry")
	}
}

func ptr(s string) *string { return &s }

// Postgres sorts NaN above every number, Infinity included, and NaN equals NaN (so the
// id breaks the tie)
func TestNaNSortsHighest(t *testing.T) {
	db := NewMockDB(1)
	for user, v := range []string{"5", "NaN", "Inf", "-1", "NaN"} {
		db.Set(int64(user), "k", v)
	}
	for _, c := range []struct {
		ascending bool
		want      string
	}{{false, "[4 1 2 0 3]"}, {true, "[3 0 2 1 4]"}} {
		entries, err := db.TopEntries("k", 10, 0, c.ascending)
		if err != nil {
			t.Fatal(err)
		}
		var users []int64
		for _, e := range entries {
			users = append(users, e.UserID)
		}
		if got := fmt.Sprint(users); got != c.want {
			t.Errorf("ascending %v: %s, want %s", c.ascending, got, c.want)
		}
	}
}
