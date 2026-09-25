// Package state provides mock implementations of YAGPDB's stateful services.
package state

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// MockDB provides an in-memory implementation of YAGPDB's database.
type MockDB struct {
	mu      sync.RWMutex
	entries map[string]*types.LightDBEntry
	guildID int64
	nextID  int64
}

// NewMockDB creates a new mock database for the given guild.
func NewMockDB(guildID int64) *MockDB {
	return &MockDB{
		entries: make(map[string]*types.LightDBEntry),
		guildID: guildID,
		nextID:  1,
	}
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

	// Check expiration
	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry
}

// Set stores a value in the database.
func (m *MockDB) Set(userID int64, key string, value interface{}) *types.LightDBEntry {
	return m.SetWithExpiry(userID, key, value, 0)
}

// SetWithExpiry stores a value with an expiration time (in seconds, 0 = no expiry).
func (m *MockDB) SetWithExpiry(userID int64, key string, value interface{}, ttlSeconds int) *types.LightDBEntry {
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

	// Convert nested maps to SDict for proper template method access. Numbers come back
	// from YAGPDB as float64 (ToLightDBEntry substitutes value_num), so store them so.
	convertedValue := convertToSDict(value)
	if isNumber(convertedValue) {
		convertedValue = toFloat(convertedValue)
	}

	entry := &types.LightDBEntry{
		ID:        id,
		GuildID:   m.guildID,
		UserID:    userID,
		CreatedAt: createdAt,
		UpdatedAt: now,
		Key:       key,
		Value:     types.WrapValue(convertedValue),
		ValueSize: estimateSize(convertedValue),
		ExpiresAt: expiresAt,
	}

	m.entries[compositeKey] = entry
	return entry
}

// convertToSDict recursively converts map[string]interface{} to types.SDict
// so that template methods like .Get work properly.
func convertToSDict(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(types.SDict)
		for k, v := range val {
			result[k] = convertToSDict(v)
		}
		return result
	case map[interface{}]interface{}:
		// YAML sometimes produces this type
		result := make(types.SDict)
		for k, v := range val {
			result[fmt.Sprint(k)] = convertToSDict(v)
		}
		return result
	case []interface{}:
		result := make(types.Slice, len(val))
		for i, item := range val {
			result[i] = convertToSDict(item)
		}
		return result
	default:
		return v
	}
}

// Del deletes a database entry by key.
func (m *MockDB) Del(userID int64, key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	compositeKey := makeKey(userID, key)
	if _, ok := m.entries[compositeKey]; ok {
		delete(m.entries, compositeKey)
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
			return true
		}
	}
	return false
}

// Incr increments a numeric value, creating it if it doesn't exist.
func (m *MockDB) Incr(userID int64, key string, amount float64) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	compositeKey := makeKey(userID, key)
	now := time.Now()

	existing, exists := m.entries[compositeKey]
	var currentVal float64
	var id int64
	var createdAt time.Time
	var expiresAt time.Time

	if exists {
		// Check expiration
		if !existing.ExpiresAt.IsZero() && now.After(existing.ExpiresAt) {
			// Treat as non-existent
			id = m.nextID
			m.nextID++
			createdAt = now
			currentVal = 0
		} else {
			id = existing.ID
			createdAt = existing.CreatedAt
			expiresAt = existing.ExpiresAt // YAGPDB's upsert keeps the expiry
			// YAGPDB adds to value_num, which is 0 for values that aren't numbers
			currentVal = valueNum(existing)
		}
	} else {
		id = m.nextID
		m.nextID++
		createdAt = now
		currentVal = 0
	}

	newVal := currentVal + amount
	entry := &types.LightDBEntry{
		ID:        id,
		GuildID:   m.guildID,
		UserID:    userID,
		CreatedAt: createdAt,
		UpdatedAt: now,
		Key:       key,
		Value:     newVal,
		ValueSize: 8, // float64 size
		ExpiresAt: expiresAt,
	}

	m.entries[compositeKey] = entry
	return newVal, nil
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
		// Check expiration
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			continue
		}
		// Simple pattern matching (% is wildcard)
		if matchPattern(entry.Key, pattern) {
			results = append(results, entry)
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
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			continue
		}
		if matchPattern(entry.Key, pattern) {
			results = append(results, entry)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		a, b := valueNum(results[i]), valueNum(results[j])
		if a != b {
			return (a < b) == ascending
		}
		return (results[i].ID < results[j].ID) == ascending
	})
	return page(results, limit, skip)
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

// valueNum is YAGPDB's value_num column: ToFloat64 of the value (strings are parsed,
// anything else that isn't a number is 0).
func valueNum(e *types.LightDBEntry) float64 {
	return toFloat(types.UnwrapValue(e.Value))
}

func isNumber(v interface{}) bool {
	rv := reflect.ValueOf(v)
	return rv.CanInt() || rv.CanUint() || rv.CanFloat()
}

func toFloat(v interface{}) float64 {
	rv := reflect.ValueOf(v)
	switch {
	case rv.CanInt():
		return float64(rv.Int())
	case rv.CanUint():
		return float64(rv.Uint())
	case rv.CanFloat():
		return rv.Float()
	case rv.Kind() == reflect.String:
		f, _ := strconv.ParseFloat(rv.String(), 64)
		return f
	default:
		return 0
	}
}

// Count returns the number of entries matching optional criteria.
func (m *MockDB) Count(userID *int64, pattern *string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	now := time.Now()

	for _, entry := range m.entries {
		// Check expiration
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
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
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			continue
		}
		results = append(results, entry)
	}

	return results
}

// matchPattern implements simple SQL LIKE pattern matching.
// % matches any sequence of characters.
func matchPattern(s, pattern string) bool {
	// Simple implementation - could be improved
	if pattern == "%" {
		return true
	}
	if pattern == s {
		return true
	}

	// Handle prefix match (pattern%)
	if len(pattern) > 1 && pattern[len(pattern)-1] == '%' {
		prefix := pattern[:len(pattern)-1]
		return len(s) >= len(prefix) && s[:len(prefix)] == prefix
	}

	// Handle suffix match (%pattern)
	if len(pattern) > 1 && pattern[0] == '%' {
		suffix := pattern[1:]
		return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
	}

	return false
}

// estimateSize provides a rough estimate of the serialized size of a value.
func estimateSize(v interface{}) int {
	switch val := v.(type) {
	case string:
		return len(val)
	case []byte:
		return len(val)
	case int, int64, float64:
		return 8
	case bool:
		return 1
	case nil:
		return 0
	default:
		// Rough estimate for complex types
		return 100
	}
}
