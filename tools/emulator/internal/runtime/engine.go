package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
	template "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagtemplate"
)

// Engine handles template parsing and execution with YAGPDB's template package
// (internal/yagtemplate), which provides try/catch, while, return, execTemplate and the
// built-ins (and, or, not, eq, ne, lt, le, gt, ge, len, index) exactly as YAGPDB has them.
type Engine struct {
	ctx *ExecutionContext
	yag yagstd.Context // per-run state of YAGPDB's context functions (regex cache)

	lastMessageID int64 // ID of the message sendMessage last sent, for sendMessageRetID
}

// NewEngine creates a new template engine with the given context.
func NewEngine(ctx *ExecutionContext) *Engine {
	return &Engine{ctx: ctx}
}

// BuildFuncMap creates the FuncMap for template execution.
func (e *Engine) BuildFuncMap() template.FuncMap {
	dbFuncs := funcs.NewDatabaseFuncs(e.ctx.DB, e.ctx.GuildID)
	dbFuncs.OnStore = func(fn string, userID int64, key string, value interface{}) {
		if msg := e.ctx.Schema.Check(userID, key, value); msg != "" {
			e.ctx.Warn(KindSchema, "%s: %s", fn, msg)
		}
	}

	// YAGPDB's own implementations: standard functions, and the context functions that
	// keep per-run state (regex cache, sort). The rest are emulator mocks.
	m := template.FuncMap{}
	for name, fn := range yagstd.StandardFuncs() {
		m[name] = fn
	}
	for name, fn := range e.yag.Funcs() {
		m[name] = fn
	}
	mocks := template.FuncMap{
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
		"sendMessage":               e.sendMessage,
		"sendMessageRetID":          e.sendMessageRetID,
		"sendDM":                    e.sendDM,
		"editMessage":               e.editMessage,
		"getMessage":                e.getMessage,
		"deleteMessage":             e.deleteMessage,
		"deleteTrigger":             e.deleteTrigger,
		"deleteResponse":            e.deleteResponse,
		"addReactions":              e.addReactions,
		"addMessageReactions":       e.addMessageReactions,
		"deleteAllMessageReactions": e.deleteAllMessageReactions,

		// Role functions
		"hasRole":         e.hasRole,
		"hasRoleID":       e.hasRoleID,
		"targetHasRole":   e.targetHasRole,
		"targetHasRoleID": e.targetHasRoleID,
		"addRole":         e.addRole,
		"giveRole":        e.giveRole,
		"removeRole":      e.removeRole,
		"takeRole":        e.takeRole,
		"setRoles":        e.setRoles,
		"giveRoleID":      e.giveRoleID,
		"takeRoleID":      e.takeRoleID,
		"addRoleID":       e.addRoleID,
		"removeRoleID":    e.removeRoleID,

		// Member/user functions
		"getMember":              e.getMember,
		"userArg":                e.userArg,
		"getTargetPermissionsIn": e.getTargetPermissionsIn,

		// Channel functions
		"getChannel":         e.getChannel,
		"getChannelOrThread": e.getChannelOrThread,

		// Discord - Roles (lookup)
		"getRole": e.getRole,

		// Discord - Tickets
		"createTicket": e.createTicket,

		// Message builders (YAGPDB's build Discord structs)
		"cembed":             e.cembed,
		"complexMessage":     e.complexMessage,
		"complexMessageEdit": e.complexMessageEdit,
		"sendTemplate":       e.sendTemplate,

		// Control flow
		"execCC":                  e.execCC,
		"exec":                    e.exec,
		"execAdmin":               e.execAdmin,
		"scheduleUniqueCC":        e.scheduleUniqueCC,
		"cancelScheduledUniqueCC": e.cancelScheduledUniqueCC,
		"sleep":                   e.sleep,

		// Mention functions
		"mentionRoleID":   e.mentionRoleID,
		"mentionRole":     e.mentionRole,
		"mentionEveryone": e.mentionEveryone,
		"mentionHere":     e.mentionHere,

		// Argument parsing
		"parseArgs": e.parseArgs,
		"carg":      funcs.Carg,
	}
	for name, fn := range mocks {
		m[name] = fn
	}
	for name, fn := range m {
		m[name] = e.withLimits(name, fn)
	}
	return m
}

