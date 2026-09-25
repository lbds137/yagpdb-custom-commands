// Package state provides mock implementations of YAGPDB's stateful services.
package state

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// MockDB provides an in-memory implementation of YAGPDB's database. Like YAGPDB's
// templates_user_database table, each entry has a raw value (entries[k].Value) and a
// numeric column (valueNums[k]): dbSet writes both, dbIncr changes only value_num, and
// reading an entry whose raw value is a number gives value_num.
type MockDB struct {
	mu        sync.RWMutex
	entries   map[string]*types.LightDBEntry
	valueNums map[string]float64
	guildID   int64
	nextID    int64
}

// NewMockDB creates a new mock database for the given guild.
func NewMockDB(guildID int64) *MockDB {
	return &MockDB{
		entries:   make(map[string]*types.LightDBEntry),
		valueNums: make(map[string]float64),
		guildID:   guildID,
		nextID:    1,
	}
}

// view is an entry as YAGPDB's ToLightDBEntry reads it: a raw number reads as value_num.
func (m *MockDB) view(e *types.LightDBEntry) *types.LightDBEntry {
	if !isNumber(e.Value) {
		return e
	}
	c := *e
	c.Value = m.valueNums[makeKey(e.UserID, e.Key)]
	return &c
}

func expired(e *types.LightDBEntry, now time.Time) bool {
	return !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt)
}

// makeKey creates a composite key from userID and key name.
func makeKey(userID int64, key string) string {
	return fmt.Sprintf("%d:%s", userID, key)
}

// Get retrieves a database entry, returning nil if not found or expired.
func (m *MockDB) Get(userID int64, key string) *types.LightDBEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	compositeKey := makeKey(userID, key)
	entry, ok := m.entries[compositeKey]
	if !ok {
		return nil
	}

	if expired(entry, time.Now()) {
		return nil
	}

	return m.view(entry)
}

// Set stores a value in the database. It fails, storing nothing, if the value doesn't
// serialize within YAGPDB's limit.
func (m *MockDB) Set(userID int64, key string, value interface{}) (*types.LightDBEntry, error) {
	return m.SetWithExpiry(userID, key, value, 0)
}

// SetWithExpiry stores a value with an expiration time (in seconds, 0 = no expiry).
func (m *MockDB) SetWithExpiry(userID int64, key string, value interface{}, ttlSeconds int) (*types.LightDBEntry, error) {
	serialized, err := serializeValue(value)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	compositeKey := makeKey(userID, key)
	now := time.Now()

	var expiresAt time.Time
	if ttlSeconds > 0 {
		expiresAt = now.Add(time.Duration(ttlSeconds) * time.Second)
	}

	// Check if entry already exists
	existing, exists := m.entries[compositeKey]
	var id int64
	var createdAt time.Time
	if exists {
		id = existing.ID
		createdAt = existing.CreatedAt
	} else {
		id = m.nextID
		m.nextID++
		createdAt = now
	}

	// Store a copy, as YAGPDB stores a serialized value, and value_num = ToFloat64(value)
	convertedValue := types.ForStorage(value)
	m.valueNums[compositeKey] = yagstd.ToFloat64(convertedValue)

	entry := &types.LightDBEntry{
		ID:        id,
		GuildID:   m.guildID,
		UserID:    userID,
		CreatedAt: createdAt,
		UpdatedAt: now,
		Key:       key,
		Value:     convertedValue,
		ValueSize: len(serialized),
		ExpiresAt: expiresAt,
	}

	m.entries[compositeKey] = entry
	return entry, nil
}

// Del deletes a database entry by key.
func (m *MockDB) Del(userID int64, key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	compositeKey := makeKey(userID, key)
	if _, ok := m.entries[compositeKey]; ok {
		delete(m.entries, compositeKey)
		delete(m.valueNums, compositeKey)
		return true
	}
	return false
}

// DelByID deletes a database entry by ID.
func (m *MockDB) DelByID(userID int64, id int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for compositeKey, entry := range m.entries {
		if entry.ID == id && entry.UserID == userID {
			delete(m.entries, compositeKey)
			delete(m.valueNums, compositeKey)
			return true
		}
	}
	return false
}

// Incr follows YAGPDB's dbIncr upsert: a new entry stores the amount; an existing one
// only adds to value_num, keeping its raw value, creation time and expiry; an expired
// one restarts value_num at the amount and loses its expiry (its raw value stays).
func (m *MockDB) Incr(userID int64, key string, amount float64) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	compositeKey := makeKey(userID, key)
	now := time.Now()

	existing, exists := m.entries[compositeKey]
	if !exists {
		m.entries[compositeKey] = &types.LightDBEntry{
			ID:        m.nextID,
			GuildID:   m.guildID,
			UserID:    userID,
			CreatedAt: now,
			UpdatedAt: now,
			Key:       key,
			Value:     amount,
			ValueSize: 9, // msgpack float64
		}
		m.nextID++
		m.valueNums[compositeKey] = amount
		return amount, nil
	}

	updated := *existing
	updated.UpdatedAt = now
	if expired(existing, now) {
		m.valueNums[compositeKey] = amount
		updated.CreatedAt = now
		updated.ExpiresAt = time.Time{}
	} else {
		m.valueNums[compositeKey] += amount
	}
	m.entries[compositeKey] = &updated
	return m.valueNums[compositeKey], nil
}

