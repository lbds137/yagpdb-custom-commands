package runtime

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// Engine handles template parsing and execution.
type Engine struct {
	ctx *ExecutionContext
}

// NewEngine creates a new template engine with the given context.
func NewEngine(ctx *ExecutionContext) *Engine {
	return &Engine{ctx: ctx}
}

// BuildFuncMap creates the FuncMap for template execution.
func (e *Engine) BuildFuncMap() template.FuncMap {
	dbFuncs := funcs.NewDatabaseFuncs(e.ctx.DB, e.ctx.GuildID)

	return template.FuncMap{
		// Type conversion
		"str":        funcs.ToString,
		"toString":   funcs.ToString,
		"toInt":      funcs.ToInt,
		"toInt64":    funcs.ToInt64,
		"toFloat":    funcs.ToFloat64,
		"toDuration": funcs.ToDuration,
		"toRune":     funcs.ToRune,
		"toByte":     funcs.ToByte,

		// String manipulation
		"lower":       strings.ToLower,
		"upper":       strings.ToUpper,
		"title":       strings.Title,
		"hasPrefix":   strings.HasPrefix,
		"hasSuffix":   strings.HasSuffix,
		"trimSpace":   strings.TrimSpace,
		"split":       strings.Split,
		"joinStr":     funcs.JoinStrings,
		"slice":       funcs.SliceFunc,
		"urlescape":   funcs.URLEscape,
		"urlunescape": funcs.URLUnescape,
		"print":       fmt.Sprint,
		"println":     fmt.Sprintln,
		"printf":      fmt.Sprintf,

		// Math
		"add":        funcs.Add,
		"sub":        funcs.Sub,
		"mult":       funcs.Mult,
		"div":        funcs.Div,
		"fdiv":       funcs.FDiv,
		"mod":        funcs.Mod,
		"abs":        funcs.Abs,
		"sqrt":       funcs.Sqrt,
		"cbrt":       funcs.Cbrt,
		"pow":        funcs.Pow,
		"log":        funcs.Log,
		"round":      funcs.Round,
		"roundCeil":  funcs.RoundCeil,
		"roundFloor": funcs.RoundFloor,
		"roundEven":  funcs.RoundEven,
		"min":        funcs.Min,
		"max":        funcs.Max,

		// Collections
		"dict":        types.Dictionary,
		"sdict":       types.StringKeyDictionary,
		"cslice":      types.CreateSlice,
		"json":        types.ToJSON,
		"jsonToSdict": types.JSONToSDict,

		// Time
		"currentTime": funcs.CurrentTime,
		"formatTime":  funcs.FormatTime,
		"parseTime":   funcs.ParseTime,
		"newDate":     funcs.NewDate,

		// Regex
		"reFind":      funcs.ReFind,
		"reFindAll":   funcs.ReFindAll,
		"reReplace":   funcs.ReReplace,
		"reQuoteMeta": funcs.ReQuoteMeta,

		// Utilities
		"in":     funcs.In,
		"inFold": funcs.InFold,
		"kindOf": funcs.KindOf,
		"seq":    funcs.Seq,
		"randInt": funcs.RandInt,

		// Database
		"dbGet":               dbFuncs.DbGet,
		"dbSet":               dbFuncs.DbSet,
		"dbSetExpire":         dbFuncs.DbSetExpire,
		"dbDel":               dbFuncs.DbDel,
		"dbDelById":           dbFuncs.DbDelByID,
		"dbDelByID":           dbFuncs.DbDelByID,
		"dbIncr":              dbFuncs.DbIncr,
		"dbGetPattern":        dbFuncs.DbGetPattern,
		"dbGetPatternReverse": dbFuncs.DbGetPatternReverse,
		"dbCount":             dbFuncs.DbCount,
		"dbTopEntries":        dbFuncs.DbTopEntries,
		"dbBottomEntries":     dbFuncs.DbBottomEntries,
		"dbRank":              dbFuncs.DbRank,

		// Discord mocks (output capture)
		"sendMessage": e.sendMessage,
		"sendDM":      e.sendDM,
		"editMessage": e.editMessage,
		"getMessage":  e.getMessage,
		"deleteMessage":     e.deleteMessage,
		"deleteTrigger":     e.deleteTrigger,
		"deleteResponse":    e.deleteResponse,
		"addReactions":      e.addReactions,
		"addMessageReactions": e.addMessageReactions,

		// Role functions
		"hasRole":        e.hasRole,
		"hasRoleID":      e.hasRoleID,
		"targetHasRole":  e.targetHasRole,
		"addRole":        e.addRole,
		"giveRole":       e.giveRole,
		"removeRole":     e.removeRole,
		"takeRole":       e.takeRole,
		"setRoles":       e.setRoles,
		"giveRoleID":     e.giveRoleID,
		"takeRoleID":     e.takeRoleID,
		"addRoleID":      e.addRoleID,
		"removeRoleID":   e.removeRoleID,

		// Member/user functions
		"getMember":      e.getMember,
		"userArg":        e.userArg,
		"getTargetPermissionsIn": e.getTargetPermissionsIn,

		// Channel functions
		"getChannel":     e.getChannel,

		// Embed building
		"cembed":         e.cembed,
		"complexMessage": e.complexMessage,
		"sendTemplate":   e.sendTemplate,

		// Control flow
		"execCC":                   e.execCC,
		"scheduleUniqueCC":         e.scheduleUniqueCC,
		"cancelScheduledUniqueCC":  e.cancelScheduledUniqueCC,

		// Argument parsing
		"parseArgs": e.parseArgs,
		"carg":      funcs.Carg,

		// Misc
		"or":      e.orFunc,
		"and":     e.andFunc,
		"not":     e.notFunc,
		"eq":      e.eqFunc,
		"ne":      e.neFunc,
		"lt":      e.ltFunc,
		"le":      e.leFunc,
		"gt":      e.gtFunc,
		"ge":      e.geFunc,
		"len":     e.lenFunc,
		"index":   e.indexFunc,
		"return":  e.returnFunc,
		"try":     e.tryFunc,
	}
}