// Execute parses and executes a template.
func (e *Engine) Execute(source string) (string, error) {
	e.ctx.StartTime = time.Now()

	if err := e.ctx.checkSourceLength(source); err != nil {
		return "", err
	}

	tmpl := template.New("yagtest").Funcs(e.BuildFuncMap()).MaxOps(e.ctx.maxOps())
	if !e.ctx.Strict {
		tmpl = tmpl.OnMaxOps(func(ops, max int) {
			e.ctx.Warn(KindLimit, "the template ran over %d operations; YAGPDB stops a custom command at %d "+
				"(\"exceeded max operations\"). Loops over large ranges are the usual cause.", max, max)
		})
	}
	tmpl, err := tmpl.Parse(source)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	for _, f := range findLoopDBCalls(tmpl) {
		e.ctx.Warn(KindLoopDB, "%s", f.Message(e.ctx.SourceName))
	}

	// Like YAGPDB's LimitWriter, stop at the output cap. Outside strict mode the cap is
	// higher, so an oversized response is reported as a warning, but a runaway loop still
	// can't fill memory.
	var buf bytes.Buffer
	outCap := maxOutputBytes
	if !e.ctx.Strict {
		outCap = maxOutputBytesLenient
	}
	if err := tmpl.Execute(&limitWriter{w: &buf, n: outCap}, e.ctx.BuildTemplateData()); err != nil {
		if errors.Is(err, io.ErrShortWrite) || strings.Contains(err.Error(), io.ErrShortWrite.Error()) {
			err = fmt.Errorf("response grew too big (>%d bytes)", outCap)
		}
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return e.ctx.checkOutput(buf.String(), time.Since(e.ctx.StartTime))
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
		case *types.MessageSend:
			content = v.Content
			if len(v.Embeds) > 0 {
				embed = v.Embeds[0]
			}
			if v.HasFile {
				e.ctx.RecordFileUpload(channelID, v.Filename, v.File)
			}
		case types.Embed:
			embed = v
		default:
			content = funcs.ToString(v)
		}
	}

	e.lastMessageID = e.ctx.RecordSentMessage(channelID, content, embed)
	return ""
}

func (e *Engine) sendMessageRetID(args ...interface{}) int64 {
	e.sendMessage(args...)
	return e.lastMessageID
}

func (e *Engine) sendDM(msg interface{}) string {
	content := funcs.ToString(msg)
	e.ctx.RecordSentMessage(0, content, nil) // 0 = DM
	return ""
}

func (e *Engine) editMessage(channel, msgID, content interface{}) string {
	return ""
}

