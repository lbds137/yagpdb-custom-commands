package runtime

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

func TestCheckMatchComponent(t *testing.T) {
	pager := Trigger{Type: "Message Component", Text: `^pg:(\w+):(\d+)$`}
	tests := []struct {
		name     string
		t        Trigger
		cID      string
		match    bool
		stripped string
		args     []string
	}{
		{"anchored regex", pager, "pg:stale:2", true, "", []string{"pg:stale:2"}},
		{"case-insensitive by default", pager, "PG:stale:2", true, "", []string{"PG:stale:2"}},
		{"case-sensitive", Trigger{Type: "Component", Text: "^pg:", CaseSensitive: true}, "PG:x", false, "", nil},
		{"no match", pager, "confirm:delete:7", false, "", nil},
		{"the rest is split into arguments", Trigger{Type: "Component", Text: "^bucket"}, "bucket a b",
			true, " a b", []string{"bucket", "a", "b"}},
		{"a message trigger never matches a click", Trigger{Type: "Command", Text: "pg"}, "pg", false, "", nil},
	}
	for _, tt := range tests {
		match, stripped, args := CheckMatchComponent(tt.t, tt.cID)
		if match != tt.match || stripped != tt.stripped || !reflect.DeepEqual(args, tt.args) {
			t.Errorf("%s: got %v %q %q", tt.name, match, stripped, args)
		}
	}
}

func TestReadDeferMode(t *testing.T) {
	for src, want := range map[string]DeferMode{
		"{{/* Trigger type: `Message Component` */}}":                    DeferModeNone,
		"{{/* Defer mode: `None` */}}":                                   DeferModeNone,
		"{{/* Defer mode: `Message Response` */}}":                       DeferModeMessage,
		"{{/* Defer mode: `ephemeral message response` */}}":             DeferModeEphemeral,
		"{{- /*\n  Defer mode: `Update Message Response`\n*/ -}}\nhello": DeferModeUpdate,
	} {
		if got := ReadDeferMode(src); got != want {
			t.Errorf("%q: got %v, want %v", src, got, want)
		}
	}
	err := ValidateHeader("{{/* Defer mode: `Later` */}}")
	if err == nil || !strings.Contains(err.Error(), "isn't one of the panel's") {
		t.Errorf("a bad defer mode should fail the header: %v", err)
	}
	if err := ValidateHeader("{{/* Defer mode: `Update Message Response` */}}"); err != nil {
		t.Errorf("a panel label is valid: %v", err)
	}
}

// componentCtx is a run answering a click on message 7 in the run's channel.
func componentCtx(t *testing.T, trigger Trigger, customID string, mode DeferMode) *ExecutionContext {
	t.Helper()
	ctx := newCtx(false, true)
	ctx.Messages = append(ctx.Messages, types.CtxMessage{ID: 7, ChannelID: ctx.ChannelID, GuildID: ctx.GuildID,
		Author: botUser, Content: "page 1"})
	click := ComponentClick{CustomID: customID, MessageID: 7, ComponentType: types.ButtonComponent}
	if err := ctx.SetInteractionComponent(trigger, click, mode); err != nil {
		t.Fatalf("SetInteractionComponent: %v", err)
	}
	return ctx
}

