package runtime

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// A message component click as YAGPDB handles it (vendor customcommands/
// handle_component.go, common/templates/context_interactions.go and context.go
// SendResponse, c579722): the Component trigger's data keys, the deferral the panel's
// defer mode makes before the run, the response functions, and where the run's output
// goes when there is an interaction.

// errTooManyInteractionResponses is YAGPDB's ErrTooManyInteractionResponses
// (context_interactions.go:13): an interaction takes one response; the rest are followups.
var errTooManyInteractionResponses = errors.New(
	"cannot respond to an interaction > 1 time; consider using a followup")

// errNoInteractionUpdate is updateMessage's error without an interaction
// (context_interactions.go:377).
var errNoInteractionUpdate = errors.New(
	"no interaction data in context; consider editMessage or editResponse")

// errInvalidToken is tokenArg's failure (context_interactions.go:315, :422-426).
var errInvalidToken = errors.New("invalid interaction token")

// errAlreadyAcked is Discord's answer to a second interaction response, which a deferral
// made before the run counts as (40060 "Interaction has already been acknowledged").
var errAlreadyAcked = discordError{"400 Bad Request", 40060,
	"Interaction has already been acknowledged"}

// errNoMessageToUpdate stands in for Discord's refusal of an UPDATE_MESSAGE response to
// an application command interaction (INF: shape not captured from Discord).
var errNoMessageToUpdate = discordError{"400 Bad Request", 50035, "Invalid Form Body"}

// errUnknownWebhook stands in for Discord's refusal of a followup to an interaction that
// was never acknowledged (INF: shape not captured from Discord).
var errUnknownWebhook = discordError{"404 Not Found", 10015, "Unknown Webhook"}

// deferralFailed reports that the run's deferral never reached Discord: an Update
// deferral of a modal no message opened (see SetInteractionModal). YAGPDB still marks
// the interaction responded to and deferred, but it was never acknowledged.
func (ctx *ExecutionContext) deferralFailed() bool {
	in := ctx.Interaction
	return in != nil && in.Deferred && ctx.deferMode == DeferModeUpdate && in.Message == nil
}

// Response kinds (InteractionResponse.Kind), by what YAGPDB sends: SendResponse's
// three modes (context.go:662-693) and updateMessage's InteractionResponseUpdateMessage.
const (
	ResponseMessage      = "message"       // the interaction's response: a new message
	ResponseFollowup     = "followup"      // a followup, the interaction already answered
	ResponseDeferredEdit = "deferred_edit" // an edit of the deferred response
	ResponseUpdate       = "update"        // an update of the component's message in place
)

// InteractionResponse is one answer to the run's interaction. A message, followup or
// deferred edit is also a SentMessage (or, for a deferred update, an EditedMessage);
// an update is an EditedMessage, and edits the component's message in place. A modal
// (ResponseModal, modals.go) is neither: it has no message.
type InteractionResponse struct {
	Kind      string
	Ephemeral bool
	ChannelID int64
	MessageID int64 // the message sent or edited
	Content   string
	Embeds    []interface{}
	// Components are the message's action rows; nil when the response didn't set them
	Components []types.TopLevelComponent
	// A modal's (Kind ResponseModal, which has no message): its title, its custom ID and
	// its text inputs' custom IDs in order, both without the templates- prefix
	Title    string
	CustomID string
	Fields   []string
}

func (r InteractionResponse) String() string {
	if r.Kind == ResponseModal {
		return fmt.Sprintf("modal %q (custom_id %q) with fields %q", r.Title, r.CustomID, r.Fields)
	}
	s := r.Kind
	if r.Ephemeral {
		s += " (ephemeral)"
	}
	return fmt.Sprintf("%s message %d in channel %d: %q", s, r.MessageID, r.ChannelID, r.Content)
}

// ComponentTrigger is what the click gave the Component handler (handle_component.go:
// 273-309): the custom ID without its templates- prefix, the match's Cmd/CmdArgs/
// StrippedID, and the component's type with a menu's selected values.
type ComponentTrigger struct {
	CustomID      string
	Cmd           string
	CmdArgs       []string
	Stripped      string
	ComponentType types.ComponentType
	Values        []string
}

