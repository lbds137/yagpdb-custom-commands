package runtime

import (
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

// YAGPDB's per-execution limits. Sources are in vendor/yagpdb (see the comments).
const (
	maxOutputBytes        = 25000            // common/templates/context.go: LimitWriter in executeParsed
	maxResponseRunes      = 2000             // ExecuteAndSendWithErrors replaces longer output
	maxSourceRunes        = 10000            // customcommands.MaxCCResponsesLength
	maxSourceRunesPremium = 20000            // customcommands.MaxCCResponsesLengthPremium
	maxDuration           = 10 * time.Second // custom command execution timeout
)

// callLimit is one of YAGPDB's Context.Counters with its normal and premium limits.
type callLimit struct {
	key             string
	normal, premium int
}

var (
	limitDB          = callLimit{"db_interactions", 10, 50}
	limitDBMultiple  = callLimit{"db_multiple", 2, 10}
	limitAPI         = callLimit{"api_call", 100, 100}
	limitRunCC       = callLimit{"runcc", 1, 10}
	limitCancelCC    = callLimit{"cancelcc", 10, 10}
	limitSendDM      = callLimit{"send_dm", 1, 1}
	limitSort        = callLimit{"sort", 1, 3}
	limitTicket      = callLimit{"ticket", 1, 1}
	limitReactTrig   = callLimit{"add_reaction_trigger", 20, 20}
	limitReactMsg    = callLimit{"add_reaction_message", 20, 20}
	limitDelReactMsg = callLimit{"del_reaction_message", 10, 10}
)

// limitedFunc lists the counters a template function increments, in YAGPDB's order.
// Silent functions return a zero value without error once over the limit (YAGPDB's
// sendMessage, for example, just stops sending), so the emulator always warns for them.
type limitedFunc struct {
	limits []callLimit
	silent bool
}

var limitedFuncs = map[string]limitedFunc{
	"dbGet":               {limits: []callLimit{limitDB}},
	"dbSet":               {limits: []callLimit{limitDB}},
	"dbSetExpire":         {limits: []callLimit{limitDB}},
	"dbDel":               {limits: []callLimit{limitDB}},
	"dbDelById":           {limits: []callLimit{limitDB}},
	"dbDelByID":           {limits: []callLimit{limitDB}},
	"dbIncr":              {limits: []callLimit{limitDB}},
	"dbGetPattern":        {limits: []callLimit{limitDB, limitDBMultiple}},
	"dbGetPatternReverse": {limits: []callLimit{limitDB, limitDBMultiple}},
	"dbCount":             {limits: []callLimit{limitDB, limitDBMultiple}},
	"dbTopEntries":        {limits: []callLimit{limitDB, limitDBMultiple}},
	"dbBottomEntries":     {limits: []callLimit{limitDB, limitDBMultiple}},
	"dbRank":              {limits: []callLimit{limitDB, limitDBMultiple}},

	"execCC":                  {limits: []callLimit{limitRunCC}},
	"scheduleUniqueCC":        {limits: []callLimit{limitRunCC}},
	"cancelScheduledUniqueCC": {limits: []callLimit{limitCancelCC}},
	"sort":                    {limits: []callLimit{limitSort}},
	"createTicket":            {limits: []callLimit{limitTicket}},

	"addReactions":              {limits: []callLimit{limitReactTrig}},
	"addMessageReactions":       {limits: []callLimit{limitReactMsg}},
	"deleteAllMessageReactions": {limits: []callLimit{limitAPI, limitDelReactMsg}},

	"sendMessage":      {limits: []callLimit{limitAPI}, silent: true},
	"sendMessageRetID": {limits: []callLimit{limitAPI}, silent: true},
	"sendDM":           {limits: []callLimit{limitSendDM, limitAPI}, silent: true},
	"giveRole":         {limits: []callLimit{limitAPI}, silent: true},
	"giveRoleID":       {limits: []callLimit{limitAPI}, silent: true},
	"takeRole":         {limits: []callLimit{limitAPI}, silent: true},
	"takeRoleID":       {limits: []callLimit{limitAPI}, silent: true},
	"hasRole":          {limits: []callLimit{limitAPI}, silent: true},
	"hasRoleID":        {limits: []callLimit{limitAPI}, silent: true},
	"mentionRole":      {limits: []callLimit{limitAPI}, silent: true},
	"mentionRoleID":    {limits: []callLimit{limitAPI}, silent: true},

	"editMessage":            {limits: []callLimit{limitAPI}},
	"getMessage":             {limits: []callLimit{limitAPI}},
	"getMember":              {limits: []callLimit{limitAPI}},
	"getChannel":             {limits: []callLimit{limitAPI}},
	"getChannelOrThread":     {limits: []callLimit{limitAPI}},
	"getTargetPermissionsIn": {limits: []callLimit{limitAPI}},
	"getRole":                {limits: []callLimit{limitAPI}},
	"userArg":                {limits: []callLimit{limitAPI}},
	"addRole":                {limits: []callLimit{limitAPI}},
	"addRoleID":              {limits: []callLimit{limitAPI}},
	"removeRole":             {limits: []callLimit{limitAPI}},
	"removeRoleID":           {limits: []callLimit{limitAPI}},
	"targetHasRole":          {limits: []callLimit{limitAPI}},
	"targetHasRoleID":        {limits: []callLimit{limitAPI}},
	"setRoles":               {limits: []callLimit{limitAPI}},
}

// countCall increments a counter the way YAGPDB's IncreaseCheckCallCounterPremium does
// and returns the error YAGPDB would once the count passes the limit.
func (ctx *ExecutionContext) countCall(fn string, l callLimit) error {
	ctx.Counters[l.key]++
	limit := l.normal
	if ctx.IsPremium {
		limit = l.premium
	}
	if ctx.Counters[l.key] <= limit {
		return nil
	}
	err := ErrTooManyCalls
	if l.key == limitAPI.key {
		err = ErrTooManyAPICalls
	}
	return fmt.Errorf("%w (%s: over the limit of %d %s calls per run)", err, fn, limit, l.key)
}

// limitBreach handles a breached limit: strict mode returns the error, otherwise it is
// recorded as a warning and execution continues.
func (ctx *ExecutionContext) limitBreach(err error) error {
	if ctx.Strict {
		return err
	}
	ctx.Warn(KindLimit, "%v", err)
	return nil
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// withLimits wraps a template function so each call counts against YAGPDB's call limits.
// The wrapper returns (T, error), which text/template turns into an execution error.
// Functions YAGPDB doesn't limit are returned unchanged.
func (e *Engine) withLimits(name string, fn interface{}) interface{} {
	spec, ok := limitedFuncs[name]
	if !ok {
		return fn
	}

	v := reflect.ValueOf(fn)
	t := v.Type()
	ins := make([]reflect.Type, t.NumIn())
	for i := range ins {
		ins[i] = t.In(i)
	}
	outs := []reflect.Type{t.Out(0), errorType}

	wrapped := reflect.MakeFunc(reflect.FuncOf(ins, outs, t.IsVariadic()), func(args []reflect.Value) []reflect.Value {
		// The first counter over its limit stops the call, like YAGPDB's early returns.
		// Warnings are recorded once per counter, when it first passes the limit.
		for _, l := range spec.limits {
			err := e.ctx.countCall(name, l)
			if err == nil {
				continue
			}
			firstBreach := e.ctx.Counters[l.key] == limitFor(l, e.ctx.IsPremium)+1
			if spec.silent {
				if firstBreach {
					e.ctx.Warn(KindLimit, "%v; YAGPDB skips this call silently", err)
				}
				if e.ctx.Strict {
					return []reflect.Value{reflect.Zero(outs[0]), reflect.Zero(errorType)}
				}
				break
			}
			if e.ctx.Strict {
				return []reflect.Value{reflect.Zero(outs[0]), reflect.ValueOf(&err).Elem()}
			}
			if firstBreach {
				e.ctx.Warn(KindLimit, "%v", err)
			}
			break
		}

		var res []reflect.Value
		if t.IsVariadic() {
			res = v.CallSlice(args)
		} else {
			res = v.Call(args)
		}
		if len(res) == 1 {
			res = append(res, reflect.Zero(errorType))
		}
		return res
	})
	return wrapped.Interface()
}

func limitFor(l callLimit, premium bool) int {
	if premium {
		return l.premium
	}
	return l.normal
}

// checkSourceLength applies YAGPDB's limit on a custom command's length.
func (ctx *ExecutionContext) checkSourceLength(source string) error {
	limit := maxSourceRunes
	if ctx.IsPremium {
		limit = maxSourceRunesPremium
	}
	n := utf8.RuneCountInString(strings.ReplaceAll(source, "\r", ""))
	if n <= limit {
		return nil
	}
	return ctx.limitBreach(fmt.Errorf("the template is %d characters; YAGPDB refuses to save a custom command over %d", n, limit))
}

// checkOutput applies YAGPDB's output limits and returns the output YAGPDB would send.
func (ctx *ExecutionContext) checkOutput(output string, elapsed time.Duration) (string, error) {
	if elapsed > maxDuration {
		if err := ctx.limitBreach(fmt.Errorf("execution took %s; YAGPDB stops custom commands after %s", elapsed.Round(time.Millisecond), maxDuration)); err != nil {
			return output, err
		}
	}
	if len(output) > maxOutputBytes {
		if err := ctx.limitBreach(fmt.Errorf("response grew too big (>25k): the template printed %d bytes", len(output))); err != nil {
			return output, err
		}
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(output)); n > maxResponseRunes {
		if ctx.Strict {
			return "Template output for " + ctx.Cmd + " was longer than 2k (contact an admin on the server...)", nil
		}
		ctx.Warn(KindLimit, "the response is %d characters; YAGPDB replaces responses over %d with a notice", n, maxResponseRunes)
	}
	return output, nil
}
