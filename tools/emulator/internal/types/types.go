// Package types provides YAGPDB-compatible types for the emulator.
package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	yagstd "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// SDict, Dict and Slice are YAGPDB's own container types (sdict, dict, cslice), with
// their methods, from internal/yagstd.
type (
	SDict = yagstd.SDict
	Dict  = yagstd.Dict
	Slice = yagstd.Slice
)

// ForStorage returns a value as the mock database keeps it: a deep copy with the types
// YAGPDB's msgpack round trip (v4, sdict/dict/cslice registered as extensions) gives back.
// Changing the original afterwards doesn't change the database.
//   - SDict, Dict and Slice (and pointers to them) stay those types
//   - other maps stay plain maps (map[string]interface{} when every key is a string)
//   - other slices and arrays become []interface{}
//   - signed integers of every kind (time.Duration too) become int64, unsigned uint64
//   - times come back in the local time zone
//
// Map keys are converted the same way, so a dict keyed by int is keyed by int64.
func ForStorage(v interface{}) interface{} {
	switch t := v.(type) {
	case nil:
		return nil
	case SDict:
		return copySDict(t, ForStorage)
	case *SDict:
		if t == nil {
			return nil
		}
		return copySDict(*t, ForStorage)
	case Dict:
		return copyDict(t, ForStorage)
	case *Dict:
		if t == nil {
			return nil
		}
		return copyDict(*t, ForStorage)
	case Slice:
		return copySlice(t, ForStorage)
	case *Slice:
		if t == nil {
			return nil
		}
		return copySlice(*t, ForStorage)
	case time.Time:
		return t.Local()
	case *time.Time:
		if t == nil {
			return nil
		}
		return t.Local()
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint()
	case reflect.Map:
		allStrings := true
		for _, k := range rv.MapKeys() {
			if k.Kind() != reflect.String && !(k.Kind() == reflect.Interface && k.Elem().Kind() == reflect.String) {
				allStrings = false
				break
			}
		}
		if allStrings {
			out := make(map[string]interface{}, rv.Len())
			for it := rv.MapRange(); it.Next(); {
				out[fmt.Sprint(it.Key().Interface())] = ForStorage(it.Value().Interface())
			}
			return out
		}
		out := make(map[interface{}]interface{}, rv.Len())
		for it := rv.MapRange(); it.Next(); {
			out[ForStorage(it.Key().Interface())] = ForStorage(it.Value().Interface())
		}
		return out
	case reflect.Slice, reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return v // []byte is stored as bytes
		}
		out := make([]interface{}, rv.Len())
		for i := range out {
			out[i] = ForStorage(rv.Index(i).Interface())
		}
		return out
	}
	return v
}

// FixtureForStorage stores a value from a YAML or JSON test fixture. Fixtures stand in
// for data a command saved with sdict/dict, so their maps become SDicts, or Dicts when
// a key isn't a string; the rest is ForStorage.
func FixtureForStorage(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := make(SDict, len(t))
		for k, e := range t {
			out[k] = FixtureForStorage(e)
		}
		return ForStorage(out)
	case map[interface{}]interface{}:
		allStrings := true
		for k := range t {
			if _, ok := k.(string); !ok {
				allStrings = false
				break
			}
		}
		if allStrings {
			out := make(SDict, len(t))
			for k, e := range t {
				out[k.(string)] = FixtureForStorage(e)
			}
			return ForStorage(out)
		}
		out := make(Dict, len(t))
		for k, e := range t {
			out[k] = FixtureForStorage(e)
		}
		return ForStorage(out)
	case []interface{}:
		out := make(Slice, len(t))
		for i, e := range t {
			out[i] = FixtureForStorage(e)
		}
		return ForStorage(out)
	}
	return ForStorage(v)
}

