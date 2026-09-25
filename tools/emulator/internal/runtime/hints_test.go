package runtime

import (
	"os"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHints(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{`{{dbGett 0 "a"}}`, "Did you mean dbGet?"},
		{`{{sendMessageNoEscape nil "x"}}`, "the emulator doesn't implement yet"},
		{`{{totallyMadeUpThing}}`, "YAGPDB has no function totallyMadeUpThing"},
		{`{{dbSet 0 "k" "v"}}{{(dbGet 0 "k").Foo}}`, "with .Value"},
		{`{{$a := parseArgs 1 "usage" (carg "string" "x")}}`, "-args"},
	}
	for _, c := range cases {
		_, err := run(t, newCtx(false, true), c.src)
		if err == nil {
			t.Errorf("%s: expected an error", c.src)
			continue
		}
		if hint := Hint(err); !strings.Contains(hint, c.want) {
			t.Errorf("%s:\n  error: %v\n  hint:  %q\n  want:  %q", c.src, err, hint, c.want)
		}
	}
}

func TestHintForLimitError(t *testing.T) {
	_, err := run(t, newCtx(true, false), elevenDBGets)
	if !strings.Contains(Hint(err), "caps how often") {
		t.Errorf("got %q", Hint(err))
	}
}
