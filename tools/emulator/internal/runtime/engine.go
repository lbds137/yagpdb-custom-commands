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
		"dbDelMultiple":       dbFuncs.DbDelMultiple,

		// Discord mocks (output capture)
		"sendMessage":               e.sendMessage,
		"sendMessageRetID":          e.sendMessageRetID,
		"sendDM":                    e.sendDM,
		"editMessage":               e.editMessage,
		"editMessageNoEscape":       e.editMessage,
		"getMessage":                e.getMessage,
		"deleteMessage":             e.deleteMessage,
		"deleteTrigger":             e.deleteTrigger,
		"deleteResponse":            e.deleteResponse,
		"addReactions":              e.addReactions,
		"addMessageReactions":       e.addMessageReactions,
		"deleteAllMessageReactions": e.deleteAllMessageReactions,

		// Role functions
		"setRoles": e.setRoles,

		// Member/user functions
		"getMember":              e.getMember,
		"userArg":                e.userArg,
		"getTargetPermissionsIn": e.getTargetPermissionsIn,

		// Channel functions
		"getChannel":         e.getChannel,
		"getChannelOrThread": e.getChannelOrThread,

		// Discord - Roles (lookup)
		"roleAbove": e.roleAbove,

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
		"mentionEveryone": e.mentionEveryone,
		"mentionHere":     e.mentionHere,

		// Argument parsing
		"parseArgs": e.parseArgs,
		"carg":      funcs.Carg,
	}
	for name, fn := range mocks {
		m[name] = fn
	}
	for name, fn := range e.roleFuncs() {
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

	// The output goes through YAGPDB's LimitWriter: leading whitespace is dropped, and
	// output past 25k fails unless the rest is whitespace. Outside strict mode a shadow
	// writer records that verdict while the run goes on (to 1 MiB, so a runaway loop
	// can't fill memory), and the breach becomes a warning.
	var buf bytes.Buffer
	var w io.Writer = yagstd.LimitWriter(&buf, maxOutputBytes)
	var yagpdbCap *shadowWriter
	if !e.ctx.Strict {
		yagpdbCap = &shadowWriter{
			shadow: yagstd.LimitWriter(io.Discard, maxOutputBytes),
			w:      yagstd.LimitWriter(&buf, maxOutputBytesLenient),
		}
		w = yagpdbCap
	}
	if err := tmpl.Execute(w, e.ctx.BuildTemplateData()); err != nil {
		if yagpdbCap != nil && yagpdbCap.err != nil {
			// YAGPDB would have stopped at the 25k limit, before this error
			e.ctx.Warn(KindLimit, "response grew too big (>25k); YAGPDB stops there, before the error below")
		}
		if err == io.ErrShortWrite { // the output writer's own error, as in YAGPDB; not a function's
			err = errors.New("response grew too big (>25k)")
			if !e.ctx.Strict {
				err = fmt.Errorf("response grew too big (>%d bytes)", maxOutputBytesLenient)
			}
		}
		// YAGPDB still sends what the template printed before the error
		return e.ctx.response(buf.String()), fmt.Errorf("template execution error: %w", err)
	}

	return e.ctx.checkOutput(buf.String(), time.Since(e.ctx.StartTime), yagpdbCap != nil && yagpdbCap.err != nil)
}

// Mock Discord functions

func (e *Engine) sendMessage(args ...interface{}) (string, error) {
	var channelID int64 = e.ctx.ChannelID
	var content string
	var embeds []interface{}
	var file *types.MessageSend
	var hasOther bool

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
			embeds = v.Embeds
			if v.HasFile {
				file = v
			}
			hasOther = v.HasOther
		case types.Embed:
			embeds = []interface{}{v}
		default:
			content = funcs.ToString(v)
		}
	}

	if ok, err := e.ctx.checkSend("sendMessage", content, embeds, file != nil || hasOther, false); !ok {
		return "", err
	}
	if file != nil {
		e.ctx.RecordFileUpload(channelID, file.Filename, file.File)
	}
	var embed interface{}
	if len(embeds) > 0 {
		embed = embeds[0]
	}
	e.lastMessageID = e.ctx.RecordSentMessage(channelID, content, embed)
	return "", nil
}