func TestSetInteractionComponent(t *testing.T) {
	pager := Trigger{Type: "Message Component", Text: `^pg:(\w+):(\d+)$`}
	ctx := componentCtx(t, pager, "pg:stale:2", DeferModeNone)
	if ctx.Interaction.RespondedTo || ctx.Interaction.Deferred {
		t.Error("nothing responded yet")
	}
	if ctx.Interaction.Type != types.InteractionMessageComponent || ctx.Interaction.Message.ID != 7 {
		t.Errorf("interaction: %+v", ctx.Interaction.Interaction)
	}
	if got := ctx.Interaction.Data.(types.MessageComponentInteractionData).CustomID; got != "templates-pg:stale:2" {
		t.Errorf("the raw custom ID keeps the prefix: %q", got)
	}
	if ctx.Component.CustomID != "pg:stale:2" || ctx.Component.Cmd != "pg:stale:2" || len(ctx.Component.CmdArgs) != 0 {
		t.Errorf("component data: %+v", ctx.Component)
	}
	if sendType, token := ctx.tokenArg(nil); token != ctx.Interaction.Token || sendType != sendMessageInteractionResponse {
		t.Errorf("nil token is the current interaction's response: %v %q", sendType, token)
	}
	if sendType, _ := ctx.tokenArg(ctx.Interaction.Token); sendType != sendMessageInteractionResponse {
		t.Error("the interaction's own token is accepted")
	}
	if _, token := ctx.tokenArg("not a token"); token != "" {
		t.Errorf("a token that doesn't decode is invalid: %q", token)
	}
	data := ctx.BuildTemplateData()
	if data["IsButton"] != true || data["CustomID"] != "pg:stale:2" || data["StrippedID"] != "" {
		t.Errorf("data keys: %v %v %v", data["IsButton"], data["CustomID"], data["StrippedID"])
	}
	if msg := data["Message"].(types.CtxMessage); msg.Author.ID != ctx.UserID || msg.Member == nil || msg.ID != 7 {
		t.Errorf(".Message is the clicked message by the clicker: %+v", msg)
	}
	if ctx.Messages[0].Author.ID != botUser.ID {
		t.Error("the stored message keeps its real author")
	}

	// A mismatch is a trigger mismatch a no_trigger test can detect
	other := newCtx(false, true)
	other.Messages = ctx.Messages
	err := other.SetInteractionComponent(pager, ComponentClick{CustomID: "confirm:1", MessageID: 7}, DeferModeNone)
	if err == nil || !strings.Contains(err.Error(), "doesn't match") {
		t.Errorf("want a mismatch, got %v", err)
	}
	if _, ok := err.(*triggerMismatchError); !ok {
		t.Errorf("a mismatch unwraps to ErrTriggerMismatch: %T", err)
	}
	err = other.SetInteractionComponent(pager, ComponentClick{CustomID: "pg:a:1", MessageID: 8}, DeferModeNone)
	if err == nil || !strings.Contains(err.Error(), "isn't a message in channel") {
		t.Errorf("an undeclared message is refused: %v", err)
	}

	// A deferral counts as the response
	deferred := componentCtx(t, pager, "pg:stale:2", DeferModeEphemeral)
	if !deferred.Interaction.RespondedTo || !deferred.Interaction.Deferred {
		t.Error("a defer mode responds before the run")
	}
}

func TestInteractionResponseErrors(t *testing.T) {
	ctx := componentCtx(t, Trigger{Type: "Component", Text: "^pg:"}, "pg:1", DeferModeNone)
	e := NewEngine(ctx)
	if _, err := e.updateMessage("updateMessage", true, "one"); err != nil {
		t.Fatalf("first update: %v", err)
	}
	_, err := e.updateMessage("updateMessage", true, "two")
	if err == nil || err.Error() != "cannot respond to an interaction > 1 time; consider using a followup" {
		t.Errorf("second update: %v", err)
	}
	if len(ctx.InteractionResponses) != 1 || ctx.InteractionResponses[0].Kind != ResponseUpdate {
		t.Errorf("one update recorded: %+v", ctx.InteractionResponses)
	}
	if ctx.Messages[0].Content != "one" || ctx.Interaction.Message.Content != "one" {
		t.Error("the update edits the channel's message and the interaction's copy")
	}
	// After the update, sendResponse is a followup, with the message's ID for RetID
	id, err := e.sendResponse("sendResponseRetID", true, true, nil, "more")
	if err != nil || id == "" || ctx.InteractionResponses[1].Kind != ResponseFollowup {
		t.Errorf("followup: %v %v %+v", id, err, ctx.InteractionResponses)
	}

	plain := newCtx(false, true)
	pe := NewEngine(plain)
	if _, err := pe.updateMessage("updateMessage", true, "x"); err == nil ||
		err.Error() != "no interaction data in context; consider editMessage or editResponse" {
		t.Errorf("updateMessage without an interaction: %v", err)
	}
	if _, err := pe.sendResponse("sendResponse", true, false, nil, "x"); err == nil || err.Error() != "invalid interaction token" {
		t.Errorf("sendResponse without an interaction: %v", err)
	}
	if pe.ephemeralResponse(); plain.ephemeralResponse {
		t.Error("ephemeralResponse is a no-op without an interaction")
	}
}

// TestUpdateMessageChecksTheInteractionBeforeCounting pins the vendor order
// (context_interactions.go:376-380): at the api_call cap with no interaction the error is
// the no-interaction one; with one, the cap's error (strict), and lenient mode goes on.
func TestUpdateMessageChecksTheInteractionBeforeCounting(t *testing.T) {
	// Through the function map, as a template calls it (the withLimits wrapper included)
	plain := newCtx(true, true)
	plain.Counters["api_call"] = limitAPI.premium
	update := NewEngine(plain).BuildFuncMap()["updateMessage"].(func(interface{}) (interface{}, error))
	_, err := update("x")
	if err == nil || err.Error() != errNoInteractionUpdate.Error() {
		t.Errorf("no interaction at the cap: %v", err)
	}
	if plain.Counters["api_call"] != limitAPI.premium {
		t.Error("nothing counted without an interaction")
	}

	strict := componentCtx(t, Trigger{Type: "Component", Text: "^pg:"}, "pg:1", DeferModeNone)
	strict.Strict = true
	strict.Counters["api_call"] = limitAPI.premium
	if _, err := NewEngine(strict).updateMessage("updateMessage", true, "x"); err == nil ||
		err.Error() != ErrTooManyAPICalls.Error() {
		t.Errorf("at the cap with -strict: %v", err)
	}

	lenient := componentCtx(t, Trigger{Type: "Component", Text: "^pg:"}, "pg:1", DeferModeNone)
	lenient.Counters["api_call"] = limitAPI.premium
	if _, err := NewEngine(lenient).updateMessage("updateMessage", true, "x"); err != nil ||
		len(lenient.InteractionResponses) != 1 || len(kinds(lenient, KindLimit)) != 1 {
		t.Errorf("at the cap, lenient: err %v, %d responses, warnings %q", err,
			len(lenient.InteractionResponses), kinds(lenient, KindLimit))
	}
}