// setData sets the handler's data keys (handle_component.go:278-309).
func (c *ComponentTrigger) setData(data map[string]interface{}) {
	data["InteractionData"] = types.MessageComponentInteractionData{
		CustomID: TemplateCustomIDPrefix + c.CustomID, ComponentType: c.ComponentType, Values: c.Values}
	data["CustomID"] = c.CustomID
	data["Cmd"] = c.Cmd
	data["CmdArgs"] = c.CmdArgs
	data["StrippedID"] = c.Stripped
	data["StrippedMsg"] = c.Stripped

	switch c.ComponentType {
	case types.ButtonComponent:
		data["IsButton"] = true
	case types.SelectMenuComponent, types.UserSelectMenuComponent, types.RoleSelectMenuComponent,
		types.MentionableSelectMenuComponent, types.ChannelSelectMenuComponent:
		data["IsMenu"] = true
		data["MenuType"] = menuTypeName(c.ComponentType)
		data["Values"] = c.Values
	}
}

// menuTypeName is the handler's .MenuType for a menu's component type
// (handle_component.go:296-307).
func menuTypeName(t types.ComponentType) string {
	switch t {
	case types.SelectMenuComponent:
		return "string"
	case types.UserSelectMenuComponent:
		return "user"
	case types.RoleSelectMenuComponent:
		return "role"
	case types.MentionableSelectMenuComponent:
		return "mentionable"
	case types.ChannelSelectMenuComponent:
		return "channel"
	}
	return ""
}

// componentMessage is a component run's .Message (and YAGPDB's ctx.Msg): the
// interaction's own copy of the component's message (the re-fetch the handler puts in
// interaction.Message, handle_component.go:76-80), given the clicker as author and member
// in place, as the handler does (:311-316), so .Interaction.Message is the same. The
// stored message keeps its real author, so a getMessage of it reads as Discord returns it.
func (ctx *ExecutionContext) componentMessage() types.CtxMessage {
	ctx.byUser(ctx.Interaction.Message)
	return *ctx.Interaction.Message
}

// ComponentClick is a click on a component of a message the emulator knows, as a test
// declares it: the custom ID as the command wrote it (the templates- prefix is added when
// missing, then stripped again as the handler does), the message clicked, the component's
// type and, for a menu, the values chosen.
type ComponentClick struct {
	CustomID      string
	MessageID     int64
	ComponentType types.ComponentType
	Values        []string
}

// SetInteractionComponent makes the run answer a component click on the trigger t: the
// custom ID must match the trigger regex (ErrTriggerMismatch otherwise, so a test can
// assert no_trigger), and the clicked message must be one the test declares in the run's
// channel (messages:). The panel's defer mode is applied as deferResponseToCCs does before the run
// (handle_component.go:161-190): a deferral counts as the response, to be edited by the
// output.
func (ctx *ExecutionContext) SetInteractionComponent(t Trigger, click ComponentClick,
	mode DeferMode) error {
	if !t.ComponentTriggered() {
		return fmt.Errorf("a component click needs a Message Component trigger; the template's is %q",
			t.Type)
	}
	// Discord gives the full ID; the handler only runs commands for the templates- prefix
	// and strips it (handle_component.go:82-88, :280)
	rawID := click.CustomID
	if !strings.HasPrefix(rawID, TemplateCustomIDPrefix) {
		rawID = TemplateCustomIDPrefix + rawID
	}
	cID := strings.TrimPrefix(rawID, TemplateCustomIDPrefix)

	match, stripped, cmdArgs := CheckMatchComponent(t, cID)
	if !match {
		return &triggerMismatchError{msg: fmt.Sprintf("the custom ID %q doesn't match the %s trigger %q",
			cID, t.Type, t.Text)}
	}

	// Only a declared (or sent) message: not the trigger message knownMessage falls
	// back to, which a component run doesn't have
	var msg *types.CtxMessage
	for i := range ctx.Messages {
		if m := &ctx.Messages[i]; m.ID == click.MessageID && m.ChannelID == ctx.ChannelID {
			msg = m
			break
		}
	}
	if msg == nil {
		return fmt.Errorf("interaction message_id %d isn't a message in channel %d: declare it in "+
			"messages: with that channel_id", click.MessageID, ctx.ChannelID)
	}

	member := ctx.member(ctx.UserID)
	clicked := *msg // its own copy (the re-fetch): the channel's messages may move as the run sends more
	ctx.Interaction = &types.CustomCommandInteraction{Interaction: &types.Interaction{
		ID:   click.MessageID + 1, // any ID after the message's (Discord's are snowflakes)
		Type: types.InteractionMessageComponent,
		Data: types.MessageComponentInteractionData{CustomID: rawID,
			ComponentType: click.ComponentType, Values: click.Values},
		GuildID:   ctx.GuildID,
		ChannelID: ctx.ChannelID,
		Message:   &clicked,
		Member:    &member,
		Token:     interactionToken(click.MessageID + 1),
		Version:   1,
	}}
	ctx.Component = &ComponentTrigger{
		CustomID:      cID,
		Cmd:           cmdArgs[0],
		CmdArgs:       cmdArgs[1:], // []string{} without arguments (handle_component.go:283-287)
		Stripped:      stripped,
		ComponentType: click.ComponentType,
		Values:        click.Values,
	}

	ctx.deferMode = mode
	if mode != DeferModeNone {
		ctx.Interaction.RespondedTo = true
		ctx.Interaction.Deferred = true
	}
	return nil
}

