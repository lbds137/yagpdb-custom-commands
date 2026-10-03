package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/loader"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagtemplate/parse"
)

// minifyLines is minify's output with the kept header folded away for the tests that
// don't care about it.
func minifyBody(t *testing.T, source string) string {
	t.Helper()
	out, err := minify("t.gohtml", source)
	if err != nil {
		t.Fatalf("minify: %v", err)
	}
	if header := headerBlock(source); header != "" {
		return strings.TrimPrefix(out, header+"\n")
	}
	return out
}

func TestMinifyRenamesVariablesByUseCount(t *testing.T) {
	// $long appears 4 times, $other 2, $z 1: $long -> $a, $other -> $b, $z -> $c
	src := `{{$long := 1}}{{$other := 2}}{{$z := 3}}{{$long}}{{$other}}{{$long}}{{$other}}{{$long}}`
	out := minifyBody(t, src)
	want := `{{$a := 1}}{{$b := 2}}{{$c := 3}}{{$a}}{{$b}}{{$a}}{{$b}}{{$a}}`
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestMinifyTiesByFirstAppearance(t *testing.T) {
	// Both used once; the earlier-declared one appears first, so it gets $a
	src := `{{$second := 1}}{{$first := 2}}{{$second}}{{$first}}`
	out := minifyBody(t, src)
	want := `{{$a := 1}}{{$b := 2}}{{$a}}{{$b}}`
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestMinifyKeepsFieldChainsAndDot(t *testing.T) {
	src := `{{$x := .Data}}{{$x.Field.Chan}}{{$.User.ID}}{{$y := $x.B}}{{$y}}`
	out := minifyBody(t, src)
	want := `{{$a := .Data}}{{$a.Field.Chan}}{{$.User.ID}}{{$b := $a.B}}{{$b}}`
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestMinifyRenamesRangeDeclarations(t *testing.T) {
	src := `{{range $index, $value := .Items}}{{$index}}: {{$value}}{{end}}`
	out := minifyBody(t, src)
	if !strings.Contains(out, "range $a, $b := .Items") {
		t.Fatalf("range declaration not renamed: %q", out)
	}
	if !strings.Contains(out, "{{$a}}: {{$b}}") {
		t.Fatalf("range variables not renamed in body: %q", out)
	}
}

func TestMinifyDoesNotTouchStrings(t *testing.T) {
	src := `{{$x := "literal $x stays"}}{{$y := print "$x" (print $x)}}{{$y}}`
	out := minifyBody(t, src)
	if !strings.Contains(out, `"literal $x stays"`) {
		t.Fatalf("string literal changed: %q", out)
	}
	if !strings.Contains(out, `print "$x"`) {
		t.Fatalf("string literal changed: %q", out)
	}
}

func TestMinifyRenamesInjectively(t *testing.T) {
	// 53 variables: the 53rd needs a two-letter name, and none may collide
	src := "{{$v0 := 0}}"
	for i := 1; i < 53; i++ {
		src += "{{$v" + itoa(i) + " := " + itoa(i) + "}}"
	}
	for i := 0; i < 53; i++ {
		src += "{{$v" + itoa(i) + "}}"
	}
	out := minifyBody(t, src)

	// Re-parse and count the distinct variables the output declares
	trees, err := commandTrees("t.gohtml", out)
	if err != nil {
		t.Fatalf("output does not re-parse: %v", err)
	}
	names := map[string]bool{}
	for _, tree := range trees {
		walkNodes(tree.root, func(v *parse.VariableNode) {
			if v.Ident[0] != "$" {
				names[v.Ident[0]] = true
			}
		})
	}
	if len(names) != 53 {
		t.Fatalf("53 variables became %d distinct names: %v", len(names), sortedKeys(names))
	}
	if !names["$aa"] {
		t.Fatalf("the 53rd name should be $aa, got %v", sortedKeys(names))
	}
}

func TestMinifySerializesDefines(t *testing.T) {
	// The main tree is walked first, so its variable gets $a; the define's gets $b
	src := "{{define \"inner\"}}{{$x := 1}}{{$x}}{{end}}{{$y := 2}}{{$y}}{{template \"inner\"}}"
	out := minifyBody(t, src)
	if !strings.Contains(out, `{{define "inner"}}{{$b := 1}}{{$b}}{{end}}`) {
		t.Fatalf("define body not renamed: %q", out)
	}
	if !strings.Contains(out, `{{$a := 2}}{{$a}}{{template "inner"}}`) {
		t.Fatalf("main body wrong: %q", out)
	}
}

func TestMinifyDropsCommentsAndWhitespace(t *testing.T) {
	src := "{{/* a comment */}}{{$x := 1}}\n\t{{$x}}\n"
	out := minifyBody(t, src)
	want := "{{$a := 1}}{{$a}}"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestMinifyKeepsTextNodesWithContent(t *testing.T) {
	src := "{{if true}}Use /db{{else}}none{{end}}"
	if out := minifyBody(t, src); out != src {
		t.Fatalf("text with content was changed: %q", out)
	}
}

func TestMinifyKeepsHeaderVerbatim(t *testing.T) {
	src := "{{- /*\n  Trigger type: `Command`\n  Trigger: `db`\n*/}}\n{{$x := 1}}{{$x}}"
	out, err := minify("db.gohtml", src)
	if err != nil {
		t.Fatalf("minify: %v", err)
	}
	header := headerBlock(src)
	if !strings.HasPrefix(out, header) {
		t.Fatalf("header not kept verbatim at the start: %q", out)
	}
	if !strings.Contains(out, "{{$a := 1}}{{$a}}") {
		t.Fatalf("body not minified after the header: %q", out)
	}
}

func TestMinifyRejectsUnknownFunction(t *testing.T) {
	if _, err := minify("t.gohtml", "{{nosuchfunc}}"); err == nil {
		t.Fatal("an unknown function should fail the parse")
	}
}

func TestMinifyKeepsAssignments(t *testing.T) {
	// {{$x = v}} assigns the existing variable; re-printing it as := would shadow it
	// instead (parse's PipeNode.String drops the IsAssign flag, so yagmin prints
	// pipes itself)
	src := "{{$x := \"first\"}}{{if true}}{{$x = \"second\"}}{{end}}{{$x}}"
	out := minifyBody(t, src)
	want := "{{$a := \"first\"}}{{if true}}{{$a = \"second\"}}{{end}}{{$a}}"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestMinifyOutputReParses(t *testing.T) {
	for _, src := range []string{
		"{{$x := (dbGet 0 \"Global\").Value}}{{$x}}",
		"{{$x := true}}{{$y := true}}{{if $x}}{{else if $y}}both{{end}}",
		"{{with $w := 5}}{{$w}}{{end}}",
		"{{try}}{{print \"x\"}}{{catch}}c{{end}}",
		"{{range $i, $v := .X}}{{$i}}{{$v}}{{end}}",
	} {
		if _, err := minify("t.gohtml", src); err != nil {
			t.Errorf("minify(%q): %v", src, err)
		}
	}
}

func TestMinifyElseIfBranch(t *testing.T) {
	// {{else if}} re-prints as nested {{else}}{{if}}; both forms run the same
	src := "{{if false}}a{{else if true}}b{{else}}c{{end}}"
	out := minifyBody(t, src)
	if _, err := parseCommand("t.gohtml", out); err != nil {
		t.Fatalf("re-printed else-if does not re-parse: %v\n%s", err, out)
	}
	if !strings.Contains(out, "{{else}}{{if true}}b{{else}}c{{end}}{{end}}") {
		t.Fatalf("unexpected else-if print: %q", out)
	}
}

func TestMinifyShrinksCommands(t *testing.T) {
	for _, path := range []string{
		"../../commands/db/db_slash.gohtml",
		"../../commands/gematria/gematria.gohtml",
		"../../commands/edit/edit_slash.gohtml",
	} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("no %s here: %v", path, err)
		}
		out, err := minify("t.gohtml", string(source))
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if n := utf8.RuneCountInString(out); n >= 10000 {
			t.Errorf("%s: %d runes, still over the free-tier cap", path, n)
		}
	}
}

func TestBatchOnlyOverThreshold(t *testing.T) {
	srcDir := t.TempDir()
	dst := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(srcDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("big/a.gohtml", "{{$a := 1}}{{$a}} "+strings.Repeat("x", 120))
	write("small/b.gohtml", "{{$a := 1}}{{$a}}")

	if err := batchRun(srcDir, dst, 20); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "big/a.gohtml")); err != nil {
		t.Errorf("the big file was not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "small/b.gohtml")); !os.IsNotExist(err) {
		t.Errorf("the small file should have been skipped, got %v", err)
	}
}

func TestNormTextClassifiesAllowedDifferences(t *testing.T) {
	cases := []struct {
		normal, minified string
		allowed          bool
	}{
		{"same", "same", true},
		{"error at line 12: bad", "error at line 40: bad", true},
		{"t.gohtml:12: undefined \"$foo\"", "t.gohtml:31: undefined \"$a\"", true},
		{"output one", "output two", false},
		{"count: 5", "count: 9", false},
	}
	for _, c := range cases {
		d := fieldDiff{field: "x", normal: c.normal, mini: c.minified}
		if got := isAllowed(d); got != c.allowed {
			t.Errorf("isAllowed(%q, %q) = %v, want %v", c.normal, c.minified, got, c.allowed)
		}
	}
}

func TestCompareResultsFlagsRealDifference(t *testing.T) {
	results := func(outputs ...string) []loader.TestResult {
		var rs []loader.TestResult
		for _, o := range outputs {
			rs = append(rs, loader.TestResult{Name: "t", Passed: true, Output: o})
		}
		return rs
	}

	real, allowed := compareResults(results("hello"), results("hello "))
	if len(real) != 1 || len(allowed) != 0 {
		t.Fatalf("an output difference must be real, got real=%v allowed=%v", real, allowed)
	}

	real, allowed = compareResults(results("hello"), results("hello at line 9 and more words"))
	if len(real) != 1 || len(allowed) != 0 {
		t.Fatalf("changed output words are real differences, got real=%v allowed=%v", real, allowed)
	}

	real, allowed = compareResults(results("hello at line 9"), results("hello at line 40"))
	if len(real) != 0 || len(allowed) != 1 {
		t.Fatalf("a line-number-only difference is allowed, got real=%v allowed=%v", real, allowed)
	}
}

func TestHeaderBlock(t *testing.T) {
	if got := headerBlock("{{- /*\nTrigger type: `Command`\n*/}}\nbody"); got != "{{- /*\nTrigger type: `Command`\n*/}}" {
		t.Fatalf("got %q", got)
	}
	if got := headerBlock("{{$x := 1}}"); got != "" {
		t.Fatalf("no header expected, got %q", got)
	}
	// A comment later in the file is not a header
	if got := headerBlock("{{$x := 1}}\n{{/* later */}}"); got != "" {
		t.Fatalf("later comments are not the header, got %q", got)
	}
}

func TestShortName(t *testing.T) {
	cases := map[string]int{
		"a": 0, "b": 1, "z": 25, "A": 26, "Z": 51,
		"aa": 52, "ab": 53, "aZ": 103, "ba": 104,
	}
	for w, idx := range cases {
		if got := shortName(idx); got != w {
			t.Errorf("shortName(%d) = %q, want %q", idx, got, w)
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
