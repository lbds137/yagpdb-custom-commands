package funcs

import (
	"errors"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
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
func (d *DatabaseFuncs) DbGet(userID int64, key interface{}) interface{} {
	k := limitString(ToString(key), 256)
	entry := d.DB.Get(userID, k)
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
func (d *DatabaseFuncs) DbSet(userID int64, key interface{}, value interface{}) (string, error) {
	k := limitString(ToString(key), 256)
	_, err := d.DB.Set(userID, k, value)
	if err == nil {
		d.stored("dbSet", userID, k, value)
	}
	return "", err
}

// DbSetExpire stores a value with an expiration time.
// ttl is in seconds.
func (d *DatabaseFuncs) DbSetExpire(userID int64, key interface{}, value interface{}, ttl int) (string, error) {
	k := limitString(ToString(key), 256)
	_, err := d.DB.SetWithExpiry(userID, k, value, ttl)
	if err == nil {
		d.stored("dbSetExpire", userID, k, value)
	}
	return "", err
}

// DbDel deletes a database entry by key.
func (d *DatabaseFuncs) DbDel(userID int64, key interface{}) interface{} {
	k := limitString(ToString(key), 256)
	d.DB.Del(userID, k)
	return ""
}

// DbDelByID deletes a database entry by ID.
func (d *DatabaseFuncs) DbDelByID(userID int64, id int64) interface{} {
	d.DB.DelByID(userID, id)
	return ""
}

// DbIncr increments a numeric value in the database.
func (d *DatabaseFuncs) DbIncr(userID int64, key interface{}, amount interface{}) (interface{}, error) {
	k := limitString(ToString(key), 256)
	amt := ToFloat64(amount)
	d.stored("dbIncr", userID, k, amt)
	return d.DB.Incr(userID, k, amt)
}

// DbGetPattern retrieves entries matching a pattern.
func (d *DatabaseFuncs) DbGetPattern(userID int64, pattern interface{}, amount interface{}, skip interface{}) (interface{}, error) {
	p := limitString(ToString(pattern), 256)
	a := ToInt(amount)
	s := ToInt(skip)
	if s < 0 {
		return nil, errNegativeOffset
	}

	// Cap at 100 as YAGPDB does
	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	entries := d.DB.GetPattern(userID, p, a, s, false)

	// Convert to slice of interfaces for template use
	return forTemplateSlice(entries), nil
}

// DbGetPatternReverse retrieves entries matching a pattern in reverse order.
func (d *DatabaseFuncs) DbGetPatternReverse(userID int64, pattern interface{}, amount interface{}, skip interface{}) (interface{}, error) {
	p := limitString(ToString(pattern), 256)
	a := ToInt(amount)
	s := ToInt(skip)
	if s < 0 {
		return nil, errNegativeOffset
	}

	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	entries := d.DB.GetPattern(userID, p, a, s, true)

	return forTemplateSlice(entries), nil
}

// DbCount is YAGPDB's dbCount: its one argument is a user ID, a key pattern, or a query
// dict (userID, pattern); without it every entry counts.
func (d *DatabaseFuncs) DbCount(args ...interface{}) (interface{}, error) {
	var userID *int64
	var pattern *string
	if len(args) > 0 {
		switch arg := args[0].(type) {
		case int64:
			userID = &arg
		case int:
			uid := int64(arg)
			userID = &uid
		case string:
			p := limitString(arg, 256)
			pattern = &p
		default:
			q, err := queryFromArg(arg)
			if err != nil {
				return "", err
			}
			userID, pattern = q.UserID, q.Pattern
		}
	}
	return int64(d.DB.Count(userID, pattern)), nil
}

// DbTopEntries returns the top N entries of all users by value_num.
func (d *DatabaseFuncs) DbTopEntries(pattern interface{}, amount interface{}, skip interface{}) (interface{}, error) {
	p := limitString(ToString(pattern), 256)
	a := ToInt(amount)
	s := ToInt(skip)
	if s < 0 {
		return nil, errNegativeOffset
	}

	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	entries := d.DB.TopEntries(p, a, s, false)

	return forTemplateSlice(entries), nil
}

// DbBottomEntries returns the bottom N entries by value_num.
func (d *DatabaseFuncs) DbBottomEntries(pattern interface{}, amount interface{}, skip interface{}) (interface{}, error) {
	p := limitString(ToString(pattern), 256)
	a := ToInt(amount)
	s := ToInt(skip)
	if s < 0 {
		return nil, errNegativeOffset
	}

	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	entries := d.DB.TopEntries(p, a, s, true)

	return forTemplateSlice(entries), nil
}

// DbRank is YAGPDB's dbRank: the position of the user's key among the entries the query
// selects (0 if it isn't among them).
func (d *DatabaseFuncs) DbRank(query interface{}, userID int64, key string) (interface{}, error) {
	q, err := queryFromArg(query)
	if err != nil {
		return "", err
	}
	if q.UserID != nil && *q.UserID != userID { // some optimization
		return 0, nil
	}
	if rank := d.DB.Rank(q.UserID, q.Pattern, q.Reverse, userID, key); rank > 0 {
		return rank, nil
	}
	return 0, nil // YAGPDB's sql.ErrNoRows case: an untyped 0, so an int
}

// DbDelMultiple is YAGPDB's dbDelMultiple: it deletes up to amount (at most 100) of the
// entries the query selects, expired ones included, and returns how many.
func (d *DatabaseFuncs) DbDelMultiple(query interface{}, amount interface{}, skip interface{}) (interface{}, error) {
	q, err := queryFromArg(query)
	if err != nil {
		return "", err
	}
	a := int(ToInt64(amount))
	if a > 100 || a == 0 {
		a = 100
	}
	s := int(ToInt64(skip))
	if s < 0 { // Postgres checks OFFSET before LIMIT
		return "", errNegativeOffset
	}
	if a < 0 {
		return "", errors.New("pq: LIMIT must not be negative")
	}
	return d.DB.DelMultiple(q.UserID, q.Pattern, q.Reverse, a, s), nil
}

// errNegativeOffset is Postgres's answer to a negative skip.
var errNegativeOffset = errors.New("pq: OFFSET must not be negative")

// query is YAGPDB's Query: nil fields match everything.
type query struct {
	UserID  *int64
	Pattern *string
	Reverse bool
}

// queryFromArg is YAGPDB's: a dict with userID (a number), pattern and reverse (a bool).
func queryFromArg(arg interface{}) (*query, error) {
	dict, err := yagstd.StringKeyDictionary(arg)
	if err != nil {
		return nil, err
	}

	var q query
	for key, val := range dict {
		switch key {
		case "userID":
			switch val.(type) {
			case int, int64:
				uid := ToInt64(val)
				q.UserID = &uid
			default:
				return &q, errors.New("Invalid UserID datatype in query. Must be a number")
			}
		case "pattern":
			p := limitString(ToString(val), 256)
			q.Pattern = &p
		case "reverse":
			revFlag, ok := val.(bool)
			if !ok {
				return &q, errors.New("Invalid reverse flag datatype in query. Must be a boolean value.")
			}
			q.Reverse = revFlag
		default:
			return &q, errors.New("Invalid Key: " + key + " passed to query constructor")
		}
	}
	return &q, nil
}

// limitString is YAGPDB's: s cut to at most l bytes, on a character boundary.
func limitString(s string, l int) string {
	if len(s) <= l {
		return s
	}

	lastValidLoc := 0
	for i := range s {
		if i > l {
			break
		}
		lastValidLoc = i
	}

	return s[:lastValidLoc]
}