// Execute parses and executes a template.
func (e *Engine) Execute(source string) (string, error) {
	e.ctx.StartTime = time.Now()

	// Preprocess the template to handle YAGPDB-specific constructs
	source = PreprocessTemplate(source)

	tmpl, err := template.New("yagtest").
		Funcs(e.BuildFuncMap()).
		Parse(source)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	data := e.ctx.BuildTemplateData()

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}

// Mock Discord functions

func (e *Engine) sendMessage(args ...interface{}) string {
	var channelID int64 = e.ctx.ChannelID
	var content string
	var embed interface{}

	if len(args) >= 1 {
		if args[0] != nil {
			channelID = funcs.ToInt64(args[0])
		}
	}
	if len(args) >= 2 {
		switch v := args[1].(type) {
		case string:
			content = v
		case types.SDict:
			embed = v
		default:
			content = funcs.ToString(v)
		}
	}

	e.ctx.RecordSentMessage(channelID, content, embed)
	return ""
}

func (e *Engine) sendDM(msg interface{}) string {
	content := funcs.ToString(msg)
	e.ctx.RecordSentMessage(0, content, nil) // 0 = DM
	return ""
}

func (e *Engine) editMessage(channel, msgID, content interface{}) string {
	return ""
}

func (e *Engine) getMessage(channel, msgID interface{}) interface{} {
	return nil
}

func (e *Engine) deleteMessage(args ...interface{}) string {
	return ""
}

func (e *Engine) deleteTrigger(args ...interface{}) string {
	return ""
}

func (e *Engine) deleteResponse(args ...interface{}) string {
	return ""
}

func (e *Engine) addReactions(args ...interface{}) string {
	return ""
}

func (e *Engine) addMessageReactions(args ...interface{}) string {
	return ""
}

// Role functions

func (e *Engine) hasRole(roleInput interface{}) bool {
	roleID := funcs.ToInt64(roleInput)
	return e.ctx.HasRole(roleID)
}

func (e *Engine) hasRoleID(roleID interface{}) bool {
	return e.ctx.HasRole(funcs.ToInt64(roleID))
}

func (e *Engine) targetHasRole(target, roleInput interface{}) (bool, error) {
	return false, nil
}

func (e *Engine) addRole(roleInput interface{}, delay ...interface{}) string {
	roleID := funcs.ToInt64(roleInput)
	e.ctx.RecordRoleChange(e.ctx.UserID, roleID, "add")
	return ""
}

