package funcs

// parseArgs and carg, ported from YAGPDB (customcommands/tmplextensions.go) and the parts of
// its dcmd library they use (lib/dcmd: SplitArgs, ParseArgDefs, the argument types and their
// errors). Discord lookups (users, members, channels, roles) go through the emulator's mocks.

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// ArgDef is an argument definition made by carg.
type ArgDef struct {
	Type       string
	Name       string
	Min, Max   int64   // int and duration bounds (duration in nanoseconds)
	MinF, MaxF float64 // float bounds
}

// Carg is YAGPDB's carg: an unknown type is an error, and int, float and duration take
// optional min and max values.
func Carg(typ, name string, opts ...interface{}) (*ArgDef, error) {
	def := &ArgDef{Type: typ, Name: name}
	switch typ {
	case "int", "duration":
		if len(opts) >= 2 {
			def.Min, def.Max = ToInt64(opts[0]), ToInt64(opts[1])
		}
	case "float":
		if len(opts) >= 2 {
			def.MinF, def.MaxF = ToFloat64(opts[0]), ToFloat64(opts[1])
		}
	case "string", "user", "userid", "channel", "member", "role":
	default:
		return nil, errors.New("Unknown type")
	}
	return def, nil
}

// Lookups resolves Discord objects for the user, member, channel and role types.
// Each returns nil when nothing matches.
type Lookups struct {
	User    func(id int64) interface{}
	Member  func(id int64) interface{}
	Channel func(id int64) interface{}
	// Role gets RoleArg's extracted ID (an int64, int -1 for a bad mention, or the string)
	// and its name form, and returns the first role in guild order matching either.
	Role func(id interface{}, idName string) interface{}
}

// ParsedArgs is the result of parseArgs.
type ParsedArgs struct {
	defs   []*ArgDef
	values []interface{}
}

// Get returns the parsed argument at index, or nil if it wasn't given.
func (pa *ParsedArgs) Get(index int) interface{} {
	if index < 0 || index >= len(pa.values) {
		return nil
	}
	v := pa.values[index]
	if v != nil && pa.defs[index].Type == "int" {
		return int(v.(int64)) // dcmd's ParsedArg.Int()
	}
	return v
}

// IsSet reports whether the argument at index was given.
func (pa *ParsedArgs) IsSet(index int) interface{} {
	return pa.Get(index) != nil
}

// ParseArgs parses stripped (the message after the trigger) against defs, like dcmd's
// ParseArgDefs: arguments are split on spaces, "…" and `…` group words, and the last
// definition takes the rest of the message. On failure the error is what YAGPDB responds
// with: dcmd's error, then the failed message or a usage line.
func ParseArgs(stripped string, numRequired int, failedMessage string, defs []*ArgDef, lookups Lookups) (*ParsedArgs, error) {
	result := &ParsedArgs{defs: defs, values: make([]interface{}, len(defs))}
	split := SplitArgs(stripped)
	for i, def := range defs {
		if i >= len(split) {
			if i >= numRequired {
				break
			}
			return result, usageError(errors.New("Not enough arguments passed"), failedMessage, defs, numRequired)
		}

		combined := ""
		if i == len(defs)-1 && len(split)-1 > i {
			// Last arg, but still more after: combine and rebuild them
			for j := i; j < len(split); j++ {
				if j != i {
					combined += " "
				}
				if c := split[j].Container; c != 0 {
					combined += string(c) + split[j].Str + string(c)
				} else {
					combined += split[j].Str
				}
			}
		} else {
			combined = split[i].Str
		}

		val, err := parseArg(def, combined, lookups)
		if err != nil {
			return result, usageError(err, failedMessage, defs, numRequired)
		}
		result.values[i] = val
	}
	return result, nil
}

func usageError(err error, failedMessage string, defs []*ArgDef, numRequired int) error {
	if failedMessage != "" {
		return fmt.Errorf("%v\n%s", err, failedMessage)
	}
	return fmt.Errorf("%v\nUsage: `%s`", err, usageLine(defs, numRequired))
}

