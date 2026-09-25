package runtime

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// YAGPDB's per-execution limits. Sources are in vendor/yagpdb (see the comments).
const (
	maxOutputBytes        = 25000            // common/templates/context.go: LimitWriter in executeParsed
	maxOutputBytesLenient = 1 << 20          // outside -strict: warn past 25k, stop at 1 MiB
	maxResponseRunes      = 2000             // customcommands/bot.go replaces longer responses
	maxSourceRunes        = 10000            // customcommands.MaxCCResponsesLength
	maxSourceRunesPremium = 20000            // customcommands.MaxCCResponsesLengthPremium
	maxDuration           = 10 * time.Second // custom command execution timeout
	maxOpsNormal          = 1000000          // common/templates/context.go: MaxOpsNormal
	maxOpsPremium         = 2500000          // MaxOpsPremium
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
	limitExecChild   = callLimit{"exec_child", 3, 3}
	limitExec        = callLimit{"exec", 5, 5} // commands/tmplexec.go: maxExec, shared by exec and execAdmin
	limitReactTrig   = callLimit{"add_reaction_trigger", 20, 20}
	limitReactMsg    = callLimit{"add_reaction_message", 20, 20}
	limitDelReactMsg = callLimit{"del_reaction_message", 10, 10}
)

var errMaxExec = errors.New("Max number of commands executed in custom command")

// limitedFunc lists the counters a template function increments, in YAGPDB's order.
// Silent functions return a zero value without error once over the limit (YAGPDB's
// sendMessage, for example, just stops sending). check replaces limits for functions
// whose counting depends on their arguments.
type limitedFunc struct {
	limits []callLimit
	silent bool
	check  func(ctx *ExecutionContext, name string, args []reflect.Value) error
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
	"dbDelMultiple":       {limits: []callLimit{limitDB, limitDBMultiple}},

	"execCC":                  {limits: []callLimit{limitRunCC}},
	"scheduleUniqueCC":        {limits: []callLimit{limitRunCC}},
	"cancelScheduledUniqueCC": {limits: []callLimit{limitCancelCC}},
	"exec":                    {limits: []callLimit{limitExec}},
	"execAdmin":               {limits: []callLimit{limitExec}},
	"sendTemplate":            {limits: []callLimit{limitExecChild}},
	"sort":                    {limits: []callLimit{limitSort}},
	"createTicket":            {limits: []callLimit{limitTicket}},

	// One count per emoji (context_funcs.go tmplAddReactions / tmplAddMessageReactions)
	"addReactions":        {check: perEmoji(0, limitReactTrig)},
	"addMessageReactions": {check: perEmoji(2, limitReactMsg)},
	// Per emoji when emoji are given, otherwise one API call (tmplDelAllMessageReactions)
	"deleteAllMessageReactions": {check: checkDeleteReactions},
	// One API call, and one call per target user (tmplSetRoles)
	"setRoles": {check: checkSetRoles},

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
	"editMessageNoEscape":    {limits: []callLimit{limitAPI}},
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
}

// countCall increments a counter the way YAGPDB's IncreaseCheckCallCounterPremium does
// and returns the error YAGPDB would once the count passes the limit.
func (ctx *ExecutionContext) countCall(fn string, l callLimit) error {
	limit := l.normal
	if ctx.IsPremium {
		limit = l.premium
	}
	base := ErrTooManyCalls
	switch l.key {
	case limitAPI.key:
		base = ErrTooManyAPICalls
	case limitExec.key:
		base = errMaxExec
	}
	return ctx.count(fn, l.key, limit, base)
}

func (ctx *ExecutionContext) count(fn, key string, limit int, base error) error {
	ctx.Counters[key]++
	if ctx.Counters[key] <= limit {
		return nil
	}
	return fmt.Errorf("%w (%s: over the limit of %d %s calls per run)", base, fn, limit, key)
}

// perEmoji counts one call per emoji argument from index first on; slices of emoji are
// flattened, as YAGPDB's callVariadic does.
func perEmoji(first int, l callLimit) func(*ExecutionContext, string, []reflect.Value) error {
	return func(ctx *ExecutionContext, name string, args []reflect.Value) error {
		for i := 0; i < countFlattened(args, first); i++ {
			if err := ctx.countCall(name, l); err != nil {
				return err
			}
		}
		return nil
	}
}

