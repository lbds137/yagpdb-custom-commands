package types

import "fmt"

// The interaction types a template sees, copied from discordgo (vendor
// lib/discordgo/interactions.go, YAGPDB c579722) and YAGPDB's CustomCommandInteraction
// (common/templates/context.go:286-290), as far as the emulator models them: a message
// component (button or menu) click. Slash commands, context menus and modals aren't
// modelled yet (docs/design/emulator-interactions.md, units 3 and 4).

// InteractionType indicates the type of an interaction event (interactions.go:159-168).
type InteractionType uint8

// Interaction types.
const (
	InteractionPing                           InteractionType = 1
	InteractionApplicationCommand             InteractionType = 2
	InteractionMessageComponent               InteractionType = 3
	InteractionApplicationCommandAutocomplete InteractionType = 4
	InteractionModalSubmit                    InteractionType = 5
)

// String is discordgo's (interactions.go:170-182).
func (t InteractionType) String() string {
	switch t {
	case InteractionPing:
		return "Ping"
	case InteractionApplicationCommand:
		return "ApplicationCommand"
	case InteractionMessageComponent:
		return "MessageComponent"
	case InteractionModalSubmit:
		return "ModalSubmit"
	}
	return fmt.Sprintf("InteractionType(%d)", t)
}

// InteractionData is a common interface for all types of interaction data
// (interactions.go:297-300).
type InteractionData interface {
	Type() InteractionType
}

// MessageComponentInteractionData contains the data of message component interaction
// (interactions.go:337-345): the clicked component's full custom ID (templates- prefix
// included), its type, and a menu's selected values (nil for a button).
type MessageComponentInteractionData struct {
	CustomID      string        `json:"custom_id"`
	ComponentType ComponentType `json:"component_type"`

	// NOTE: Only filled when ComponentType is SelectMenuComponent (3). Otherwise is nil.
	Values []string `json:"values"`
}

// Type returns the type of interaction data (interactions.go:347-349).
func (MessageComponentInteractionData) Type() InteractionType {
	return InteractionMessageComponent
}

// Interaction represents data of an interaction (interactions.go:185-220) with the
// fields the emulator fills. Message is the component's message and Member the clicker,
// as Discord sends them (before the handler makes the clicker .Message's author).
type Interaction struct {
	ID            int64
	ApplicationID int64
	Type          InteractionType
	Data          InteractionData
	GuildID       int64
	ChannelID     int64

	// The message on which interaction was used.
	// NOTE: this field is only filled when a button click triggered the interaction.
	// Otherwise it will be nil.
	Message *CtxMessage

	// The member who invoked this interaction.
	Member *CtxMember
	// The user who invoked this interaction (only in a DM; nil in a server).
	User *DiscordUser

	Token   string
	Version int
}

// CustomCommandInteraction is YAGPDB's .Interaction (context.go:286-290): the interaction
// with whether it has been responded to, and whether that response was a deferral that
// the run's output will edit. One pointer is shared with execCC children.
type CustomCommandInteraction struct {
	*Interaction
	RespondedTo bool
	Deferred    bool
}
