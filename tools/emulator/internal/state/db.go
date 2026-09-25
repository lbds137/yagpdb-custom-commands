// Package state provides mock implementations of YAGPDB's stateful services.
package state

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	yagstd "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
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
func (m *MockDB) GetPattern(userID int64, pattern string, limit, skip int, descending bool) ([]*types.LightDBEntry, error) {
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
		if ok, err := matchPattern(entry.Key, pattern); err != nil {
			return nil, err
		} else if ok {
			results = append(results, m.view(entry))
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if descending {
			return results[i].ID > results[j].ID
		}
		return results[i].ID < results[j].ID
	})
	return page(results, limit, skip), nil
}

// TopEntries returns entries of every user whose key matches the LIKE pattern, ordered
// like YAGPDB's dbTopEntries ("value_num DESC, id DESC") or dbBottomEntries (ascending).
// Non-numeric values count as 0, as they do in YAGPDB's value_num column.
func (m *MockDB) TopEntries(pattern string, limit, skip int, ascending bool) ([]*types.LightDBEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*types.LightDBEntry
	now := time.Now()
	for _, entry := range m.entries {
		if expired(entry, now) {
			continue
		}
		if ok, err := matchPattern(entry.Key, pattern); err != nil {
			return nil, err
		} else if ok {
			results = append(results, entry)
		}
	}

	m.sortByValueNum(results, ascending)
	results = page(results, limit, skip)
	for i, e := range results {
		results[i] = m.view(e)
	}
	return results, nil
}

// sortByValueNum orders entries as "value_num DESC, id DESC", or ascending.
func (m *MockDB) sortByValueNum(results []*types.LightDBEntry, ascending bool) {
	sort.Slice(results, func(i, j int) bool {
		if c := pgCompare(m.valueNum(results[i]), m.valueNum(results[j])); c != 0 {
			return (c < 0) == ascending
		}
		return (results[i].ID < results[j].ID) == ascending
	})
}

// Rank is YAGPDB's dbRank query: the 1-based position of the user's key among the
// unexpired entries matching userID and pattern (nil matches all), ordered by value_num
// then id, descending unless ascending. 0 if the entry isn't among them.
func (m *MockDB) Rank(userID *int64, pattern *string, ascending bool, targetUser int64, key string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*types.LightDBEntry
	now := time.Now()
	for _, entry := range m.entries {
		if expired(entry, now) || (userID != nil && entry.UserID != *userID) {
			continue
		}
		if ok, err := matchOptional(entry.Key, pattern); err != nil {
			return 0, err
		} else if ok {
			results = append(results, entry)
		}
	}
	m.sortByValueNum(results, ascending)
	for i, e := range results {
		if e.UserID == targetUser && e.Key == key {
			return int64(i + 1), nil
		}
	}
	return 0, nil
}

