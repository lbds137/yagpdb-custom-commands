package funcs

import (
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// DatabaseFuncs provides database-related template functions.
// It wraps a MockDB instance to provide YAGPDB-compatible function signatures.
type DatabaseFuncs struct {
	DB      *state.MockDB
	GuildID int64
}

// NewDatabaseFuncs creates a new DatabaseFuncs wrapper.
func NewDatabaseFuncs(db *state.MockDB, guildID int64) *DatabaseFuncs {
	return &DatabaseFuncs{
		DB:      db,
		GuildID: guildID,
	}
}

// DbGet retrieves a value from the database.
// Returns the LightDBEntry or nil if not found.
func (d *DatabaseFuncs) DbGet(userID interface{}, key interface{}) interface{} {
	uid := ToInt64(userID)
	k := ToString(key)
	entry := d.DB.Get(uid, k)
	if entry == nil {
		return nil
	}
	return entry
}

// DbSet stores a value in the database.
// Returns an empty string (for template compatibility).
func (d *DatabaseFuncs) DbSet(userID interface{}, key interface{}, value interface{}) string {
	uid := ToInt64(userID)
	k := ToString(key)
	d.DB.Set(uid, k, value)
	return ""
}

// DbSetExpire stores a value with an expiration time.
// ttl is in seconds.
func (d *DatabaseFuncs) DbSetExpire(userID interface{}, key interface{}, value interface{}, ttl interface{}) string {
	uid := ToInt64(userID)
	k := ToString(key)
	t := ToInt(ttl)
	d.DB.SetWithExpiry(uid, k, value, t)
	return ""
}

// DbDel deletes a database entry by key.
func (d *DatabaseFuncs) DbDel(userID interface{}, key interface{}) interface{} {
	uid := ToInt64(userID)
	k := ToString(key)
	d.DB.Del(uid, k)
	return ""
}

// DbDelByID deletes a database entry by ID.
func (d *DatabaseFuncs) DbDelByID(userID interface{}, id interface{}) interface{} {
	uid := ToInt64(userID)
	i := ToInt64(id)
	d.DB.DelByID(uid, i)
	return ""
}

// DbIncr increments a numeric value in the database.
func (d *DatabaseFuncs) DbIncr(userID interface{}, key interface{}, amount interface{}) (interface{}, error) {
	uid := ToInt64(userID)
	k := ToString(key)
	amt := ToFloat64(amount)
	return d.DB.Incr(uid, k, amt)
}

// DbGetPattern retrieves entries matching a pattern.
func (d *DatabaseFuncs) DbGetPattern(userID interface{}, pattern interface{}, amount interface{}, skip interface{}) interface{} {
	uid := ToInt64(userID)
	p := ToString(pattern)
	a := ToInt(amount)
	s := ToInt(skip)

	// Cap at 100 as YAGPDB does
	if a > 100 {
		a = 100
	}

	entries := d.DB.GetPattern(uid, p, a, s)

	// Convert to slice of interfaces for template use
	result := make(types.Slice, len(entries))
	for i, entry := range entries {
		result[i] = entry
	}
	return result
}

// DbGetPatternReverse retrieves entries matching a pattern in reverse order.
func (d *DatabaseFuncs) DbGetPatternReverse(userID interface{}, pattern interface{}, amount interface{}, skip interface{}) interface{} {
	uid := ToInt64(userID)
	p := ToString(pattern)
	a := ToInt(amount)
	s := ToInt(skip)

	if a > 100 {
		a = 100
	}

	entries := d.DB.GetPattern(uid, p, a, s)

	// Reverse the results
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	result := make(types.Slice, len(entries))
	for i, entry := range entries {
		result[i] = entry
	}
	return result
}

// DbCount returns the number of database entries.
func (d *DatabaseFuncs) DbCount(args ...interface{}) interface{} {
	var userID *int64
	var pattern *string

	for i, arg := range args {
		switch i {
		case 0:
			if arg != nil {
				uid := ToInt64(arg)
				userID = &uid
			}
		case 1:
			if arg != nil {
				p := ToString(arg)
				pattern = &p
			}
		}
	}

	return d.DB.Count(userID, pattern)
}

// DbTopEntries returns the top N entries by value_num.
// This is a simplified implementation - YAGPDB uses SQL window functions.
func (d *DatabaseFuncs) DbTopEntries(pattern interface{}, amount interface{}, skip interface{}) interface{} {
	p := ToString(pattern)
	a := ToInt(amount)
	s := ToInt(skip)

	if a > 100 {
		a = 100
	}

	// Get all matching entries
	entries := d.DB.GetPattern(0, p, 1000, 0)

	// Sort by Value (numeric) descending - simplified
	// In a real implementation, this would sort properly

	// Apply skip and limit
	if s >= len(entries) {
		return types.Slice{}
	}
	entries = entries[s:]
	if a > 0 && len(entries) > a {
		entries = entries[:a]
	}

	result := make(types.Slice, len(entries))
	for i, entry := range entries {
		result[i] = entry
	}
	return result
}

// DbBottomEntries returns the bottom N entries by value_num.
func (d *DatabaseFuncs) DbBottomEntries(pattern interface{}, amount interface{}, skip interface{}) interface{} {
	p := ToString(pattern)
	a := ToInt(amount)
	s := ToInt(skip)

	if a > 100 {
		a = 100
	}

	entries := d.DB.GetPattern(0, p, 1000, 0)

	// Would sort ascending in real implementation

	if s >= len(entries) {
		return types.Slice{}
	}
	entries = entries[s:]
	if a > 0 && len(entries) > a {
		entries = entries[:a]
	}

	result := make(types.Slice, len(entries))
	for i, entry := range entries {
		result[i] = entry
	}
	return result
}

// DbRank returns the rank of an entry (simplified).
func (d *DatabaseFuncs) DbRank(query interface{}, userID interface{}, key interface{}) interface{} {
	// Simplified implementation - returns -1 (not ranked)
	// Real implementation uses SQL window functions
	return -1
}