func (e *Engine) giveRole(target, roleInput interface{}, delay ...interface{}) string {
	userID := funcs.ToInt64(target)
	roleID := funcs.ToInt64(roleInput)
	e.ctx.RecordRoleChange(userID, roleID, "add")
	return ""
}

func (e *Engine) removeRole(roleInput interface{}, delay ...interface{}) string {
	roleID := funcs.ToInt64(roleInput)
	e.ctx.RecordRoleChange(e.ctx.UserID, roleID, "remove")
	return ""
}

func (e *Engine) takeRole(target, roleInput interface{}, delay ...interface{}) string {
	userID := funcs.ToInt64(target)
	roleID := funcs.ToInt64(roleInput)
	e.ctx.RecordRoleChange(userID, roleID, "remove")
	return ""
}

func (e *Engine) setRoles(target interface{}, roles interface{}) string {
	return ""
}

func (e *Engine) giveRoleID(target, roleID interface{}) string {
	return e.giveRole(target, roleID)
}

func (e *Engine) takeRoleID(target, roleID interface{}) string {
	return e.takeRole(target, roleID)
}

func (e *Engine) addRoleID(roleID interface{}, delay ...interface{}) string {
	return e.addRole(roleID, delay...)
}

func (e *Engine) removeRoleID(roleID interface{}, delay ...interface{}) string {
	return e.removeRole(roleID, delay...)
}

// Member/user functions

func (e *Engine) getMember(userID interface{}) interface{} {
	return types.CtxMember{
		User: types.DiscordUser{
			ID:       funcs.ToInt64(userID),
			Username: "MockUser",
		},
	}
}

func (e *Engine) userArg(arg interface{}) interface{} {
	return types.DiscordUser{
		ID:       funcs.ToInt64(arg),
		Username: "MockUser",
	}
}

func (e *Engine) getTargetPermissionsIn(userID, channelID interface{}) int64 {
	return 0
}

// Channel functions

func (e *Engine) getChannel(channelID interface{}) interface{} {
	return types.CtxChannel{
		ID:      funcs.ToInt64(channelID),
		GuildID: e.ctx.GuildID,
		Name:    "mock-channel",
	}
}

// Embed/message building

func (e *Engine) cembed(args ...interface{}) (types.SDict, error) {
	result := make(types.SDict)
	for i := 0; i+1 < len(args); i += 2 {
		key := funcs.ToString(args[i])
		result[key] = args[i+1]
	}
	return result, nil
}

func (e *Engine) complexMessage(args ...interface{}) (types.SDict, error) {
	return e.cembed(args...)
}

func (e *Engine) sendTemplate(args ...interface{}) string {
	return ""
}

// Cross-command execution

func (e *Engine) execCC(ccID, channel, delay interface{}, data interface{}) string {
	commandID := funcs.ToInt64(ccID)

	// Check depth limit
	if e.ctx.ExecCCDepth >= e.ctx.MaxExecCCDepth {
		// YAGPDB silently fails when depth is exceeded
		return ""
	}

	// Look up command template path
	templatePath, ok := e.ctx.CommandIDMap[commandID]
	if !ok {
		// Command not found in registry - this is normal for unmapped commands
		return ""
	}

	// Resolve template path
	if e.ctx.TemplateBaseDir != "" && !filepath.IsAbs(templatePath) {
		templatePath = filepath.Join(e.ctx.TemplateBaseDir, templatePath)
	}

	// Load template
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		// Template file not found
		return ""
	}

	// Create child context (shares DB and other state)
	childCtx := &ExecutionContext{
		GuildID:         e.ctx.GuildID,
		GuildName:       e.ctx.GuildName,
		ChannelID:       funcs.ToInt64(channel),
		ChannelName:     e.ctx.ChannelName,
		UserID:          e.ctx.UserID,
		Username:        e.ctx.Username,
		Discriminator:   e.ctx.Discriminator,
		UserRoles:       e.ctx.UserRoles,
		Args:            []interface{}{},
		CmdArgs:         []interface{}{},
		ExecData:        data,
		IsPremium:       e.ctx.IsPremium,
		DB:              e.ctx.DB, // Share database
		MaxOps:          e.ctx.MaxOps,
		CurrentOps:      e.ctx.CurrentOps,
		MaxOutput:       e.ctx.MaxOutput,
		StartTime:       e.ctx.StartTime,
		MaxDuration:     e.ctx.MaxDuration,
		AvailableRoles:  e.ctx.AvailableRoles,
		CommandIDMap:    e.ctx.CommandIDMap,
		ExecCCDepth:     e.ctx.ExecCCDepth + 1,
		MaxExecCCDepth:  e.ctx.MaxExecCCDepth,
		TemplateBaseDir: e.ctx.TemplateBaseDir,
	}

	// Execute child template
	childEngine := NewEngine(childCtx)
	_, err = childEngine.Execute(string(templateContent))

	// Propagate side effects back to parent
	e.ctx.SentMessages = append(e.ctx.SentMessages, childCtx.SentMessages...)
	e.ctx.RoleChanges = append(e.ctx.RoleChanges, childCtx.RoleChanges...)
	e.ctx.CurrentOps = childCtx.CurrentOps

	// execCC doesn't return output to the caller
	return ""
}

