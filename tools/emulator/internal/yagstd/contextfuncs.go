// Copied from YAGPDB (github.com/botlabs-gg/yagpdb, commit 0cf2ec5), common/templates/
// context_funcs.go: the regex functions with their per-run cache, and sort. MIT license, see
// LICENSE-YAGPDB. Changes: package name, a minimal Context holding the regex cache, and
// sort's call counter removed (the emulator's limits wrapper counts sort calls).

package yagstd

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ErrRegexCacheLimit is returned for an 11th distinct regular expression in one run.
var ErrRegexCacheLimit = errors.New("too many unique regular expressions (regex)")

// Context is the per-run state these functions need: YAGPDB's Context.RegexCache.
type Context struct {
	RegexCache map[string]*regexp.Regexp
}

// Funcs returns the context functions bound to c: reFind, reFindAll, reFindAllSubmatches,
// reReplace, reSplit and sort.
func (c *Context) Funcs() map[string]interface{} {
	return map[string]interface{}{
		"reFind":              c.reFind,
		"reFindAll":           c.reFindAll,
		"reFindAllSubmatches": c.reFindAllSubmatches,
		"reReplace":           c.reReplace,
		"reSplit":             c.reSplit,
		"sort":                c.tmplSort,
	}
}

func (c *Context) reFind(r, s string) (string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return "", err
	}

	return compiled.FindString(s), nil
}

func (c *Context) reFindAll(r, s string, i ...int) ([]string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return nil, err
	}

	var n int
	if len(i) > 0 {
		n = i[0]
	}

	if n > 1000 || n <= 0 {
		n = 1000
	}

	return compiled.FindAllString(s, n), nil
}

func (c *Context) reFindAllSubmatches(r, s string, i ...int) ([][]string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return nil, err
	}

	var n int
	if len(i) > 0 {
		n = i[0]
	}

	if n > 100 || n <= 0 {
		n = 100
	}

	return compiled.FindAllStringSubmatch(s, n), nil
}

func (c *Context) reReplace(r, s, repl string) (string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return "", err
	}
	if len(s)*len(repl) > MaxStringLength {
		return "", ErrStringTooLong
	}
	ret := compiled.ReplaceAllString(s, repl)
	if len(ret) > MaxStringLength {
		return "", ErrStringTooLong
	}
	return ret, nil
}

func (c *Context) reSplit(r, s string, i ...int) ([]string, error) {
	compiled, err := c.compileRegex(r)
	if err != nil {
		return nil, err
	}

	var n int
	if len(i) > 0 {
		n = i[0]
	}

	if n > 500 || n <= 0 {
		n = 500
	}

	return compiled.Split(s, n), nil
}

func (c *Context) compileRegex(r string) (*regexp.Regexp, error) {
	if c.RegexCache == nil {
		c.RegexCache = make(map[string]*regexp.Regexp)
	}

	cached, ok := c.RegexCache[r]
	if ok {
		return cached, nil
	}

	if len(c.RegexCache) >= 10 {
		return nil, ErrRegexCacheLimit
	}

	compiled, err := regexp.Compile(r)
	if err != nil {
		return nil, err
	}

	c.RegexCache[r] = compiled

	return compiled, nil
}

func (c *Context) tmplSort(input interface{}, args ...interface{}) (interface{}, error) {
	v, _ := indirect(reflect.ValueOf(input))
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		// ok
	default:
		return "", fmt.Errorf("cannot sort value of type %T", input)
	}

	opts, err := parseSortOpts(args...)
	if err != nil {
		return nil, err
	}

	type cmpVal struct {
		Key  reflect.Value // Key is the value to sort by.
		Orig reflect.Value
	}
	vals := make([]cmpVal, v.Len())
	cmp := invalidComparator
	for i := 0; i < v.Len(); i++ {
		el, _ := indirect(v.Index(i))
		key := el
		if opts.Key.IsValid() {
			key, err = indexContainer(el, opts.Key)
			if err != nil {
				return nil, err
			}
			key, _ = indirect(key)
		}

		curCmp, err := comparatorOf(key)
		if err != nil {
			return nil, err
		}

		if i == 0 {
			cmp = curCmp
		} else if curCmp != cmp {
			return nil, errors.New("input contains incompatible element types")
		}
		vals[i] = cmpVal{key, el}
	}

	sort.SliceStable(vals, func(i, j int) bool {
		if opts.Reverse {
			i, j = j, i
		}
		return cmp.Less(vals[i].Key, vals[j].Key)
	})
	out := make(Slice, len(vals))
	for i, v := range vals {
		out[i] = v.Orig.Interface()
	}
	return out, nil
}

