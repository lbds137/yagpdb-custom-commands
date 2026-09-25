package state

import (
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

func TestValueSizeIsTheMsgpackSize(t *testing.T) {
	db := NewMockDB(1)
	for _, c := range []struct {
		value interface{}
		size  int
	}{
		{int64(32), 9}, // YAGPDB's msgpack writes every int64 in full
		{32.5, 9},
		{"x", 2},
		{true, 1},
		{types.SDict{"a": int64(1)}, 15}, // the sdict extension adds 3 bytes to the map
	} {
		entry, err := db.Set(0, "k", c.value)
		if err != nil {
			t.Fatal(err)
		}
		if entry.ValueSize != c.size {
			t.Errorf("%#v: size %d, want %d", c.value, entry.ValueSize, c.size)
		}
	}
}

func TestValuesOverTheLimitFail(t *testing.T) {
	db := NewMockDB(1)
	if _, err := db.Set(0, "ok", strings.Repeat("x", 99990)); err != nil {
		t.Errorf("99995 bytes fit: %v", err)
	}
	if _, err := db.Set(0, "big", strings.Repeat("x", 100000)); err == nil || err.Error() != "short write" {
		t.Errorf("want YAGPDB's short write, got %v", err)
	}
	if db.Get(0, "big") != nil {
		t.Error("a failed set stores nothing")
	}
}
