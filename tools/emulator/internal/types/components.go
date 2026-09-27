package types

import "encoding/json"

// The message component types a template can build with cbutton and cmenu, copied from
// discordgo (vendor lib/discordgo/components.go, YAGPDB c579722) with their field names
// and JSON tags, so a message read back and a snapshot show Discord's shape. Components
// V2 (sections, containers, ...) isn't modelled.

// ComponentType is type of component (components.go:12-36; only the types the emulator
// builds are listed).
type ComponentType uint

// MessageComponent types.
const (
	ActionsRowComponent            ComponentType = 1
	ButtonComponent                ComponentType = 2
	SelectMenuComponent            ComponentType = 3
	UserSelectMenuComponent        ComponentType = 5
	RoleSelectMenuComponent        ComponentType = 6
	MentionableSelectMenuComponent ComponentType = 7
	ChannelSelectMenuComponent     ComponentType = 8
)

// MessageComponent is a base interface for all message components (components.go:39-42).
type MessageComponent interface {
	json.Marshaler
	Type() ComponentType
}

// TopLevelComponent is an interface for message components which can be used on the
// top level of a message (components.go:112-116).
type TopLevelComponent interface {
	MessageComponent
	IsTopLevel() bool
}

// InteractiveComponent is an interface for message components which can be interacted
// with (components.go:119-123).
type InteractiveComponent interface {
	MessageComponent
	IsInteractive() bool
}

// ActionsRow is a container for interactive components within one row
// (components.go:148-150).
type ActionsRow struct {
	Components []InteractiveComponent `json:"components"`
}

// MarshalJSON is a method for marshaling ActionsRow to a JSON object (components.go:153).
func (r ActionsRow) MarshalJSON() ([]byte, error) {
	type actionsRow ActionsRow

	return json.Marshal(struct {
		actionsRow
		Type ComponentType `json:"type"`
	}{
		actionsRow: actionsRow(r),
		Type:       r.Type(),
	})
}

// Type is a method to get the type of a component (components.go:188).
func (r ActionsRow) Type() ComponentType {
	return ActionsRowComponent
}

// IsTopLevel is a method to assert the component as top level (components.go:193).
func (ActionsRow) IsTopLevel() bool {
	return true
}

// ButtonStyle is style of button (components.go:203).
type ButtonStyle uint

// Button styles (components.go:206-218).
const (
	// PrimaryButton is a button with blurple color.
	PrimaryButton ButtonStyle = 1
	// SecondaryButton is a button with grey color.
	SecondaryButton ButtonStyle = 2
	// SuccessButton is a button with green color.
	SuccessButton ButtonStyle = 3
	// DangerButton is a button with red color.
	DangerButton ButtonStyle = 4
	// LinkButton is a special type of button which navigates to a URL. Has grey color.
	LinkButton ButtonStyle = 5
)

// ComponentEmoji represents button emoji, if it does have one (components.go:220-224).
type ComponentEmoji struct {
	Name     string `json:"name,omitempty"`
	ID       int64  `json:"id,string,omitempty"`
	Animated bool   `json:"animated,omitempty"`
}

// Button represents button component (components.go:227-236).
type Button struct {
	Label    string          `json:"label"`
	Style    ButtonStyle     `json:"style"`
	Disabled bool            `json:"disabled"`
	Emoji    *ComponentEmoji `json:"emoji,omitempty"`

	// NOTE: Only button with LinkButton style can have link. Also, URL is mutually
	// exclusive with CustomID.
	URL      string `json:"url,omitempty"`
	CustomID string `json:"custom_id,omitempty"`
}

// MarshalJSON is a method for marshaling Button to a JSON object (components.go:239-253):
// a button without a style is a primary one.
func (b Button) MarshalJSON() ([]byte, error) {
	type button Button

	if b.Style == 0 {
		b.Style = PrimaryButton
	}

	return json.Marshal(struct {
		button
		Type ComponentType `json:"type"`
	}{
		button: button(b),
		Type:   b.Type(),
	})
}

// Type is a method to get the type of a component (components.go:256).
func (Button) Type() ComponentType {
	return ButtonComponent
}

// IsInteractive is a method to assert the component as interactive (components.go:261).
func (Button) IsInteractive() bool {
	return true
}

// SelectMenuOption represents an option for a select menu (components.go:275-282).
type SelectMenuOption struct {
	Label       string          `json:"label,omitempty"`
	Value       string          `json:"value"`
	Description string          `json:"description"`
	Emoji       *ComponentEmoji `json:"emoji,omitempty"`
	// Determines whenever option is selected by default or not.
	Default bool `json:"default"`
}

// SelectMenuDefaultValueType represents the type of an entity selected by default in
// auto-populated select menus (components.go:285).
type SelectMenuDefaultValueType string

