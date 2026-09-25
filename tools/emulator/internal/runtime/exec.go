package runtime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	yagstd "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// Exec is a bot command a run executed with exec or execAdmin. The emulator records it
// and doesn't run it: the call returns "", where YAGPDB returns the command's response.
type Exec struct {
	Admin     bool // execAdmin: run as the bot, not the triggering user
	ChannelID int64
	Line      string // the command line, as YAGPDB builds it (without its trailing space)
}

func (x Exec) String() string {
	fn := "exec"
	if x.Admin {
		fn = "execAdmin"
	}
	return fmt.Sprintf("%s %s in channel %d", fn, x.Line, x.ChannelID)
}

// exec and execAdmin follow YAGPDB's TmplExecCmdFuncs (commands/tmplexec.go): the command
// line is built from the arguments, in the run's channel; the shared limit of 5 is in
// limits.go.
func (e *Engine) exec(cmd string, args ...interface{}) (interface{}, error) {
	return e.recordExec(false, cmd, args...)
}

func (e *Engine) execAdmin(cmd string, args ...interface{}) (interface{}, error) {
	return e.recordExec(true, cmd, args...)
}

func (e *Engine) recordExec(admin bool, cmd string, args ...interface{}) (interface{}, error) {
	line, err := buildExecCmdLine(cmd, args...)
	if err != nil {
		return "", err
	}
	e.ctx.Execs = append(e.ctx.Execs, Exec{Admin: admin, ChannelID: e.ctx.ChannelID,
		Line: strings.TrimSuffix(line, " ")})
	return "", nil
}

// buildExecCmdLine is YAGPDB's (commands/tmplexec.go), with the emulator's user type for
// discordgo's; the mentions it also returns aren't needed here.
func buildExecCmdLine(cmd string, args ...any) (string, error) {
	cmdLine := cmd + " "

	for _, arg := range args {
		if arg == nil {
			return "", errors.New("Nil arg passed")
		}

		switch t := arg.(type) {
		case string:
			if strings.HasPrefix(t, "-") {
				// Don't put quotes around switches
				cmdLine += t
			} else if strings.HasPrefix(t, "\\-") {
				// Escaped -
				cmdLine += "\"" + t[1:] + "\""
			} else {
				cmdLine += "\"" + t + "\""
			}
		case int:
			cmdLine += strconv.FormatInt(int64(t), 10)
		case int32:
			cmdLine += strconv.FormatInt(int64(t), 10)
		case int64:
			cmdLine += strconv.FormatInt(t, 10)
		case uint:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint8:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint16:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint32:
			cmdLine += strconv.FormatUint(uint64(t), 10)
		case uint64:
			cmdLine += strconv.FormatUint(t, 10)
		case float32:
			cmdLine += strconv.FormatFloat(float64(t), 'E', -1, 32)
		case float64:
			cmdLine += strconv.FormatFloat(t, 'E', -1, 64)
		case *types.DiscordUser:
			cmdLine += "<@" + strconv.FormatInt(t.ID, 10) + ">"
		case types.DiscordUser:
			cmdLine += "<@" + strconv.FormatInt(t.ID, 10) + ">"
		case []string:
			for i, str := range t {
				if i != 0 {
					cmdLine += " "
				}
				cmdLine += str
			}
		default:
			return "", errors.New("Unknown type in exec, only strings, numbers, users and string slices are supported")
		}
		cmdLine += " "

		if len(cmdLine) > yagstd.MaxStringLength {
			return "", yagstd.ErrStringTooLong
		}
	}

	return cmdLine, nil
}
