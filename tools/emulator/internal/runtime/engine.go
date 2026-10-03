package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	yagstd "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
	template "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagtemplate"
)

// Engine handles template parsing and execution with YAGPDB's template package
// (internal/yagtemplate), which provides try/catch, while, return, execTemplate and the
// built-ins (and, or, not, eq, ne, lt, le, gt, ge, len, index) exactly as YAGPDB has them.
type Engine struct {
	ctx *ExecutionContext
	yag yagstd.Context // per-run state of YAGPDB's context functions (regex cache)

	lastMessageID int64          // ID of the message sendMessage last sent, for sendMessageRetID
	mockRoles     map[int64]bool // role IDs already warned about, see guildRole
	mockChannels  map[int64]bool // channel IDs already warned about, see channelArg
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
	dbFuncs.OnPlanRisk = func(fn, pattern string) {
		e.ctx.Warn(KindDB, "%s: the pattern %q ends with the escape character after a wildcard; "+
			"Postgres may reject it (\"LIKE pattern must not end with escape character\") while "+
			"planning, whatever this server's keys are", fn, pattern)
	}

	// YAGPDB's own implementations: standard functions, and the context functions that
	// keep per-run state (regex cache, sort). The rest are emulator mocks.
	m := template.FuncMap{}
	for name, fn := range yagstd.StandardFuncs() {
		m[name] = fn
	}
	for name, fn := range yagstd.RunFuncs(e.ctx.Now, e.ctx.random()) {
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
		"sendMessageNoEscape":       e.sendMessageNoEscape,
		"sendMessageNoEscapeRetID":  e.sendMessageNoEscapeRetID,
		"sendDM":                    e.sendDM,
		"editMessage":               e.editMessage,
		"editMessageNoEscape":       e.editMessage,
		"getMessage":                e.getMessage,
		"deleteMessage":             e.deleteMessage,
		"deleteTrigger":             e.deleteTrigger,
		"deleteResponse":            e.deleteResponse,
		"addReactions":              e.addReactions,
		"addMessageReactions":       e.addMessageReactions,
		"addResponseReactions":      e.addResponseReactions,
		"deleteMessageReaction":     e.deleteMessageReaction,
		"deleteAllMessageReactions": e.deleteAllMessageReactions,

		// Interaction responses (context_interactions.go:17-30; editResponse, getResponse
		// and deleteInteractionResponse aren't modelled yet)
		"sendResponse":              e.sendResponseFunc("sendResponse", true, false),
		"sendResponseNoEscape":      e.sendResponseFunc("sendResponseNoEscape", false, false),
		"sendResponseNoEscapeRetID": e.sendResponseFunc("sendResponseNoEscapeRetID", false, true),
		"sendResponseRetID":         e.sendResponseFunc("sendResponseRetID", true, true),
		"updateMessage":             e.updateMessageFunc("updateMessage", true),
		"updateMessageNoEscape":     e.updateMessageFunc("updateMessageNoEscape", false),
		"ephemeralResponse":         e.ephemeralResponse,
		"sendModal":                 e.sendModal,

		// Role functions
		"setRoles": e.setRoles,

		// Member/user functions
		"getMember":              e.getMember,
		"userArg":                e.userArg,
		"getTargetPermissionsIn": e.getTargetPermissionsIn,

		// Channel functions
		"getChannel":         e.getChannel,
		"getChannelOrThread": e.getChannelOrThread,

		// Forum functions
		"createForumPost": e.createForumPost,
		"createThread":    e.createThread,

		// Discord - Roles (lookup)
		"roleAbove": e.roleAbove,

		// Discord - Tickets
		"createTicket": e.createTicket,

		// Message builders (YAGPDB's build Discord structs)
		"cembed":             e.cembed,
		"complexMessage":     e.complexMessage,
		"complexMessageEdit": e.complexMessage, // both CreateComplexMessage (context.go:113-114)
		"sendTemplate":       e.sendTemplate,
		// Message component and modal builders (context.go:100-105)
		"cbutton":      CreateButton,
		"cmenu":        CreateSelectMenu,
		"cmodal":       CreateModal,
		"modalBuilder": CreateModalBuilder,
		"clabel":       CreateLabel,
		"ctextInput":   CreateTextInput,

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
	out, err := e.execute(source)
	settings := ReadErrorSettings(source)
	// The response is sent (a deleteResponse delay under 1 sends none; a failed run's
	// output still holds it here; with show_errors the output goes out with the error
	// message instead, interaction or not: bot.go:773-780)
	responds := strings.TrimSpace(out) != "" && (err == nil || !settings.ShowErrors) &&
		!(e.ctx.delResponse && e.ctx.delResponseDelay < 1)
	switch {
	case e.ctx.Interaction != nil:
		// With an interaction the output answers it (SendResponse, context.go:603-611),
		// for an execCC child too, whose interaction is its caller's
		if responds {
			e.ctx.respondWithOutput(out)
		}
		if e.ctx.ExecCCDepth == 0 {
			e.ctx.warnUnanswered()
		}
	case e.ctx.ExecCCDepth == 0 && responds:
		// An execCC child's is recorded by execCC, which knows its message.
		e.ctx.recordResponseSent(e.ctx.ChannelID, 0)
	}
	if err == nil {
		return out, nil
	}
	if settings.ShowErrors {
		// ExecuteCustomCommand posts the output and the error in the command's (or the
		// redirect-errors) channel with ChannelMessageSend, whose empty allowed mentions
		// ping no one, and sends no response
		errChannel := e.ctx.ChannelID
		if settings.RedirectChannel != 0 {
			errChannel = settings.RedirectChannel
		}
		msg := out + "\nAn error caused the execution of the custom command template to stop:\n" +
			formatCustomCommandRunErr(source, err)
		// ChannelMessageSend's error is discarded (bot.go), so a message Discord would
		// reject (over 2000 runes, on top of out's own cap) is silently never posted
		if ok, _ := e.ctx.checkSend("show_errors message", msg, nil, false, true); ok {
			e.ctx.RecordSentMessage(errChannel, msg, nil, nil, Pings{})
		}
		e.ctx.ResponsePings = Pings{}
	} else if e.ctx.delResponse && e.ctx.delResponseDelay < 1 {
		// Without show_errors the output is the response, which deleteResponse can drop
		e.ctx.ResponsePings = Pings{}
		return "", err
	}
	return out, err
}

// execute runs the template and returns its response, or what it printed before an error.
func (e *Engine) execute(source string) (string, error) {
	// An execCC child keeps its caller's start: the trigger's timestamp and join times
	// don't move with the caller's sleeps
	if e.ctx.ExecCCDepth == 0 {
		e.ctx.StartTime = e.ctx.Now()
	}

	if err := e.ctx.checkSourceLength(source); err != nil {
		return "", err
	}

	// YAGPDB names a command's template "CC #<number>", which its errors quote
	name := "yagtest"
	if e.ctx.CCID != 0 {
		name = fmt.Sprintf("CC #%d", e.ctx.CCID)
	}
	tmpl := template.New(name).Funcs(e.BuildFuncMap()).MaxOps(e.ctx.maxOps())
	tmpl = tmpl.OnCall(func(inTry bool) { e.ctx.inTry = inTry })
	if !e.ctx.Strict {
		tmpl = tmpl.OnMaxOps(func(ops, max int) {
			e.ctx.Warn(KindLimit, "the template ran over %d operations; YAGPDB stops a custom command at %d "+
				"(\"exceeded max operations\"). Loops over large ranges are the usual cause.", max, max)
		})
	}
	tmpl, err := tmpl.Parse(source)
	if err != nil {
		return "", fmt.Errorf("Failed parsing template: %w", err)
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
	err = tmpl.Execute(w, e.ctx.BuildTemplateData())
	e.ctx.inTry = false
	if err != nil {
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
		return e.ctx.response(buf.String()), fmt.Errorf("Failed executing template: %w", err)
	}

	return e.ctx.checkOutput(buf.String(), yagpdbCap != nil && yagpdbCap.err != nil)
}

// Mock Discord functions

func (e *Engine) sendMessage(args ...interface{}) (string, error) {
	return e.send("sendMessage", true, args...)
}

// sendMessageNoEscape is sendMessage with every mention the bot may ping pinging, whatever
// a complexMessage's allowed_mentions said.
func (e *Engine) sendMessageNoEscape(args ...interface{}) (string, error) {
	return e.send("sendMessageNoEscape", false, args...)
}

// parseMessageInput is YAGPDB's (context_funcs.go:32-70): what the send, edit and
// interaction response functions make of their message argument. An embed read back from
// a message is a *MessageEmbed (discordgo's), cembed's result a types.Embed. A cmodal's
// .Data is an *InteractionResponseData, whose ToMessageSend (discordgo interactions.go:
// 600-612) keeps its content, embeds, components and allowed mentions (none set: nobody
// pinged; its flags a template can't set). YAGPDB's *ComponentBuilder case isn't here:
// the emulator has no componentBuilder, so such a value can't reach this.
func parseMessageInput(msg interface{}) *types.MessageSend {
	msgSend := &types.MessageSend{AllowedMentions: usersOnly()}

	switch typedMsg := msg.(type) {
	case types.Embed:
		msgSend.Embeds = []interface{}{typedMsg}
	case *types.MessageEmbed:
		msgSend.Embeds = []interface{}{embedOrNil(typedMsg)}
	case []*types.MessageEmbed:
		if typedMsg != nil {
			msgSend.Embeds = make([]interface{}, 0, len(typedMsg))
		}
		for _, embed := range typedMsg {
			msgSend.Embeds = append(msgSend.Embeds, embedOrNil(embed))
		}
	case *types.MessageSend:
		msgSend = typedMsg
	case *types.InteractionResponseData:
		msgSend = &types.MessageSend{Content: typedMsg.Content, Components: typedMsg.Components}
		if typedMsg.Embeds != nil {
			msgSend.Embeds = make([]interface{}, 0, len(typedMsg.Embeds))
		}
		for _, embed := range typedMsg.Embeds {
			msgSend.Embeds = append(msgSend.Embeds, embedOrNil(embed))
		}
		if typedMsg.AllowedMentions != nil {
			msgSend.AllowedMentions = *typedMsg.AllowedMentions
		}
	default:
		msgSend.Content = funcs.ToString(msg)
	}

	if !msgSend.ComponentsV2 && len(msgSend.Embeds) > 0 {
		// only keep valid embeds (YAGPDB also drops GetMarshalNil ones, which no template
		// function makes)
		var embeds []interface{}
		for _, embed := range msgSend.Embeds {
			if embed != nil {
				embeds = append(embeds, embed)
			}
		}
		msgSend.Embeds = embeds
	}

	return msgSend
}

// embedOrNil converts a discordgo-shaped embed to an Embed, keeping a nil one an untyped
// nil (a typed nil in an interface{} is not == nil) so parseMessageInput drops it.
func embedOrNil(embed *types.MessageEmbed) interface{} {
	if embed == nil {
		return nil
	}
	return types.EmbedMap(embed)
}

// send is YAGPDB's tmplSendMessage; filterSpecialMentions keeps roles and @everyone from
// pinging unless a complexMessage allows them.
func (e *Engine) send(fn string, filterSpecialMentions bool, args ...interface{}) (string, error) {
	var channelID int64 = e.ctx.ChannelID

	e.lastMessageID = 0 // nothing sent yet
	if len(args) >= 1 {
		// tmplSendMessage: a channel that isn't there sends nothing, without an error
		if channelID = e.channelArg(args[0]); channelID == 0 {
			return "", nil
		}
	}
	msgSend := &types.MessageSend{AllowedMentions: usersOnly()}
	if len(args) >= 2 {
		msgSend = parseMessageInput(args[1])
	}
	content, embeds, hasOther := msgSend.Content, msgSend.Embeds, msgSend.HasOther
	allowed, replyTo := msgSend.AllowedMentions, msgSend.ReplyTo
	components := msgSend.Components
	var file *types.MessageSend
	if msgSend.HasFile {
		file = msgSend
	}

	if !filterSpecialMentions {
		allowed = noEscape()
	}

	// context_funcs.go's tmplSendMessage has no permission pre-check: the error below is
	// what ChannelMessageSendComplex itself would return
	if e.ctx.ChannelsCannotSend[channelID] {
		return "", e.ctx.discordRefuses(fn, errMissingPerms,
			fmt.Sprintf("the bot lacks permission to send messages in channel %d", channelID))
	}

	notEmpty := file != nil || hasOther || len(components) > 0
	if ok, err := e.ctx.checkSend(fn, content, embeds, notEmpty, false); !ok {
		return "", err
	}
	if refused, err := e.ctx.checkComponents(fn, components); refused {
		return "", err
	}
	if file != nil {
		e.ctx.RecordFileUpload(channelID, file.Filename, file.File)
	}
	pings := e.ctx.pings(content, allowed, channelID, replyTo)
	e.lastMessageID = e.ctx.RecordSentMessage(channelID, content, embeds, components, pings)
	return "", nil
}

// sendMessageRetID returns the sent message's ID, or "" (as YAGPDB) when nothing was sent.
func (e *Engine) sendMessageRetID(args ...interface{}) (interface{}, error) {
	_, err := e.sendMessage(args...)
	return e.sentID(), err
}

// sentID is what the RetID functions return: the ID, or "" when nothing was sent.
func (e *Engine) sentID() interface{} {
	if e.lastMessageID == 0 {
		return ""
	}
	return e.lastMessageID
}

func (e *Engine) sendMessageNoEscapeRetID(args ...interface{}) (interface{}, error) {
	_, err := e.sendMessageNoEscape(args...)
	return e.sentID(), err
}

func (e *Engine) sendDM(msg interface{}) (string, error) {
	if e.ctx.NoMember {
		// tmplSendDM returns "" with a nil member (context_funcs.go:75): a context menu run
		// (handle_contextmenu.go:118-120, so an entry can't DM an arbitrary target) or a
		// scheduled one has no member to DM
		e.ctx.Warn(KindResponse, "sendDM: the run has no member (a context menu or scheduled run), so "+
			"YAGPDB sends no DM and returns \"\"")
		return "", nil
	}
	content := funcs.ToString(msg)
	// YAGPDB adds a server-info button, so a DM is never empty
	if ok, _ := e.ctx.checkSend("sendDM", content, nil, true, true); !ok {
		return "", nil
	}
	e.ctx.RecordSentMessage(0, content, nil, nil, Pings{}) // 0 = DM; a DM pings no one
	return "", nil
}

// editMessage is YAGPDB's editMessage: it edits a message the bot sent, and Discord's
// limits apply to the edited message as they do to a sent one. Editing a message that
// doesn't exist, or someone else's, fails as Discord fails it.
//
// As tmplEditMessage (context_funcs.go:440-470) it takes any message input
// (parseMessageInput) and edits with msgSend.ToMessageEdit(), which always sets the
// content (lib/discordgo/message.go:683-690): an edit without content clears the old
// text. Embeds and components are sent only when non-nil (MessageEdit.MarshalJSON), so
// nil keeps them; an empty "components" slice clears the message's buttons and menus.
func (e *Engine) editMessage(channel, msgID, msg interface{}) (string, error) {
	channelID := e.channelArg(channel) // ChannelArgNoDM
	if channelID == 0 {
		return "", errors.New("unknown channel")
	}

	change := parseMessageInput(msg)

	id := funcs.ToInt64(msgID)
	target := e.ctx.knownMessage(channelID, id)
	switch {
	case target == nil:
		return "", e.ctx.discordRefuses("editMessage", errUnknownMessage,
			fmt.Sprintf("no message %d in channel %d", id, channelID))
	case target.Author.ID != botUser.ID:
		return "", e.ctx.discordRefuses("editMessage", errEditOthers,
			fmt.Sprintf("message %d is by user %d", id, target.Author.ID))
	}

	content, embeds := change.Content, types.EmbedMaps(target.Embeds)
	if change.Embeds != nil {
		embeds = change.Embeds
	}
	components := target.Components
	if change.Components != nil {
		components = change.Components
	}
	notEmpty := change.HasOther || len(components) > 0
	if ok, err := e.ctx.checkSend("editMessage", content, embeds, notEmpty, false); !ok {
		return "", err
	}
	if refused, err := e.ctx.checkComponents("editMessage", components); refused {
		return "", err
	}
	target.Content, target.Embeds = content, types.EmbedStructs(embeds)
	target.Components = components
	target.EditedTimestamp = types.NewTimestamp(e.ctx.Now()) // Discord sets it on every edit
	edited := SentMessage{ID: id, ChannelID: channelID, Content: content, Embeds: embeds,
		Components: components}
	e.ctx.EditedMessages = append(e.ctx.EditedMessages, edited)
	return "", nil
}

// getMessage returns a message the emulator knows (knownMessage: a test's, a sent one, the
// triggering message), or a nil *CtxMessage like YAGPDB's for a message that doesn't exist:
// `if $msg` is false and $msg.Author is a nil pointer error, as in production. A nil
// channel is the current one. The message is a copy, as YAGPDB's is fetched from Discord
// on each call: a later edit doesn't show through a message fetched before it.
func (e *Engine) getMessage(channel, msgID interface{}) *types.CtxMessage {
	id := funcs.ToInt64(msgID)
	channelID := e.channelArg(channel) // an unknown channel finds nothing, as in YAGPDB
	known := e.ctx.knownMessage(channelID, id)
	if known == nil {
		return nil
	}
	fetched := *known
	fetched.Attachments = append([]interface{}(nil), known.Attachments...)
	fetched.Embeds = types.EmbedStructs(types.EmbedMaps(known.Embeds)) // new embeds, not shared ones
	fetched.Components = types.CloneComponents(known.Components)
	return &fetched
}

// deleteMessage is YAGPDB's tmplDelMessage: the message is deleted after the delay (10
// seconds by default, at most a day), and nothing happens for an unknown channel. The
// deletion is recorded; whether the message exists doesn't matter (YAGPDB ignores the error).
func (e *Engine) deleteMessage(channel, msgID interface{}, args ...interface{}) string {
	return e.delMessage("message", channel, msgID, args...)
}

// deleteTrigger is YAGPDB's tmplDelTrigger: deleteMessage of the run's message, which is
// the reacted-to message in a reaction run and the caller's in an execCC child. A run
// without a message (an interval run) has Context.Execute's stand-in with ID 0: Discord
// refuses to delete it and YAGPDB ignores the error, so nothing is recorded.
func (e *Engine) deleteTrigger(args ...interface{}) string {
	m := e.ctx.triggerMsg()
	if m.ID == 0 {
		return ""
	}
	return e.delMessage("trigger", m.ChannelID, m.ID, args...)
}

func (e *Engine) delMessage(of string, channel, msgID interface{}, args ...interface{}) string {
	channelID := e.channelArg(channel) // ChannelArgNoDM
	if channelID == 0 {
		return ""
	}
	dur := 10
	if len(args) > 0 {
		dur = int(funcs.ToInt64(args[0]))
	}
	if dur > 86400 {
		dur = 86400
	}
	e.ctx.recordDeletion(of, channelID, funcs.ToInt64(msgID), dur)
	return ""
}

// deleteResponse is YAGPDB's tmplDelResponse: the response is deleted after the delay (10
// seconds by default, at most a day), which is recorded when the response is sent; with a
// delay under 1 it isn't sent at all.
func (e *Engine) deleteResponse(args ...interface{}) string {
	dur := 10
	if len(args) > 0 {
		dur = int(funcs.ToInt64(args[0]))
	}
	if dur > 86400 {
		dur = 86400
	}
	e.ctx.delResponseDelay = dur
	e.ctx.delResponse = true
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
	return &types.DiscordUser{ID: id, Username: "MockUser", Discriminator: "0"}
}

func (e *Engine) getTargetPermissionsIn(userID, channelID interface{}) int64 {
	return 0
}

// Channel functions

// getChannel is YAGPDB's tmplGetChannel: nil (no error) for an unresolvable channel; a
// resolved thread errors "channel not in state" instead, since GS.GetChannel (channels
// only) never finds it (vendor common/templates/context_funcs.go). A channel assumed to
// exist (no channels declared) has no name.
func (e *Engine) getChannel(channel interface{}) (*types.CtxChannel, error) {
	id := e.channelArg(channel)
	if id == 0 {
		return nil, nil // vendor: don't send an error, a nil output means invalid/unknown
	}
	if _, ok := e.ctx.Threads[id]; ok {
		return nil, errors.New("channel not in state")
	}
	c := e.ctx.channelState(id)
	return &c, nil
}

// getChannelOrThread is YAGPDB's tmplGetChannelOrThread: like getChannel, but
// GS.GetChannelOrThread also finds a resolved thread, so it never errors for one.
func (e *Engine) getChannelOrThread(channel interface{}) (*types.CtxChannel, error) {
	id := e.channelArg(channel)
	if id == 0 {
		return nil, nil
	}
	c := e.ctx.channelState(id)
	return &c, nil
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
	case *types.MessageEmbed: // CreateEmbed returns a *discordgo.MessageEmbed as it is
		return types.EmbedMap(t), nil
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

// builderPairs is YAGPDB's CreateComponentBuilder (general.go:117-175) as
// CreateComplexMessage reads it: one map's entries (in map order, as there), or the
// key-value pairs in order, a repeated key kept each time. Its *ComponentBuilder
// argument isn't modelled (no componentBuilder).
func builderPairs(values ...interface{}) (keys []string, vals []interface{}, err error) {
	if len(values) == 1 {
		// The one-map case reads the map as StringKeyDictionary does, with its errors
		dict, err := yagstd.StringKeyDictionary(values...)
		if err != nil {
			return nil, nil, err
		}
		for key, val := range dict {
			keys, vals = append(keys, key), append(vals, val)
		}
		return keys, vals, nil
	}
	if len(values)%2 != 0 {
		return nil, nil, errors.New("invalid dict call")
	}
	for i := 0; i < len(values); i += 2 {
		s, ok := values[i].(string)
		if !ok {
			return nil, nil, errors.New("Only string keys supported in sdict")
		}
		keys, vals = append(keys, s), append(vals, values[i+1])
	}
	return keys, vals, nil
}

// complexMessage is YAGPDB's CreateComplexMessage (common/templates/general.go:263), which
// complexMessageEdit is too (context.go:113-114): known keys are case-insensitive and read
// in order, an unknown key is an error, "embed" takes one embed or a slice of up to 10 and
// replaces any earlier "embed", and a file gets a .txt name. "buttons" (up to 40),
// "menus" (up to 5) and "components" (built ones, flat or as rows) become action rows
// (components.go), and every button and menu ends with a valid templates- custom ID.
func (e *Engine) complexMessage(args ...interface{}) (*types.MessageSend, error) {
	if len(args) < 1 {
		return &types.MessageSend{}, nil
	}
	if m, ok := args[0].(*types.MessageSend); len(args) == 1 && ok {
		return m, nil
	}
	keys, vals, err := builderPairs(args...)
	if err != nil {
		return nil, err
	}

	msg := &types.MessageSend{AllowedMentions: usersOnly()}
	filename := "attachment_" + e.ctx.Now().Format("2006-01-02_15-04-05")
	for i, key := range keys {
		val := vals[i]

		switch strings.ToLower(key) {
		case "content":
			msg.Content = funcs.ToString(val)
		case "embed":
			if val == nil {
				continue
			}
			msg.Embeds = make([]interface{}, 0)
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
		case "components": // general.go:352-373
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				msg.Components, err = distributeComponentsIntoActionsRows(v)
				if err != nil {
					return nil, err
				}
			} else {
				var component types.InteractiveComponent
				switch comp := val.(type) {
				case *types.SelectMenu:
					component = comp
				case *types.Button:
					component = comp
				default:
					return nil, errors.New("invalid component passed to send message builder")
				}
				msg.Components = append(msg.Components,
					&types.ActionsRow{Components: []types.InteractiveComponent{component}})
			}
		case "buttons": // general.go:379-408
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				buttons := []*types.Button{}
				const maxButtons = 40 // Discord limitation
				for i := 0; i < v.Len() && i < maxButtons; i++ {
					button, err := CreateButton(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					buttons = append(buttons, button)
				}
				comps, err := distributeComponentsIntoActionsRows(reflect.ValueOf(buttons))
				if err != nil {
					return nil, err
				}
				msg.Components = append(msg.Components, comps...)
			} else {
				button, err := CreateButton(val)
				if err != nil {
					return nil, err
				}
				if button.Style == types.LinkButton {
					button.CustomID = ""
				}
				msg.Components = append(msg.Components,
					&types.ActionsRow{Components: []types.InteractiveComponent{button}})
			}
		case "menus": // general.go:409-435
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				menus := []*types.SelectMenu{}
				const maxMenus = 5 // Discord limitation
				for i := 0; i < v.Len() && i < maxMenus; i++ {
					menu, err := CreateSelectMenu(v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					menus = append(menus, menu)
				}
				comps, err := distributeComponentsIntoActionsRows(reflect.ValueOf(menus))
				if err != nil {
					return nil, err
				}
				msg.Components = append(msg.Components, comps...)
			} else {
				menu, err := CreateSelectMenu(val)
				if err != nil {
					return nil, err
				}
				msg.Components = append(msg.Components,
					&types.ActionsRow{Components: []types.InteractiveComponent{menu}})
			}
		case "forward", "sticker":
			// Not modelled, but they count as message content for Discord
			if val != nil {
				msg.HasOther = true
			}
		case "allowed_mentions":
			if val == nil {
				msg.AllowedMentions = types.AllowedMentions{}
				continue
			}
			parsed, err := parseAllowedMentions(val)
			if err != nil {
				return nil, err
			}
			msg.AllowedMentions = *parsed
		case "reply":
			msgID := funcs.ToInt64(val)
			if msgID <= 0 {
				return nil, fmt.Errorf("invalid message id '%s' provided to reply.", funcs.ToString(val))
			}
			msg.ReplyTo = msgID
		case "is_components_v2":
			if val == nil || val == false {
				continue
			}
			msg.ComponentsV2 = true
		case "ephemeral": // general.go:374-378
			if val == nil || val == false {
				continue
			}
			msg.Ephemeral = true
		case "silent", "suppress_embeds":
			// Accepted; the emulator doesn't model these
		default:
			return nil, fmt.Errorf(`invalid key "%s" passed to send message builder.`, key)
		}
	}
	if msg.HasFile {
		msg.Filename = filename + ".txt"
	}

	if len(msg.Components) > 0 { // general.go:502-507
		err := validateTopLevelComponentsCustomIDs(msg.Components, nil)
		if err != nil {
			return nil, err
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

// findCC is tmplRunCC's and tmplScheduleUniqueCC's command lookup, with a test's
// command_map standing in for the server's commands: a mapped command's template is read,
// and an Interval, Crontab or Role-trigger command is refused as YAGPDB refuses it (vendor
// customcommands/tmplextensions.go). mapped is false for a command the test doesn't map,
// which may exist in production.
func (e *Engine) findCC(fn string, ccID int64) (path string, source []byte, mapped bool, err error) {
	path, mapped = e.ctx.CommandIDMap[ccID]
	if !mapped {
		return "", nil, false, nil
	}
	if remapped, ok := RemapTemplateRoot(e.ctx.TemplateRoot, path); ok {
		path = remapped
	} else if e.ctx.TemplateBaseDir != "" && !filepath.IsAbs(path) {
		path = filepath.Join(e.ctx.TemplateBaseDir, path)
	}
	source, err = os.ReadFile(path)
	if err != nil { // the test's mistake, not YAGPDB's behaviour
		return "", nil, true, fmt.Errorf("%s %d: can't read its command_map template: %w", fn, ccID, err)
	}
	if t, ok := ReadTrigger(string(source)); ok {
		if name, disallowed := t.disallowedExecCCType(); disallowed {
			return "", nil, true, fmt.Errorf("custom commands of type %s cannot be used with %s", name, fn)
		}
	}
	return path, source, true, nil
}

// execCC is YAGPDB's tmplRunCC: the command is looked up, then the channel; with a delay
// the run is scheduled (see schedule.go), otherwise it runs now, at most two levels deep.
// ccID is an int, as there: the template engine refuses a float or a string.
func (e *Engine) execCC(ccID int, channel, delay interface{}, data interface{}) (string, error) {
	commandID := int64(ccID)
	templatePath, templateContent, mapped, err := e.findCC("execCC", commandID)
	if err != nil {
		return "", err
	}

	channelID := e.channelArg(channel)
	if channelID == 0 {
		return "", errors.New("Unknown channel")
	}

	if yagstd.ToInt64(delay) > 0 {
		return "", e.ctx.schedule(commandID, channelID, delay, nil, data)
	}

	if e.ctx.ExecCCDepth >= e.ctx.MaxExecCCDepth {
		return "", errors.New("Max nested immediate execCC calls reached (2)")
	}

	if !mapped {
		e.ctx.Warn(KindExecCC, "execCC %d isn't in the test's command_map, so it didn't run (YAGPDB "+
			"runs that command, or fails \"Couldn't find custom command\" if there's none)", commandID)
		return "", nil
	}

	// Create child context (shares DB and other state)
	childCtx := &ExecutionContext{
		GuildID:                  e.ctx.GuildID,
		GuildName:                e.ctx.GuildName,
		Prefix:                   e.ctx.Prefix,
		ChannelID:                channelID,
		ChannelName:              e.ctx.channelName(channelID),
		UserID:                   e.ctx.UserID,
		Username:                 e.ctx.Username,
		Discriminator:            e.ctx.Discriminator,
		UserRoles:                e.ctx.UserRoles,
		MessageContent:           e.ctx.MessageContent,
		ExecData:                 data,
		IsPremium:                e.ctx.IsPremium,
		Strict:                   e.ctx.Strict,
		DB:                       e.ctx.DB, // Share database
		Schema:                   e.ctx.Schema,
		Counters:                 make(map[string]int), // execCC starts a new run with its own limits
		StartTime:                e.ctx.StartTime,
		Clock:                    e.ctx.Clock,
		Random:                   e.ctx.Random,
		timeSlept:                e.ctx.timeSlept, // the caller's sleeps have passed
		AvailableRoles:           e.ctx.AvailableRoles,
		Channels:                 e.ctx.Channels,
		ChannelOrder:             e.ctx.ChannelOrder,
		ChannelDetails:           e.ctx.ChannelDetails,
		OwnerID:                  e.ctx.OwnerID,
		BotCannotMentionEveryone: e.ctx.BotCannotMentionEveryone,
		CommandIDMap:             e.ctx.CommandIDMap,
		CCID:                     commandID,
		ExecCCDepth:              e.ctx.ExecCCDepth + 1,
		MaxExecCCDepth:           e.ctx.MaxExecCCDepth,
		TemplateBaseDir:          e.ctx.TemplateBaseDir,
		SourceName:               templatePath,
		// The same server; a copy, since YAGPDB runs the child in a goroutine alongside the
		// caller (the emulator runs it inline, so messages are in call order)
		Messages:        append([]types.CtxMessage(nil), e.ctx.Messages...),
		sentIDs:         e.ctx.sentMessageIDs(),
		Members:         e.ctx.Members,
		MemberRoles:     e.ctx.MemberRoles,
		MemberNicks:     e.ctx.MemberNicks,
		MemberJoinedAgo: e.ctx.MemberJoinedAgo,
		ExecResponses:   e.ctx.ExecResponses,
		scheduled:       e.ctx.scheduledRuns(),
	}

	// YAGPDB passes the caller's message on (tmplextensions.go tmplRunCC: newCtx.Msg)
	inherited := e.ctx.triggerMsg()
	childCtx.InheritedMessage = &inherited
	childCtx.inheritedFromReaction = e.ctx.Reaction != nil || e.ctx.inheritedFromReaction
	childCtx.NoMember = e.ctx.NoMember // the child's context has the caller's (nil) member
	// ... and the same Interaction pointer (tmplextensions.go:240-243), so whichever of
	// the two responds first takes the interaction's one response and the other's output
	// is a followup. DIVERGENCE: YAGPDB runs the child in a goroutine (:249), so a caller
	// that both execCCs and prints races the child for that response; the emulator runs
	// the child inline, so the child's sendResponse always wins and the caller's output
	// is always the followup. Commands shouldn't mix the two (docs/FUTURE_IMPROVEMENTS.md).
	childCtx.Interaction = e.ctx.Interaction
	childCtx.deferMode = e.ctx.deferMode

	// Execute child template
	childEngine := NewEngine(childCtx)
	if err := ValidateHeader(string(templateContent)); err != nil {
		return "", fmt.Errorf("execCC %d (%s): %w", commandID, filepath.Base(templatePath), err)
	}
	// The output is the child's response, sent to its channel with its pings, unless the
	// child failed with show_errors on (Execute has posted the error message instead).
	// With an interaction, Execute has already routed it as an interaction response.
	out, err := childEngine.Execute(string(templateContent))
	if out != "" && childCtx.Interaction == nil && (err == nil || !ReadErrorSettings(string(templateContent)).ShowErrors) {
		id := childCtx.RecordSentMessage(channelID, out, nil, nil, childCtx.ResponsePings)
		childCtx.recordResponseSent(channelID, id) // a deleteResponse delay under 1 left out ""
	}

	// Propagate side effects back to parent
	e.ctx.SentMessages = append(e.ctx.SentMessages, childCtx.SentMessages...)
	e.ctx.EditedMessages = append(e.ctx.EditedMessages, childCtx.EditedMessages...)
	e.ctx.InteractionResponses = append(e.ctx.InteractionResponses, childCtx.InteractionResponses...)
	e.ctx.RoleChanges = append(e.ctx.RoleChanges, childCtx.RoleChanges...)
	e.ctx.Deletions = append(e.ctx.Deletions, childCtx.Deletions...)
	e.ctx.Reactions = append(e.ctx.Reactions, childCtx.Reactions...)
	e.ctx.Execs = append(e.ctx.Execs, childCtx.Execs...)
	e.ctx.FileUploads = append(e.ctx.FileUploads, childCtx.FileUploads...)
	if err != nil { // the caller carries on
		e.ctx.Warn(KindExecCC, "execCC %d (%s) failed: %v", commandID, filepath.Base(templatePath), err)
	}
	for _, d := range childCtx.Diagnostics {
		d.Message = fmt.Sprintf("execCC %d (%s): %s", commandID, filepath.Base(templatePath), d.Message)
		e.ctx.Diagnostics = append(e.ctx.Diagnostics, d)
	}

	// execCC doesn't return output to the caller
	return "", nil
}

// channelArg is YAGPDB's ChannelArg as the mocks read it: nil is the current channel.
func (e *Engine) channelArg(channel interface{}) int64 {
	// baseChannelArg (common/templates/context_funcs.go): nil is the current channel, an
	// int or int64 an ID, a string an ID or a channel's name (any case); anything else,
	// a float included, is no channel
	var id int64
	switch t := channel.(type) {
	case nil:
		return e.ctx.ChannelID
	case int, int64:
		id = funcs.ToInt64(t)
	case string:
		parsed, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return e.ctx.channelNamed(t)
		}
		id = parsed
	default:
		return 0
	}
	// GetChannelOrThread: the channel must exist, among either declared channels or
	// declared threads (baseChannelArg resolves both, vendor context_funcs.go)
	if len(e.ctx.Channels) > 0 || len(e.ctx.Threads) > 0 {
		if _, ok := e.ctx.Channels[id]; ok {
			return id
		}
		if _, ok := e.ctx.Threads[id]; ok {
			return id
		}
		return 0
	}
	if id != 0 && id != e.ctx.ChannelID && !e.mockChannels[id] {
		if e.mockChannels == nil {
			e.mockChannels = make(map[int64]bool)
		}
		e.mockChannels[id] = true
		e.ctx.Warn(KindChannel, "channel %d is assumed to exist, since no guild channels are declared (a test's guild.channels)", id)
	}
	return id
}

// sleep is YAGPDB's tmplSleep (common/templates/context_funcs.go) without the wait: the
// run's clock moves on by the seconds slept instead.
func (e *Engine) sleep(duration interface{}) (string, error) {
	seconds := yagstd.ToIntTmpl(duration)
	if e.ctx.secondsSlept+seconds > 60 || seconds < 1 {
		return "", errors.New("can sleep for max 60 seconds combined")
	}

	e.ctx.secondsSlept += seconds
	e.ctx.timeSlept += time.Duration(seconds) * time.Second
	return "", nil
}

// Mention functions

// mentionEveryone and mentionHere let the response ping everyone.
func (e *Engine) mentionEveryone() string {
	e.ctx.mentionEveryone = true
	return "@everyone"
}

func (e *Engine) mentionHere() string {
	e.ctx.mentionEveryone = true
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
		Channel: func(id int64) interface{} {
			if c, _ := e.getChannel(id); c != nil { // not a typed nil in the interface
				return c
			}
			return nil
		},
		Role: func(id interface{}, idName string) interface{} {
			if r := e.roleArg(id, idName); r != nil {
				return r
			}
			return nil
		},
	})
}