// SelectMenuDefaultValue types (components.go:288-292).
const (
	SelectMenuDefaultValueUser    SelectMenuDefaultValueType = "user"
	SelectMenuDefaultValueRole    SelectMenuDefaultValueType = "role"
	SelectMenuDefaultValueChannel SelectMenuDefaultValueType = "channel"
)

// SelectMenuDefaultValue represents an entity selected by default in auto-populated
// select menus (components.go:295-300).
type SelectMenuDefaultValue struct {
	// ID of the entity.
	ID string `json:"id"`
	// Type of the entity.
	Type SelectMenuDefaultValueType `json:"type"`
}

// SelectMenuType represents select menu type (components.go:303).
type SelectMenuType ComponentType

// SelectMenu types (components.go:306-312).
const (
	StringSelectMenu      = SelectMenuType(SelectMenuComponent)
	UserSelectMenu        = SelectMenuType(UserSelectMenuComponent)
	RoleSelectMenu        = SelectMenuType(RoleSelectMenuComponent)
	MentionableSelectMenu = SelectMenuType(MentionableSelectMenuComponent)
	ChannelSelectMenu     = SelectMenuType(ChannelSelectMenuComponent)
)

// SelectMenu represents select menu component (components.go:315-340). ChannelTypes are
// discordgo.ChannelType values (ints; the emulator has no type for them).
type SelectMenu struct {
	// Type of the select menu.
	MenuType SelectMenuType `json:"type,omitempty"`
	// CustomID is a developer-defined identifier for the select menu.
	CustomID string `json:"custom_id,omitempty"`
	// The text which will be shown in the menu if there's no default options or all
	// options was deselected and component was closed.
	Placeholder string `json:"placeholder"`
	// This value determines the minimal amount of selected items in the menu.
	MinValues *int `json:"min_values,omitempty"`
	// This value determines the maximal amount of selected items in the menu.
	// If MaxValues or MinValues are greater than one then the user can select multiple
	// items in the component.
	MaxValues int `json:"max_values,omitempty"`
	// List of default values for auto-populated select menus.
	// NOTE: Number of entries should be in the range defined by MinValues and MaxValues.
	DefaultValues []SelectMenuDefaultValue `json:"default_values,omitempty"`

	// Values is a list of values selected by the user, only filled when the select menu
	// is submitted.
	Values []string `json:"values,omitempty"`

	Options  []SelectMenuOption `json:"options,omitempty"`
	Disabled bool               `json:"disabled"`
	Required bool               `json:"required"`

	// NOTE: Can only be used in SelectMenu with Channel menu type.
	ChannelTypes []int `json:"channel_types,omitempty"`
}

// Type is a method to get the type of a component (components.go:343-348).
func (s SelectMenu) Type() ComponentType {
	if s.MenuType != 0 {
		return ComponentType(s.MenuType)
	}
	return SelectMenuComponent
}

// MarshalJSON is a method for marshaling SelectMenu to a JSON object
// (components.go:355-366).
func (s SelectMenu) MarshalJSON() ([]byte, error) {
	type selectMenu SelectMenu

	return json.Marshal(struct {
		selectMenu
		Type ComponentType `json:"type"`
	}{
		selectMenu: selectMenu(s),
		Type:       s.Type(),
	})
}

// IsInteractive is a method to assert the component as interactive (components.go:369).
func (SelectMenu) IsInteractive() bool {
	return true
}

// CloneComponents copies a message's action rows and their buttons and menus, so a
// message fetched with getMessage (a fresh fetch from Discord in YAGPDB) doesn't share
// them with the stored message. Rows and components of other types are kept as they are.
func CloneComponents(rows []TopLevelComponent) []TopLevelComponent {
	if rows == nil {
		return nil
	}
	out := make([]TopLevelComponent, 0, len(rows))
	for _, row := range rows {
		r, ok := row.(*ActionsRow)
		if !ok {
			out = append(out, row)
			continue
		}
		clone := &ActionsRow{Components: make([]InteractiveComponent, 0, len(r.Components))}
		for _, c := range r.Components {
			switch t := c.(type) {
			case *Button:
				b := *t
				if t.Emoji != nil {
					emoji := *t.Emoji
					b.Emoji = &emoji
				}
				clone.Components = append(clone.Components, &b)
			case *SelectMenu:
				m := *t
				if t.MinValues != nil {
					min := *t.MinValues
					m.MinValues = &min
				}
				m.Options = append([]SelectMenuOption(nil), t.Options...)
				m.DefaultValues = append([]SelectMenuDefaultValue(nil), t.DefaultValues...)
				m.Values = append([]string(nil), t.Values...)
				m.ChannelTypes = append([]int(nil), t.ChannelTypes...)
				clone.Components = append(clone.Components, &m)
			default:
				clone.Components = append(clone.Components, c)
			}
		}
		out = append(out, clone)
	}
	return out
}
