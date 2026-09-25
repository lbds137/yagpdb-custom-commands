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

// Header lines are read from the template's leading comment only, keys in any case.
var (
	headerTriggerType = regexp.MustCompile("(?i:Trigger type): `([^`]*)`")
	headerTrigger     = regexp.MustCompile("(?i:Trigger): `([^`]*)`")
	headerCase        = regexp.MustCompile("(?i:Case sensitive): `([^`]*)`")
	headerShowErrors  = regexp.MustCompile("(?i:Show errors): `([^`]*)`")
	headerRedirect    = regexp.MustCompile("(?i:Redirect errors): `([^`]*)`")
)

// headerComment is the template's leading {{/* ... */}} comment, or "" without one.
func headerComment(source string) string {
	s := strings.TrimLeft(source, " \t\r\n")
	if !strings.HasPrefix(s, "{{") {
		return ""
	}
	s = strings.TrimLeft(s[2:], "- ")
	if !strings.HasPrefix(s, "/*") {
		return ""
	}
	if end := strings.Index(s, "*/"); end >= 0 {
		return s[:end]
	}
	return ""
}

// headerValue is the value of a header line, and whether the header has it.
func headerValue(re *regexp.Regexp, source string) (string, bool) {
	if m := re.FindStringSubmatch(headerComment(source)); m != nil {
		return m[1], true
	}
	return "", false
}

// ErrorSettings are a command's error settings from the control panel: show_errors (on
// by default) and the channel errors are redirected to (0: the command's own). A header
// sets them with "Show errors: `false`" and "Redirect errors: `<channel ID>`".
type ErrorSettings struct {
	ShowErrors      bool
	RedirectChannel int64
}

// ReadErrorSettings reads a command's error settings from its header (see ValidateHeader).
func ReadErrorSettings(source string) ErrorSettings {
	s := ErrorSettings{ShowErrors: true}
	if v, ok := headerValue(headerShowErrors, source); ok {
		s.ShowErrors = !strings.EqualFold(v, "false")
	}
	if v, ok := headerValue(headerRedirect, source); ok {
		s.RedirectChannel, _ = strconv.ParseInt(v, 10, 64)
	}
	return s
}

// ValidateHeader rejects header settings the emulator would otherwise read as their
// defaults: the switches take true or false, the redirect a channel ID.
func ValidateHeader(source string) error {
	for _, sw := range []struct {
		name string
		re   *regexp.Regexp
	}{{"Case sensitive", headerCase}, {"Show errors", headerShowErrors}} {
		if v, ok := headerValue(sw.re, source); ok && !strings.EqualFold(v, "true") && !strings.EqualFold(v, "false") {
			return fmt.Errorf("header %s: `%s` isn't true or false", sw.name, v)
		}
	}
	if v, ok := headerValue(headerRedirect, source); ok {
		if id, err := strconv.ParseInt(v, 10, 64); err != nil || id <= 0 {
			return fmt.Errorf("header Redirect errors: `%s` isn't a channel ID", v)
		}
	}
	return nil
}

// ReadTrigger reads a command's trigger from its header comment ("Trigger type: `Command`",
// "Trigger: `db`"). ok is false if the header names no trigger type.
func ReadTrigger(source string) (t Trigger, ok bool) {
	v, ok := headerValue(headerTriggerType, source)
	if !ok {
		return Trigger{}, false
	}
	t.Type = v
	t.Text, _ = headerValue(headerTrigger, source)
	if v, ok := headerValue(headerCase, source); ok {
		t.CaseSensitive = strings.EqualFold(v, "true")
	}
	return t, true
}

// Scheduled reports whether the trigger runs the command on a schedule: an interval
// ("Hourly interval", "Minute interval") or YAGPDB's "Crontab" (the control panel's
// "Crontab (Beta)"). No message or member starts such a run.
func (t Trigger) Scheduled() bool {
	typ := strings.ToLower(t.Type)
	return strings.HasSuffix(typ, "interval") || strings.HasPrefix(typ, "crontab") || typ == "cron"
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