var defaultSortOpts = sortOpts{Reverse: false, Key: reflect.Value{}}

type sortOpts struct {
	Reverse bool
	Key     reflect.Value
}

func parseSortOpts(args ...interface{}) (*sortOpts, error) {
	opts := defaultSortOpts
	if len(args) == 0 {
		return &opts, nil
	}

	dict, err := StringKeyDictionary(args...)
	if err != nil {
		return nil, err
	}

	for k, v := range dict {
		switch {
		case strings.EqualFold(k, "reverse"):
			b, ok := v.(bool)
			if !ok {
				return nil, fmt.Errorf("expected reverse option to be of type bool, but got type %T instead", v)
			}
			opts.Reverse = b
		case strings.EqualFold(k, "key"):
			opts.Key = reflect.ValueOf(v)
		default:
			return nil, fmt.Errorf("invalid option %q", k)
		}
	}
	return &opts, nil
}

func indexContainer(container, key reflect.Value) (reflect.Value, error) {
	container, _ = indirect(container)
	key, _ = indirect(key)

	switch container.Kind() {
	case reflect.Array, reflect.Slice:
		switch key.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i := int(key.Int())
			if i < 0 || i >= container.Len() {
				return reflect.Value{}, fmt.Errorf("index %d out of range", i)
			}
			return container.Index(i), nil

		case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			u := key.Uint()
			if u >= uint64(container.Len()) {
				return reflect.Value{}, fmt.Errorf("index %d out of range", u)
			}
			return container.Index(int(u)), nil

		default:
			return reflect.Value{}, fmt.Errorf("cannot index array/slice by key of type %T", key.Type())
		}

	case reflect.Map:
		if key.Type().AssignableTo(container.Type().Key()) {
			v := container.MapIndex(key)
			if !v.IsValid() {
				return reflect.Value{}, fmt.Errorf("key %v not found in map", key)
			}
			return v, nil
		}

	case reflect.Struct:
		if key.Kind() != reflect.String {
			return reflect.Value{}, fmt.Errorf("cannot index struct with non-string key")
		}

		s := key.String()
		ft, ok := container.Type().FieldByName(s)
		if !ok {
			return reflect.Value{}, fmt.Errorf("no field named %q in %s struct", s, container.Type())
		}
		if !ft.IsExported() {
			return reflect.Value{}, fmt.Errorf("field %q of %s struct is not exported", s, container.Type())
		}
		return container.FieldByName(s), nil
	}

	return reflect.Value{}, fmt.Errorf("cannot index value of type %s", container.Type())
}

func comparatorOf(v reflect.Value) (comparator, error) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intComparator, nil
	case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return uintComparator, nil
	case reflect.Float32, reflect.Float64:
		return floatComparator, nil
	case reflect.String:
		return stringComparator, nil
	default:
		if v.Type() == timeType {
			return timeComparator, nil
		}
		return invalidComparator, fmt.Errorf("cannot compare value of type %s", v.Type())
	}
}

type comparator int

const (
	invalidComparator comparator = iota
	intComparator
	uintComparator
	floatComparator
	stringComparator
	timeComparator
)

func (c comparator) Less(a, b reflect.Value) bool {
	switch c {
	case intComparator:
		return a.Int() < b.Int()
	case uintComparator:
		return a.Uint() < b.Uint()
	case floatComparator:
		af, bf := a.Float(), b.Float()
		return af < bf || (math.IsNaN(af) && !math.IsNaN(bf))
	case stringComparator:
		return a.String() < b.String()
	case timeComparator:
		return a.Interface().(time.Time).Before(b.Interface().(time.Time))
	default:
		panic("invalid comparator")
	}
}

var timeType = reflect.TypeOf(time.Time{})
