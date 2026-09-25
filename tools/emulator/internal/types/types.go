// Package types provides YAGPDB-compatible types for the emulator.
package types

import (
	"fmt"
	"reflect"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// SDict, Dict and Slice are YAGPDB's own container types (sdict, dict, cslice), with
// their methods, from internal/yagstd.
type (
	SDict = yagstd.SDict
	Dict  = yagstd.Dict
	Slice = yagstd.Slice
)

// ForStorage returns a value as the mock database keeps it: a deep copy, with maps and
// slices as SDict, Dict and Slice. YAGPDB serializes what dbSet stores, so changing the
// original afterwards doesn't change the database; the copy gives the emulator the same
// behavior. Maps from YAML or JSON become SDicts, or Dicts when they have non-string keys.
func ForStorage(v interface{}) interface{} {
	switch t := v.(type) {
	case SDict:
		return copySDict(t, ForStorage)
	case *SDict:
		if t == nil {
			return nil
		}
		return copySDict(*t, ForStorage)
	case map[string]interface{}:
		return copySDict(t, ForStorage)
	case Dict:
		return copyDict(t, ForStorage)
	case *Dict:
		if t == nil {
			return nil
		}
		return copyDict(*t, ForStorage)
	case map[interface{}]interface{}:
		// YAML gives this type for maps with non-string keys (a dict with int keys)
		allStrings := true
		for k := range t {
			if _, ok := k.(string); !ok {
				allStrings = false
				break
			}
		}
		if !allStrings {
			return copyDict(Dict(t), ForStorage)
		}
		out := make(SDict, len(t))
		for k, e := range t {
			out[k.(string)] = ForStorage(e)
		}
		return out
	case Slice:
		return copySlice(t, ForStorage)
	case *Slice:
		if t == nil {
			return nil
		}
		return copySlice(*t, ForStorage)
	}
	// Other slices ([]string from split, ...) are stored as plain arrays, which is how
	// YAGPDB's msgpack decoding returns them: []interface{}
	rv := reflect.ValueOf(v)
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) && rv.Type().Elem().Kind() != reflect.Uint8 {
		out := make([]interface{}, rv.Len())
		for i := range out {
			out[i] = ForStorage(rv.Index(i).Interface())
		}
		return out
	}
	return v
}

// ForTemplate returns a stored value the way YAGPDB's dbGet hands it to a template: a
// fresh copy in which every sdict, dict and cslice is a pointer (*SDict, *Dict, *Slice),
// as YAGPDB's msgpack decoding produces them. Changing it doesn't change the database
// until the template calls dbSet.
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
		out[k] = conv(e)
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

// AvatarURL returns the user's avatar URL.
func (u DiscordUser) AvatarURL(size ...string) string {
	if u.Avatar == "" {
		// Default avatar based on discriminator
		return fmt.Sprintf("https://cdn.discordapp.com/embed/avatars/%d.png", u.ID%5)
	}
	ext := "png"
	if len(u.Avatar) > 2 && u.Avatar[:2] == "a_" {
		ext = "gif"
	}
	s := "128"
	if len(size) > 0 {
		s = size[0]
	}
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%d/%s.%s?size=%s", u.ID, u.Avatar, ext, s)
}

// Mention returns the user mention string.
func (u DiscordUser) Mention() string {
	return fmt.Sprintf("<@%d>", u.ID)
}

// String returns the user's username.
func (u DiscordUser) String() string {
	return u.Username
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
}

// CtxGuild represents a Discord guild/server context.
type CtxGuild struct {
	ID          int64
	Name        string
	Icon        string
	OwnerID     int64
	MemberCount int
	Roles       []CtxRole
	Channels    []CtxChannel
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

// TemplateTime wraps time.Time with additional methods for YAGPDB template compatibility.
type TemplateTime struct {
	time.Time
}

// Parse returns the time itself (for YAGPDB method chaining compatibility).
// In YAGPDB, this is used to allow chaining like: $member.JoinedAt.Parse.Sub currentTime
func (t TemplateTime) Parse() time.Time {
	return t.Time
}

// CtxMember represents a Discord guild member.
type CtxMember struct {
	User     DiscordUser
	Nick     string
	Roles    []int64
	JoinedAt TemplateTime
}

// CtxMessage represents a Discord message.
type CtxMessage struct {
	ID              int64
	ChannelID       int64
	GuildID         int64
	Author          DiscordUser
	Content         string
	Timestamp       time.Time
	EditedTimestamp time.Time
	Attachments     []interface{}
	Embeds          []interface{}
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
}