func (e *Engine) sendMessageRetID(args ...interface{}) (int64, error) {
	if _, err := e.sendMessage(args...); err != nil {
		return 0, err
	}
	return e.lastMessageID, nil
}

func (e *Engine) sendDM(msg interface{}) (string, error) {
	content := funcs.ToString(msg)
	// YAGPDB adds a server-info button, so a DM is never empty
	if ok, _ := e.ctx.checkSend("sendDM", content, nil, true, true); !ok {
		return "", nil
	}
	e.ctx.RecordSentMessage(0, content, nil) // 0 = DM
	return "", nil
}

// editMessage is YAGPDB's editMessage: it edits a message the bot sent, and Discord's
// limits apply to the edited message as they do to a sent one. Editing a message that
// doesn't exist, or someone else's, fails as Discord fails it.
func (e *Engine) editMessage(channel, msgID, msg interface{}) (string, error) {
	channelID := e.ctx.ChannelID
	if channel != nil {
		channelID = funcs.ToInt64(channel)
	}
	if channelID == 0 {
		return "", errors.New("unknown channel")
	}

	var change types.MessageEdit
	switch m := msg.(type) {
	case *types.MessageEdit:
		change = *m
		// YAGPDB's own check, before Discord sees the edit
		if !m.ComponentsV2 && m.Content != nil && strings.TrimSpace(*m.Content) == "" && len(m.Embeds) == 0 {
			return "", errors.New("both content and embed cannot be null")
		}
	case types.Embed:
		change.Embeds = []interface{}{m}
	default:
		content := fmt.Sprint(msg)
		change.Content = &content
	}

	id := funcs.ToInt64(msgID)
	var target *types.CtxMessage
	for i := range e.ctx.Messages {
		if m := &e.ctx.Messages[i]; m.ID == id && m.ChannelID == channelID {
			target = m
		}
	}
	switch {
	case target == nil:
		return "", e.ctx.discordRefuses("editMessage", "HTTP 404, 10008 Unknown Message",
			fmt.Sprintf("no message %d in channel %d", id, channelID))
	case target.Author.ID != botUser.ID:
		return "", e.ctx.discordRefuses("editMessage", "HTTP 403, 50005 Cannot edit a message authored by another user",
			fmt.Sprintf("message %d is by user %d", id, target.Author.ID))
	}

	content, embeds := target.Content, target.Embeds
	if change.Content != nil {
		content = *change.Content
	}
	if change.Embeds != nil {
		embeds = change.Embeds
	}
	if ok, err := e.ctx.checkSend("editMessage", content, embeds, change.HasOther, false); !ok {
		return "", err
	}
	target.Content, target.Embeds = content, embeds
	edited := SentMessage{ID: id, ChannelID: channelID, Content: content}
	if len(embeds) > 0 {
		edited.Embed = embeds[0]
	}
	e.ctx.EditedMessages = append(e.ctx.EditedMessages, edited)
	return "", nil
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

func (e *Engine) setRoles(target interface{}, roles interface{}) string {
	return ""
}

// Member/user functions

// getMember returns a mock member. When the test lists members (context.members), anyone
// else is not in the server and gets a nil *CtxMember, as in YAGPDB.
func (e *Engine) getMember(userID interface{}) *types.CtxMember {
	id := targetUserID(userID)
	if id == 0 || !e.ctx.isMember(id) {
		return nil
	}
	m := e.ctx.member(id)
	return &m
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
		case "components", "buttons", "menus", "forward", "sticker":
			// Not modelled, but they count as message content for Discord
			if val != nil {
				msg.HasOther = true
			}
		case "allowed_mentions", "reply", "silent", "ephemeral", "suppress_embeds", "is_components_v2":
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

// complexMessageEdit is YAGPDB's CreateMessageEdit: content and embeds, with the keys it
// accepts but the emulator doesn't model.
func (e *Engine) complexMessageEdit(args ...interface{}) (*types.MessageEdit, error) {
	if len(args) < 1 {
		return &types.MessageEdit{}, nil
	}
	if m, ok := args[0].(*types.MessageEdit); len(args) == 1 && ok {
		return m, nil
	}
	dict, err := yagstd.StringKeyDictionary(args...)
	if err != nil {
		return nil, err
	}

	msg := &types.MessageEdit{}
	for key, val := range dict {
		switch strings.ToLower(key) {
		case "content":
			temp := fmt.Sprint(val)
			msg.Content = &temp
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
		case "components", "buttons", "menus":
			if val != nil {
				msg.HasOther = true
			}
		case "is_components_v2":
			msg.ComponentsV2 = val != nil && val != false
		case "silent", "allowed_mentions", "suppress_embeds":
			// Accepted; the emulator doesn't model these
		default:
			return nil, errors.New(`invalid key "` + key + `" passed to message edit builder`)
		}
	}
	return msg, nil
}

func (e *Engine) sendTemplate(args ...interface{}) string {
	return ""
}

// roleAbove is YAGPDB's roleAbove (common.IsRoleAbove): a is above b by position, ties
// going to the lower ID; a nil a is never above, and anything is above a nil b.
func (e *Engine) roleAbove(a, b *types.CtxRole) bool {
	if a == nil {
		return false
	}

	if b == nil {
		return true
	}

	if a.Position != b.Position {
		return a.Position > b.Position
	}

	if a.ID == b.ID {
		return false
	}

	return a.ID < b.ID
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

	// YAGPDB's ChannelArg: nil is the current channel
	channelID := e.ctx.ChannelID
	if channel != nil {
		channelID = funcs.ToInt64(channel)
	}

	// Create child context (shares DB and other state)
	childCtx := &ExecutionContext{
		GuildID:         e.ctx.GuildID,
		GuildName:       e.ctx.GuildName,
		Prefix:          e.ctx.Prefix,
		ChannelID:       channelID,
		ChannelName:     e.ctx.ChannelName,
		UserID:          e.ctx.UserID,
		Username:        e.ctx.Username,
		Discriminator:   e.ctx.Discriminator,
		UserRoles:       e.ctx.UserRoles,
		MessageContent:  e.ctx.MessageContent, // YAGPDB passes the caller's message on
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
		Messages:        append([]types.CtxMessage(nil), e.ctx.Messages...),
		sentIDs:         e.ctx.sentMessageIDs(),
		Members:         e.ctx.Members,
		MemberRoles:     e.ctx.MemberRoles,
		MemberNicks:     e.ctx.MemberNicks,
		MemberJoinedAgo: e.ctx.MemberJoinedAgo,
	}

	// Execute child template
	childEngine := NewEngine(childCtx)
	_, err = childEngine.Execute(string(templateContent))

	// Propagate side effects back to parent
	e.ctx.SentMessages = append(e.ctx.SentMessages, childCtx.SentMessages...)
	e.ctx.EditedMessages = append(e.ctx.EditedMessages, childCtx.EditedMessages...)
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

func (e *Engine) mentionEveryone() string {
	return "@everyone"
}

func (e *Engine) mentionHere() string {
	return "@here"
}

// parseArgs is YAGPDB's parseArgs. It parses only for a message trigger: run by execCC, a
// reaction or an interval there is no .StrippedMsg, so it returns no arguments and no error
// (commands called both ways read .ExecData instead).
func (e *Engine) parseArgs(numRequired int, failedMessage string, argDefs ...*funcs.ArgDef) (*funcs.ParsedArgs, error) {
	if len(argDefs) == 0 || !e.ctx.triggered {
		return funcs.ParseArgs("", 0, "", nil, funcs.Lookups{})
	}
	return funcs.ParseArgs(e.ctx.StrippedMsg, numRequired, failedMessage, argDefs, funcs.Lookups{
		User: func(id int64) interface{} {
			if u := e.userArg(id); u != nil {
				return u
			}
			return nil
		},
		Member: func(id int64) interface{} {
			if m := e.getMember(id); m != nil {
				return m
			}
			return nil
		},
		Channel: func(id int64) interface{} { return e.getChannel(id) },
		Role: func(arg string) interface{} {
			if r := e.findRole(arg, acceptAllRoleInput); r != nil {
				return r
			}
			return nil
		},
	})
}
