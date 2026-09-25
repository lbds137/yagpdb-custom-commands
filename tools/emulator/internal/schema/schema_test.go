package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

func load(t *testing.T, content string) (*Schema, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func TestLoadAndCheck(t *testing.T) {
	s, err := load(t, `
entries:
  - user_id: 0
    key: Global
    type: sdict
  - key: "score_*"
    type: int
  - key: "*"
    type: any
`)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		user  int64
		key   string
		value interface{}
		bad   bool
	}{
		{0, "Global", types.SDict{"a": 1}, false},
		{0, "Global", &types.SDict{}, false}, // read with dbGet, stored back
		{0, "Global", "text", true},
		{9, "Global", "text", false}, // rule is for user 0 only; "*" matches
		{3, "score_ann", 12.5, false},
		{3, "score_ann", "12", true},
		{3, "anything", types.Slice{1}, false},
	}
	for _, c := range cases {
		msg := s.Check(c.user, c.key, c.value)
		if (msg != "") != c.bad {
			t.Errorf("Check(%d, %q, %#v) = %q, want mismatch=%v", c.user, c.key, c.value, msg, c.bad)
		}
	}
}

func TestLoadRejectsBadSchemas(t *testing.T) {
	for content, want := range map[string]string{
		"entries:\n  - key: A\n    type: sdic\n":     `unknown type "sdic"`,
		"entries:\n  - type: dict\n":                 "has no key",
		"entries:\n  - key: \"[\"\n    type: dict\n": "bad key pattern",
	} {
		if _, err := load(t, content); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: want error containing %q, got %v", content, want, err)
		}
	}
}

func TestNilSchemaChecksNothing(t *testing.T) {
	var s *Schema
	if msg := s.Check(0, "k", 1); msg != "" {
		t.Errorf("got %q", msg)
	}
}