// ForTemplate returns a stored value the way YAGPDB's dbGet hands it to a template: a
// fresh copy in which every sdict, dict and cslice is a pointer (*SDict, *Dict, *Slice)
// and times are *time.Time, as msgpack's extension decoding produces them. Changing it
// doesn't change the database until the template calls dbSet.
func ForTemplate(v interface{}) interface{} {
	switch t := v.(type) {
	case SDict:
		c := copySDict(t, ForTemplate)
		return &c
	case Dict:
		c := copyDict(t, ForTemplate)
		return &c
	case Slice:
		c := copySlice(t, ForTemplate)
		return &c
	case time.Time:
		return &t
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, e := range t {
			out[k] = ForTemplate(e)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[interface{}]interface{}, len(t))
		for k, e := range t {
			out[k] = ForTemplate(e)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, e := range t {
			out[i] = ForTemplate(e)
		}
		return out
	}
	return v
}

func copySDict(in map[string]interface{}, conv func(interface{}) interface{}) SDict {
	out := make(SDict, len(in))
	for k, e := range in {
		out[k] = conv(e)
	}
	return out
}

func copyDict(in Dict, conv func(interface{}) interface{}) Dict {
	out := make(Dict, len(in))
	for k, e := range in {
		out[ForStorage(k)] = conv(e)
	}
	return out
}

func copySlice(in []interface{}, conv func(interface{}) interface{}) Slice {
	out := make(Slice, len(in))
	for i, e := range in {
		out[i] = conv(e)
	}
	return out
}

// LightDBEntry represents a database entry, matching YAGPDB's LightDBEntry.
type LightDBEntry struct {
	ID        int64
	GuildID   int64
	UserID    int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Key       string
	Value     interface{}
	ValueSize int
	User      DiscordUser
	ExpiresAt time.Time
}

// DiscordUser represents a minimal Discord user for database entries.
type DiscordUser struct {
	ID            int64
	Username      string
	Discriminator string
	Avatar        string
	Bot           bool
}

// AvatarURL is discordgo's User.AvatarURL: size is required, and "" adds no size.
func (u DiscordUser) AvatarURL(size string) string {
	var URL string
	if u.Avatar == "" {
		// "For users on the new username system, `index` will be `(user_id >> 22) % 6`.
		// For users on the legacy username system, `index` will be `discriminator % 5`."
		var index int
		if u.Discriminator == "0" {
			index = int((u.ID >> 22) % 6)
		} else {
			discrim, _ := strconv.Atoi(u.Discriminator)
			index = discrim % 5
		}
		URL = "https://cdn.discordapp.com/embed/avatars/" + strconv.Itoa(index) + ".png"
	} else if strings.HasPrefix(u.Avatar, "a_") {
		URL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%d/%s.gif", u.ID, u.Avatar)
	} else {
		URL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%d/%s.png", u.ID, u.Avatar)
	}

	if size != "" {
		return URL + "?size=" + size
	}
	return URL
}

// Mention returns the user mention string.
func (u DiscordUser) Mention() string {
	return fmt.Sprintf("<@%d>", u.ID)
}

// String is discordgo's User.String: username#discriminator, or the username alone on the
// new username system (discriminator "0").
func (u DiscordUser) String() string {
	if u.Discriminator == "0" {
		return u.Username
	}
	return fmt.Sprintf("%s#%s", u.Username, u.Discriminator)
}

// CtxChannel represents a Discord channel context.
type CtxChannel struct {
	ID        int64
	GuildID   int64
	Name      string
	Topic     string
	NSFW      bool
	Position  int
	ParentID  int64
	IsPrivate bool
	IsThread  bool
	IsForum   bool
	Type      int // discordgo.ChannelType: 0 text, 2 voice, 4 category, 5 announcement, 15 forum
}

// CtxGuild represents a Discord guild/server context.
type CtxGuild struct {
	ID          int64
	Name        string
	Icon        string
	OwnerID     int64
	MemberCount int
	Roles       []CtxRole
	Channels    []ChannelState
}

// ChannelState is dstate.ChannelState, what YAGPDB's .Guild.Channels holds: unlike
// .Channel's CtxChannel it has no IsThread or IsForum, so reading those is an error, as
// in production, and IsPrivate is a method. Its thread, permission and forum fields
// aren't modelled.
type ChannelState struct {
	ID               int64
	GuildID          int64
	Name             string
	Topic            string
	Type             int
	NSFW             bool
	Icon             string
	Position         int
	Bitrate          int
	UserLimit        int
	ParentID         int64
	RateLimitPerUser int
	Flags            int
	OwnerID          int64
}

// Mention is YAGPDB's CtxChannel.Mention, a pointer receiver as there.
func (c *CtxChannel) Mention() (string, error) {
	if c == nil {
		return "", errors.New("channel not found")
	}
	return "<#" + strconv.FormatInt(c.ID, 10) + ">", nil
}

// IsPrivate and Mention are dstate.ChannelState's, pointer receivers included.
func (c *ChannelState) IsPrivate() bool {
	return c.Type == 1 || c.Type == 3 // discordgo.ChannelTypeDM, ChannelTypeGroupDM
}

func (c *ChannelState) Mention() (string, error) {
	if c == nil {
		return "", errors.New("channel not found")
	}
	return "<#" + strconv.FormatInt(c.ID, 10) + ">", nil
}

// State is the channel as .Guild.Channels holds it.
func (c CtxChannel) State() ChannelState {
	return ChannelState{ID: c.ID, GuildID: c.GuildID, Name: c.Name, Topic: c.Topic,
		Type: c.Type, NSFW: c.NSFW, Position: c.Position, ParentID: c.ParentID}
}

// GetRole returns a role by ID, or nil if not found.
func (g CtxGuild) GetRole(roleID interface{}) *CtxRole {
	var id int64
	switch v := roleID.(type) {
	case int64:
		id = v
	case int:
		id = int64(v)
	case string:
		// Try to parse as int
		for _, r := range g.Roles {
			if fmt.Sprint(r.ID) == v {
				return &r
			}
		}
		return nil
	default:
		return nil
	}

	for _, role := range g.Roles {
		if role.ID == id {
			return &role
		}
	}
	return nil
}

// CtxRole represents a Discord role.
type CtxRole struct {
	ID          int64
	Name        string
	Color       int
	Permissions int64
	Position    int
	Mentionable bool
	Managed     bool
}

// Timestamp is discordgo's Timestamp: an RFC 3339 string, as Discord sends it.
type Timestamp string

// Parse parses a timestamp string into a time.Time object.
// The only time this can fail is if Discord changes their timestamp format.
func (t Timestamp) Parse() (time.Time, error) {
	tim, err := time.Parse(time.RFC3339, string(t))
	return tim.UTC(), err
}

// NewTimestamp is the Timestamp Discord would send for t (2021-01-01T00:00:00.000000+00:00).
func NewTimestamp(t time.Time) Timestamp {
	return Timestamp(t.UTC().Format("2006-01-02T15:04:05.000000-07:00"))
}

// CtxMember represents a Discord guild member.
type CtxMember struct {
	User     DiscordUser
	Nick     string
	Roles    []int64
	JoinedAt Timestamp
}

// CtxMessage represents a Discord message.
type CtxMessage struct {
	ID              int64
	ChannelID       int64
	GuildID         int64
	Author          DiscordUser
	Content         string
	Timestamp       Timestamp
	EditedTimestamp Timestamp // "" until edited
	Attachments     []interface{}
	Embeds          []*MessageEmbed // as discordgo.Message holds them
}

// Link is discordgo's Message.Link. A value receiver, so it works on the mocks' values and
// pointers alike.
func (m CtxMessage) Link() string {
	return fmt.Sprintf("https://discord.com/channels/%v/%v/%v", m.GuildID, m.ChannelID, m.ID)
}

// CtxReaction mirrors discordgo.MessageReaction, the .Reaction of reaction-triggered commands.
type CtxReaction struct {
	UserID    int64
	MessageID int64
	ChannelID int64
	GuildID   int64
	Emoji     CtxEmoji
}

// CtxEmoji mirrors discordgo.Emoji.
type CtxEmoji struct {
	ID       int64
	Name     string
	Animated bool
}

// APIName returns the emoji as Discord's API names it: "name:id" for custom emoji,
// the character itself for Unicode emoji.
func (e CtxEmoji) APIName() string {
	if e.ID != 0 && e.Name != "" {
		return fmt.Sprintf("%s:%d", e.Name, e.ID)
	}
	if e.Name != "" {
		return e.Name
	}
	return fmt.Sprint(e.ID)
}

// MessageSend stands in for the *discordgo.MessageSend that YAGPDB's complexMessage builds.
type MessageSend struct {
	Content  string
	Embeds   []interface{} // each as cembed built it
	File     string        // attached file contents, if any
	Filename string        // with YAGPDB's forced .txt extension
	HasFile  bool
	// HasOther is set by keys that make a message non-empty without content, embeds or a
	// file: buttons, menus, components, a sticker or a forward (not otherwise modelled).
	HasOther bool
	// AllowedMentions says which mentions in Content ping; complexMessage allows users
	AllowedMentions AllowedMentions
	// ReplyTo is complexMessage's "reply": the ID of the message this replies to, in the
	// channel it's sent to
	ReplyTo int64
}

// AllowedMentions stands in for discordgo.AllowedMentions: Parse holds "users", "roles"
// and "everyone", and Users and Roles allow single IDs.
type AllowedMentions struct {
	Parse       []string
	Users       []int64
	Roles       []int64
	RepliedUser bool
}

// MessageEdit is what complexMessageEdit builds (YAGPDB's CreateMessageEdit): only the
// fields it sets change. Content is nil when the edit leaves it alone.
type MessageEdit struct {
	Content  *string
	Embeds   []interface{} // each as cembed built it; nil leaves the embeds alone
	HasOther bool          // components, buttons or menus (not otherwise modelled)
	// ComponentsV2 is the is_components_v2 flag, which skips YAGPDB's empty-edit check
	ComponentsV2 bool
}

// Embed is what cembed builds: the dict after YAGPDB's conversion to a Discord embed
// (CreateEmbed marshals it to JSON and decodes it into discordgo.MessageEmbed). Unknown
// keys are dropped and wrong value types are errors, as in production.
type Embed map[string]interface{}

// BuildEmbed converts a dict to an Embed the way YAGPDB's CreateEmbed does.
func BuildEmbed(m map[string]interface{}) (Embed, error) {
	encoded, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var embed MessageEmbed
	if err := json.Unmarshal(encoded, &embed); err != nil {
		return nil, err
	}
	normalized, _ := json.Marshal(embed)
	var out Embed
	if err := json.Unmarshal(normalized, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = Embed{}
	}
	return out, nil
}

// MessageEmbed is discordgo.MessageEmbed (lib/discordgo/message.go): the embeds of a
// message read back (getMessage, .Message) are these, so templates use its field names
// (.Title, .Author.Name, .Fields). BuildEmbed decodes through it too.
type MessageEmbed struct {
	URL         string                 `json:"url,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Timestamp   string                 `json:"timestamp,omitempty"`
	Color       int                    `json:"color,omitempty"`
	Footer      *MessageEmbedFooter    `json:"footer,omitempty"`
	Image       *MessageEmbedImage     `json:"image,omitempty"`
	Thumbnail   *MessageEmbedThumbnail `json:"thumbnail,omitempty"`
	Video       *MessageEmbedVideo     `json:"video,omitempty"`
	Provider    *MessageEmbedProvider  `json:"provider,omitempty"`
	Author      *MessageEmbedAuthor    `json:"author,omitempty"`
	Fields      []*MessageEmbedField   `json:"fields,omitempty"`
}

type MessageEmbedFooter struct {
	Text         string `json:"text,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

type MessageEmbedImage struct {
	URL      string `json:"url,omitempty"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type MessageEmbedThumbnail struct {
	URL      string `json:"url,omitempty"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type MessageEmbedVideo struct {
	URL      string `json:"url,omitempty"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type MessageEmbedProvider struct {
	URL  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
}

type MessageEmbedAuthor struct {
	URL          string `json:"url,omitempty"`
	Name         string `json:"name,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

type MessageEmbedField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

// These conversions are functions, not methods: a template sees an embed's methods, and
// discordgo.MessageEmbed has none.

// EmbedStruct is the embed as a message holds it (see MessageEmbed).
func EmbedStruct(e Embed) *MessageEmbed {
	var out MessageEmbed
	if data, err := json.Marshal(e); err == nil {
		json.Unmarshal(data, &out) // e came from BuildEmbed, so it has MessageEmbed's shape
	}
	return &out
}

// EmbedMap is the embed as cembed builds it (see Embed).
func EmbedMap(m *MessageEmbed) Embed {
	out := Embed{}
	if data, err := json.Marshal(m); err == nil {
		json.Unmarshal(data, &out)
	}
	return out
}

// EmbedStructs is a message's embeds from what cembed built (Embed values). Empty ones
// are dropped, as discordgo drops them before sending (ValidateComplexMessageEmbeds).
func EmbedStructs(embeds []interface{}) []*MessageEmbed {
	var out []*MessageEmbed
	for _, x := range embeds {
		if e, ok := x.(Embed); ok && len(e) > 0 {
			out = append(out, EmbedStruct(e))
		}
	}
	return out
}

// EmbedMaps is the reverse of EmbedStructs.
func EmbedMaps(embeds []*MessageEmbed) []interface{} {
	var out []interface{}
	for _, m := range embeds {
		out = append(out, EmbedMap(m))
	}
	return out
}
