package runtime

// A failed custom command's error message, copied from YAGPDB's customcommands/bot.go
// (formatCustomCommandRunErr and its helpers); limitString is funcs.LimitString.
// emperror's errors.Cause stops at a template.ExecError, which has no Unwrap; errors.As
// finds the same error in the emulator's chain.

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
	template "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagtemplate"
)

func formatCustomCommandRunErr(src string, err error) string {
	// check if we can retrieve the original ExecError
	var eerr template.ExecError
	if errors.As(err, &eerr) {
		data := parseExecError(eerr)
		// couldn't parse error, fall back to the original error message
		if data == nil {
			return "`" + err.Error() + "`"
		}

		out := fmt.Sprintf("`Failed executing CC #%d, line %d, row %d: %s`", data.CCID, data.Line, data.Row, data.Msg)
		lines := strings.Split(src, "\n")
		if len(lines) < int(data.Line) {
			return out
		}

		out += "\n```"
		out += getSurroundingLines(lines, int(data.Line-1)) // data.Line is 1-based, convert to 0-based.
		out += "\n```"
		return out
	}

	// otherwise, fall back to the normal error message
	return "`" + err.Error() + "`"
}

func getSurroundingLines(lines []string, lineIndex int) string {
	var lineNums []int
	var res []string

	commonLeadingSpaces := -1
	addLine := func(n int) {
		line := lines[n]

		leadingSpaceCount := 0
		var cleaned strings.Builder
		var i int
	Loop:
		for i = 0; i < len(line); i++ {
			switch line[i] {
			case '\t':
				cleaned.WriteString("    ") // tabs -> 4 spaces
				leadingSpaceCount += 4
			case ' ':
				cleaned.WriteByte(' ')
				leadingSpaceCount++
			default:
				break Loop
			}
		}

		if i != len(line) {
			cleaned.WriteString(line[i:])
		}

		if commonLeadingSpaces == -1 || leadingSpaceCount < commonLeadingSpaces {
			commonLeadingSpaces = leadingSpaceCount
		}

		res = append(res, cleaned.String())
		lineNums = append(lineNums, n+1) // line numbers shown to the user are 1-based
	}

	// add previous line if possible
	if lineIndex > 0 && len(lines) > 1 {
		addLine(lineIndex - 1)
	}

	addLine(lineIndex)

	// add next line if possible
	if lineIndex != len(lines)-1 {
		addLine(lineIndex + 1)
	}

	var out strings.Builder
	for i, line := range res {
		if i > 0 {
			out.WriteByte('\n')
		}

		// remove common leading whitespace
		line = line[commonLeadingSpaces:]
		if len(line) > 35 {
			line = funcs.LimitString(line, 30) + "..."
		}
		// replace all ` with ` + a ZWS to make sure that all the code will stay formatted nicely in the codeblock
		line = strings.ReplaceAll(line, "`", "`\u200b")

		out.WriteString(strconv.FormatInt(int64(lineNums[i]), 10))
		out.WriteString("    ")
		out.WriteString(line)
	}

	return out.String()
}

type execErrorData struct {
	CCID, Line, Row int64
	Msg             string
}

var execErrorInfoRe = regexp.MustCompile(`\Atemplate: CC #(\d+):(\d+):(\d+): ([\S\s]+)`)

// parseExecError uses regex to extract the individual parts out of an ExecError.
// It returns nil if an error occurred during parsing.
func parseExecError(err template.ExecError) *execErrorData {
	parts := execErrorInfoRe.FindStringSubmatch(err.Error())
	if parts == nil {
		return nil
	}

	ccid, perr := strconv.ParseInt(parts[1], 10, 64)
	if perr != nil {
		return nil
	}

	line, perr := strconv.ParseInt(parts[2], 10, 64)
	if perr != nil {
		return nil
	}

	row, perr := strconv.ParseInt(parts[3], 10, 64)
	if perr != nil {
		return nil
	}

	return &execErrorData{ccid, line, row, parts[4]}
}