func (e *Engine) scheduleUniqueCC(ccID, channel, delay, key, data interface{}) string {
	return ""
}

func (e *Engine) cancelScheduledUniqueCC(ccID, key interface{}) string {
	return ""
}

// Logic/comparison functions

func (e *Engine) orFunc(args ...interface{}) interface{} {
	for _, arg := range args {
		if arg != nil && arg != false && arg != "" && arg != 0 {
			return arg
		}
	}
	if len(args) > 0 {
		return args[len(args)-1]
	}
	return nil
}

func (e *Engine) andFunc(args ...interface{}) interface{} {
	var result interface{}
	for _, arg := range args {
		result = arg
		if arg == nil || arg == false || arg == "" || arg == 0 {
			return arg
		}
	}
	return result
}

func (e *Engine) notFunc(arg interface{}) bool {
	return arg == nil || arg == false || arg == "" || arg == 0
}

func (e *Engine) eqFunc(a, b interface{}) bool {
	return a == b
}

func (e *Engine) neFunc(a, b interface{}) bool {
	return a != b
}

func (e *Engine) ltFunc(a, b interface{}) bool {
	return funcs.ToFloat64(a) < funcs.ToFloat64(b)
}

func (e *Engine) leFunc(a, b interface{}) bool {
	return funcs.ToFloat64(a) <= funcs.ToFloat64(b)
}

func (e *Engine) gtFunc(a, b interface{}) bool {
	return funcs.ToFloat64(a) > funcs.ToFloat64(b)
}

func (e *Engine) geFunc(a, b interface{}) bool {
	return funcs.ToFloat64(a) >= funcs.ToFloat64(b)
}

func (e *Engine) lenFunc(item interface{}) int {
	switch v := item.(type) {
	case string:
		return len(v)
	case []interface{}:
		return len(v)
	case types.Slice:
		return len(v)
	case types.SDict:
		return len(v)
	case types.Dict:
		return len(v)
	case map[string]interface{}:
		return len(v)
	default:
		return 0
	}
}

func (e *Engine) indexFunc(item interface{}, indices ...interface{}) interface{} {
	if len(indices) == 0 {
		return nil
	}

	current := item
	for _, idx := range indices {
		switch v := current.(type) {
		case types.Slice:
			i := funcs.ToInt(idx)
			if i >= 0 && i < len(v) {
				current = v[i]
			} else {
				return nil
			}
		case []interface{}:
			i := funcs.ToInt(idx)
			if i >= 0 && i < len(v) {
				current = v[i]
			} else {
				return nil
			}
		case types.SDict:
			current = v[funcs.ToString(idx)]
		case map[string]interface{}:
			current = v[funcs.ToString(idx)]
		case types.Dict:
			current = v[idx]
		default:
			return nil
		}
	}
	return current
}

func (e *Engine) returnFunc(args ...interface{}) string {
	// In templates, return just stops execution
	return ""
}

func (e *Engine) tryFunc(args ...interface{}) interface{} {
	// Simplified try - in real implementation would catch panics
	if len(args) > 0 {
		return args[0]
	}
	return nil
}

// parseArgs wraps the funcs.ArgsParser for template use.
func (e *Engine) parseArgs(numRequired int, failedMessage string, argDefs ...*funcs.ArgDef) (*funcs.ParsedArgs, error) {
	parser := funcs.NewArgsParser(e.ctx.CmdArgs)
	return parser.ParseArgs(numRequired, failedMessage, argDefs...)
}
