package loader

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// Decoded with types.StrictYAML, as test files are
func TestClockAndSeedValues(t *testing.T) {
	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	for src, want := range map[string]struct {
		clock time.Time
		seed  int64
	}{
		"clock: 2026-01-02\nseed: -7":                                {day, -7},
		"clock: 2026-01-02 15:04:05\nseed: 0x10":                     {day.Add(15*time.Hour + 4*time.Minute + 5*time.Second), 16},
		"clock: \"2026-01-02T00:00:00Z\"\nseed: 9223372036854775807": {day, math.MaxInt64},
	} {
		var c ContextDef
		if err := types.StrictYAML([]byte(src), &c); err != nil {
			t.Errorf("%q: %v", src, err)
			continue
		}
		if got := time.Time(*c.Clock); !got.Equal(want.clock) || int64(*c.Seed) != want.seed {
			t.Errorf("%q: got clock %v seed %d", src, got, *c.Seed)
		}
	}

	var unset ContextDef
	if err := types.StrictYAML([]byte("clock: null\nseed:\n"), &unset); err != nil || unset.Clock != nil || unset.Seed != nil {
		t.Errorf("null leaves them unset: %v %v %v", err, unset.Clock, unset.Seed)
	}

	for src, want := range map[string]string{
		"user: {}\nclock: 2026-13-01T00:00:00Z": "line 2: clock: parsing time",
		"clock: tomorrow":                       "line 1: clock:",
		"clock: {a: 1}":                         "line 1: clock: write a time",
		"clock: {}":                             "line 1: clock: write a time",
		"seed: 1.5":                             `line 1: seed: "1.5" isn't an integer`,
		"seed: 1e3":                             `line 1: seed: "1e3" isn't an integer`,
		"seed: abc":                             `line 1: seed: "abc" isn't an integer`,
		"seed: '3'":                             `line 1: seed: "3" isn't an integer`,
		"seed: 99999999999999999999":            "fits in 64 bits",
		"seed: [1]":                             "line 1: seed: write an integer",
	} {
		var c ContextDef
		err := types.StrictYAML([]byte(src), &c)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: want an error containing %q, got %v", src, want, err)
		}
	}
}
