package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/loader"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/runtime"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
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

// execute runs source through the emulator and returns the response YAGPDB would send
// (trimmed, as customcommands/bot.go does).
func execute(t *testing.T, source string) string {
	t.Helper()
	ctx := runtime.NewExecutionContext(1, state.NewMockDB(1))
	out, err := runtime.NewEngine(ctx).Execute(source)
	if err != nil {
		t.Fatalf("executing %q: %v", source, err)
	}
	return out
}

// sameOutput minifies source and checks the minified copy's response is the original's.
// It returns the minified body for assertions on what was dropped.
func sameOutput(t *testing.T, source, want string) string {
	t.Helper()
	mini := minifyBody(t, source)
	if got := execute(t, source); got != want {
		t.Fatalf("the original prints %q, the test expects %q", got, want)
	}
	if got := execute(t, mini); got != want {
		t.Fatalf("minified output %q, want %q\nminified: %q", got, want, mini)
	}
	return mini
}

// whitespaceNodes counts the whitespace-only text nodes left in a minified body.
func whitespaceNodes(t *testing.T, body string) int {
	t.Helper()
	trees, err := commandTrees("t.gohtml", body)
	if err != nil {
		t.Fatalf("minified output does not parse: %v", err)
	}
	n := 0
	for _, tree := range trees {
		n += countWhitespaceText(tree.root)
	}
	return n
}

// countWhitespaceText counts whitespace-only text nodes in a list and the lists under
// its control nodes (lists never sit inside pipes).
func countWhitespaceText(list *parse.ListNode) int {
	if list == nil {
		return 0
	}
	n := 0
	for _, child := range list.Nodes {
		switch node := child.(type) {
		case *parse.TextNode:
			if len(node.Text) > 0 && strings.TrimSpace(string(node.Text)) == "" {
				n++
			}
		case *parse.IfNode:
			n += countWhitespaceText(node.List) + countWhitespaceText(node.ElseList)
		case *parse.WithNode:
			n += countWhitespaceText(node.List) + countWhitespaceText(node.ElseList)
		case *parse.RangeNode:
			n += countWhitespaceText(node.List) + countWhitespaceText(node.ElseList)
		case *parse.WhileNode:
			n += countWhitespaceText(node.List) + countWhitespaceText(node.ElseList)
		case *parse.TryNode:
			n += countWhitespaceText(node.List) + countWhitespaceText(node.CatchList)
		}
	}
	return n
}

func TestMinifyKeepsSpaceBetweenTwoPrints(t *testing.T) {
	// The auditor's repro: the space is in the middle of the response, so it stays
	src := "{{$a := \"x\"}}{{$b := \"y\"}}{{ $a }} {{ $b }}"
	mini := sameOutput(t, src, "x y")
	if n := whitespaceNodes(t, mini); n != 1 {
		t.Fatalf("want the one separator kept, got %d whitespace nodes: %q", n, mini)
	}
}

func TestMinifyKeepsRangeSeparator(t *testing.T) {
	// The auditor's second repro: the separator prints between every two items
	src := "{{range $e := cslice 1 2 3}}{{ $e }} {{ end }}"
	sameOutput(t, src, "1 2 3")
}

func TestMinifyKeepsNewlineBetweenSilentAssignments(t *testing.T) {
	// The newline sits between two assignments that print nothing, but what prints
	// before and after it is non-whitespace, so it is part of the response
	src := "{{$a := \"x\"}}{{$b := \"y\"}}{{$a}}{{$x := 1}}\n{{$y := 2}}{{$b}}"
	mini := sameOutput(t, src, "x\ny")
	if n := whitespaceNodes(t, mini); n != 1 {
		t.Fatalf("want the newline kept, got %d whitespace nodes: %q", n, mini)
	}
}

func TestMinifyLoopBodyWhitespace(t *testing.T) {
	// A loop runs its body again after the whitespace, so a printer anywhere in the
	// loop keeps the whitespace, wherever it sits in the body
	src := "{{range $e := cslice 1 2}}\n{{$e}}{{end}}"
	mini := sameOutput(t, src, "1\n2")
	if n := whitespaceNodes(t, mini); n != 1 {
		t.Fatalf("want the loop's newline kept, got %d whitespace nodes: %q", n, mini)
	}

	// A loop that prints nothing itself: its whitespace is leading (nothing printed
	// before the loop) and goes
	src = "{{$x := 0}}{{range $e := cslice 1 2}}\n{{$x = $e}}\n{{end}}{{$x}}"
	mini = sameOutput(t, src, "2")
	if n := whitespaceNodes(t, mini); n != 0 {
		t.Fatalf("want the silent loop's whitespace dropped, got %d nodes: %q", n, mini)
	}
}