// interactionToken is a token of the shape tokenArg accepts (context_interactions.go:
// 429-446): base64 of "interaction:<id>:<rest>".
func interactionToken(id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("interaction:%d:emulator", id)))
}

// sendMessageType is YAGPDB's (context.go:728-736), the interaction cases.
type sendMessageType uint

const (
	sendMessageInteractionResponse sendMessageType = iota + 2
	sendMessageInteractionFollowup
	sendMessageInteractionDeferred
)

// tokenArg is YAGPDB's (context_interactions.go:415-452): nil is the current interaction's
// token; a string must decode (base64, URL alphabet) to "interaction:...". The result is
// a response while the current interaction hasn't been responded to, else a followup.
// A valid token of another interaction isn't modelled (design (e)): it's a followup there
// too, but the emulator has no other interaction to send it to, so it warns.
func (ctx *ExecutionContext) tokenArg(interactionToken interface{}) (sendType sendMessageType,
	token string) {
	sendType = sendMessageInteractionFollowup
	sToken, ok := interactionToken.(string)
	if !ok {
		if interactionToken == nil && ctx.Interaction != nil {
			// no token provided, assume current interaction
			token = ctx.Interaction.Token
		} else {
			return
		}
	} else {
		sToken = strings.TrimSpace(sToken)
		//rudimentary check for valid token because people don't read docs and will send anything.
		decoded, err := base64.RawURLEncoding.DecodeString(sToken)
		if err != nil {
			decoded, err = base64.URLEncoding.DecodeString(sToken)
		}
		if err != nil {
			return
		}
		parts := strings.SplitN(string(decoded), ":", 3)
		if len(parts) < 3 {
			return
		}
		if parts[0] != "interaction" {
			return
		}
		token = sToken
	}

	if ctx.Interaction != nil && token == ctx.Interaction.Token && !ctx.Interaction.RespondedTo {
		sendType = sendMessageInteractionResponse
	}
	return
}

// countAPICall is IncreaseCheckGenericAPICall for a function that checks something
// before counting (updateMessage, context_interactions.go:376-380), so the withLimits
// wrapper can't count for it: the same strict/lenient handling as the wrapper's.
func (ctx *ExecutionContext) countAPICall(fn string) error {
	err := ctx.countCall(fn, limitAPI)
	if err == nil {
		return nil
	}
	if ctx.returnsError(err) {
		return err
	}
	ctx.warnOnce(explain(err) + "; YAGPDB stops the command here")
	return nil
}

// countInteractionResponse is IncreaseCheckCallCounter("interaction_response", 1)
// (context_interactions.go:335, :384): one response per interaction, premium or not,
// and the vendor error on the second. Not a rate limit the emulator softens: Discord
// refuses a second response, so the error always returns.
func (ctx *ExecutionContext) countInteractionResponse() error {
	ctx.Counters["interaction_response"]++
	if ctx.Counters["interaction_response"] > 1 {
		return errTooManyInteractionResponses
	}
	return nil
}

