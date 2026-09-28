package runtime

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// modalFields is the field IDs of a built modal (prefix stripped), in order.
func modalFields(t *testing.T, m *types.InteractionResponse) []string {
	t.Helper()
	if m == nil || m.Data == nil {
		t.Fatal("no modal built")
	}
	return modalFieldIDs(m.Data.Components)
}

// fieldSlice is n cmodal fields as sdicts, with the custom IDs given ("" leaves it out).
func fieldSlice(ids ...string) types.Slice {
	out := types.Slice{}
	for _, id := range ids {
		f := types.SDict{"label": "L"}
		if id != "" {
			f["custom_id"] = id
		}
		out = append(out, f)
	}
	return out
}

// Every error text of CreateModal (context_interactions.go:51-146) and its quirks
func TestCreateModal(t *testing.T) {
	if _, err := CreateModal(); err == nil || err.Error() != "no values passed to component builder" {
		t.Errorf("no values: %v", err)
	}
	_, err := CreateModal("fields", fieldSlice("a"), "components", types.Slice{})
	if err == nil || err.Error() != "cannot have both 'components' and 'fields' in a cmodal" {
		t.Errorf("both keys: %v", err)
	}
	if _, err := CreateModal("title", "x", "footer", "y"); err == nil ||
		err.Error() != `invalid key "footer" passed to send message builder` {
		t.Errorf("unknown key: %v", err)
	}
	if _, err := CreateModal("custom_id", strings.Repeat("x", 91)); err == nil ||
		err.Error() != "custom id too long (max 90 chars)" {
		t.Errorf("long custom_id: %v", err)
	}

	// The default custom ID (:77), the modal response type, the title
	m, err := CreateModal("title", "Edit rule")
	if err != nil || m.Type != types.InteractionResponseModal || m.Data.CustomID != "templates--0" ||
		m.Data.Title != "Edit rule" || m.Data.Components != nil {
		t.Errorf("defaults: %+v %v", m, err)
	}

	// fields: blank IDs numbered by the fields before them, style short by default
	m, err = CreateModal("custom_id", "edit:rule:3", "fields", fieldSlice("", "text", ""))
	if err != nil {
		t.Fatal(err)
	}
	if m.Data.CustomID != "templates-edit:rule:3" {
		t.Errorf("custom_id prefixed: %q", m.Data.CustomID)
	}
	if got := modalFields(t, m); !reflect.DeepEqual(got, []string{"0", "text", "2"}) {
		t.Errorf("blank IDs numbered: %q", got)
	}
	row := m.Data.Components[0].(types.ActionsRow)
	if in := row.Components[0].(types.TextInput); in.Style != types.TextInputShort || in.Label != "L" ||
		in.CustomID != "templates-0" || in.Required {
		t.Errorf("a field is a short text input in an action row: %+v", in)
	}

	// A slice keeps only the first 5 fields, silently (:105-107)
	m, err = CreateModal("fields", fieldSlice("a", "b", "c", "d", "e", "f", "g"))
	if err != nil {
		t.Fatal(err)
	}
	if got := modalFields(t, m); !reflect.DeepEqual(got, []string{"a", "b", "c", "d", "e"}) {
		t.Errorf("five fields kept: %q", got)
	}

	// A single field (not a slice) is one text input, a blank ID templates-0
	m, err = CreateModal("fields", types.SDict{"label": "Only", "style": 2})
	if err != nil {
		t.Fatal(err)
	}
	if got := modalFields(t, m); !reflect.DeepEqual(got, []string{"0"}) {
		t.Errorf("single field: %q", got)
	}
	if in := m.Data.Components[0].(types.ActionsRow).Components[0].(types.TextInput); in.Style != types.TextInputParagraph {
		t.Errorf("style 2 kept: %+v", in)
	}
	if _, err := CreateModal("fields", fieldSlice("a"), "title", 5); err != nil {
		t.Errorf("a title is any value: %v", err)
	}
	if _, err := CreateModal("fields", types.Slice{types.SDict{"label": 5}}); err == nil ||
		!strings.Contains(err.Error(), "cannot unmarshal number") {
		t.Errorf("a bad field is the decode error: %v", err)
	}
}