// TestDeferredMessageResponseStaysVisible pins that ephemeralResponse can't make a
// Message Response deferral ephemeral (only the Ephemeral defer mode is), while the
// response reactions are still skipped.
func TestDeferredMessageResponseStaysVisible(t *testing.T) {
	ctx := componentCtx(t, Trigger{Type: "Component", Text: "^pg:"}, "pg:1", DeferModeMessage)
	ctx.ephemeralResponse = true
	ctx.responseReactions = []string{"👋"}
	ctx.respondWithOutput("done")
	r := ctx.InteractionResponses[0]
	if r.Kind != ResponseDeferredEdit || r.Ephemeral || ctx.SentMessages[0].Ephemeral {
		t.Errorf("a deferred Message Response stays visible: %+v", r)
	}
	if len(ctx.Reactions) != 0 {
		t.Error("ephemeralResponse still skips the response reactions")
	}
	if len(ctx.Messages) != 2 {
		t.Errorf("a visible message is in the channel: %d messages", len(ctx.Messages))
	}
}

// TestClickNeedsADeclaredMessage pins that the clicked message must be in messages: (or
// sent), never the trigger message knownMessage falls back to.
func TestClickNeedsADeclaredMessage(t *testing.T) {
	ctx := newCtx(false, true) // no messages: the default trigger message has ID 234567890
	if ctx.knownMessage(ctx.ChannelID, ctx.MessageID) == nil {
		t.Fatal("positive control: knownMessage finds the trigger message")
	}
	click := ComponentClick{CustomID: "pg:1", MessageID: ctx.MessageID, ComponentType: types.ButtonComponent}
	err := ctx.SetInteractionComponent(Trigger{Type: "Component", Text: "^pg:"}, click, DeferModeNone)
	if err == nil || !strings.Contains(err.Error(), "isn't a message in channel") {
		t.Errorf("the trigger message isn't a clicked message: %v", err)
	}
}

func TestRespondWithOutput(t *testing.T) {
	// The output is the response, then a followup
	ctx := componentCtx(t, Trigger{Type: "Component", Text: "^pg:"}, "pg:1", DeferModeNone)
	ctx.ephemeralResponse = true
	ctx.responseReactions = []string{"👋"}
	ctx.respondWithOutput("hello")
	ctx.respondWithOutput("again")
	if len(ctx.InteractionResponses) != 2 || ctx.InteractionResponses[0].Kind != ResponseMessage ||
		!ctx.InteractionResponses[0].Ephemeral || ctx.InteractionResponses[1].Kind != ResponseFollowup {
		t.Errorf("responses: %+v", ctx.InteractionResponses)
	}
	if len(ctx.Reactions) != 0 {
		t.Error("an ephemeral response gets no response reactions")
	}
	if len(ctx.SentMessages) != 2 || !ctx.SentMessages[0].Ephemeral || len(ctx.Messages) != 1 {
		t.Errorf("ephemeral messages are sent but not in the channel's messages: %d sent, %d in channel",
			len(ctx.SentMessages), len(ctx.Messages))
	}

	// Under Update Message Response the deferred edit is of the clicked message
	upd := componentCtx(t, Trigger{Type: "Component", Text: "^pg:"}, "pg:1", DeferModeUpdate)
	upd.respondWithOutput("edited")
	if len(upd.EditedMessages) != 1 || upd.Messages[0].Content != "edited" || upd.Interaction.Deferred ||
		upd.InteractionResponses[0].Kind != ResponseDeferredEdit || len(upd.SentMessages) != 0 {
		t.Errorf("deferred update: edits %+v, responses %+v", upd.EditedMessages, upd.InteractionResponses)
	}

	// Under Ephemeral Message Response the deferred edit is a new ephemeral message
	eph := componentCtx(t, Trigger{Type: "Component", Text: "^pg:"}, "pg:1", DeferModeEphemeral)
	eph.respondWithOutput("done")
	if len(eph.SentMessages) != 1 || !eph.SentMessages[0].Ephemeral || eph.InteractionResponses[0].Kind != ResponseDeferredEdit {
		t.Errorf("deferred ephemeral: %+v", eph.InteractionResponses)
	}
}
