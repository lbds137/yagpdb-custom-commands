package runtime

import (
	"errors"
	"regexp"
	"sort"
	"strings"
)

const docsURL = "https://help.yagpdb.xyz/docs/reference/templates/functions/"

var (
	reUndefinedFunc = regexp.MustCompile(`function "([^"]+)" not defined`)
	reCantEvalField = regexp.MustCompile(`can't evaluate field (\w+) in type ([\w.*\[\]{}]+)`)
	reErrorCalling  = regexp.MustCompile(`error calling (\w+):`)
	reWrongArgs     = regexp.MustCompile(`wrong number of args for (\w+)`)
	reWrongType     = regexp.MustCompile(`wrong type for value; expected (\S+); got (\S+)`)
)

// Hint explains an execution error and suggests a fix, or returns "" when it has
// nothing useful to add.
func Hint(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()

	if errors.Is(err, ErrTooManyCalls) || errors.Is(err, ErrTooManyAPICalls) {
		return "YAGPDB caps how often a function can run in one execution. Move the call out of " +
			"loops, reuse earlier results, or fetch many entries with one dbGetPattern. " +
			"Run without -strict to see every limit as a warning."
	}

	if m := reUndefinedFunc.FindStringSubmatch(msg); m != nil {
		name := m[1]
		if yagpdbFuncs[name] {
			return name + " is a YAGPDB function the emulator doesn't implement yet. Add it to " +
				"BuildFuncMap in tools/emulator/internal/runtime/engine.go. Docs: " + docsLink(name)
		}
		if s := closest(name); s != "" {
			return "YAGPDB has no function " + name + ". Did you mean " + s + "? Docs: " + docsLink(s)
		}
		return "YAGPDB has no function " + name + ". Function list: " + docsURL
	}

	if m := reCantEvalField.FindStringSubmatch(msg); m != nil {
		field, typ := m[1], m[2]
		if strings.Contains(typ, "LightDBEntry") {
			return "dbGet returns an entry, not the stored value. Read the value with .Value " +
				"(for example (dbGet 0 \"Key\").Value." + field + ")."
		}
		if strings.Contains(typ, "TemplateValue") || strings.Contains(typ, "interface") {
			return "the value has no field " + field + ". For a dict, use .Get \"" + field + "\" or (index $d \"" + field + "\")."
		}
		return typ + " has no field " + field + ". Field names are case-sensitive."
	}

	if strings.Contains(msg, "nil pointer evaluating") {
		return "a value was nil. dbGet returns nil for a missing key; check it with {{if $entry}} " +
			"or give a default with (or $value (sdict))."
	}

	if strings.Contains(msg, "error calling parseArgs") {
		return "the command needs arguments. Pass them with -args \"a,b\" (yagtest run) or " +
			"context.args (test YAML)."
	}

	if m := reWrongArgs.FindStringSubmatch(msg); m != nil {
		return "check the arguments " + m[1] + " takes: " + docsLink(m[1])
	}

	if m := reWrongType.FindStringSubmatch(msg); m != nil {
		return "a " + m[2] + " was passed where " + m[1] + " is needed. Convert it first, " +
			"for example with toInt, toInt64, toFloat or str."
	}

	if m := reErrorCalling.FindStringSubmatch(msg); m != nil {
		return "Docs for " + m[1] + ": " + docsLink(m[1])
	}

	return ""
}

func docsLink(name string) string {
	return docsURL + "#" + strings.ToLower(name)
}

// closest returns the YAGPDB function name nearest to name, if one is close enough to
// be a likely typo.
func closest(name string) string {
	names := make([]string, 0, len(yagpdbFuncs))
	for n := range yagpdbFuncs {
		names = append(names, n)
	}
	sort.Strings(names) // deterministic ties

	best, bestDist := "", len(name)/3+2
	for _, n := range names {
		if strings.EqualFold(n, name) {
			return n
		}
		if d := editDistance(strings.ToLower(name), strings.ToLower(n)); d < bestDist {
			best, bestDist = n, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
