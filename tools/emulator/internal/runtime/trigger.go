package runtime

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
)

// DefaultPrefix is YAGPDB's default command prefix.
const DefaultPrefix = "-"

// Trigger is a custom command's trigger, with the type named as the control panel names it
// ("Command", "Starts with", "Contains", "Regex", "Exact match", "Reaction", "None", ...).
// Triggers are case-insensitive, YAGPDB's default, unless the header says
// "Case sensitive: `true`" (the control panel's checkbox).
type Trigger struct {
	Type          string
	Text          string
	CaseSensitive bool
}

var (
	headerTriggerType = regexp.MustCompile("Trigger type: `([^`]*)`")
	headerTrigger     = regexp.MustCompile("Trigger: `([^`]*)`")
	headerCase        = regexp.MustCompile("Case sensitive: `([^`]*)`")
)

// ReadTrigger reads a command's trigger from its header comment ("Trigger type: `Command`",
// "Trigger: `db`"). ok is false if the header names no trigger type.
func ReadTrigger(source string) (t Trigger, ok bool) {
	m := headerTriggerType.FindStringSubmatch(source)
	if m == nil {
		return Trigger{}, false
	}
	t.Type = m[1]
	if m := headerTrigger.FindStringSubmatch(source); m != nil {
		t.Text = m[1]
	}
	if m := headerCase.FindStringSubmatch(source); m != nil {
		t.CaseSensitive = strings.EqualFold(m[1], "true")
	}
	return t, true
}

// Scheduled reports whether the trigger runs the command on a schedule (an interval or
// cron): no message or member starts such a run.
func (t Trigger) Scheduled() bool {
	return strings.HasSuffix(strings.ToLower(t.Type), "interval") || strings.EqualFold(t.Type, "Cron")
}

// MessageTriggered reports whether the trigger runs the command on messages.
func (t Trigger) MessageTriggered() bool {
	switch t.Type {
	case "Command", "Starts with", "Contains", "Regex", "Exact match":
		return true
	}
	return false
}

// CheckMatch is YAGPDB's customcommands.CheckMatch: whether msg triggers the command, the
// message after the trigger, and the arguments (the first being the message up to and
// including the trigger). prefix is the server's command prefix.
func CheckMatch(prefix string, t Trigger, msg string) (match bool, stripped string, args []string) {
	cmdMatch := "(?m)"
	if !t.CaseSensitive {
		cmdMatch += "(?i)"
	}

	switch t.Type {
	case "Command":
		// Regex is:
		// \A(<@!?bot_id> ?|server_cmd_prefix)trigger(\z|[[:space:]])
		cmdMatch += `\A(<@!?` + strconv.FormatInt(botUser.ID, 10) + "> ?|" + regexp.QuoteMeta(prefix) + ")" + regexp.QuoteMeta(t.Text) + `(\z|[[:space:]])`
	case "Starts with":
		cmdMatch += `\A` + regexp.QuoteMeta(t.Text)
	case "Contains":
		cmdMatch += regexp.QuoteMeta(t.Text)
	case "Regex":
		cmdMatch += t.Text
	case "Exact match":
		cmdMatch += `\A` + regexp.QuoteMeta(t.Text) + `\z`
	default:
		return false, "", nil
	}

	return matchRegexSplitArgs(cmdMatch, msg)
}

// matchRegexSplitArgs is YAGPDB's, without its regex cache.
func matchRegexSplitArgs(pattern, msg string) (match bool, stripped string, args []string) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, "", nil
	}

	idx := re.FindStringIndex(msg)
	if idx == nil {
		return false, "", nil
	}

	argsRaw := funcs.SplitArgs(msg[idx[1]:])
	args = make([]string, len(argsRaw)+1)
	args[0] = strings.TrimSpace(msg[:idx[1]])
	for i, v := range argsRaw {
		args[i+1] = v.Str
	}

	stripped = msg[idx[1]:]
	return true, stripped, args
}

// SetTriggerMessage makes msg the message that triggered the command, and builds .Args,
// .Cmd, .CmdArgs and .StrippedMsg from it as YAGPDB's ExecuteCustomCommandFromMessage does:
// .Args is the whole message split, trigger included. It errors if msg doesn't trigger t.
func (ctx *ExecutionContext) SetTriggerMessage(t Trigger, msg string) error {
	match, stripped, cmdArgs := CheckMatch(ctx.Prefix, t, msg)
	if !match {
		return fmt.Errorf("the message %q doesn't match the %s trigger %q", msg, t.Type, t.Text)
	}

	ctx.MessageContent = msg
	args := funcs.SplitArgs(msg)
	ctx.Args = make([]interface{}, len(args))
	for i, a := range args {
		ctx.Args[i] = a.Str
	}
	ctx.Cmd = cmdArgs[0]
	ctx.CmdArgs = make([]interface{}, len(cmdArgs)-1)
	for i, a := range cmdArgs[1:] {
		ctx.CmdArgs[i] = a
	}
	ctx.StrippedMsg = stripped
	ctx.triggered = true
	return nil
}

// TriggerMessage is the message that runs a command with the given arguments: the prefix
// and the trigger, then the arguments, quoted where they need it.
func TriggerMessage(prefix string, t Trigger, args []string) string {
	msg := t.Text
	if t.Type == "Command" {
		msg = prefix + msg
	}
	if len(args) > 0 {
		joined := make([]interface{}, len(args))
		for i, a := range args {
			joined[i] = a
		}
		msg += " " + funcs.JoinArgs(joined)
	}
	return msg
}