// sendResponseFunc is a sendResponse variant for the function map.
func (e *Engine) sendResponseFunc(fn string, filterSpecialMentions,
	returnID bool) func(token, msg interface{}) (interface{}, error) {
	return func(token, msg interface{}) (interface{}, error) {
		return e.sendResponse(fn, filterSpecialMentions, returnID, token, msg)
	}
}

// updateMessageFunc is an updateMessage variant for the function map.
func (e *Engine) updateMessageFunc(fn string,
	filterSpecialMentions bool) func(msg interface{}) (interface{}, error) {
	return func(msg interface{}) (interface{}, error) {
		return e.updateMessage(fn, filterSpecialMentions, msg)
	}
}

// sendResponse is YAGPDB's tmplSendInteractionResponse (context_interactions.go:
// 307-372): the message is the interaction's response (and counts as its one response),
// or a followup once it has been responded to. NoEscape lets roles and @everyone ping;
// RetID returns the sent message's ID (YAGPDB fetches the response for it).
func (e *Engine) sendResponse(fn string, filterSpecialMentions, returnID bool,
	interactionToken, msg interface{}) (interface{}, error) {
	sendType, token := e.ctx.tokenArg(interactionToken)
	if token == "" {
		return "", errInvalidToken
	}
	if e.ctx.Interaction == nil || token != e.ctx.Interaction.Token {
		// design (e): foreign tokens aren't modelled; the emulator can't reach that
		// interaction (YAGPDB would send a followup there)
		e.ctx.Warn(KindResponse, "%s: the token isn't this run's interaction's; a followup to another "+
			"interaction isn't modelled, so nothing was sent", fn)
		return "", nil
	}

	msgSend := parseMessageInput(msg)
	content, embeds, components := msgSend.Content, msgSend.Embeds, msgSend.Components
	allowed := msgSend.AllowedMentions
	if !filterSpecialMentions {
		allowed = noEscape()
	}
	if sendType == sendMessageInteractionResponse {
		if err := e.ctx.countInteractionResponse(); err != nil {
			return "", err
		}
	}
	notEmpty := msgSend.HasFile || msgSend.HasOther || len(components) > 0
	if ok, err := e.ctx.checkSend(fn, content, embeds, notEmpty, false); !ok {
		return "", err
	}
	if refused, err := e.ctx.checkComponents(fn, components); refused {
		return "", err
	}
	if sendType == sendMessageInteractionFollowup && e.ctx.deferralFailed() {
		// CreateFollowupMessage's error is returned (context_interactions.go:356-371)
		return "", e.ctx.discordRefuses(fn, errUnknownWebhook,
			"the interaction was never acknowledged (the deferred update of a modal no message "+
				"opened failed), so it takes no followup")
	}
	if msgSend.HasFile {
		e.ctx.RecordFileUpload(e.ctx.ChannelID, msgSend.Filename, msgSend.File)
	}
	kind := ResponseFollowup
	if sendType == sendMessageInteractionResponse {
		kind = ResponseMessage
		e.ctx.Interaction.RespondedTo = true
	}
	pings := e.ctx.pings(content, allowed, e.ctx.ChannelID, msgSend.ReplyTo)
	id := e.ctx.recordInteractionMessage(kind, msgSend.Ephemeral, content, embeds, components, pings)
	if returnID {
		return id, nil
	}
	return "", nil
}

// recordInteractionMessage records a message an interaction response created: a
// SentMessage in the run's channel (an ephemeral one isn't in the channel's messages,
// since Discord's channel endpoints don't return it), and the InteractionResponse.
func (ctx *ExecutionContext) recordInteractionMessage(kind string, ephemeral bool, content string,
	embeds []interface{}, components []types.TopLevelComponent, pings Pings) int64 {
	id := ctx.RecordSentMessage(ctx.ChannelID, content, embeds, components, pings)
	if ephemeral {
		ctx.SentMessages[len(ctx.SentMessages)-1].Ephemeral = true
		if n := len(ctx.Messages); n > 0 && ctx.Messages[n-1].ID == id {
			ctx.Messages = ctx.Messages[:n-1]
		}
	}
	ctx.InteractionResponses = append(ctx.InteractionResponses, InteractionResponse{
		Kind: kind, Ephemeral: ephemeral, ChannelID: ctx.ChannelID, MessageID: id,
		Content: content, Embeds: embeds, Components: components,
	})
	return id
}

