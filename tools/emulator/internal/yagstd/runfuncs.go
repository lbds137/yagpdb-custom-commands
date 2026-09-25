// Emulator code, not copied from YAGPDB: the functions that read the system clock or
// random source, built for one run so a test can fix them (a test's clock: and seed:).

package templates

import (
	"math/rand"
	"time"
)

// Random is where the random functions draw from: math/rand's global source, as in
// YAGPDB, or a seeded *rand.Rand.
type Random interface {
	Int63n(n int64) int64
	Intn(n int) int
	Perm(n int) []int
}

// GlobalRandom is math/rand's global source.
type GlobalRandom struct{}

func (GlobalRandom) Int63n(n int64) int64 { return rand.Int63n(n) }
func (GlobalRandom) Intn(n int) int       { return rand.Intn(n) }
func (GlobalRandom) Perm(n int) []int     { return rand.Perm(n) }

// RunFuncs returns YAGPDB's clock and random functions, reading now and random.
func RunFuncs(now func() time.Time, random Random) map[string]interface{} {
	return map[string]interface{}{
		"currentTime":           tmplCurrentTime(now),
		"humanizeTimeSinceDays": tmplHumanizeTimeSinceDays(now),
		"randInt":               randInt(random),
		"shuffle":               shuffle(random),
		"adjective":             RandomAdjective(random),
		"noun":                  RandomNoun(random),
		"verb":                  RandomVerb(random),
	}
}
