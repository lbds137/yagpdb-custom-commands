package types

import "fmt"

// The interaction types a template sees, copied from discordgo (vendor
// lib/discordgo/interactions.go, YAGPDB c579722) and YAGPDB's CustomCommandInteraction
// (common/templates/context.go:286-290), as far as the emulator models them: a message
// component (button or menu) click, a slash command, a user or message context menu
// entry (application commands), and a modal submission; and the modal response cmodal
// builds (docs/design/emulator-interactions.md).

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

// ModalSubmitInteractionData is the data of a modal submission (interactions.go:352-355):
// the modal's full custom ID (templates- prefix included) and its submitted rows, each
// holding a text input with its value.
type ModalSubmitInteractionData struct {
	CustomID   string              `json:"custom_id"`
	Components []TopLevelComponent `json:"components"`
}

// Type returns the type of interaction data (interactions.go:358-360).
func (ModalSubmitInteractionData) Type() InteractionType {
	return InteractionModalSubmit
}

// InteractionResponseType is type of interaction response (interactions.go:510-529).
type InteractionResponseType uint8

// Interaction response types.
const (
	InteractionResponsePong                             InteractionResponseType = 1
	InteractionResponseChannelMessageWithSource         InteractionResponseType = 4
	InteractionResponseDeferredChannelMessageWithSource InteractionResponseType = 5
	InteractionResponseDeferredMessageUpdate            InteractionResponseType = 6
	InteractionResponseUpdateMessage                    InteractionResponseType = 7
	InteractionApplicationCommandAutocompleteResult     InteractionResponseType = 8
	InteractionResponseModal                            InteractionResponseType = 9
)

// InteractionResponse is a response to an interaction (interactions.go:532-535): what
// cmodal returns, a modal response.
type InteractionResponse struct {
	Type InteractionResponseType  `json:"type,omitempty"`
	Data *InteractionResponseData `json:"data,omitempty"`
}

// InteractionResponseData is response data for an interaction (interactions.go:538-555),
// the fields a template can reach (File, Files and the autocomplete Choices aren't here;
// the emulator's embeds and allowed mentions stand in for discordgo's).
type InteractionResponseData struct {
	TTS             bool                `json:"tts"`
	Content         string              `json:"content"`
	Components      []TopLevelComponent `json:"components"`
	Embeds          []*MessageEmbed     `json:"embeds"`
	AllowedMentions *AllowedMentions    `json:"allowed_mentions,omitempty"`
	Flags           int                 `json:"flags,omitempty"`

	// NOTE: modal interaction only.

	CustomID string `json:"custom_id,omitempty"`
	Title    string `json:"title,omitempty"`
}

// ApplicationCommandType is the kind of application command (interactions.go:20-29): a
// slash command, or an entry of the user or message context menu.
type ApplicationCommandType uint8

// Application command types.
const (
	ChatApplicationCommand    ApplicationCommandType = 1
	UserApplicationCommand    ApplicationCommandType = 2
	MessageApplicationCommand ApplicationCommandType = 3
)

// ApplicationCommandOptionType is the type of a slash command option
// (interactions.go:50-64).
type ApplicationCommandOptionType uint8

// Application command option types.
const (
	ApplicationCommandOptionSubCommand  ApplicationCommandOptionType = 1
	ApplicationCommandOptionString      ApplicationCommandOptionType = 3
	ApplicationCommandOptionInteger     ApplicationCommandOptionType = 4
	ApplicationCommandOptionBoolean     ApplicationCommandOptionType = 5
	ApplicationCommandOptionUser        ApplicationCommandOptionType = 6
	ApplicationCommandOptionChannel     ApplicationCommandOptionType = 7
	ApplicationCommandOptionRole        ApplicationCommandOptionType = 8
	ApplicationCommandOptionMentionable ApplicationCommandOptionType = 9
	ApplicationCommandOptionNumber      ApplicationCommandOptionType = 10
)

// ApplicationCommandInteractionData is the data of a slash command or context menu
// interaction (interactions.go:302-318): the command's name and type, its options (a
// subcommand's nested under it), a context menu's target, and what Discord resolved.
type ApplicationCommandInteractionData struct {
	ID          int64                                      `json:"id,string"`
	Name        string                                     `json:"name"`
	CommandType ApplicationCommandType                     `json:"type"`
	Resolved    *ApplicationCommandInteractionDataResolved `json:"resolved"`
	GuildID     int64                                      `json:"guild_id,string"`

	// Slash command options
	Options []*ApplicationCommandInteractionDataOption `json:"options"`
	// Target (user/message) id on which context menu command was called.
	// The details are stored in Resolved according to command type.
	TargetID int64 `json:"target_id,string"`
}

// Type returns the type of interaction data (interactions.go:333-335).
func (ApplicationCommandInteractionData) Type() InteractionType {
	return InteractionApplicationCommand
}

// ApplicationCommandInteractionDataResolved is what Discord resolved for the snowflakes
// an interaction names (interactions.go:323-330), as the emulator knows them.
type ApplicationCommandInteractionDataResolved struct {
	Users    map[int64]*DiscordUser `json:"users"`
	Members  map[int64]*CtxMember   `json:"members"`
	Roles    map[int64]*CtxRole     `json:"roles"`
	Channels map[int64]*CtxChannel  `json:"channels"`
	Messages map[int64]*CtxMessage  `json:"messages"`
}

// ApplicationCommandInteractionDataOption is one option of a slash command interaction
// (interactions.go:388-397). Value is typed by Type as discordgo decodes it
// (structs.go:1708-1750): string, int64 (integer and every snowflake type), float64
// (number) or bool; a subcommand carries its options in Options instead.
type ApplicationCommandInteractionDataOption struct {
	Name    string                                     `json:"name"`
	Type    ApplicationCommandOptionType               `json:"type"`
	Value   interface{}                                `json:"value,omitempty"`
	Options []*ApplicationCommandInteractionDataOption `json:"options,omitempty"`
}

// Interaction represents data of an interaction (interactions.go:185-220) with the
// fields the emulator fills. Message is the component's message (for a modal submission,
// the message whose component opened the modal, if any) and Member the clicker, as
// Discord sends them (before the handler makes the clicker .Message's author).
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

	// DataCommand is Data again, as the pointer discordgo fills for every application
	// command interaction (interactions.go:217, :247); nil for a component click.
	DataCommand *ApplicationCommandInteractionData
}

// CustomCommandInteraction is YAGPDB's .Interaction (context.go:286-290): the interaction
// with whether it has been responded to, and whether that response was a deferral that
// the run's output will edit. One pointer is shared with execCC children.
type CustomCommandInteraction struct {
	*Interaction
	RespondedTo bool
	Deferred    bool
}