// updateMessage is YAGPDB's tmplUpdateMessage (context_interactions.go:374-413): the
// interaction's one response is an update of the component's message. The response data
// carries content, embeds and components without omitempty (discordgo
// InteractionResponseData), so an update replaces all three: a plain string clears the
// message's embeds and buttons. After a deferral (RespondedTo already, Deferred) the
// response is Discord's 40060, which YAGPDB returns as the error.
func (e *Engine) updateMessage(fn string, filterSpecialMentions bool,
	msg interface{}) (interface{}, error) {
	if e.ctx.Interaction == nil {
		return "", errNoInteractionUpdate
	}
	if err := e.ctx.countAPICall(fn); err != nil {
		return "", err
	}
	if err := e.ctx.countInteractionResponse(); err != nil {
		return "", err
	}
	msgSend := parseMessageInput(msg)
	if e.ctx.Interaction.RespondedTo {
		return "", e.ctx.discordRefuses(fn, errAlreadyAcked,
			fmt.Sprintf("the interaction was deferred (Defer mode: `%s`), which is its response; "+
				"print the update instead, or set Defer mode to None", e.ctx.deferMode))
	}
	if e.ctx.Interaction.Message == nil {
		// A slash command or context menu interaction, or the submission of a modal no
		// message opened, has no component message; YAGPDB still sends the UPDATE_MESSAGE
		// response (:400-403) and Discord refuses it (INF: the code and text are the
		// emulator's stand-in, not a captured response)
		return "", e.ctx.discordRefuses(fn, errNoMessageToUpdate,
			"a slash command or context menu interaction (or a modal no message opened) has no "+
				"message to update; use sendResponse or print the reply")
	}
	content, embeds, components := msgSend.Content, msgSend.Embeds, msgSend.Components
	notEmpty := msgSend.HasFile || msgSend.HasOther || len(components) > 0
	if ok, err := e.ctx.checkSend(fn, content, embeds, notEmpty, false); !ok {
		return "", err
	}
	if refused, err := e.ctx.checkComponents(fn, components); refused {
		return "", err
	}
	target := e.ctx.editComponentMessage(content, embeds, components)
	e.ctx.recordInteractionEdit(ResponseUpdate, false, target)
	e.ctx.Interaction.RespondedTo = true
	return "", nil
}

// editComponentMessage edits the interaction's message in place, as editMessage edits
// one: the channel's copy (which getMessage reads) and the interaction's own.
func (ctx *ExecutionContext) editComponentMessage(content string, embeds []interface{},
	components []types.TopLevelComponent) *types.CtxMessage {
	own := ctx.Interaction.Message
	targets := []*types.CtxMessage{own}
	if known := ctx.knownMessage(own.ChannelID, own.ID); known != nil && known != own {
		targets = append(targets, known)
	}
	edited := types.NewTimestamp(ctx.Now())
	for _, target := range targets {
		target.Content, target.Embeds = content, types.EmbedStructs(embeds)
		target.Components = components
		target.EditedTimestamp = edited
	}
	return own
}

// recordInteractionEdit records an interaction response that edited a message: an
// EditedMessage, and the InteractionResponse.
func (ctx *ExecutionContext) recordInteractionEdit(kind string, ephemeral bool,
	target *types.CtxMessage) {
	embeds := types.EmbedMaps(target.Embeds)
	ctx.EditedMessages = append(ctx.EditedMessages, SentMessage{ID: target.ID,
		ChannelID: target.ChannelID, Content: target.Content, Embeds: embeds,
		Components: target.Components, Ephemeral: ephemeral})
	ctx.InteractionResponses = append(ctx.InteractionResponses, InteractionResponse{
		Kind: kind, Ephemeral: ephemeral, ChannelID: target.ChannelID, MessageID: target.ID,
		Content: target.Content, Embeds: embeds, Components: target.Components,
	})
}