// "components" goes through ModalBuilder.Set, whose error cmodal discards (:96): a bad
// entry ends the list there, a sixth label is dropped, and a non-slice leaves it empty
func TestCreateModalComponentsDiscardsSetError(t *testing.T) {
	label := func(id string) *types.Label {
		in, err := CreateTextInput("custom_id", id)
		if err != nil {
			t.Fatal(err)
		}
		l, err := CreateLabel("label", "L", "component", in)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	bare, _ := CreateTextInput("custom_id", "bare")
	m, err := CreateModal("components", types.Slice{label("a"), bare, label("c")})
	if err != nil {
		t.Fatalf("the Set error is discarded: %v", err)
	}
	if got := modalFields(t, m); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("the list ends at the bare text input: %q", got)
	}
	six := types.Slice{label("a"), label("b"), label("c"), label("d"), label("e"), label("f")}
	m, err = CreateModal("components", six)
	if err != nil {
		t.Fatalf("the Set error is discarded: %v", err)
	}
	if got := modalFields(t, m); !reflect.DeepEqual(got, []string{"a", "b", "c", "d", "e"}) {
		t.Errorf("five labels kept: %q", got)
	}
	m, err = CreateModal("components", label("a"))
	if err != nil || len(m.Data.Components) != 0 {
		t.Errorf("a non-slice leaves the list empty: %+v %v", m.Data.Components, err)
	}
}

