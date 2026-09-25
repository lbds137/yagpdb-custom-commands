package state

import (
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
	for _, e := range db.TopEntries("xp", 10, 0, false) {
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
	}
	for _, c := range cases {
		if got := matchPattern(c.s, c.pattern); got != c.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", c.s, c.pattern, got, c.want)
		}
	}
}