// ephemeralResponse is YAGPDB's tmplEphemeralResponse (context_interactions.go:226-231):
// the output's response is visible to the clicker only; a no-op without an interaction.
func (e *Engine) ephemeralResponse() string {
	if e.ctx.Interaction != nil {
		e.ctx.ephemeralResponse = true
	}
	return ""
}

// respondWithOutput is SendResponse (context.go:598-693) with an interaction: the
// output is the interaction's response, a followup once it has been responded to, or
// the edit of its deferred response (which, under Update Message Response, is the
// component's message itself). The output alone: SendResponse's interaction cases
// carry no components. An ephemeral response (ephemeralResponse, or an ephemeral
// deferral, which Discord keeps) gets no response reactions (:710-717).
func (ctx *ExecutionContext) respondWithOutput(out string) {
	in := ctx.Interaction
	ephemeral := ctx.ephemeralResponse
	pings := ctx.ResponsePings
	switch {
	case !in.RespondedTo:
		in.RespondedTo = true
		id := ctx.recordInteractionMessage(ResponseMessage, ephemeral, out, nil, nil, pings)
		ctx.recordResponseSentEphemeral(id, ephemeral)
	case ctx.deferralFailed():
		// A modal no message opened: the deferral failed (see SetInteractionModal), so
		// there is no message the edit could reach (INF)
		ctx.Warn(KindResponse, "the output has no message to edit: the deferred update of a "+
			"modal no message opened fails on Discord's side (INF), so nothing is shown")
	case in.Deferred && ctx.deferMode == DeferModeUpdate:
		// EditOriginalInteractionResponse with WebhookParams (omitempty): the content
		// changes, the message's embeds and components stay
		in.Deferred = false
		target := ctx.editComponentMessage(out, types.EmbedMaps(in.Message.Embeds),
			in.Message.Components)
		ctx.recordInteractionEdit(ResponseDeferredEdit, false, target)
		ctx.recordResponseSentEphemeral(target.ID, false)
	case in.Deferred:
		// The deferral fixed the message's visibility: an Ephemeral deferral stays
		// ephemeral, and a Message Response one can't be made ephemeral by the edit
		// (YAGPDB sends the flag, context.go:688; Discord ignores it). The reactions are
		// still skipped once ephemeralResponse ran (:710).
		in.Deferred = false
		deferredEphemeral := ctx.deferMode == DeferModeEphemeral
		id := ctx.recordInteractionMessage(ResponseDeferredEdit, deferredEphemeral, out, nil, nil, pings)
		ctx.recordResponseSentEphemeral(id, ephemeral || deferredEphemeral)
	default:
		id := ctx.recordInteractionMessage(ResponseFollowup, ephemeral, out, nil, nil, pings)
		ctx.recordResponseSentEphemeral(id, ephemeral)
	}
}

// recordResponseSentEphemeral is recordResponseSent for an interaction response: the
// deletion (deleteResponse) always, the reactions (addResponseReactions) unless ephemeral.
func (ctx *ExecutionContext) recordResponseSentEphemeral(messageID int64, ephemeral bool) {
	if ctx.delResponse {
		ctx.recordDeletion("response", ctx.ChannelID, messageID, ctx.delResponseDelay)
	}
	if !ephemeral {
		ctx.recordResponseReactions(ctx.ChannelID, messageID)
	}
}

// warnUnanswered records, at the end of a top-level interaction run, what Discord's
// side would show: "The application did not respond" for a run that never responded
// (nor deferred), and a deferred "thinking..." that nothing ever filled.
func (ctx *ExecutionContext) warnUnanswered() {
	in := ctx.Interaction
	switch {
	case !in.RespondedTo:
		ctx.Warn(KindResponse, "the run ended without responding to the interaction: Discord shows "+
			"\"The application did not respond\" (print something, or call sendResponse/updateMessage, "+
			"or set a Defer mode)")
	case in.Deferred && ctx.deferMode != DeferModeUpdate:
		ctx.Warn(KindResponse, "the run was deferred (Defer mode: `%s`) but printed nothing, so the "+
			"\"thinking...\" response is never filled", ctx.deferMode)
	}
}