func TestMinifyDefineWhitespace(t *testing.T) {
	// A define's whitespace lands wherever the define is called; with printers in the
	// file it stays
	src := "{{define \"sep\"}} {{end}}{{$a := \"x\"}}{{$b := \"y\"}}{{$a}}{{template \"sep\"}}{{$b}}"
	mini := sameOutput(t, src, "x y")
	if !strings.Contains(mini, "{{define \"sep\"}} {{end}}") {
		t.Fatalf("the define's space was dropped: %q", mini)
	}

	// execTemplate runs the define on the main output too: the space after a silent
	// assignment is between the define's "x" and the "y"
	src = "{{define \"f\"}}x{{return 1}}{{end}}{{$b := \"y\"}}{{$a := execTemplate \"f\"}} {{$b}}"
	mini = sameOutput(t, src, "x y")
	if n := whitespaceNodes(t, mini); n != 1 {
		t.Fatalf("want the space kept, got %d whitespace nodes: %q", n, mini)
	}
}

func TestMinifyDropsLeadingAndTrailingWhitespace(t *testing.T) {
	// Only whitespace prints before and after the one printer: YAGPDB trims it anyway
	src := "\n\t{{$a := \"x\"}}\n{{$y := 2}}\n\t{{$a}}\n{{$z := 3}}\n"
	mini := sameOutput(t, src, "x")
	if n := whitespaceNodes(t, mini); n != 0 {
		t.Fatalf("want every whitespace node dropped, got %d: %q", n, mini)
	}
}

func TestMinifyDropsAllWhitespaceInSilentCommand(t *testing.T) {
	// Nothing prints: sendMessage, dbSet and an sdict's Set all return ""
	src := "{{define \"h\"}}\n{{$d := sdict}}\n{{$d.Set \"k\" 1}}\n{{end}}\n" +
		"{{sendMessage nil \"hi\"}}\n\t{{dbSet 0 \"k\" 1}}\n{{template \"h\"}}\n"
	mini := sameOutput(t, src, "")
	if n := whitespaceNodes(t, mini); n != 0 {
		t.Fatalf("want every whitespace node dropped, got %d: %q", n, mini)
	}
}

func TestMinifySetIsSilentOnlyForKnownContainers(t *testing.T) {
	// $v is an sdict: its Set prints "", so the newline after it is leading whitespace
	src := "{{$b := \"y\"}}{{$v := sdict}}\n{{$v.Set \"k\" 1}}\n{{$b}}"
	mini := sameOutput(t, src, "y")
	if n := whitespaceNodes(t, mini); n != 0 {
		t.Fatalf("an sdict's Set is silent; want 0 whitespace nodes, got %d: %q", n, mini)
	}

	// $v's binding isn't a known container (YAGPDB's modal builder has a Set that
	// prints), so the call counts as a printer and the newlines around it stay
	src = "{{$b := \"y\"}}{{$v := .ExecData}}\n{{$v.Set \"k\" 1}}\n{{$b}}"
	mini = minifyBody(t, src)
	if n := whitespaceNodes(t, mini); n != 1 {
		t.Fatalf("want the newline after the Set kept, got %d whitespace nodes: %q", n, mini)
	}
}

func TestMinifySetOnNilReceiverPrints(t *testing.T) {
	// A Set with arguments on a nil receiver doesn't error: it prints "<no value>", so
	// the space before it is in the middle of the response
	src := "{{$x := dbGet 0 \"missing\"}}{{\"a\"}} {{$x.Set \"k\" 1}}"
	mini := sameOutput(t, src, "a <no value>")
	if n := whitespaceNodes(t, mini); n != 1 {
		t.Fatalf("want the space kept, got %d whitespace nodes: %q", n, mini)
	}
}

func TestMinifyDollarIsNeverAContainer(t *testing.T) {
	// `$` starts out bound to the tree's dot (in the define, the nil .ExecData), a
	// binding no `$ = sdict` elsewhere in the file can rule out, so a Set on it may
	// print "<no value>"
	src := "{{define \"f\"}}{{ $.Set \"k\" 1 }}{{end}}{{ $ = sdict }}{{\"a\"}} " +
		"{{template \"f\" .ExecData}}"
	mini := sameOutput(t, src, "a <no value>")
	if n := whitespaceNodes(t, mini); n != 1 {
		t.Fatalf("want the space kept, got %d whitespace nodes: %q", n, mini)
	}
}

