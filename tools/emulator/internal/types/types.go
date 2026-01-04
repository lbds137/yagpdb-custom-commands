// Package types provides YAGPDB-compatible types for the emulator.
package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// SDict is a string-keyed dictionary, matching YAGPDB's SDict type.
type SDict map[string]interface{}

// Set sets a key-value pair and returns an empty string (for template compatibility).
func (s SDict) Set(key string, value interface{}) string {
	s[key] = value
	return ""
}

// Get retrieves a value by key, returning nil if not found.
// Accepts interface{} for template compatibility.
func (s SDict) Get(key interface{}) interface{} {
	return s[fmt.Sprint(key)]
}

// Del deletes a key and returns an empty string.
func (s SDict) Del(key string) string {
	delete(s, key)
	return ""
}

// HasKey returns true if the key exists.
func (s SDict) HasKey(key string) bool {
	_, ok := s[key]
	return ok
}

// Dict is a dictionary with any key type, matching YAGPDB's Dict type.
type Dict map[interface{}]interface{}

// Set sets a key-value pair and returns an empty string.
func (d Dict) Set(key, value interface{}) string {
	d[key] = value
	return ""
}

// Get retrieves a value by key, returning nil if not found.
func (d Dict) Get(key interface{}) interface{} {
	return d[key]
}

// Del deletes a key and returns an empty string.
func (d Dict) Del(key interface{}) string {
	delete(d, key)
	return ""
}

// HasKey returns true if the key exists.
func (d Dict) HasKey(key interface{}) bool {
	_, ok := d[key]
	return ok
}

// Slice is a dynamic slice type, matching YAGPDB's Slice type.
type Slice []interface{}

// Append adds an item and returns the new slice.
func (s Slice) Append(item interface{}) Slice {
	return append(s, item)
}

// AppendSlice appends another slice and returns the new slice.
func (s Slice) AppendSlice(other Slice) Slice {
	return append(s, other...)
}

// Set sets an item at an index and returns an empty string.
func (s Slice) Set(index int, value interface{}) (string, error) {
	if index < 0 || index >= len(s) {
		return "", fmt.Errorf("index out of range: %d", index)
	}
	s[index] = value
	return "", nil
}

// StringSlice converts the slice to []string.
func (s Slice) StringSlice(strict ...bool) ([]string, error) {
	isStrict := len(strict) > 0 && strict[0]
	result := make([]string, len(s))
	for i, v := range s {
		if str, ok := v.(string); ok {
			result[i] = str
		} else if isStrict {
			return nil, fmt.Errorf("element %d is not a string", i)
		} else {
			result[i] = fmt.Sprint(v)
		}
	}
	return result, nil
}

// TemplateValue wraps any value and provides Get/Set methods for template access.
// This ensures that when templates call .Value.Get, the method is always available.
type TemplateValue struct {
	V interface{}
}

// Get retrieves a value from the wrapped dict-like type.
// Returns a TemplateValue for dict-like results to enable method chaining.
func (tv TemplateValue) Get(key interface{}) interface{} {
	var result interface{}
	switch v := tv.V.(type) {
	case SDict:
		result = v.Get(key)
	case map[string]interface{}:
		result = v[fmt.Sprint(key)]
	case Dict:
		result = v.Get(key)
	case map[interface{}]interface{}:
		result = v[key]
	default:
		return tv.V // Return the raw value if not a dict type
	}
	// Wrap dict-like results in TemplateValue for method chaining
	switch result.(type) {
	case SDict, map[string]interface{}, Dict, map[interface{}]interface{}:
		return TemplateValue{V: result}
	default:
		return result
	}
}

// Set sets a value in the wrapped dict-like type.
func (tv TemplateValue) Set(key interface{}, value interface{}) string {
	switch v := tv.V.(type) {
	case SDict:
		return v.Set(fmt.Sprint(key), value)
	case map[string]interface{}:
		v[fmt.Sprint(key)] = value
	case Dict:
		return v.Set(key, value)
	case map[interface{}]interface{}:
		v[key] = value
	}
	return ""
}

// HasKey returns true if the key exists in the wrapped dict-like type.
func (tv TemplateValue) HasKey(key interface{}) bool {
	switch v := tv.V.(type) {
	case SDict:
		return v.HasKey(fmt.Sprint(key))
	case map[string]interface{}:
		_, ok := v[fmt.Sprint(key)]
		return ok
	case Dict:
		return v.HasKey(key)
	case map[interface{}]interface{}:
		_, ok := v[key]
		return ok
	default:
		return false
	}
}

// LightDBEntry represents a database entry, matching YAGPDB's LightDBEntry.
type LightDBEntry struct {
	ID        int64
	GuildID   int64
	UserID    int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Key       string
	Value     TemplateValue
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

// StringKeyDictionary creates an SDict from key-value pairs.
func StringKeyDictionary(pairs ...interface{}) (SDict, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("sdict requires an even number of arguments")
	}
	result := make(SDict)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("sdict keys must be strings, got %T", pairs[i])
		}
		result[key] = pairs[i+1]
	}
	return result, nil
}

// Dictionary creates a Dict from key-value pairs.
func Dictionary(pairs ...interface{}) (Dict, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict requires an even number of arguments")
	}
	result := make(Dict)
	for i := 0; i < len(pairs); i += 2 {
		result[pairs[i]] = pairs[i+1]
	}
	return result, nil
}

// CreateSlice creates a Slice from the given items.
func CreateSlice(items ...interface{}) Slice {
	return Slice(items)
}

// JSONToSDict parses a JSON string into an SDict.
func JSONToSDict(jsonStr string) (SDict, error) {
	var result SDict
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return result, nil
}

// ToJSON converts a value to a JSON string.
func ToJSON(v interface{}, pretty ...bool) (string, error) {
	var data []byte
	var err error
	if len(pretty) > 0 && pretty[0] {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}