// usageLine approximates dcmd's StdHelpFormatter.ArgDefLine: <required> [optional].
func usageLine(defs []*ArgDef, numRequired int) string {
	parts := make([]string, len(defs))
	for i, def := range defs {
		name := fmt.Sprintf("%s:%s", def.Name, helpName(def.Type))
		if i < numRequired {
			parts[i] = "<" + name + ">"
		} else {
			parts[i] = "[" + name + "]"
		}
	}
	return strings.Join(parts, " ")
}

func helpName(typ string) string {
	switch typ {
	case "int":
		return "Whole number"
	case "float":
		return "Decimal number"
	case "string":
		return "Text"
	case "duration":
		return "Duration"
	case "channel":
		return "Channel"
	case "role":
		return "Role"
	default:
		return "User"
	}
}

func parseArg(def *ArgDef, part string, lookups Lookups) (interface{}, error) {
	switch def.Type {
	case "int":
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a whole number", part)
		}
		if def.Min != def.Max || def.Min != 0 {
			if v < def.Min || v > def.Max {
				return nil, rangeError(def, v < def.Min, "%d", def.Min, def.Max)
			}
		}
		return v, nil
	case "float":
		v, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", part)
		}
		if def.MinF != def.MaxF || def.MinF != 0 {
			if v < def.MinF || v > def.MaxF {
				return nil, rangeError(def, v < def.MinF, "%f", def.MinF, def.MaxF)
			}
		}
		return v, nil
	case "duration":
		d, err := yagstd.ParseDuration(part)
		if err != nil {
			return nil, err
		}
		min, max := time.Duration(def.Min), time.Duration(def.Max)
		if (min != 0 && d < min) || (max != 0 && d > max) {
			return nil, durationRangeError(def.Name, d, min, max)
		}
		return d, nil
	case "string":
		return part, nil
	case "userid":
		if strings.HasPrefix(part, "<@") && len(part) > 3 {
			id := strings.TrimPrefix(part[2:len(part)-1], "!")
			v, err := strconv.ParseInt(id, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Improper mention %q", part)
			}
			return v, nil
		}
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Improper mention %q", part)
		}
		return v, nil
	case "user", "member":
		id, err := parseArg(&ArgDef{Type: "userid"}, part, lookups)
		if err != nil {
			if def.Type == "member" {
				return nil, errors.New("Invalid mention or id")
			}
			return nil, err
		}
		lookup := lookups.User
		if def.Type == "member" {
			lookup = lookups.Member
		}
		if v := lookup(id.(int64)); v != nil {
			return v, nil
		}
		if def.Type == "member" {
			return nil, errors.New("User not a member of the server")
		}
		return nil, fmt.Errorf("User %q not found", part)
	case "channel":
		// dcmd's ChannelArg: a mention or an ID, of a channel the server has
		id := part
		if strings.HasPrefix(part, "<#") && len(part) > 3 {
			id = part[2 : len(part)-1]
		}
		v, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Improper mention %q", part)
		}
		if c := lookups.Channel(v); c != nil {
			return c, nil
		}
		return nil, fmt.Errorf("Improper mention %q", part)
	case "role":
		// YAGPDB's commands.RoleArg, whose bad-mention -1 is an int: the int64 assertion
		// panics, as it does there
		id := extractRoleID(part)
		var idName string
		switch t := id.(type) {
		case int, int32, int64:
			idName = strconv.FormatInt(t.(int64), 10)
		case string:
			idName = t
		default:
			idName = ""
		}
		if r := lookups.Role(id, idName); r != nil {
			return r, nil
		}
		return nil, errors.New("Invalid role mention or id")
	}
	return nil, fmt.Errorf("unknown argument type %q", def.Type)
}

// extractRoleID is RoleArg.ExtractID.
func extractRoleID(part string) interface{} {
	if strings.HasPrefix(part, "<@&") && len(part) > 3 {
		// Direct mention
		id := part[3 : len(part)-1]

		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return -1
		}

		return parsed
	}

	id, err := strconv.ParseInt(part, 10, 64)
	if err == nil {
		return id
	}

	return part
}

// rangeError is dcmd's OutOfRangeError message.
func rangeError(def *ArgDef, tooSmall bool, verb string, min, max interface{}) error {
	pre := "too big"
	if tooSmall {
		pre = "too small"
	}
	return fmt.Errorf("%s is %s (has to be within "+verb+" - "+verb+")", def.Name, pre, min, max)
}