// DelMultiple is YAGPDB's dbDelMultiple query: it deletes up to limit entries matching
// userID and pattern (nil matches all), expired ones included, in value_num order after
// skipping skip, and returns how many it deleted.
func (m *MockDB) DelMultiple(userID *int64, pattern *string, ascending bool, limit, skip int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []*types.LightDBEntry
	for _, entry := range m.entries {
		if userID != nil && entry.UserID != *userID {
			continue
		}
		if ok, err := matchOptional(entry.Key, pattern); err != nil {
			return 0, err // the statement fails whole: nothing is deleted
		} else if ok {
			results = append(results, entry)
		}
	}
	m.sortByValueNum(results, ascending)
	results = page(results, limit, skip)
	for _, e := range results {
		compositeKey := makeKey(e.UserID, e.Key)
		delete(m.entries, compositeKey)
		delete(m.valueNums, compositeKey)
	}
	return int64(len(results)), nil
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

// pgCompare orders float8 values as Postgres does: NaN equals NaN and sorts above every
// number (Go's cmp.Compare puts it below).
func pgCompare(a, b float64) int {
	switch an, bn := math.IsNaN(a), math.IsNaN(b); {
	case an && bn:
		return 0
	case an:
		return 1
	case bn:
		return -1
	}
	return cmp.Compare(a, b)
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
func (m *MockDB) Count(userID *int64, pattern *string) (int, error) {
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
		if ok, err := matchOptional(entry.Key, pattern); err != nil {
			return 0, err
		} else if ok {
			count++
		}
	}

	return count, nil
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
// case-sensitive and covers the whole key. A pattern ending in the escape is an error only
// when matching reaches it, as in Postgres.
func matchPattern(s, pattern string) (bool, error) {
	matched, err := matchText(s, pattern)
	return matched == likeTrue, err
}

// matchOptional is matchPattern where a nil pattern matches every key.
func matchOptional(s string, pattern *string) (bool, error) {
	if pattern == nil {
		return true, nil
	}
	return matchPattern(s, *pattern)
}

// ErrLikeEscape is Postgres's error for a LIKE pattern that ends with its escape character.
var ErrLikeEscape = errors.New("pq: LIKE pattern must not end with escape character")

const (
	likeFalse = iota
	likeTrue
	likeAbort // no match, and no later start in the text can match either
)

// matchText is Postgres's MatchText (src/backend/utils/adt/like_match.c, the UTF-8,
// case-sensitive variant) on bytes: it steps by byte in lockstep and by character after
// a wildcard.
func matchText(t, p string) (int, error) {
	// Fast path for match-everything pattern
	if len(p) == 1 && p[0] == '%' {
		return likeTrue, nil
	}
	for len(t) > 0 && len(p) > 0 {
		if p[0] == '\\' {
			// Next pattern byte must match literally, whatever it is
			p = p[1:]
			// ... and there had better be one, per SQL standard
			if len(p) == 0 {
				return likeFalse, ErrLikeEscape
			}
			if p[0] != t[0] {
				return likeFalse, nil
			}
		} else if p[0] == '%' {
			// Skip the wildcards after the %: N _'s and any %'s match at least N characters
			p = p[1:]
			for len(p) > 0 {
				if p[0] == '%' {
					p = p[1:]
				} else if p[0] == '_' {
					if len(t) == 0 {
						return likeAbort, nil
					}
					t = nextChar(t)
					p = p[1:]
				} else {
					break
				}
			}
			// A trailing % matches any remaining text
			if len(p) == 0 {
				return likeTrue, nil
			}
			// Scan for a text position where the rest of the pattern, which starts with a
			// literal, matches
			var firstpat byte
			if p[0] == '\\' {
				if len(p) < 2 {
					return likeFalse, ErrLikeEscape
				}
				firstpat = p[1]
			} else {
				firstpat = p[0]
			}
			for len(t) > 0 {
				if t[0] == firstpat {
					matched, err := matchText(t, p)
					if err != nil || matched != likeFalse {
						return matched, err
					}
				}
				t = nextChar(t)
			}
			return likeAbort, nil
		} else if p[0] == '_' {
			// _ matches any single character, and we know there is one
			t = nextChar(t)
			p = p[1:]
			continue
		} else if p[0] != t[0] {
			return likeFalse, nil
		}
		t = t[1:]
		p = p[1:]
	}
	if len(t) > 0 {
		return likeFalse, nil // end of pattern, but not of text
	}
	// End of text: match iff the rest of the pattern is %'s
	for len(p) > 0 && p[0] == '%' {
		p = p[1:]
	}
	if len(p) == 0 {
		return likeTrue, nil
	}
	return likeAbort, nil
}

// nextChar is like_match.c's UTF-8 NextChar: past one byte and its continuation bytes.
func nextChar(s string) string {
	s = s[1:]
	for len(s) > 0 && s[0]&0xC0 == 0x80 {
		s = s[1:]
	}
	return s
}
