// Package schema checks database values against their expected types.
package schema

import (
	"fmt"
	"os"
	"path"

	"gopkg.in/yaml.v3"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// Type names a schema can use.
const (
	TypeDict   = "dict" // sdict or dict
	TypeSlice  = "slice"
	TypeString = "string"
	TypeNumber = "number"
	TypeBool   = "bool"
	TypeAny    = "any"
)

var typeAliases = map[string]string{
	"dict": TypeDict, "sdict": TypeDict, "map": TypeDict,
	"slice": TypeSlice, "cslice": TypeSlice, "list": TypeSlice,
	"string": TypeString, "str": TypeString,
	"number": TypeNumber, "int": TypeNumber, "float": TypeNumber,
	"bool": TypeBool,
	"any":  TypeAny,
}

// Rule gives the expected type for keys matching Key (a path.Match glob).
// UserID nil matches every user ID.
type Rule struct {
	UserID *int64 `yaml:"user_id"`
	Key    string `yaml:"key"`
	Type   string `yaml:"type"`
}

// Schema is an ordered list of rules; the first matching rule applies.
type Schema struct {
	Entries []Rule `yaml:"entries"`
}

// Load reads and validates a schema file.
func Load(filename string) (*Schema, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading schema: %w", err)
	}
	var s Schema
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing schema %s: %w", filename, err)
	}
	for i, r := range s.Entries {
		if r.Key == "" {
			return nil, fmt.Errorf("schema %s: entry %d has no key", filename, i+1)
		}
		if _, err := path.Match(r.Key, ""); err != nil {
			return nil, fmt.Errorf("schema %s: entry %d: bad key pattern %q: %w", filename, i+1, r.Key, err)
		}
		canonical, ok := typeAliases[r.Type]
		if !ok {
			return nil, fmt.Errorf("schema %s: entry %d (%s): unknown type %q (use dict, slice, string, number, bool or any)",
				filename, i+1, r.Key, r.Type)
		}
		s.Entries[i].Type = canonical
	}
	return &s, nil
}

// Check returns a description of the mismatch when value does not have the type the
// schema expects for this entry, or "" when it matches or no rule covers the key.
func (s *Schema) Check(userID int64, key string, value interface{}) string {
	if s == nil {
		return ""
	}
	for _, r := range s.Entries {
		if r.UserID != nil && *r.UserID != userID {
			continue
		}
		if ok, _ := path.Match(r.Key, key); !ok {
			continue
		}
		got := TypeOf(value)
		if r.Type == TypeAny || got == r.Type {
			return ""
		}
		return fmt.Sprintf("user %d, key %q: schema expects %s, got %s (%s)", userID, key, r.Type, got, preview(value))
	}
	return ""
}

// TypeOf classifies a template value using the schema's type names.
func TypeOf(value interface{}) string {
	switch value.(type) {
	case types.SDict, *types.SDict, map[string]interface{}, types.Dict, *types.Dict, map[interface{}]interface{}:
		return TypeDict
	case types.Slice, *types.Slice, []interface{}, []string, []int, []int64, []float64:
		return TypeSlice
	case string:
		return TypeString
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return TypeNumber
	case bool:
		return TypeBool
	case nil:
		return "nil"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func preview(value interface{}) string {
	r := []rune(fmt.Sprintf("%v", value))
	s := string(r)
	if len(r) > 40 {
		s = string(r[:37]) + "..."
	}
	return fmt.Sprintf("%q", s)
}