// getMessage returns a message the test declared (context.messages) or the run sent, or a
// nil *CtxMessage like YAGPDB's for a message that doesn't exist: `if $msg` is false and
// $msg.Author is a nil pointer error, as in production. A nil channel is the current one.
func (e *Engine) getMessage(channel, msgID interface{}) *types.CtxMessage {
	id := funcs.ToInt64(msgID)
	channelID := e.ctx.ChannelID
	if channel != nil {
		channelID = funcs.ToInt64(channel)
	}
	for i := range e.ctx.Messages {
		m := &e.ctx.Messages[i]
		if m.ID == id && m.ChannelID == channelID {
			return m
		}
	}
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

func (e *Engine) deleteAllMessageReactions(args ...interface{}) string {
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

// targetHasRole follows YAGPDB's targetHasRole: an unknown target, a user who isn't a
// member, or a role the guild doesn't have is an error.
func (e *Engine) targetHasRole(target, roleInput interface{}) (bool, error) {
	id := targetUserID(target)
	if id == 0 {
		return false, fmt.Errorf("target %v not found", target)
	}
	if !e.ctx.isMember(id) {
		return false, fmt.Errorf("member not found in state")
	}
	role := e.findRole(roleInput)
	if role == nil {
		return false, fmt.Errorf("role %v not found", roleInput)
	}
	for _, r := range e.ctx.rolesOf(id) {
		if r == role.ID {
			return true, nil
		}
	}
	return false, nil
}

func (e *Engine) targetHasRoleID(target, roleID interface{}) (bool, error) {
	return e.targetHasRole(target, roleID)
}

// findRole resolves a role ID, mention or name. When the test declares the guild's roles,
// unknown roles are nil, as in YAGPDB; otherwise any ID is taken to exist.
func (e *Engine) findRole(roleInput interface{}) *types.CtxRole {
	var id int64
	if s, ok := roleInput.(string); ok {
		t := strings.TrimSpace(s)
		if strings.HasPrefix(t, "<@&") && strings.HasSuffix(t, ">") {
			id = funcs.ToInt64(t[3 : len(t)-1])
		} else if id = funcs.ToInt64(t); id == 0 {
			for _, r := range e.ctx.AvailableRoles {
				if strings.EqualFold(r.Name, t) {
					role := r
					return &role
				}
			}
			return nil
		}
	} else {
		id = funcs.ToInt64(roleInput)
	}
	if role, ok := e.ctx.AvailableRoles[id]; ok {
		return &role
	}
	if len(e.ctx.AvailableRoles) > 0 || id == 0 {
		return nil
	}
	return &types.CtxRole{ID: id, Name: "MockRole", Color: 0x7289DA}
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

// getMember returns a mock member. When the test lists members (context.members), anyone
// else is not in the server and gets a nil *CtxMember, as in YAGPDB.
func (e *Engine) getMember(userID interface{}) *types.CtxMember {
	id := targetUserID(userID)
	if id == 0 || !e.ctx.isMember(id) {
		return nil
	}
	return &types.CtxMember{
		User: types.DiscordUser{
			ID:       id,
			Username: "MockUser",
		},
		Roles: e.ctx.rolesOf(id),
	}
}

// userArg follows YAGPDB's userArg (commands/tmplexec.go): an ID or mention of a server
// member gives that user; anything else that isn't a string or number is returned as is;
// otherwise the result is nil, so (userArg $x).ID reads as no value.
func (e *Engine) userArg(arg interface{}) interface{} {
	id := funcs.ToInt64(arg)
	if id == 0 {
		str, ok := arg.(string)
		if !ok {
			return arg
		}
		str = strings.TrimSpace(str)
		if len(str) < 5 || !strings.HasPrefix(str, "<@") || !strings.HasSuffix(str, ">") {
			return nil
		}
		id = funcs.ToInt64(strings.TrimPrefix(str[2:len(str)-1], "!"))
	}
	if id == 0 || !e.ctx.isMember(id) {
		return nil
	}
	if id == e.ctx.UserID {
		return &types.DiscordUser{ID: id, Username: e.ctx.Username, Discriminator: e.ctx.Discriminator}
	}
	return &types.DiscordUser{ID: id, Username: "MockUser"}
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

func (e *Engine) getChannelOrThread(channelID interface{}) interface{} {
	// Same as getChannel - threads are just channels in Discord's API
	return e.getChannel(channelID)
}

// Embed/message building

// cembed follows YAGPDB's CreateEmbed: one sdict (or map) as the embed, or key/value
// pairs under sdict's rules, converted to a Discord embed (wrong value types are errors).
func (e *Engine) cembed(args ...interface{}) (types.Embed, error) {
	if len(args) < 1 {
		return types.Embed{}, nil
	}
	return toEmbed(args...)
}

func toEmbed(args ...interface{}) (types.Embed, error) {
	switch t := args[0].(type) {
	case types.Embed:
		return t, nil
	case types.SDict:
		return types.BuildEmbed(t)
	case *types.SDict:
		return types.BuildEmbed(*t)
	case map[string]interface{}:
		return types.BuildEmbed(t)
	}
	d, err := yagstd.StringKeyDictionary(args...)
	if err != nil {
		return nil, err
	}
	return types.BuildEmbed(d)
}

// complexMessage follows YAGPDB's CreateMessageSend (common/templates/general.go): known keys
// are case-insensitive, an unknown key is an error, "embed" takes one embed or a slice of
// up to 10, and a file gets a .txt name.
func (e *Engine) complexMessage(args ...interface{}) (*types.MessageSend, error) {
	if len(args) < 1 {
		return &types.MessageSend{}, nil
	}
	if m, ok := args[0].(*types.MessageSend); len(args) == 1 && ok {
		return m, nil
	}
	// Keys and values as YAGPDB's StringKeyDictionary reads them (one sdict, or pairs)
	dict, err := yagstd.StringKeyDictionary(args...)
	if err != nil {
		return nil, err
	}

	msg := &types.MessageSend{}
	filename := "attachment_" + time.Now().Format("2006-01-02_15-04-05")
	for key, val := range dict {
		switch strings.ToLower(key) {
		case "content":
			msg.Content = funcs.ToString(val)
		case "embed":
			if val == nil {
				continue
			}
			rv := reflect.ValueOf(val)
			for rv.Kind() == reflect.Pointer {
				rv = rv.Elem()
			}
			if rv.Kind() == reflect.Slice {
				for j := 0; j < rv.Len() && j < 10; j++ {
					embed, err := toEmbed(rv.Index(j).Interface())
					if err != nil {
						return nil, err
					}
					msg.Embeds = append(msg.Embeds, embed)
				}
			} else {
				embed, err := toEmbed(val)
				if err != nil {
					return nil, err
				}
				msg.Embeds = append(msg.Embeds, embed)
			}
		case "file":
			file := funcs.ToString(val)
			if len(file) > 100000 {
				return nil, fmt.Errorf("file length for send message builder exceeded size limit")
			}
			msg.File, msg.HasFile = file, true
		case "filename":
			filename = funcs.ToString(val)
			if r := []rune(filename); len(r) > 64 {
				filename = string(r[:64])
			}
		case "allowed_mentions", "reply", "silent", "components", "ephemeral", "buttons", "menus",
			"forward", "sticker", "suppress_embeds", "is_components_v2":
			// Accepted; the emulator doesn't model these
		default:
			return nil, fmt.Errorf(`invalid key "%s" passed to send message builder.`, key)
		}
	}
	if msg.HasFile {
		msg.Filename = filename + ".txt"
	}
	return msg, nil
}

// complexMessageEdit builds the dict editMessage (a no-op mock) receives, under sdict's
// rules for keys and values.
func (e *Engine) complexMessageEdit(args ...interface{}) (types.SDict, error) {
	if len(args) == 0 {
		return types.SDict{}, nil
	}
	return yagstd.StringKeyDictionary(args...)
}

func (e *Engine) sendTemplate(args ...interface{}) string {
	return ""
}

// getRole returns the role, or a nil *CtxRole for a role the guild doesn't have (YAGPDB
// returns nil without an error, so (getRole $id).Color then fails).
func (e *Engine) getRole(roleID interface{}) *types.CtxRole {
	return e.findRole(roleID)
}

// createTicket creates a mock ticket and returns result with ChannelID
func (e *Engine) createTicket(user, reason interface{}) types.SDict {
	// Return mock ticket result
	return types.SDict{
		"ChannelID": int64(999999999),
		"TicketID":  int64(1),
	}
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
		Strict:          e.ctx.Strict,
		DB:              e.ctx.DB, // Share database
		Schema:          e.ctx.Schema,
		Counters:        make(map[string]int), // execCC starts a new run with its own limits
		StartTime:       e.ctx.StartTime,
		AvailableRoles:  e.ctx.AvailableRoles,
		OwnerID:         e.ctx.OwnerID,
		CommandIDMap:    e.ctx.CommandIDMap,
		ExecCCDepth:     e.ctx.ExecCCDepth + 1,
		MaxExecCCDepth:  e.ctx.MaxExecCCDepth,
		TemplateBaseDir: e.ctx.TemplateBaseDir,
		SourceName:      templatePath,
		// The same server; a copy, since execCC runs after the caller finishes
		Messages:    append([]types.CtxMessage(nil), e.ctx.Messages...),
		sentIDs:     e.ctx.sentMessageIDs(),
		Members:     e.ctx.Members,
		MemberRoles: e.ctx.MemberRoles,
	}

	// Execute child template
	childEngine := NewEngine(childCtx)
	_, err = childEngine.Execute(string(templateContent))

	// Propagate side effects back to parent
	e.ctx.SentMessages = append(e.ctx.SentMessages, childCtx.SentMessages...)
	e.ctx.RoleChanges = append(e.ctx.RoleChanges, childCtx.RoleChanges...)
	e.ctx.FileUploads = append(e.ctx.FileUploads, childCtx.FileUploads...)
	if err != nil {
		// YAGPDB posts a failed execCC's error in the target channel; the caller carries on.
		e.ctx.Warn(KindExecCC, "execCC %d (%s) failed: %v", commandID, filepath.Base(templatePath), err)
	}
	for _, d := range childCtx.Diagnostics {
		d.Message = fmt.Sprintf("execCC %d (%s): %s", commandID, filepath.Base(templatePath), d.Message)
		e.ctx.Diagnostics = append(e.ctx.Diagnostics, d)
	}

	// execCC doesn't return output to the caller
	return ""
}

func (e *Engine) scheduleUniqueCC(ccID, channel, delay, key, data interface{}) string {
	return ""
}

func (e *Engine) cancelScheduledUniqueCC(ccID, key interface{}) string {
	return ""
}

// sleep is a no-op in emulator (YAGPDB uses this to delay execution)
func (e *Engine) sleep(args ...interface{}) string {
	// In real YAGPDB this pauses execution; we skip for testing speed
	return ""
}

// exec executes another template inline (simpler than execCC)
func (e *Engine) exec(name string, data ...interface{}) string {
	// In YAGPDB this executes a named template
	// For now, just return empty - templates would need to be registered
	return ""
}

func (e *Engine) execAdmin(name string, data ...interface{}) string {
	// In YAGPDB this executes a template with admin privileges
	// For testing, same as exec
	return e.exec(name, data...)
}

// Mention functions

func (e *Engine) mentionRoleID(roleID interface{}) string {
	return fmt.Sprintf("<@&%d>", funcs.ToInt64(roleID))
}

func (e *Engine) mentionRole(roleName interface{}) string {
	// In real YAGPDB this would look up the role by name
	return fmt.Sprintf("@%s", funcs.ToString(roleName))
}

func (e *Engine) mentionEveryone() string {
	return "@everyone"
}

func (e *Engine) mentionHere() string {
	return "@here"
}

// parseArgs wraps the funcs.ArgsParser for template use.
func (e *Engine) parseArgs(numRequired int, failedMessage string, argDefs ...*funcs.ArgDef) (*funcs.ParsedArgs, error) {
	parser := funcs.NewArgsParser(e.ctx.CmdArgs)
	return parser.ParseArgs(numRequired, failedMessage, argDefs...)
}