// GetPattern retrieves entries matching a LIKE pattern, ordered by ID like YAGPDB
// ("id asc", or "id desc" for dbGetPatternReverse).
func (m *MockDB) GetPattern(userID int64, pattern string, limit, skip int, descending bool) []*types.LightDBEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*types.LightDBEntry
	now := time.Now()

	for _, entry := range m.entries {
		if entry.UserID != userID {
			continue
		}
		if expired(entry, now) {
			continue
		}
		if matchPattern(entry.Key, pattern) {
			results = append(results, m.view(entry))
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if descending {
			return results[i].ID > results[j].ID
		}
		return results[i].ID < results[j].ID
	})
	return page(results, limit, skip)
}

// TopEntries returns entries of every user whose key matches the LIKE pattern, ordered
// like YAGPDB's dbTopEntries ("value_num DESC, id DESC") or dbBottomEntries (ascending).
// Non-numeric values count as 0, as they do in YAGPDB's value_num column.
func (m *MockDB) TopEntries(pattern string, limit, skip int, ascending bool) []*types.LightDBEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*types.LightDBEntry
	now := time.Now()
	for _, entry := range m.entries {
		if expired(entry, now) {
			continue
		}
		if matchPattern(entry.Key, pattern) {
			results = append(results, entry)
		}
	}

	m.sortByValueNum(results, ascending)
	results = page(results, limit, skip)
	for i, e := range results {
		results[i] = m.view(e)
	}
	return results
}

// sortByValueNum orders entries as "value_num DESC, id DESC", or ascending.
func (m *MockDB) sortByValueNum(results []*types.LightDBEntry, ascending bool) {
	sort.Slice(results, func(i, j int) bool {
		a, b := m.valueNum(results[i]), m.valueNum(results[j])
		if a != b {
			return (a < b) == ascending
		}
		return (results[i].ID < results[j].ID) == ascending
	})
}

// Rank is YAGPDB's dbRank query: the 1-based position of the user's key among the
// unexpired entries matching userID and pattern (nil matches all), ordered by value_num
// then id, descending unless ascending. 0 if the entry isn't among them.
func (m *MockDB) Rank(userID *int64, pattern *string, ascending bool, targetUser int64, key string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*types.LightDBEntry
	now := time.Now()
	for _, entry := range m.entries {
		if expired(entry, now) || (userID != nil && entry.UserID != *userID) ||
			(pattern != nil && !matchPattern(entry.Key, *pattern)) {
			continue
		}
		results = append(results, entry)
	}
	m.sortByValueNum(results, ascending)
	for i, e := range results {
		if e.UserID == targetUser && e.Key == key {
			return int64(i + 1)
		}
	}
	return 0
}

// DelMultiple is YAGPDB's dbDelMultiple query: it deletes up to limit entries matching
// userID and pattern (nil matches all), expired ones included, in value_num order after
// skipping skip, and returns how many it deleted.
func (m *MockDB) DelMultiple(userID *int64, pattern *string, ascending bool, limit, skip int) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []*types.LightDBEntry
	for _, entry := range m.entries {
		if (userID != nil && entry.UserID != *userID) || (pattern != nil && !matchPattern(entry.Key, *pattern)) {
			continue
		}
		results = append(results, entry)
	}
	m.sortByValueNum(results, ascending)
	results = page(results, limit, skip)
	for _, e := range results {
		compositeKey := makeKey(e.UserID, e.Key)
		delete(m.entries, compositeKey)
		delete(m.valueNums, compositeKey)
	}
	return int64(len(results))
}

func page(results []*types.LightDBEntry, limit, skip int) []*types.LightDBEntry {
	if skip >= len(results) {
		return nil
	}
	results = results[skip:]
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

// valueNum is the entry's value_num column.
func (m *MockDB) valueNum(e *types.LightDBEntry) float64 {
	return m.valueNums[makeKey(e.UserID, e.Key)]
}

func isNumber(v interface{}) bool {
	rv := reflect.ValueOf(v)
	return rv.CanInt() || rv.CanUint() || rv.CanFloat()
}

// Count returns the number of entries matching optional criteria.
func (m *MockDB) Count(userID *int64, pattern *string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	now := time.Now()

	for _, entry := range m.entries {
		if expired(entry, now) {
			continue
		}
		// Filter by userID if provided
		if userID != nil && entry.UserID != *userID {
			continue
		}
		// Filter by pattern if provided
		if pattern != nil && !matchPattern(entry.Key, *pattern) {
			continue
		}
		count++
	}

	return count
}

// GetAll returns all non-expired entries (for debugging).
func (m *MockDB) GetAll() []*types.LightDBEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*types.LightDBEntry
	now := time.Now()

	for _, entry := range m.entries {
		if expired(entry, now) {
			continue
		}
		results = append(results, m.view(entry))
	}

	return results
}

// matchPattern is Postgres LIKE, which YAGPDB's pattern functions use: % matches any run
// of characters, _ matches one, a backslash escapes the next character, and the match is
// case-sensitive and covers the whole key.
func matchPattern(s, pattern string) bool {
	return likeRegexp(pattern).MatchString(s)
}

var (
	likeMu    sync.Mutex
	likeCache = map[string]*regexp.Regexp{}
)

func likeRegexp(pattern string) *regexp.Regexp {
	likeMu.Lock()
	defer likeMu.Unlock()
	if re, ok := likeCache[pattern]; ok {
		return re
	}
	var b strings.Builder
	b.WriteString(`(?s)\A`)
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; r {
		case '%':
			b.WriteString(".*")
		case '_':
			b.WriteString(".")
		case '\\':
			if i+1 < len(runes) {
				i++
				b.WriteString(regexp.QuoteMeta(string(runes[i])))
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString(`\z`)
	re := regexp.MustCompile(b.String())
	likeCache[pattern] = re
	return re
}