// durationRangeError is YAGPDB's DurationOutOfRangeError message.
func durationRangeError(name string, got, min, max time.Duration) error {
	pre := "too big"
	if got < min {
		pre = "too small"
	}
	switch {
	case min == 0:
		return fmt.Errorf("%s is %s, has to be smaller than %s", name, pre, humanizeMinutes(max))
	case max == 0:
		return fmt.Errorf("%s is %s, has to be bigger than %s", name, pre, humanizeMinutes(min))
	}
	return fmt.Errorf("%s is %s (has to be within `%s` and `%s`)", name, pre, humanizeMinutes(min), humanizeMinutes(max))
}

// humanizeMinutes is YAGPDB's HumanizeDuration at minute precision: "1 day and 2 hours".
func humanizeMinutes(d time.Duration) string {
	sec := int64(d.Seconds())
	days := sec / 86400
	units := []struct {
		n    int64
		name string
	}{
		{(sec / 60) % 60, "minute"},
		{(sec / 3600) % 24, "hour"},
		{(days - days/365) % 7, "day"},
		{(days % 365) / 7, "week"},
		{days / 365, "year"},
	}
	var out []string
	for _, u := range units {
		if u.n > 0 {
			s := fmt.Sprintf("%d %s", u.n, u.name)
			if u.n != 1 {
				s += "s"
			}
			out = append(out, s)
		}
	}
	str := ""
	for i := len(out) - 1; i >= 0; i-- {
		if i == 0 && i != len(out)-1 {
			str += " and "
		} else if i != len(out)-1 {
			str += " "
		}
		str += out[i]
	}
	if str == "" {
		str = "less than 1 minute"
	}
	return str
}

// RawArg is one argument of a split message; Container is the quote that grouped it.
type RawArg struct {
	Str       string
	Container rune
}

func isArgContainer(r rune) bool {
	return r == '"' || r == '`'
}

// SplitArgs is dcmd's SplitArgs, unchanged.
func SplitArgs(in string) []*RawArg {
	var rawArgs []*RawArg

	var buf strings.Builder
	escape := false
	var container rune
	for _, r := range in {
		// Apply or remove escape mode
		if r == '\\' {
			if escape {
				escape = false
				buf.WriteByte('\\')
			} else {
				escape = true
			}

			continue
		}

		// Check for other special tokens
		isSpecialToken := true
		if r == ' ' {
			// Maybe separate by space
			if buf.Len() > 0 && container == 0 && !escape {
				rawArgs = append(rawArgs, &RawArg{buf.String(), 0})
				buf.Reset()
			} else if buf.Len() > 0 {
				buf.WriteByte(' ')
			}
		} else if r == container && container != 0 {
			// Split arg here
			if escape {
				buf.WriteRune(r)
			} else {
				rawArgs = append(rawArgs, &RawArg{buf.String(), container})
				buf.Reset()
				container = 0
			}
		} else if container == 0 && buf.Len() == 0 {
			// Check if we should start containing a arg
			if isArgContainer(r) {
				if escape {
					buf.WriteRune(r)
				} else {
					container = r
				}
			} else {
				isSpecialToken = false
			}
		} else {
			isSpecialToken = false
		}

		if !isSpecialToken {
			if escape {
				buf.WriteByte('\\')
			}
			buf.WriteRune(r)
		}

		// Reset escape mode
		escape = false
	}

	// Something was left in the buffer just add it to the end
	if buf.Len() > 0 {
		item := buf.String()
		if container != 0 {
			item = string(container) + item
		}
		rawArgs = append(rawArgs, &RawArg{item, 0})
	}

	return rawArgs
}

// JoinArgs rebuilds a message from separate arguments, quoting any that SplitArgs would
// otherwise break apart (escaping \ and " inside the quotes), so SplitArgs(JoinArgs(args))
// gives args back.
func JoinArgs(args []interface{}) string {
	escaper := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	parts := make([]string, len(args))
	for i, a := range args {
		s := ToString(a)
		if s == "" || strings.ContainsAny(s, " \\\"`") {
			s = `"` + escaper.Replace(s) + `"`
		}
		parts[i] = s
	}
	return strings.Join(parts, " ")
}