func TestMinifyMainTemplateReentry(t *testing.T) {
	// The main template can be called by name (YAGPDB's "CC #<id>", the emulator's
	// "yagtest"): the space in the if-branch then prints between A and B
	src := "{{if eq (printf \"%T\" .) \"int\"}} {{\"B\"}}{{else}}A{{template \"yagtest\" 1}}{{end}}"
	mini := sameOutput(t, src, "A B")
	if n := whitespaceNodes(t, mini); n != 1 {
		t.Fatalf("want the space kept, got %d whitespace nodes: %q", n, mini)
	}

	// a computed execTemplate name may name the main template too
	src = "{{$n := \"yagtest\"}}{{if eq (printf \"%T\" .) \"int\"}} {{\"B\"}}" +
		"{{else}}A{{$r := execTemplate $n 1}}{{end}}"
	mini = sameOutput(t, src, "A B")
	if n := whitespaceNodes(t, mini); n != 1 {
		t.Fatalf("want the space kept, got %d whitespace nodes: %q", n, mini)
	}

	// with nothing printing anywhere a re-entrant main still drops its whitespace
	src = "{{if eq (printf \"%T\" .) \"int\"}} {{$x := 1}}{{else}}{{template \"yagtest\" 1}}{{end}}"
	mini = sameOutput(t, src, "")
	if n := whitespaceNodes(t, mini); n != 0 {
		t.Fatalf("want the space dropped, got %d whitespace nodes: %q", n, mini)
	}
}