func checkDeleteReactions(ctx *ExecutionContext, name string, args []reflect.Value) error {
	if n := countFlattened(args, 2); n > 0 {
		for i := 0; i < n; i++ {
			if err := ctx.countCall(name, limitDelReactMsg); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.countCall(name, limitAPI)
}

func checkSetRoles(ctx *ExecutionContext, name string, args []reflect.Value) error {
	if err := ctx.countCall(name, limitAPI); err != nil {
		return err
	}
	target := ctx.UserID
	if all := flatArgs(args); len(all) > 0 && all[0] != nil {
		target = targetUserID(all[0])
	}
	if target == 0 {
		return nil // YAGPDB returns early without counting
	}
	return ctx.count(name, fmt.Sprintf("set_roles%d", target), 1,
		errors.New("too many calls for specific user ID (max 1 / user)"))
}

// flatArgs unpacks a wrapped function's arguments, including a variadic slice.
func flatArgs(args []reflect.Value) []interface{} {
	var out []interface{}
	for i, a := range args {
		if i == len(args)-1 && a.Kind() == reflect.Slice && a.Type().Elem().Kind() == reflect.Interface {
			for j := 0; j < a.Len(); j++ {
				out = append(out, a.Index(j).Interface())
			}
			continue
		}
		out = append(out, a.Interface())
	}
	return out
}

// countFlattened counts arguments from index first on, counting each element of a slice
// and skipping nils, as YAGPDB's callVariadic does.
func countFlattened(args []reflect.Value, first int) int {
	n := 0
	for i, a := range flatArgs(args) {
		if i < first || a == nil {
			continue
		}
		if v := reflect.ValueOf(a); (v.Kind() == reflect.Slice || v.Kind() == reflect.Array) && v.Type().Elem().Kind() != reflect.Uint8 {
			for j := 0; j < v.Len(); j++ {
				if v.Index(j).Interface() != nil {
					n++
				}
			}
		} else {
			n++
		}
	}
	return n
}

// targetUserID follows YAGPDB's TargetUserID: a user, a mention ("<@id>" or "<@!id>"),
// or anything ToInt64 understands. A member (.Member) isn't accepted and gives 0, as in
// YAGPDB, where that makes setRoles a no-op and getMember nil.
func targetUserID(input interface{}) int64 {
	switch t := input.(type) {
	case types.DiscordUser:
		return t.ID
	case *types.DiscordUser:
		return t.ID
	case string:
		s := strings.TrimSpace(t)
		if strings.HasPrefix(s, "<@") && strings.HasSuffix(s, ">") && len(s) > 4 {
			s = strings.TrimPrefix(s[2:len(s)-1], "!")
		}
		return funcs.ToInt64(s)
	default:
		return funcs.ToInt64(input)
	}
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

// warnOnce records a limit warning unless the same one was already recorded this run.
func (ctx *ExecutionContext) warnOnce(msg string) {
	if ctx.warned == nil {
		ctx.warned = map[string]bool{}
	}
	if !ctx.warned[msg] {
		ctx.warned[msg] = true
		ctx.Warn(KindLimit, "%s", msg)
	}
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
		var err error
		if spec.check != nil {
			err = spec.check(e.ctx, name, args)
		} else {
			for _, l := range spec.limits {
				if err = e.ctx.countCall(name, l); err != nil {
					break
				}
			}
		}

		if err != nil {
			if spec.silent {
				e.ctx.warnOnce(err.Error() + "; YAGPDB skips this call silently")
				if e.ctx.Strict {
					return []reflect.Value{reflect.Zero(outs[0]), reflect.Zero(errorType)}
				}
			} else {
				if e.ctx.Strict {
					return []reflect.Value{reflect.Zero(outs[0]), reflect.ValueOf(&err).Elem()}
				}
				e.ctx.warnOnce(err.Error() + "; YAGPDB stops the command here")
			}
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

// checkOutput applies YAGPDB's output limits to a finished run and returns the output
// YAGPDB would send. overCap is set when YAGPDB's output writer would have failed
// (outside strict mode, where the run went on).
func (ctx *ExecutionContext) checkOutput(output string, elapsed time.Duration, overCap bool) (string, error) {
	if elapsed > maxDuration {
		if err := ctx.limitBreach(fmt.Errorf("execution took %s; YAGPDB stops custom commands after %s", elapsed.Round(time.Millisecond), maxDuration)); err != nil {
			return ctx.response(output), err
		}
	}
	if overCap {
		if err := ctx.limitBreach(fmt.Errorf("response grew too big (>25k): the template printed %d bytes", len(output))); err != nil {
			return output, err
		}
	}
	return ctx.response(output), nil
}

// response is the output as YAGPDB sends it (customcommands/bot.go): trimmed, and over
// 2000 characters replaced by a notice; outside strict mode the full output is kept and
// the notice is a warning.
func (ctx *ExecutionContext) response(output string) string {
	output = strings.TrimSpace(output)
	if n := utf8.RuneCountInString(output); n > maxResponseRunes {
		if ctx.Strict {
			return fmt.Sprintf("Custom command (#%d) response was longer than 2k (contact an admin on the server...)", ctx.CCID)
		}
		ctx.Warn(KindLimit, "the response is %d characters; YAGPDB replaces responses over %d with a notice", n, maxResponseRunes)
	}
	return output
}

// maxOps is YAGPDB's operation limit for this run.
func (ctx *ExecutionContext) maxOps() int {
	if ctx.IsPremium {
		return maxOpsPremium
	}
	return maxOpsNormal
}

// shadowWriter writes to shadow, remembering its first error instead of returning it,
// then to w. Like text/template, it ignores byte counts: YAGPDB's LimitWriter reports 0
// bytes for whitespace it drops, which io.MultiWriter would take as a short write.
type shadowWriter struct {
	shadow io.Writer
	err    error
	w      io.Writer
}

func (s *shadowWriter) Write(p []byte) (int, error) {
	if s.err == nil {
		_, s.err = s.shadow.Write(p)
	}
	_, err := s.w.Write(p)
	return len(p), err
}

// Discord's limits on a sent message (discord.com/developers/docs/resources/message#embed-object-embed-limits).
// YAGPDB doesn't check them: Discord rejects the send with HTTP 400 "Invalid Form Body",
// and sendMessage returns that as the command's error. cembed itself never fails on them.
const (
	maxContentRunes     = 2000
	maxEmbedTitle       = 256
	maxEmbedDescription = 4096
	maxEmbedFields      = 25
	maxEmbedFieldName   = 256
	maxEmbedFieldValue  = 1024
	maxEmbedFooter      = 2048
	maxEmbedAuthor      = 256
	maxEmbedTotal       = 6000 // title, description, field names and values, footer and author, over all embeds
)

// checkSend applies Discord's message limits to a send and reports whether to record it.
// sendMessage gets Discord's error back, so strict mode returns it; sendDM discards the
// error (context_funcs.go), so a rejected DM is silently not delivered. Outside strict
// mode a breach is a warning and the message is recorded anyway.
func (ctx *ExecutionContext) checkSend(fn, content string, embeds []interface{}, notEmpty, silent bool) (bool, error) {
	problems := sendProblems(content, embeds, notEmpty)
	if len(problems) == 0 {
		return true, nil
	}
	reason := "HTTP 400 Invalid Form Body"
	if problems[0] == msgEmpty {
		reason = "HTTP 400, 50006 Cannot send an empty message"
	}
	err := fmt.Errorf("%s: Discord rejects this message (%s): %s", fn, reason, strings.Join(problems, "; "))
	if !ctx.Strict {
		ctx.warnOnce(err.Error())
		return true, nil
	}
	if silent {
		ctx.warnOnce(err.Error() + "; YAGPDB skips this call silently")
		return false, nil
	}
	return false, err
}

// discordRefuses reports a call Discord would answer with an error: an error with
// -strict, otherwise a warning (and the call does nothing).
func (ctx *ExecutionContext) discordRefuses(fn, reason, detail string) error {
	err := fmt.Errorf("%s: Discord refuses this (%s): %s", fn, reason, detail)
	if !ctx.Strict {
		ctx.warnOnce(err.Error())
		return nil
	}
	return err
}

const msgEmpty = "the message is empty"

// sendProblems lists what Discord would reject in a message. notEmpty is set when the
// message has a file, components or anything else that counts as content.
func sendProblems(content string, embeds []interface{}, notEmpty bool) []string {
	var problems []string
	// discordgo drops empty embeds (ValidateComplexMessageEmbeds), and Discord refuses a
	// message with nothing left (50006 "Cannot send an empty message").
	empty := strings.TrimSpace(content) == "" && !notEmpty
	for _, e := range embeds {
		if embed, ok := e.(types.Embed); ok && len(embed) > 0 {
			empty = false
		}
	}
	if empty {
		return []string{msgEmpty}
	}
	over := func(what string, s string, limit int) int {
		n := utf8.RuneCountInString(s)
		if n > limit {
			problems = append(problems, fmt.Sprintf("%s is %d characters (max %d)", what, n, limit))
		}
		return n
	}
	over("content", content, maxContentRunes)

	total := 0
	for i, raw := range embeds {
		embed, ok := raw.(types.Embed)
		if !ok {
			continue
		}
		prefix := fmt.Sprintf("embed %d ", i+1)
		str := func(m map[string]interface{}, key string) string {
			s, _ := m[key].(string)
			return s
		}
		total += over(prefix+"title", str(embed, "title"), maxEmbedTitle)
		total += over(prefix+"description", str(embed, "description"), maxEmbedDescription)
		if footer, ok := embed["footer"].(map[string]interface{}); ok {
			total += over(prefix+"footer text", str(footer, "text"), maxEmbedFooter)
		}
		if author, ok := embed["author"].(map[string]interface{}); ok {
			total += over(prefix+"author name", str(author, "name"), maxEmbedAuthor)
		}
		fields, _ := embed["fields"].([]interface{})
		if len(fields) > maxEmbedFields {
			problems = append(problems, fmt.Sprintf("%shas %d fields (max %d)", prefix, len(fields), maxEmbedFields))
		}
		for j, raw := range fields {
			field, _ := raw.(map[string]interface{})
			fp := fmt.Sprintf("%sfield %d ", prefix, j+1)
			name, value := str(field, "name"), str(field, "value")
			// Discord requires both and trims them first, so whitespace alone is empty
			// (a zero-width space is the usual blank).
			if strings.TrimSpace(name) == "" {
				problems = append(problems, fp+"name is empty")
			}
			if strings.TrimSpace(value) == "" {
				problems = append(problems, fp+"value is empty")
			}
			total += over(fp+"name", name, maxEmbedFieldName)
			total += over(fp+"value", value, maxEmbedFieldValue)
		}
	}
	if total > maxEmbedTotal {
		problems = append(problems, fmt.Sprintf("the embeds total %d characters (max %d)", total, maxEmbedTotal))
	}
	return problems
}
