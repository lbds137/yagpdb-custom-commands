package runtime

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	yagstd "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// Modals as YAGPDB builds, sends and receives them (vendor c579722): the builders cmodal
// (CreateModal), modalBuilder (CreateModalBuilder and ModalBuilder), ctextInput
// (CreateTextInput) and clabel (CreateLabel) with their error texts, sendModal, and the
// Modal trigger's run (customcommands/handle_component.go:110-132, :337-453). A modal's
// fields are text inputs only: a label around a select menu, a checkbox or a radio group
// isn't modelled (docs/design/emulator-interactions.md (e)).

// errNoInteractionModal is sendModal's error without an interaction
// (context_interactions.go:260).
var errNoInteractionModal = errors.New("no interaction data in context")

// errMultipleModals is sendModal's second call (context_interactions.go:267-269).
var errMultipleModals = errors.New("cannot send multiple modals to the same interaction")

// errInvalidModal is sendModal's refusal of anything but a modal
// (context_interactions.go:289, :296).
var errInvalidModal = errors.New("invalid modal passed to sendModal")

// ResponseModal is an InteractionResponse that opened a modal (sendModal).
const ResponseModal = "modal"

// CreateTextInput is YAGPDB's ctextInput (components.go:447-486): a text input from one
// map or sdict key-value pairs. A custom_id gets the templates- prefix put before it
// (even when it has one) and is validated; a text input is short unless "style" says 2.
// As there, a failed build returns the empty text input along with the error.
func CreateTextInput(values ...any) (*types.TextInput, error) {
	var messageSdict map[string]any
	switch t := values[0].(type) {
	case types.SDict:
		messageSdict = t
	case *types.SDict:
		messageSdict = *t
	case map[string]any:
		messageSdict = t
	default:
		dict, err := yagstd.StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		messageSdict = dict
	}

	convertedTextInput := make(map[string]any)
	for k, v := range messageSdict {
		switch strings.ToLower(k) {
		case "custom_id":
			c, err := validateCustomID(TemplateCustomIDPrefix+yagstd.ToString(v), nil)
			if err != nil {
				return nil, err
			}
			convertedTextInput[k] = c
		default:
			convertedTextInput[k] = v
		}
	}

	t, err := createComponent(types.TextInputComponent, convertedTextInput)

	var textInput types.TextInput
	if err == nil {
		textInput = t.(types.TextInput)
	}

	return &textInput, err
}

// CreateLabel is YAGPDB's clabel (components.go:130-177): a label from one map or sdict
// key-value pairs, wrapping the "component" given. Its custom_id is validated and then
// dropped (a Label has no such field); other keys are ignored. YAGPDB also wraps modal
// select menus, checkboxes and radio groups; the emulator models text inputs only, so
// any other component a label may hold is an emulator error, not a silent pass.
func CreateLabel(values ...any) (*types.Label, error) {
	var messageSdict map[string]any
	switch t := values[0].(type) {
	case types.SDict:
		messageSdict = t
	case *types.SDict:
		messageSdict = *t
	case map[string]any:
		messageSdict = t
	case *types.Label:
		return t, nil
	default:
		dict, err := yagstd.StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		messageSdict = dict
	}

	convertedLabel := make(map[string]any)
	for k, v := range messageSdict {
		switch strings.ToLower(k) {
		case "custom_id":
			c, err := validateCustomID(yagstd.ToString(v), nil)
			if err != nil {
				return nil, err
			}
			convertedLabel[k] = c
		case "label":
			convertedLabel[k] = v
		case "description":
			convertedLabel[k] = v
		case "component":
			if c, ok := v.(types.InteractiveComponent); ok && c.IsAllowedInLabel() {
				switch c.(type) {
				case types.TextInput, *types.TextInput:
				default:
					return nil, fmt.Errorf("clabel: a label around a component of type %d "+
						"(a modal select menu, checkbox or radio group) isn't modelled by the "+
						"emulator; only text inputs (ctextInput) are", c.Type())
				}
				convertedLabel[k] = c
			} else {
				return nil, errors.New("unsupported component in label")
			}
		}
	}

	l, err := createComponent(types.LabelComponent, convertedLabel)
	if err != nil {
		return nil, err
	}
	label := l.(types.Label)
	return &label, nil
}