func TestMinifyKeepsDefineWhitespaceForSendTemplate(t *testing.T) {
	// sendTemplate sends a define's output as its own message, untrimmed by the
	// response path, so the define keeps its whitespace even with no printer anywhere
	src := "{{define \"m\"}}{{$x := 1}}\n{{$y := 2}}{{end}}{{sendTemplate nil \"m\"}}"
	mini := minifyBody(t, src)
	if !strings.Contains(mini, "{{define \"m\"}}{{$a := 1}}\n{{$b := 2}}{{end}}") {
		t.Fatalf("the define's whitespace was dropped: %q", mini)
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
	// the test runs in cmd/yagmin; the repo root is four levels up
	for _, path := range []string{
		"../../../../commands/db/db_slash.gohtml",
		"../../../../commands/gematria/gematria.gohtml",
		"../../../../commands/edit/edit_slash.gohtml",
	} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
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

func TestIsAllowedClassifiesDifferences(t *testing.T) {
	cases := []struct {
		normal, minified string
		allowed          bool
	}{
		{"same", "same", true},
		{"error at line 12: bad", "error at line 40: bad", true},
		{"`Failed executing CC #3, line 12, row 5: x`",
			"`Failed executing CC #3, line 4, row 9: x`", true},
		{"t.gohtml:12: undefined \"$foo\"", "t.gohtml:31: undefined \"$a\"", true},
		{"template: yagtest:12:5: executing \"yagtest\" at <$foo.Bar>: nil",
			"template: yagtest:3:40: executing \"yagtest\" at <$a.Bar>: nil", true},
		{"template: CC #7:12:5: x", "template: CC #7:1:9: x", true},
		// one consistent injective map, or nothing
		{"$x $y $x", "$a $b $a", true},
		{"$x $y $x", "$a $b $c", false},
		{"$x $y", "$a $a", false},
		{"$x", "$long", false},
		{"$foo stays and $x", "$foo stays and $a", true},
		// numbers outside the position shapes are real
		{"output one", "output two", false},
		{"count: 5", "count: 9", false},
		{"got 12:5 items", "got 3:4 items", false},
		{"line 12 of $x", "line 12 of $x.", false},
	}
	for _, c := range cases {
		d := fieldDiff{field: "x", normal: c.normal, mini: c.minified, lenient: true}
		if got := isAllowed(d); got != c.allowed {
			t.Errorf("isAllowed(%q, %q) = %v, want %v", c.normal, c.minified, got, c.allowed)
		}
		// a field that can't carry error text never gets the allowance
		d.lenient = false
		if c.normal != c.minified && isAllowed(d) {
			t.Errorf("isAllowed(%q, %q) must be false on a strict field", c.normal, c.minified)
		}
	}
}

func TestCheckReportsStaleMissingAndExtra(t *testing.T) {
	srcDir := t.TempDir()
	dst := t.TempDir()
	write := func(dir, rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	big := "{{$a := 1}}{{$a}} " + strings.Repeat("x", 120)
	write(srcDir, "a/fresh.gohtml", big)
	write(srcDir, "a/stale.gohtml", big)
	write(srcDir, "a/missing.gohtml", big)
	write(srcDir, "a/small.gohtml", "{{$a := 1}}{{$a}}")
	if err := batchRun(srcDir, dst, 20); err != nil {
		t.Fatal(err)
	}
	write(dst, "a/stale.gohtml", "old")
	if err := os.Remove(filepath.Join(dst, "a/missing.gohtml")); err != nil {
		t.Fatal(err)
	}
	write(dst, "a/extra.gohtml", "left over")
	write(dst, "a/notes.txt", "any file counts as extra")

	problems, err := checkRun(srcDir, dst, 20)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"missing: " + filepath.Join(dst, "a/missing.gohtml"),
		"stale: " + filepath.Join(dst, "a/stale.gohtml"),
		"extra: " + filepath.Join(dst, "a/extra.gohtml"),
		"extra: " + filepath.Join(dst, "a/notes.txt")}
	if len(problems) != len(want) {
		t.Fatalf("want %d problems, got %v", len(want), problems)
	}
	for i, prefix := range want {
		if !strings.HasPrefix(problems[i], prefix) {
			t.Errorf("problem %d = %q, want prefix %q", i, problems[i], prefix)
		}
	}

	// a dst that doesn't exist is one problem, not one per file
	if problems, err := checkRun(srcDir, filepath.Join(dst, "nowhere"), 20); err != nil ||
		len(problems) != 1 || !strings.HasPrefix(problems[0], "missing: ") {
		t.Fatalf("a missing dst should be one missing line, got %v, %v", problems, err)
	}

	// up to date: nothing to report
	for _, rel := range []string{"a/extra.gohtml", "a/notes.txt"} {
		if err := os.Remove(filepath.Join(dst, rel)); err != nil {
			t.Fatal(err)
		}
	}
	if err := batchRun(srcDir, dst, 20); err != nil {
		t.Fatal(err)
	}
	if problems, err := checkRun(srcDir, dst, 20); err != nil || len(problems) != 0 {
		t.Fatalf("a fresh tree should check clean, got %v, %v", problems, err)
	}
}

func TestProveRemovesItsScratchDir(t *testing.T) {
	// A prove run on an empty suite still minifies into a temp dir of its own; that
	// dir is gone afterwards. TMPDIR points at a dir of this test's own, so a prove
	// run by another session in the shared /tmp can't change the count
	t.Setenv("TMPDIR", t.TempDir())
	before := tempDirs(t, "yagmin-prove-")
	src := t.TempDir()
	path := filepath.Join(src, "a.gohtml")
	if err := os.WriteFile(path, []byte("{{$a := 1}}{{$a}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := proveRun(src, t.TempDir(), "../../../../db_schema.yaml", ""); err != nil {
		t.Fatal(err)
	}
	if after := tempDirs(t, "yagmin-prove-"); len(after) != len(before) {
		t.Fatalf("prove left a scratch dir: before %v, after %v", before, after)
	}
}

// tempDirs lists the entries of the temp dir with the given prefix.
func tempDirs(t *testing.T, prefix string) []string {
	t.Helper()
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			names = append(names, e.Name())
		}
	}
	return names
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

	// a successful run's output must match exactly: a moved line number or a renamed
	// variable in it is real
	real, allowed = compareResults(results("hello at line 9"), results("hello at line 40"))
	if len(real) != 1 || len(allowed) != 0 {
		t.Fatalf("an output-only position change is real, got real=%v allowed=%v", real, allowed)
	}
	real, allowed = compareResults(results("got $long"), results("got $a"))
	if len(real) != 1 || len(allowed) != 0 {
		t.Fatalf("an output-only variable name is real, got real=%v allowed=%v", real, allowed)
	}

	// a run that errored has the error text in its output: there the allowance holds
	errored := func(output string) []loader.TestResult {
		return []loader.TestResult{{Name: "t", Error: errors.New("boom"), Output: output}}
	}
	real, allowed = compareResults(errored("hello at line 9 $long"), errored("hello at line 40 $a"))
	if len(real) != 0 || len(allowed) != 1 {
		t.Fatalf("a failed run's output may move positions and rename, got real=%v allowed=%v",
			real, allowed)
	}
	// errors, failures and warnings are lenient fields
	warned := func(w string) []loader.TestResult {
		return []loader.TestResult{{Name: "t", Passed: true, Warnings: []string{w}}}
	}
	real, allowed = compareResults(warned("x.gohtml:48: dbDel in a loop"),
		warned("x.gohtml:8: dbDel in a loop"))
	if len(real) != 0 || len(allowed) != 1 {
		t.Fatalf("a warning's moved position is allowed, got real=%v allowed=%v", real, allowed)
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
