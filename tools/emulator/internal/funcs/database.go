package funcs

import (
	"errors"
	"fmt"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	yagstd "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// DatabaseFuncs provides database-related template functions.
// It wraps a MockDB instance to provide YAGPDB-compatible function signatures.
type DatabaseFuncs struct {
	DB      *state.MockDB
	GuildID int64

	// OnStore, if set, sees every value written by dbSet, dbSetExpire and dbIncr.
	OnStore func(fn string, userID int64, key string, value interface{})
	// OnPlanRisk, if set, hears of a pattern Postgres may reject while planning the query
	OnPlanRisk func(fn, pattern string)
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
	k := LimitString(ToString(key), 256)
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
	k := LimitString(ToString(key), 256)
	_, err := d.DB.Set(userID, k, value)
	if err == nil {
		d.stored("dbSet", userID, k, value)
	}
	return "", err
}

// DbSetExpire stores a value with an expiration time.
// ttl is in seconds.
func (d *DatabaseFuncs) DbSetExpire(userID int64, key interface{}, value interface{}, ttl int) (string, error) {
	k := LimitString(ToString(key), 256)
	_, err := d.DB.SetWithExpiry(userID, k, value, ttl)
	if err == nil {
		d.stored("dbSetExpire", userID, k, value)
	}
	return "", err
}

// DbDel deletes a database entry by key.
func (d *DatabaseFuncs) DbDel(userID int64, key interface{}) interface{} {
	k := LimitString(ToString(key), 256)
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
	k := LimitString(ToString(key), 256)
	amt := ToFloat64(amount)
	d.stored("dbIncr", userID, k, amt)
	return d.DB.Incr(userID, k, amt)
}

// DbGetPattern retrieves entries matching a pattern.
func (d *DatabaseFuncs) DbGetPattern(userID int64, pattern interface{}, amount interface{}, skip interface{}) (interface{}, error) {
	p := LimitString(ToString(pattern), 256)
	a := ToInt(amount)
	s := ToInt(skip)
	if s < 0 {
		return nil, boilerErr(errNegativeOffset)
	}

	// Cap at 100 as YAGPDB does
	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	d.planRisk("dbGetPattern", &p)
	entries, err := d.DB.GetPattern(userID, p, a, s, false)
	if err != nil {
		return nil, boilerErr(err)
	}

	// Convert to slice of interfaces for template use
	return forTemplateSlice(entries), nil
}

// DbGetPatternReverse retrieves entries matching a pattern in reverse order.
func (d *DatabaseFuncs) DbGetPatternReverse(userID int64, pattern interface{}, amount interface{}, skip interface{}) (interface{}, error) {
	p := LimitString(ToString(pattern), 256)
	a := ToInt(amount)
	s := ToInt(skip)
	if s < 0 {
		return nil, boilerErr(errNegativeOffset)
	}

	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	d.planRisk("dbGetPatternReverse", &p)
	entries, err := d.DB.GetPattern(userID, p, a, s, true)
	if err != nil {
		return nil, boilerErr(err)
	}

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
			p := LimitString(arg, 256)
			pattern = &p
		default:
			q, err := queryFromArg(arg)
			if err != nil {
				return "", err
			}
			userID, pattern = q.UserID, q.Pattern
		}
	}
	d.planRisk("dbCount", pattern)
	count, err := d.DB.Count(userID, pattern)
	if err != nil {
		return int64(0), err
	}
	return int64(count), nil
}

// DbTopEntries returns the top N entries of all users by value_num.
func (d *DatabaseFuncs) DbTopEntries(pattern interface{}, amount interface{}, skip interface{}) (interface{}, error) {
	p := LimitString(ToString(pattern), 256)
	a := ToInt(amount)
	s := ToInt(skip)
	if s < 0 {
		return nil, boilerErr(errNegativeOffset)
	}

	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	d.planRisk("dbTopEntries", &p)
	entries, err := d.DB.TopEntries(p, a, s, false)
	if err != nil {
		return nil, boilerErr(err)
	}

	return forTemplateSlice(entries), nil
}

// DbBottomEntries returns the bottom N entries by value_num.
func (d *DatabaseFuncs) DbBottomEntries(pattern interface{}, amount interface{}, skip interface{}) (interface{}, error) {
	p := LimitString(ToString(pattern), 256)
	a := ToInt(amount)
	s := ToInt(skip)
	if s < 0 {
		return nil, boilerErr(errNegativeOffset)
	}

	if a > 100 || a <= 0 {
		a = 100 // YAGPDB's cap, and its default for 0 or less
	}

	d.planRisk("dbBottomEntries", &p)
	entries, err := d.DB.TopEntries(p, a, s, true)
	if err != nil {
		return nil, boilerErr(err)
	}

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
	d.planRisk("dbRank", q.Pattern)
	rank, err := d.DB.Rank(q.UserID, q.Pattern, q.Reverse, userID, key)
	if err != nil {
		return int64(0), err
	}
	if rank > 0 {
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
		return "", boilerErr(errNegativeOffset)
	}
	if a < 0 {
		return "", boilerErr(errors.New("pq: LIMIT must not be negative"))
	}
	d.planRisk("dbDelMultiple", q.Pattern)
	deleted, err := d.DB.DelMultiple(q.UserID, q.Pattern, q.Reverse, a, s)
	if err != nil {
		return "", boilerErr(err)
	}
	return deleted, nil
}

// errNegativeOffset is Postgres's answer to a negative skip.
var errNegativeOffset = errors.New("pq: OFFSET must not be negative")

// boilerErr is a query error as the functions that select through sqlboiler's AllG return
// it: Bind wraps it, then the model's All (models/templates_user_database.go). dbCount and
// dbRank query directly and return lib/pq's error as it is.
func boilerErr(err error) error {
	return fmt.Errorf("models: failed to assign all query results to TemplatesUserDatabase slice: "+
		"bind failed to execute query: %w", err)
}

// planRisk reports a pattern that isn't an exact match and ends with an unpaired escape.
// Postgres plans with the real pattern and estimates its selectivity by running LIKE on
// the key column's statistics (every server's keys), so it may raise "LIKE pattern must
// not end with escape character" even when no row of this server reaches the escape.
func (d *DatabaseFuncs) planRisk(fn string, pattern *string) {
	if d.OnPlanRisk == nil || pattern == nil {
		return
	}
	wildcard, escaped := false, false
	for i := 0; i < len(*pattern); i++ {
		switch c := (*pattern)[i]; {
		case escaped:
			escaped = false
		case c == '\\':
			escaped = true
		case c == '%' || c == '_':
			wildcard = true // like_fixed_prefix stops here: not an exact match
		}
	}
	if wildcard && escaped {
		d.OnPlanRisk(fn, *pattern)
	}
}

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
			p := LimitString(ToString(val), 256)
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

// LimitString is YAGPDB's limitString: s cut to at most l bytes, on a character boundary.
func LimitString(s string, l int) string {
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
