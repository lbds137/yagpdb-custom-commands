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

	// OnStore, if set, sees every value written by dbSet, dbSetExpire and dbIncr.
	OnStore func(fn string, userID int64, key string, value interface{})
}

func (d *DatabaseFuncs) stored(fn string, userID int64, key string, value interface{}) {
	if d.OnStore != nil {
		d.OnStore(fn, userID, key, value)
	}
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
	return forTemplate(entry)
}

// forTemplate is an entry as YAGPDB's database functions return it: a copy whose Value is
// freshly decoded (containers as *SDict/*Dict/*Slice) and whose User has the entry's ID.
func forTemplate(e *types.LightDBEntry) *types.LightDBEntry {
	c := *e
	c.Value = types.ForTemplate(e.Value)
	c.User.ID = e.UserID
	return &c
}

func forTemplateSlice(entries []*types.LightDBEntry) types.Slice {
	result := make(types.Slice, len(entries))
	for i, entry := range entries {
		result[i] = forTemplate(entry)
	}
	return result
}

// DbSet stores a value in the database.
// Returns an empty string (for template compatibility).
func (d *DatabaseFuncs) DbSet(userID interface{}, key interface{}, value interface{}) string {
	uid := ToInt64(userID)
	k := ToString(key)
	d.stored("dbSet", uid, k, value)
	d.DB.Set(uid, k, value)
	return ""
}

// DbSetExpire stores a value with an expiration time.
// ttl is in seconds.
func (d *DatabaseFuncs) DbSetExpire(userID interface{}, key interface{}, value interface{}, ttl interface{}) string {
	uid := ToInt64(userID)
	k := ToString(key)
	t := ToInt(ttl)
	d.stored("dbSetExpire", uid, k, value)
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
	d.stored("dbIncr", uid, k, amt)
	return d.DB.Incr(uid, k, amt)
}

// DbGetPattern retrieves entries matching a pattern.
func (d *DatabaseFuncs) DbGetPattern(userID interface{}, pattern interface{}, amount interface{}, skip interface{}) interface{} {
	uid := ToInt64(userID)
	p := ToString(pattern)
	a := ToInt(amount)
	s := ToInt(skip)

	// Cap at 100 as YAGPDB does
	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	entries := d.DB.GetPattern(uid, p, a, s, false)

	// Convert to slice of interfaces for template use
	return forTemplateSlice(entries)
}

// DbGetPatternReverse retrieves entries matching a pattern in reverse order.
func (d *DatabaseFuncs) DbGetPatternReverse(userID interface{}, pattern interface{}, amount interface{}, skip interface{}) interface{} {
	uid := ToInt64(userID)
	p := ToString(pattern)
	a := ToInt(amount)
	s := ToInt(skip)

	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	entries := d.DB.GetPattern(uid, p, a, s, true)

	return forTemplateSlice(entries)
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

// DbTopEntries returns the top N entries of all users by value_num.
func (d *DatabaseFuncs) DbTopEntries(pattern interface{}, amount interface{}, skip interface{}) interface{} {
	p := ToString(pattern)
	a := ToInt(amount)
	s := ToInt(skip)

	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	entries := d.DB.TopEntries(p, a, s, false)

	return forTemplateSlice(entries)
}

// DbBottomEntries returns the bottom N entries by value_num.
func (d *DatabaseFuncs) DbBottomEntries(pattern interface{}, amount interface{}, skip interface{}) interface{} {
	p := ToString(pattern)
	a := ToInt(amount)
	s := ToInt(skip)

	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	entries := d.DB.TopEntries(p, a, s, true)

	return forTemplateSlice(entries)
}

// DbRank returns the rank of an entry (simplified).
func (d *DatabaseFuncs) DbRank(query interface{}, userID interface{}, key interface{}) interface{} {
	// Simplified implementation - returns -1 (not ranked)
	// Real implementation uses SQL window functions
	return -1
}
