package runtime

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// ErrTriggerMismatch is wrapped by the error SetTriggerMessage returns when msg doesn't
// match the trigger, so a caller can detect a mismatch with errors.Is instead of matching
// the message text.
var ErrTriggerMismatch = errors.New("trigger mismatch")

// triggerMismatchError carries SetTriggerMessage's existing message text while unwrapping
// to ErrTriggerMismatch for structural detection.
type triggerMismatchError struct {
	msg string
}

func (e *triggerMismatchError) Error() string { return e.msg }
func (e *triggerMismatchError) Unwrap() error { return ErrTriggerMismatch }

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
	headerDeferMode   = regexp.MustCompile("(?i:Defer mode): `([^`]*)`")
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
	if v, ok := headerValue(headerDeferMode, source); ok {
		if _, known := parseDeferMode(v); !known {
			return fmt.Errorf("header Defer mode: `%s` isn't one of the panel's: %s", v,
				strings.Join(deferModeLabels[:], ", "))
		}
	}
	// A slash command's or context menu entry's rows and name, as the panel validates
	// them (slash.go); the option rows belong to a slash command only
	t, ok := ReadTrigger(source)
	switch {
	case ok && t.SlashTriggered():
		if _, err := ReadSlashCommand(source); err != nil {
			return fmt.Errorf("header: %w", err)
		}
	case ok && t.IsContextMenu():
		if err := validateContextMenuHeader(source, t); err != nil {
			return fmt.Errorf("header: %w", err)
		}
		fallthrough
	default:
		header := headerComment(source)
		if headerSlashOption.MatchString(header) || headerSlashSubcommand.MatchString(header) {
			return fmt.Errorf("header: Slash option and Slash subcommand lines need a Slash Command trigger")
		}
	}
	return nil
}

// DeferMode is the control panel's "Interaction defer mode" (customcommands.go:135-140):
// what YAGPDB answers an interaction with before the command runs.
type DeferMode int

// Defer modes, with the panel's labels (customcommands-editcmd.html:322-347).
const (
	DeferModeNone      DeferMode = iota // no deferral: the run must respond itself
	DeferModeMessage                    // "thinking...", which the output then fills
	DeferModeEphemeral                  // the same, visible to the clicker only
	DeferModeUpdate                     // acknowledges the click; the output edits the message
)

var deferModeLabels = [...]string{"None", "Message Response", "Ephemeral Message Response",
	"Update Message Response"}

func (m DeferMode) String() string {
	if int(m) < len(deferModeLabels) {
		return deferModeLabels[m]
	}
	return fmt.Sprintf("DeferMode(%d)", int(m))
}

// parseDeferMode reads a panel label (any case); known is false for anything else.
func parseDeferMode(label string) (mode DeferMode, known bool) {
	for i, l := range deferModeLabels {
		if strings.EqualFold(strings.TrimSpace(label), l) {
			return DeferMode(i), true
		}
	}
	return DeferModeNone, false
}

// ReadDeferMode reads a command's defer mode from its header ("Defer mode: `Ephemeral
// Message Response`"; default None). See ValidateHeader for a bad value.
func ReadDeferMode(source string) DeferMode {
	if v, ok := headerValue(headerDeferMode, source); ok {
		if mode, known := parseDeferMode(v); known {
			return mode
		}
	}
	return DeferModeNone
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

// RoleTriggered reports whether the trigger runs the command on a role change (the control
// panel's "Role Change" option; YAGPDB's CommandTriggerRole). No message starts such a run.
func (t Trigger) RoleTriggered() bool {
	return t.Type == "Role Change"
}

// ComponentTriggered reports whether the trigger runs the command on a button or menu
// click (the control panel's "Message Component"; YAGPDB's CommandTriggerComponent, whose
// triggerStrings name "Component" is accepted too). A click on a component whose custom
// ID matches the trigger regex starts such a run.
func (t Trigger) ComponentTriggered() bool {
	return strings.EqualFold(t.Type, "Message Component") || strings.EqualFold(t.Type, "Component")
}

// SlashTriggered reports whether the trigger runs the command on a slash command
// (the control panel's "Slash Command"; YAGPDB's CommandTriggerSlash). The trigger text
// is the command's name, matched case-insensitively (handle_slashcommand.go:78).
func (t Trigger) SlashTriggered() bool {
	return strings.EqualFold(t.Type, "Slash Command")
}

// ContextMenuTriggered reports whether the trigger runs the command from a context menu
// (the control panel's "User Context Menu" / "Message Context Menu"; YAGPDB's
// CommandTriggerUserContextMenu / CommandTriggerMessageContextMenu), and which. The
// trigger text is the menu entry's name, matched trimmed and case-insensitively
// (handle_contextmenu.go:82).
func (t Trigger) ContextMenuTriggered() (types.ApplicationCommandType, bool) {
	switch {
	case strings.EqualFold(t.Type, "User Context Menu"):
		return types.UserApplicationCommand, true
	case strings.EqualFold(t.Type, "Message Context Menu"):
		return types.MessageApplicationCommand, true
	}
	return 0, false
}

// IsContextMenu is ContextMenuTriggered without the kind.
func (t Trigger) IsContextMenu() bool {
	_, ok := t.ContextMenuTriggered()
	return ok
}

// InteractionTriggered reports whether an interaction (a click, a slash command, a
// context menu entry or a modal submission) starts such a run, so a test must give
// context.interaction.
func (t Trigger) InteractionTriggered() bool {
	return t.ComponentTriggered() || t.SlashTriggered() || t.IsContextMenu() || t.ModalTriggered()
}

// CheckMatchComponent is YAGPDB's customcommands.CheckMatchComponent
// (handle_component.go:321-335): the trigger is a regex, (?m) and (?i) unless
// case-sensitive, matched anywhere in the custom ID with its templates- prefix already
// stripped; the ID after the match is the stripped ID, split into the arguments.
func CheckMatchComponent(t Trigger, cID string) (match bool, stripped string, args []string) {
	if !t.ComponentTriggered() {
		return false, "", nil
	}

	cmdMatch := "(?m)"
	if !t.CaseSensitive {
		cmdMatch += "(?i)"
	}
	cmdMatch += t.Text

	return matchRegexSplitArgs(cmdMatch, cID)
}

// disallowedExecCCType is YAGPDB's triggerStrings name for a trigger type execCC and
// scheduleUniqueCC refuse (customcommands/tmplextensions.go, customcommands.go
// triggerStrings): "Interval" (both hourly and minute intervals share CommandTriggerInterval),
// "Crontab", or "Role". ok is false for any other (allowed) trigger type.
func (t Trigger) disallowedExecCCType() (name string, ok bool) {
	switch {
	case t.Scheduled():
		typ := strings.ToLower(t.Type)
		if strings.HasSuffix(typ, "interval") {
			return "Interval", true
		}
		return "Crontab", true
	case t.RoleTriggered():
		return "Role", true
	}
	return "", false
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
		return &triggerMismatchError{msg: fmt.Sprintf("the message %q doesn't match the %s trigger %q", msg, t.Type, t.Text)}
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
