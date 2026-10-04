package runtime

import (
	"errors"
	"slices"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
	yagstd "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// ScheduledRun is a custom command run that execCC with a delay or scheduleUniqueCC set up.
// It happens after the test's run, so the emulator records it instead of running it.
type ScheduledRun struct {
	CCID      int64
	ChannelID int64
	Delay     time.Duration
	Key       *string     // scheduleUniqueCC's key; nil for execCC
	ExecData  interface{} // as the run gets it, after YAGPDB's msgpack round trip
	// Interaction is the state of the scheduling run's interaction when it was scheduled:
	// YAGPDB stores the caller's CurrentFrame (Interaction and RespondedTo) with the run
	// (customcommands/tmplextensions.go:256-282 for execCC, :344 for scheduleUniqueCC),
	// and the delayed run restores it (handle_timed.go:102-117).
	Interaction ScheduledInteraction
}

// ScheduledInteraction is what a scheduled run carries of its scheduler's interaction.
type ScheduledInteraction string

const (
	// ScheduledNoInteraction: the scheduling run had no interaction (a message, a reaction...)
	ScheduledNoInteraction ScheduledInteraction = "none"
	// ScheduledInteractionPending: it had one, not yet responded to; the delayed run's
	// response would be the interaction's first (and the token has long expired, in
	// practice: Discord gives 3 seconds)
	ScheduledInteractionPending ScheduledInteraction = "pending"
	// ScheduledInteractionResponded: it had one that was already responded to (or
	// deferred), so the delayed run's response is a followup
	ScheduledInteractionResponded ScheduledInteraction = "responded"
)

// scheduledInteraction reads the run's interaction state, as the CurrentFrame stored at
// schedule time would hold it.
func (ctx *ExecutionContext) scheduledInteraction() ScheduledInteraction {
	switch {
	case ctx.Interaction == nil:
		return ScheduledNoInteraction
	case ctx.Interaction.RespondedTo:
		return ScheduledInteractionResponded
	}
	return ScheduledInteractionPending
}

// ccMaxDataLimit is YAGPDB's CCMaxDataLimit, execCC's cap on encoded data.
const ccMaxDataLimit = 1000000

// scheduledRuns is the list of runs scheduled so far, shared with execCC children so a
// scheduleUniqueCC in one replaces or a cancel removes a run another scheduled.
func (ctx *ExecutionContext) scheduledRuns() *[]ScheduledRun {
	if ctx.scheduled == nil {
		ctx.scheduled = new([]ScheduledRun)
	}
	return ctx.scheduled
}

// ScheduledRuns returns the runs the command scheduled (and didn't cancel), in order.
func (ctx *ExecutionContext) ScheduledRuns() []ScheduledRun {
	return *ctx.scheduledRuns()
}

// schedule records a delayed run as YAGPDB schedules it; key is nil for execCC, which
// caps the data's size.
func (ctx *ExecutionContext) schedule(ccID, channelID int64, delay interface{}, key *string, data interface{}) error {
	run := ScheduledRun{
		CCID:      ccID,
		ChannelID: channelID,
		Delay:     time.Second * time.Duration(yagstd.ToInt64(delay)),
		Key:       key,

		Interaction: ctx.scheduledInteraction(),
	}
	if data != nil {
		decoded, size, err := state.RoundTripExecData(data)
		if err != nil {
			return err
		}
		if key == nil && size > ccMaxDataLimit {
			return errors.New("ExecData is too big")
		}
		run.ExecData = decoded
	}
	if key != nil {
		ctx.cancelScheduled(ccID, *key)
	}
	runs := ctx.scheduledRuns()
	*runs = append(*runs, run)
	return nil
}

// cancelScheduled removes the unique runs with this command and key.
func (ctx *ExecutionContext) cancelScheduled(ccID int64, key string) {
	runs := ctx.scheduledRuns()
	*runs = slices.DeleteFunc(*runs, func(r ScheduledRun) bool {
		return r.CCID == ccID && r.Key != nil && *r.Key == key
	})
}

// scheduleUniqueCC is YAGPDB's tmplScheduleUniqueCC: a delayed run that replaces any run
// scheduled for the same command and key. A delay of 0 or less does nothing.
func (e *Engine) scheduleUniqueCC(ccID int, channel, delay, key, data interface{}) (string, error) {
	if _, _, _, err := e.findCC("scheduleUniqueCC", int64(ccID)); err != nil {
		return "", err
	}
	channelID := e.channelArg(channel)
	if channelID == 0 { // checked before the delay, as in YAGPDB
		return "", errors.New("Unknown channel")
	}
	if yagstd.ToInt64(delay) <= 0 {
		return "", nil
	}
	k := yagstd.ToString(key)
	return "", e.ctx.schedule(int64(ccID), channelID, delay, &k, data)
}

// cancelScheduledUniqueCC is YAGPDB's tmplCancelUniqueCC.
func (e *Engine) cancelScheduledUniqueCC(ccID int, key interface{}) string {
	e.ctx.cancelScheduled(int64(ccID), yagstd.ToString(key))
	return ""
}