// ModalBuilder is YAGPDB's (common/templates/context.go:1292-1296), which modalBuilder
// returns and whose Set and AddComponents a template can call.
type ModalBuilder struct {
	Title      string
	CustomID   string
	Components []types.TopLevelComponent
}

// Set is context.go:1298-1325: "components" replaces the list with a slice's entries,
// each checked as AddComponents checks one.
func (s *ModalBuilder) Set(key string, value any) (*ModalBuilder, error) {
	switch key {
	case "title":
		s.Title = yagstd.ToString(value)
	case "custom_id":
		cID, err := validateCustomID(yagstd.ToString(value), nil)
		if err != nil {
			return nil, err
		}
		s.CustomID = cID
	case "components":
		val, _ := indirect(reflect.ValueOf(value))
		s.Components = make([]types.TopLevelComponent, 0)
		if val.Kind() == reflect.Slice {
			for i := 0; i < val.Len(); i++ {
				_, err := s.addComponent(val.Index(i).Interface())
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, errors.New("components must be a slice of Labels or TextFields")
		}
	default:
		return nil, errors.New("invalid key, accepted keys are: title, custom_id, components")
	}
	return s, nil
}

// addComponent is context.go:1327-1342: at most 5 top-level components, each one a
// modal takes (a label; not an action row, nor a bare text input, which isn't top-level).
func (s *ModalBuilder) addComponent(comp any) (*ModalBuilder, error) {
	if len(s.Components) == 5 {
		return nil, errors.New("modal builder can only have maximum 5 top level components")
	}

	if comp, ok := comp.(types.TopLevelComponent); ok {
		if !comp.IsModalSupported() {
			return nil, errors.New("invalid top level component passed to modal builder")
		}
		s.Components = append(s.Components, comp)
	} else {
		return nil, errors.New("invalid top level component passed to modal builder")
	}

	return s, nil
}

// AddComponents is context.go:1344-1362: each argument, or each entry of a slice one.
func (s *ModalBuilder) AddComponents(comps ...any) (*ModalBuilder, error) {
	for _, comp := range comps {
		val, _ := indirect(reflect.ValueOf(comp))
		if val.Kind() == reflect.Slice {
			for i := 0; i < val.Len(); i++ {
				_, err := s.addComponent(val.Index(i).Interface())
				if err != nil {
					return nil, err
				}
			}
		} else {
			_, err := s.addComponent(comp)
			if err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}

// toModal is context.go:1364-1373.
func (s *ModalBuilder) toModal() (*types.InteractionResponse, error) {
	return &types.InteractionResponse{
		Type: types.InteractionResponseModal,
		Data: &types.InteractionResponseData{
			Title:      s.Title,
			CustomID:   s.CustomID,
			Components: s.Components,
		},
	}, nil
}

// CreateModalBuilder is YAGPDB's modalBuilder (context_interactions.go:32-49): a builder
// with a validated custom ID, a title and the components given.
func CreateModalBuilder(customID string, title string, components ...any) (*ModalBuilder, error) {
	cid, err := validateCustomID(customID, nil)
	if err != nil {
		return nil, err
	}
	modal := &ModalBuilder{
		Title:    title,
		CustomID: cid,
	}
	for _, component := range components {
		_, err := modal.addComponent(component)
		if err != nil {
			return nil, err
		}
	}

	return modal, nil
}

// CreateModal is YAGPDB's cmodal (context_interactions.go:51-146): a modal response from
// one map or sdict key-value pairs (or a ModalBuilder). Copied quirks: "fields" keeps
// only the first 5 entries of a slice, silently (:105-107), each a text input whose blank
// custom ID is numbered by the fields before it; "components" goes through
// ModalBuilder.Set, whose error is discarded (:96), so a bad entry silently ends the list
// there (and a non-slice leaves it empty). The default custom ID is templates--0 (:77).
func CreateModal(values ...any) (*types.InteractionResponse, error) {
	if len(values) < 1 {
		return &types.InteractionResponse{}, errors.New("no values passed to component builder")
	}

	var m map[string]interface{}
	switch t := values[0].(type) {
	case types.SDict:
		m = t
	case *types.SDict:
		m = *t
	case ModalBuilder:
		return t.toModal()
	case *ModalBuilder:
		return t.toModal()
	case map[string]any:
		m = t
	default:
		dict, err := yagstd.StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		m = dict
	}

	modalBuilder := &ModalBuilder{
		CustomID: TemplateCustomIDPrefix + "-0",
	}
	_, hasComponentsKey := m["components"]
	_, hasFieldsKey := m["fields"]
	if hasComponentsKey && hasFieldsKey {
		return nil, errors.New("cannot have both 'components' and 'fields' in a cmodal")
	}

	for key, val := range m {
		switch key {
		case "title":
			modalBuilder.Title = yagstd.ToString(val)
		case "custom_id":
			cid, err := validateCustomID(yagstd.ToString(val), nil)
			if err != nil {
				return nil, err
			}
			modalBuilder.CustomID = cid
		case "components":
			// the error is discarded, as YAGPDB's (:96): the list ends at a bad entry
			modalBuilder.Set("components", val)

		// (YAGPDB's comment marks this key for deprecation, :98)
		case "fields":
			if val == nil {
				continue
			}
			v, _ := indirect(reflect.ValueOf(val))
			if v.Kind() == reflect.Slice {
				const maxRows = 5 // Discord limitation
				usedCustomIDs := make(map[string]bool)
				for i := 0; i < v.Len() && i < maxRows; i++ {
					f, err := createComponent(types.TextInputComponent, v.Index(i).Interface())
					if err != nil {
						return nil, err
					}
					field := f.(types.TextInput)
					// validation
					if field.Style == 0 {
						field.Style = types.TextInputShort
					}
					field.CustomID, err = validateCustomID(field.CustomID, usedCustomIDs)
					if err != nil {
						return nil, err
					}
					usedCustomIDs[field.CustomID] = true
					modalBuilder.Components = append(modalBuilder.Components,
						types.ActionsRow{Components: []types.InteractiveComponent{field}})
				}
			} else {
				f, err := createComponent(types.TextInputComponent, val)
				if err != nil {
					return nil, err
				}
				field := f.(types.TextInput)
				if field.Style == 0 {
					field.Style = types.TextInputShort
				}
				field.CustomID, err = validateCustomID(field.CustomID, nil)
				if err != nil {
					return nil, err
				}
				modalBuilder.Components = append(modalBuilder.Components,
					types.ActionsRow{Components: []types.InteractiveComponent{field}})
			}
		default:
			return nil, errors.New(`invalid key "` + key + `" passed to send message builder`)
		}

	}

	return modalBuilder.toModal()
}

// countModal is IncreaseCheckCallCounter("modal", 1) (context_interactions.go:267-269):
// one modal per interaction, and the vendor error on the second.
func (ctx *ExecutionContext) countModal() error {
	ctx.Counters["modal"]++
	if ctx.Counters["modal"] > 1 {
		return errMultipleModals
	}
	return nil
}

// sendModal is YAGPDB's tmplSendModal (context_interactions.go:258-305), in its order:
// an interaction is needed, then the api_call, modal and interaction_response counters,
// then the modal (a builder, cmodal's response, or an sdict/map cmodal builds). Discord
// refuses a modal to an interaction already acknowledged (a deferral, or an execCC
// caller's response) and one answering a modal submission (INF: the codes are the
// emulator's stand-ins); otherwise the modal is the interaction's response.
func (e *Engine) sendModal(modal interface{}) (interface{}, error) {
	const fn = "sendModal"
	ctx := e.ctx
	if ctx.Interaction == nil {
		return "", errNoInteractionModal
	}
	if err := ctx.countAPICall(fn); err != nil {
		return "", err
	}
	if err := ctx.countModal(); err != nil {
		return "", err
	}
	if err := ctx.countInteractionResponse(); err != nil {
		return "", err
	}

	var typedModal *types.InteractionResponse
	var err error
	switch m := modal.(type) {
	case ModalBuilder:
		typedModal, err = m.toModal()
	case *ModalBuilder:
		typedModal, err = m.toModal()
	case *types.InteractionResponse:
		typedModal = m
	case types.InteractionResponse:
		typedModal = &m
	case types.SDict, *types.SDict, map[string]any:
		typedModal, err = CreateModal(m)
	default:
		return "", errInvalidModal
	}
	if err != nil {
		return "", err
	}

	if typedModal.Type != types.InteractionResponseModal {
		return "", errInvalidModal
	}

	// CreateInteractionResponse: what Discord refuses
	switch {
	case ctx.Interaction.RespondedTo && ctx.Interaction.Deferred:
		return "", ctx.discordRefuses(fn, errAlreadyAcked,
			fmt.Sprintf("the interaction was deferred (Defer mode: `%s`), which is its response, "+
				"and a modal can only be the first response; set Defer mode to None", ctx.deferMode))
	case ctx.Interaction.RespondedTo:
		return "", ctx.discordRefuses(fn, errAlreadyAcked,
			"the interaction was already responded to (by an execCC caller or child); "+
				"a modal can only be the first response")
	case ctx.Interaction.Type == types.InteractionModalSubmit:
		return "", ctx.discordRefuses(fn, errInvalidFormBody,
			"a modal submission can't be answered with another modal; reply with a message "+
				"(or updateMessage), then open the next modal from a button")
	}

	data := typedModal.Data
	if data == nil {
		data = &types.InteractionResponseData{}
	}
	cID, _ := strings.CutPrefix(data.CustomID, TemplateCustomIDPrefix)
	ctx.InteractionResponses = append(ctx.InteractionResponses, InteractionResponse{
		Kind: ResponseModal, ChannelID: ctx.ChannelID, Title: data.Title, CustomID: cID,
		Fields: modalFieldIDs(data.Components),
	})
	ctx.Interaction.RespondedTo = true
	return "", nil
}

// modalFieldIDs are the custom IDs of a modal's text inputs, in order, without the
// templates- prefix: cmodal's "fields" put each in an action row, a label wraps one.
func modalFieldIDs(components []types.TopLevelComponent) []string {
	ids := []string{}
	add := func(c types.InteractiveComponent) {
		var id string
		switch t := c.(type) {
		case types.TextInput:
			id = t.CustomID
		case *types.TextInput:
			id = t.CustomID
		default:
			return
		}
		id, _ = strings.CutPrefix(id, TemplateCustomIDPrefix)
		ids = append(ids, id)
	}
	for _, comp := range components {
		switch t := comp.(type) {
		case types.ActionsRow:
			for _, c := range t.Components {
				add(c)
			}
		case *types.ActionsRow:
			for _, c := range t.Components {
				add(c)
			}
		case types.Label:
			add(t.Component)
		case *types.Label:
			add(t.Component)
		}
	}
	return ids
}

// ModalTriggered reports whether the trigger runs the command on a modal submission
// (the control panel's "Modal Submission"; YAGPDB's CommandTriggerModal, whose
// triggerStrings name "Modal" is accepted too). A submission whose custom ID matches the
// trigger regex starts such a run.
func (t Trigger) ModalTriggered() bool {
	return strings.EqualFold(t.Type, "Modal Submission") || strings.EqualFold(t.Type, "Modal")
}

// CheckMatchModal is YAGPDB's customcommands.CheckMatchModal (handle_component.go:
// 439-453), CheckMatchComponent's match for a Modal trigger.
func CheckMatchModal(t Trigger, cID string) (match bool, stripped string, args []string) {
	if !t.ModalTriggered() {
		return false, "", nil
	}

	cmdMatch := "(?m)"
	if !t.CaseSensitive {
		cmdMatch += "(?i)"
	}
	cmdMatch += t.Text

	return matchRegexSplitArgs(cmdMatch, cID)
}

// ModalField is one submitted text input: its custom ID as the command wrote it (the
// templates- prefix is added when missing) and the text the user entered.
type ModalField struct {
	CustomID string
	Value    string
}

// ModalSubmission is a modal submitted, as a test declares it: the modal's custom ID
// (prefix as for a field), its fields in the modal's order, and the message whose
// component opened it (0: none, a modal a slash command opened).
type ModalSubmission struct {
	CustomID  string
	Fields    []ModalField
	MessageID int64
	// Form is how Discord sends the fields back: ModalFormActionRow (the default, a
	// cmodal "fields" modal) or ModalFormLabel (a modal of clabels: modalBuilder or
	// cmodal "components")
	Form string
}

// Modal submission forms (ModalSubmission.Form).
const (
	ModalFormActionRow = "action_row"
	ModalFormLabel     = "label"
)

// ModalTrigger is what the submission gave the Modal handler (handle_component.go:
// 342-424): the custom ID without its prefix, the match's Cmd/CmdArgs/StrippedID, and
// the submitted data.
type ModalTrigger struct {
	CustomID string
	Cmd      string
	CmdArgs  []string
	Stripped string
	Data     types.ModalSubmitInteractionData
}

// setData sets the handler's data keys (handle_component.go:342-424): .Values in the
// modal's order and .ModalValues by field ID, from the submitted rows as the handler
// reads them: an action row's text inputs, or a label's (the emulator's labels only
// wrap text inputs; YAGPDB's checkbox, radio and select cases aren't modelled).
func (m *ModalTrigger) setData(data map[string]interface{}) {
	data["InteractionData"] = m.Data
	data["CustomID"] = m.CustomID
	data["Cmd"] = m.Cmd
	data["CmdArgs"] = m.CmdArgs
	data["StrippedID"] = m.Stripped
	data["StrippedMsg"] = m.Stripped
	data["IsModal"] = true
	cmdValues := []any{}

	modalValues := types.SDict{}
	for i := 0; i < len(m.Data.Components); i++ {
		switch comp := m.Data.Components[i].(type) {
		case *types.ActionsRow:
			for j := 0; j < len(comp.Components); j++ {
				field, ok := comp.Components[j].(*types.TextInput)
				if !ok {
					continue // the emulator submits text inputs only
				}
				cmdValues = append(cmdValues, field.Value)
				cID, _ := strings.CutPrefix(field.CustomID, TemplateCustomIDPrefix)
				modalValues.Set(cID, types.SDict{
					"type":      field.Type(),
					"value":     field.Value,
					"custom_id": cID,
				})
			}
		case *types.Label:
			switch comp.Component.(type) {
			case *types.TextInput:
				t, _ := comp.Component.(*types.TextInput)
				cID, _ := strings.CutPrefix(t.CustomID, TemplateCustomIDPrefix)
				cmdValues = append(cmdValues, t.Value)
				modalValues.Set(cID, types.SDict{
					"type":      t.Type(),
					"value":     t.Value,
					"custom_id": cID,
				})
			}
		}
	}
	data["Values"] = cmdValues
	data["ModalValues"] = modalValues
}

// modalMessage is a modal run's .Message (and YAGPDB's ctx.Msg): the message whose
// component opened the modal (.Interaction.Message itself, given the submitter as author
// and member in place, as the handler does to the shared pointer), or a blank one in the
// run's channel by the submitter (handle_component.go:426-434).
func (ctx *ExecutionContext) modalMessage() types.CtxMessage {
	if ctx.Interaction.Message != nil {
		ctx.byUser(ctx.Interaction.Message)
		return *ctx.Interaction.Message
	}
	msg := types.CtxMessage{GuildID: ctx.GuildID, ChannelID: ctx.ChannelID}
	member := ctx.member(ctx.UserID)
	msg.Member = &member
	msg.Author = member.User
	return msg
}

// byUser makes the user the author and member of the interaction's own copy of its
// message, as the Component and Modal handlers do to interaction.Message before the run
// (handle_component.go:311-316, :426-431), so .Interaction.Message is .Message. It runs
// when the run's data is built, so the member's join time is the run's.
func (ctx *ExecutionContext) byUser(msg *types.CtxMessage) {
	member := ctx.member(ctx.UserID)
	msg.Member = &member
	msg.Author = member.User
}

// modalInteractionID is the ID of a modal submission interaction (Discord's are
// snowflakes): clear of the messages tests declare and the emulator sends.
const modalInteractionID = applicationCommandInteractionID + 1

// SetInteractionModal makes the run answer a modal submission on the trigger t: the
// modal's custom ID must match the trigger regex (ErrTriggerMismatch otherwise, so a test
// can assert no_trigger), and a message_id must be a message the test declares in the
// run's channel. The fields are submitted in
// the order given, each a text input in its own action row or, for the label form, in a
// label. The panel's defer mode is applied as deferResponseToCCs does before
// the run (handle_component.go:132, :161-190).
func (ctx *ExecutionContext) SetInteractionModal(t Trigger, sub ModalSubmission,
	mode DeferMode) error {
	if !t.ModalTriggered() {
		return fmt.Errorf("a modal submission needs a Modal Submission trigger; the template's is %q",
			t.Type)
	}
	// Only a templates- custom ID runs a command, stripped (handle_component.go:111-117)
	rawID := WithTemplatePrefix(sub.CustomID)
	cID := strings.TrimPrefix(rawID, TemplateCustomIDPrefix)

	match, stripped, cmdArgs := CheckMatchModal(t, cID)
	if !match {
		return &triggerMismatchError{msg: fmt.Sprintf("the custom ID %q doesn't match the %s trigger %q",
			cID, t.Type, t.Text)}
	}

	var source *types.CtxMessage
	if sub.MessageID != 0 {
		for i := range ctx.Messages {
			if m := &ctx.Messages[i]; m.ID == sub.MessageID && m.ChannelID == ctx.ChannelID {
				clicked := *m // its own copy, as for a click
				source = &clicked
				break
			}
		}
		if source == nil {
			return fmt.Errorf("interaction message_id %d isn't a message in channel %d: declare it in "+
				"messages: with that channel_id", sub.MessageID, ctx.ChannelID)
		}
	}

	data := types.ModalSubmitInteractionData{CustomID: rawID}
	for _, f := range sub.Fields {
		input := &types.TextInput{CustomID: WithTemplatePrefix(f.CustomID), Value: f.Value}
		var comp types.TopLevelComponent = &types.ActionsRow{Components: []types.InteractiveComponent{input}}
		if sub.Form == ModalFormLabel {
			comp = &types.Label{Component: input}
		}
		data.Components = append(data.Components, comp)
	}

	member := ctx.member(ctx.UserID)
	ctx.Interaction = &types.CustomCommandInteraction{Interaction: &types.Interaction{
		ID:        modalInteractionID,
		Type:      types.InteractionModalSubmit,
		Data:      data,
		GuildID:   ctx.GuildID,
		ChannelID: ctx.ChannelID,
		Message:   source,
		Member:    &member,
		Token:     interactionToken(modalInteractionID),
		Version:   1,
	}}
	ctx.Modal = &ModalTrigger{
		CustomID: cID,
		Cmd:      cmdArgs[0],
		CmdArgs:  cmdArgs[1:], // []string{} without arguments (handle_component.go:347-351)
		Stripped: stripped,
		Data:     data,
	}

	ctx.deferMode = mode
	if mode != DeferModeNone {
		ctx.Interaction.RespondedTo = true
		ctx.Interaction.Deferred = true
		if mode == DeferModeUpdate && source == nil {
			// YAGPDB still marks the interaction deferred (:182-188)
			ctx.Warn(KindResponse, "Defer mode `%s` on a modal no message opened: Discord refuses "+
				"a deferred update there (INF), so YAGPDB's deferral fails and the run's output "+
				"has no message to edit", mode)
		}
	}
	return nil
}

// WithTemplatePrefix is a custom ID as Discord sends it back: with YAGPDB's templates-
// prefix, which a test may leave out.
func WithTemplatePrefix(id string) string {
	if strings.HasPrefix(id, TemplateCustomIDPrefix) {
		return id
	}
	return TemplateCustomIDPrefix + id
}