// ModalBuilder and modalBuilder (context.go:1292-1373, context_interactions.go:32-49)
func TestModalBuilder(t *testing.T) {
	in, _ := CreateTextInput("custom_id", "text", "label", "Text")
	label, err := CreateLabel("label", "Rule", "component", in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CreateModalBuilder("", "Edit", label)
	if err != nil || b.CustomID != "templates-0" || b.Title != "Edit" || len(b.Components) != 1 {
		t.Errorf("builder: %+v %v", b, err)
	}

	// A bare text input isn't top-level; an action row isn't modal supported
	if _, err := CreateModalBuilder("m", "t", in); err == nil ||
		err.Error() != "invalid top level component passed to modal builder" {
		t.Errorf("bare text input: %v", err)
	}
	row := types.ActionsRow{Components: []types.InteractiveComponent{*in}}
	if _, err := CreateModalBuilder("m", "t", row); err == nil ||
		err.Error() != "invalid top level component passed to modal builder" {
		t.Errorf("action row: %v", err)
	}
	if _, err := CreateModalBuilder(strings.Repeat("x", 91), "t"); err == nil ||
		err.Error() != "custom id too long (max 90 chars)" {
		t.Errorf("long id: %v", err)
	}
	if _, err := CreateModalBuilder("m", "t", label, label, label, label, label, label); err == nil ||
		err.Error() != "modal builder can only have maximum 5 top level components" {
		t.Errorf("six components: %v", err)
	}

	if _, err := b.Set("footer", "x"); err == nil ||
		err.Error() != "invalid key, accepted keys are: title, custom_id, components" {
		t.Errorf("Set unknown key: %v", err)
	}
	if _, err := b.Set("components", label); err == nil ||
		err.Error() != "components must be a slice of Labels or TextFields" {
		t.Errorf("Set non-slice: %v", err)
	}
	if _, err := b.Set("custom_id", "new"); err != nil || b.CustomID != "templates-new" {
		t.Errorf("Set custom_id: %q %v", b.CustomID, err)
	}
	if _, err := b.AddComponents(types.Slice{label, label}, label); err != nil || len(b.Components) != 3 {
		t.Errorf("AddComponents takes slices and single ones: %d %v", len(b.Components), err)
	}
	m, err := CreateModal(b)
	if err != nil || m.Data.CustomID != "templates-new" || len(m.Data.Components) != 3 {
		t.Errorf("cmodal of a builder: %+v %v", m, err)
	}
}

// CreateTextInput (components.go:447-486) and CreateLabel (components.go:130-177)
func TestTextInputAndLabel(t *testing.T) {
	in, err := CreateTextInput("custom_id", "why", "label", "Why?", "style", 2, "required", true)
	if err != nil || in.CustomID != "templates-why" || in.Style != types.TextInputParagraph || !in.Required {
		t.Errorf("text input: %+v %v", in, err)
	}
	// The prefix goes before even an ID that has it; no custom_id leaves it blank
	if in, _ := CreateTextInput("custom_id", "templates-x"); in.CustomID != "templates-templates-x" {
		t.Errorf("prefix added again: %q", in.CustomID)
	}
	if in, _ := CreateTextInput("label", "x"); in.CustomID != "" || in.Style != types.TextInputShort {
		t.Errorf("no custom_id, short: %+v", in)
	}
	if in, err := CreateTextInput("label", 5); err == nil || in == nil || in.Label != "" {
		t.Errorf("a failed build returns the empty input and the error: %+v %v", in, err)
	}
	unset, _ := CreateTextInput("custom_id", "u")
	encoded, _ := json.Marshal(unset)
	if !strings.Contains(string(encoded), `"required":false`) || !strings.Contains(string(encoded), `"type":4`) {
		t.Errorf("required has no omitempty: %s", encoded)
	}

	label, err := CreateLabel("label", "Rule", "description", "d", "custom_id", "lbl", "component", in)
	if err != nil || label.Label != "Rule" || label.Description != "d" {
		t.Fatalf("label: %+v %v", label, err)
	}
	if got, ok := label.Component.(*types.TextInput); !ok || got.CustomID != "templates-why" {
		t.Errorf("the label wraps the text input: %#v", label.Component)
	}
	if same, _ := CreateLabel(label); same != label {
		t.Error("a label passes through")
	}
	button, _ := CreateButton("label", "b")
	if _, err := CreateLabel("label", "x", "component", button); err == nil ||
		err.Error() != "unsupported component in label" {
		t.Errorf("a button: %v", err)
	}
	if _, err := CreateLabel("label", "x", "component", "text"); err == nil ||
		err.Error() != "unsupported component in label" {
		t.Errorf("a string: %v", err)
	}
	menu, _ := CreateSelectMenu("options", types.Slice{types.SDict{"label": "a", "value": "a"}})
	if _, err := CreateLabel("label", "x", "component", menu); err == nil ||
		!strings.Contains(err.Error(), "isn't modelled by the emulator") {
		t.Errorf("a menu is allowed in YAGPDB but not modelled: %v", err)
	}
	if _, err := CreateLabel("custom_id", strings.Repeat("x", 91)); err == nil ||
		err.Error() != "custom id too long (max 90 chars)" {
		t.Errorf("long custom_id: %v", err)
	}
}

// modalCtx is a run answering a submission of the modal edit:rule:3 with two fields,
// opened from message 7 when fromMessage.
func modalCtx(t *testing.T, fromMessage bool, mode DeferMode) *ExecutionContext {
	t.Helper()
	ctx := newCtx(false, true)
	ctx.Messages = append(ctx.Messages, types.CtxMessage{ID: 7, ChannelID: ctx.ChannelID, GuildID: ctx.GuildID,
		Author: botUser, Content: "rule 3"})
	sub := ModalSubmission{CustomID: "edit:rule:3 extra", Fields: []ModalField{
		{CustomID: "zeta", Value: "new text"}, {CustomID: "templates-alpha", Value: "typo"}}}
	if fromMessage {
		sub.MessageID = 7
	}
	if err := ctx.SetInteractionModal(Trigger{Type: "Modal Submission", Text: `^edit:rule:(\d+)`}, sub, mode); err != nil {
		t.Fatalf("SetInteractionModal: %v", err)
	}
	return ctx
}

func TestSetInteractionModal(t *testing.T) {
	ctx := modalCtx(t, true, DeferModeNone)
	if ctx.Interaction.Type != types.InteractionModalSubmit || ctx.Interaction.Message.ID != 7 {
		t.Errorf("interaction: %+v", ctx.Interaction.Interaction)
	}
	data := ctx.BuildTemplateData()
	if data["IsModal"] != true || data["CustomID"] != "edit:rule:3 extra" || data["Cmd"] != "edit:rule:3" ||
		!reflect.DeepEqual(data["CmdArgs"], []string{"extra"}) || data["StrippedID"] != " extra" {
		t.Errorf("keys: %v %v %v %v %q", data["IsModal"], data["CustomID"], data["Cmd"], data["CmdArgs"], data["StrippedID"])
	}
	if got := data["Values"]; !reflect.DeepEqual(got, []any{"new text", "typo"}) {
		t.Errorf(".Values in field order: %#v", got)
	}
	mv := data["ModalValues"].(types.SDict)
	alpha := mv["alpha"].(types.SDict)
	if alpha["value"] != "typo" || alpha["custom_id"] != "alpha" || alpha["type"] != types.TextInputComponent {
		t.Errorf(".ModalValues.alpha: %#v", alpha)
	}
	if id := data["InteractionData"].(types.ModalSubmitInteractionData).CustomID; id != "templates-edit:rule:3 extra" {
		t.Errorf("the raw custom ID keeps the prefix: %q", id)
	}
	if msg := data["Message"].(types.CtxMessage); msg.ID != 7 || msg.Author.ID != ctx.UserID || msg.Member == nil {
		t.Errorf(".Message is the source message by the submitter: %+v", msg)
	}
	if ctx.Messages[0].Author.ID != botUser.ID {
		t.Error("the stored message keeps its real author")
	}

	blank := modalCtx(t, false, DeferModeNone)
	msg := blank.BuildTemplateData()["Message"].(types.CtxMessage)
	if msg.ID != 0 || msg.ChannelID != blank.ChannelID || msg.GuildID != blank.GuildID || msg.Author.ID != blank.UserID ||
		msg.Content != "" {
		t.Errorf("without a source message, a blank one: %+v", msg)
	}

	// A mismatch is a trigger mismatch; a Component trigger doesn't take a submission
	other := newCtx(false, true)
	err := other.SetInteractionModal(Trigger{Type: "Modal", Text: "^edit:"}, ModalSubmission{CustomID: "open:1"}, DeferModeNone)
	if _, ok := err.(*triggerMismatchError); !ok {
		t.Errorf("want a trigger mismatch: %v", err)
	}
	err = other.SetInteractionModal(Trigger{Type: "Message Component", Text: "^edit:"},
		ModalSubmission{CustomID: "edit:1"}, DeferModeNone)
	if err == nil || !strings.Contains(err.Error(), "needs a Modal Submission trigger") {
		t.Errorf("a Component trigger: %v", err)
	}

	// A defer mode applies; a deferred update of a modal no message opened warns
	if d := modalCtx(t, true, DeferModeEphemeral); !d.Interaction.RespondedTo || !d.Interaction.Deferred {
		t.Error("a defer mode responds before the run")
	}
	upd := modalCtx(t, false, DeferModeUpdate)
	if len(kinds(upd, KindResponse)) != 1 {
		t.Errorf("a deferred update with no message warns: %q", kinds(upd, KindResponse))
	}
	upd.respondWithOutput("done")
	if len(upd.InteractionResponses) != 0 || len(kinds(upd, KindResponse)) != 2 {
		t.Errorf("the output has nothing to edit: %+v %q", upd.InteractionResponses, kinds(upd, KindResponse))
	}
}

// sendModal (context_interactions.go:258-305): its checks in vendor order, and what
// Discord refuses
func TestSendModal(t *testing.T) {
	fields := `"fields" (cslice (sdict "custom_id" "text" "label" "Text"))`
	open := `{{ sendModal (cmodal "title" "Edit" "custom_id" "edit:1" ` + fields + `) }}`

	ctx := componentCtx(t, Trigger{Type: "Component", Text: "^open$"}, "open", DeferModeNone)
	if _, err := run(t, ctx, open); err != nil {
		t.Fatal(err)
	}
	want := []InteractionResponse{{Kind: ResponseModal, ChannelID: ctx.ChannelID, Title: "Edit", CustomID: "edit:1",
		Fields: []string{"text"}}}
	if !reflect.DeepEqual(ctx.InteractionResponses, want) || !ctx.Interaction.RespondedTo {
		t.Errorf("responses: %+v", ctx.InteractionResponses)
	}
	if len(kinds(ctx, KindResponse)) != 0 {
		t.Errorf("a modal is a response: %q", kinds(ctx, KindResponse))
	}

	errCases := []struct {
		name, src string
		ctx       func() *ExecutionContext
		want      string
	}{
		{"no interaction", open, func() *ExecutionContext { return newCtx(false, true) }, "no interaction data in context"},
		{"a second modal", open + open, nil, "cannot send multiple modals to the same interaction"},
		{"after sendResponse", `{{ sendResponse nil "x" }}` + open, nil,
			"cannot respond to an interaction > 1 time; consider using a followup"},
		{"a dict", `{{ sendModal (dict "title" "x") }}`, nil, "invalid modal passed to sendModal"},
		{"a string", `{{ sendModal "x" }}`, nil, "invalid modal passed to sendModal"},
		{"an sdict built by cmodal's rules", `{{ sendModal (sdict "title" "x" "nope" 1) }}`, nil,
			`invalid key "nope" passed to send message builder`},
	}
	for _, c := range errCases {
		cx := componentCtx(t, Trigger{Type: "Component", Text: "^open$"}, "open", DeferModeNone)
		if c.ctx != nil {
			cx = c.ctx()
		}
		_, err := run(t, cx, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want %q, got %v", c.name, c.want, err)
		}
	}

	// An sdict, a builder: both sent; the builder's labels give the fields
	for name, src := range map[string]string{
		"sdict":   `{{ sendModal (sdict "title" "Edit" "custom_id" "edit:1" ` + fields + `) }}`,
		"builder": `{{ sendModal (modalBuilder "edit:1" "Edit" (clabel "label" "Text" "component" (ctextInput "custom_id" "text"))) }}`,
	} {
		cx := componentCtx(t, Trigger{Type: "Component", Text: "^open$"}, "open", DeferModeNone)
		if _, err := run(t, cx, src); err != nil || !reflect.DeepEqual(cx.InteractionResponses, want) {
			t.Errorf("%s: %v %+v", name, err, cx.InteractionResponses)
		}
	}

	// The api_call counter comes after the interaction check (:259-265)
	plain := newCtx(true, true)
	plain.Counters["api_call"] = limitAPI.premium
	if _, err := NewEngine(plain).sendModal(types.SDict{}); err == nil || err.Error() != errNoInteractionModal.Error() ||
		plain.Counters["api_call"] != limitAPI.premium {
		t.Errorf("no interaction at the cap: %v", err)
	}

	// Discord's side: after a deferral, and in answer to a modal submission
	deferred := componentCtx(t, Trigger{Type: "Component", Text: "^open$"}, "open", DeferModeMessage)
	deferred.Strict = true
	if _, err := run(t, deferred, open); err == nil || !strings.Contains(err.Error(), "40060") {
		t.Errorf("after a deferral (strict): %v", err)
	}
	lenient := componentCtx(t, Trigger{Type: "Component", Text: "^open$"}, "open", DeferModeMessage)
	var warned []string
	if _, err := run(t, lenient, open); err == nil {
		for _, d := range lenient.Diagnostics {
			warned = append(warned, d.Message)
		}
	}
	if len(lenient.InteractionResponses) != 0 || !strings.Contains(strings.Join(warned, "\n"), "already been acknowledged") {
		t.Errorf("after a deferral (lenient): %+v %q", lenient.InteractionResponses, warned)
	}
	submitted := modalCtx(t, true, DeferModeNone)
	submitted.Strict = true
	_, err := run(t, submitted, open)
	var detail []string
	for _, d := range submitted.Diagnostics {
		detail = append(detail, d.Message)
	}
	if err == nil || !strings.Contains(err.Error(), "50035") ||
		!strings.Contains(strings.Join(detail, "\n"), "can't be answered with another modal") {
		t.Errorf("in answer to a modal submission: %v %q", err, detail)
	}
}

// The handlers set the author and member on interaction.Message itself (handle_component.go:
// 311-316, :426-431; for a click, the re-fetched message :76-80), so .Interaction.Message
// is .Message: the clicker or submitter as author. The channel's message keeps its own.
func TestInteractionMessageIsTheHandlersMessage(t *testing.T) {
	check := func(name string, ctx *ExecutionContext) {
		t.Helper()
		data := ctx.BuildTemplateData()
		in := data["Interaction"].(*types.CustomCommandInteraction)
		msg := data["Message"].(types.CtxMessage)
		if in.Message.Author.ID != ctx.UserID || in.Message.Member == nil || in.Message.Member.User.ID != ctx.UserID {
			t.Errorf("%s: .Interaction.Message is by the user: author %d member %+v", name,
				in.Message.Author.ID, in.Message.Member)
		}
		if msg.Author.ID != ctx.UserID || msg.ID != in.Message.ID {
			t.Errorf("%s: .Message: %+v", name, msg)
		}
		if ctx.Messages[0].Author.ID != botUser.ID {
			t.Errorf("%s: the channel's message keeps its real author", name)
		}
	}
	check("click", componentCtx(t, Trigger{Type: "Component", Text: "^pg:"}, "pg:1", DeferModeNone))
	check("modal", modalCtx(t, true, DeferModeNone))
	if blank := modalCtx(t, false, DeferModeNone); blank.BuildTemplateData()["Interaction"].(*types.CustomCommandInteraction).Message != nil {
		t.Error("a modal no message opened keeps a nil .Interaction.Message (the blank one is the handler's own)")
	}
}

// clabel without "component": discordgo's Label.UnmarshalJSON asserts a nil interface
// (components.go:557), a panic safeCall reports with Go's text
func TestCreateLabelWithoutComponent(t *testing.T) {
	_, err := CreateLabel("label", "L")
	want := "interface conversion: interface is nil, not discordgo.InteractiveComponent"
	if err == nil || err.Error() != want {
		t.Errorf("want %q, got %v", want, err)
	}
}

// A modal's label form (modalBuilder, cmodal "components") comes back as labels
// (handle_component.go:373-383): the same .Values and .ModalValues
func TestModalSubmittedAsLabels(t *testing.T) {
	ctx := newCtx(false, true)
	sub := ModalSubmission{CustomID: "m", Form: ModalFormLabel, Fields: []ModalField{
		{CustomID: "b", Value: "2"}, {CustomID: "a", Value: "1"}}}
	if err := ctx.SetInteractionModal(Trigger{Type: "Modal", Text: "^m$"}, sub, DeferModeNone); err != nil {
		t.Fatal(err)
	}
	data := ctx.BuildTemplateData()
	if _, ok := data["InteractionData"].(types.ModalSubmitInteractionData).Components[0].(*types.Label); !ok {
		t.Errorf("submitted as labels: %#v", data["InteractionData"])
	}
	if !reflect.DeepEqual(data["Values"], []any{"2", "1"}) {
		t.Errorf(".Values: %#v", data["Values"])
	}
	if a := data["ModalValues"].(types.SDict)["a"].(types.SDict); a["value"] != "1" || a["type"] != types.TextInputComponent {
		t.Errorf(".ModalValues.a: %#v", a)
	}
}

// After the Update deferral of a modal no message opened failed, the interaction was
// never acknowledged, so a sendResponse (a followup) fails on Discord's side too (INF)
func TestFollowupAfterFailedModalDeferral(t *testing.T) {
	ctx := modalCtx(t, false, DeferModeUpdate)
	if _, err := NewEngine(ctx).sendResponse("sendResponse", true, false, nil, "hi"); err != nil {
		t.Fatal(err)
	}
	if len(ctx.InteractionResponses) != 0 || len(ctx.SentMessages) != 0 {
		t.Errorf("nothing delivered: %+v", ctx.InteractionResponses)
	}
	var warned []string
	for _, d := range ctx.Diagnostics {
		warned = append(warned, d.Message)
	}
	if !strings.Contains(strings.Join(warned, "\n"), "never acknowledged") {
		t.Errorf("warned: %q", warned)
	}
	strict := modalCtx(t, false, DeferModeUpdate)
	strict.Strict = true
	if _, err := NewEngine(strict).sendResponse("sendResponse", true, false, nil, "hi"); err == nil ||
		!strings.Contains(err.Error(), "10015") || len(strict.InteractionResponses) != 0 {
		t.Errorf("strict: %v", err)
	}
}

// A text input or a label only goes in a modal: Discord refuses one in a message (INF)
func TestTextInputOutsideAModal(t *testing.T) {
	src := `{{ $m := cmodal "title" "t" "fields" (sdict "label" "a" "custom_id" "a") }}` +
		`{{ sendMessage nil $m.Data }}`
	ctx := newCtx(true, true)
	_, err := run(t, ctx, src)
	// the one message sent is show_errors' error message, without components
	if err == nil || !strings.Contains(err.Error(), "50035") || len(ctx.SentMessages) != 1 ||
		ctx.SentMessages[0].Components != nil {
		t.Errorf("strict: %v, sent %+v", err, ctx.SentMessages)
	}
	lenient := newCtx(false, true)
	if _, err := run(t, lenient, src); err != nil || len(lenient.SentMessages) != 0 {
		t.Errorf("lenient: %v, %d sent", err, len(lenient.SentMessages))
	}
	var warned []string
	for _, d := range lenient.Diagnostics {
		warned = append(warned, d.Message)
	}
	if !strings.Contains(strings.Join(warned, "\n"), "only go in a modal") {
		t.Errorf("warned: %q", warned)
	}
}

// A cmodal's .Data reaches parseMessageInput as *InteractionResponseData (context_funcs.go:
// 47-48): its components, no content
func TestParseMessageInputInteractionResponseData(t *testing.T) {
	m, err := CreateModal("title", "t", "fields", fieldSlice("a"))
	if err != nil {
		t.Fatal(err)
	}
	ms := parseMessageInput(m.Data)
	if ms.Content != "" || len(ms.Components) != 1 || ms.Embeds != nil || len(ms.AllowedMentions.Parse) != 0 {
		t.Errorf("message send: %+v", ms)
	}
}
