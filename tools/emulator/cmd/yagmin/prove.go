package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/loader"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/schema"
)

// prove is the equivalence proof: minify every command into a scratch dir mirroring
// the repo layout, run the whole YAML suite twice — once as committed, once with the
// commands read from the scratch copies — and compare the results. The only allowed
// differences are error and warning texts that quote variable names or line:col
// positions, which minification changes by design (isAllowed has the exact shapes);
// anything else is a minifier bug.
func prove(args []string) {
	fs := flag.NewFlagSet("prove", flag.ExitOnError)
	commands := fs.String("commands", "commands", "Command tree")
	testdata := fs.String("testdata", "tools/emulator/testdata", "YAML suite directory")
	schemaFile := fs.String("schema", "db_schema.yaml", "Type schema for stored values")
	scratch := fs.String("scratch", "", "Directory for the minified copies (default: fresh temp dir)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if err := proveRun(*commands, *testdata, *schemaFile, *scratch); err != nil {
		fmt.Fprintf(os.Stderr, "prove: %v\n", err)
		os.Exit(1)
	}
}

func proveRun(commandsDir, testdataDir, schemaFile, scratch string) error {
	kept := scratch != ""
	if !kept {
		// a temp dir of our own making: removed when the run ends, whatever the result
		dir, err := os.MkdirTemp("", "yagmin-prove-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		scratch = dir
	}
	// batchRun mirrors commandsDir under scratch/commands, which is the layout the
	// runner's template-root remap expects (commands/<topic>/<file>.gohtml)
	fmt.Printf("minifying every command into %s\n", scratch)
	if err := batchRun(commandsDir, filepath.Join(scratch, "commands"), 0); err != nil {
		return err
	}

	sch, err := schema.Load(schemaFile)
	if err != nil {
		return err
	}
	tests, err := loader.LoadTestsFromDir(testdataDir)
	if err != nil {
		return err
	}
	base, err := filepath.Abs(testdataDir)
	if err != nil {
		return err
	}

	run := func(root string) []*loader.TestResult {
		r := loader.NewRunner(loader.RunnerConfig{BaseDir: base, Schema: sch, TemplateRoot: root})
		return r.RunTests(tests)
	}
	normal := run("")
	minified := run(scratch)

	// RunTests returns pointers; copy into values so resultDiffs can borrow them
	vals := func(rs []*loader.TestResult) []loader.TestResult {
		out := make([]loader.TestResult, len(rs))
		for i, r := range rs {
			out[i] = *r
		}
		return out
	}
	diffs, allowed := compareResults(vals(normal), vals(minified))
	for _, d := range allowed {
		fmt.Printf("≈ allowed: %s: %s\n", d.name, d.reason)
	}
	for _, d := range diffs {
		fmt.Printf("≠ %s: %s\n", d.name, d.reason)
	}
	fmt.Printf("%d test(s): %d allowed difference(s), %d real difference(s)\n",
		len(normal), len(allowed), len(diffs))
	if kept {
		fmt.Printf("scratch kept at %s\n", scratch)
	}
	if len(diffs) > 0 {
		return fmt.Errorf("%d test(s) differ beyond variable names and line:col positions", len(diffs))
	}
	fmt.Println("the minified suite matches the normal run")
	return nil
}

// testDiff is one differing test: name, and why.
type testDiff struct {
	name   string
	reason string
}

// The differences minification is allowed to make, and nothing more:
//
//   - positions: comments are dropped and actions re-printed, so line and column
//     numbers move. They appear in the shapes the emulator prints: the template's
//     name followed by :line:col or :line in exec errors ("template: yagtest:12:5:
//     executing ..." or "CC #3:12:5", yagtemplate/exec.go:187-190) and in loop-db
//     warnings ("commands/x.gohtml:48:", runtime/loopcheck.go:18-21), and "line 12,
//     row 5" or "line 12" in CC error output (runtime/ccerrors.go:29, loopcheck.go:18)
//   - variable names: every variable is renamed with one injective map per file, so
//     the two texts must agree once each original name is paired with one short name
//     (and vice versa), and every renamed name must look like one yagmin hands out.
var (
	normPos  = regexp.MustCompile(`(\.gohtml|\bCC #\d+|\byagtest):\d+(?::\d+)?`)
	normLine = regexp.MustCompile(`\bline \d+(?:, row \d+)?`)
	varToken = regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*`)
	shortVar = regexp.MustCompile(`^\$[a-zA-Z]{1,2}$`)
)

// normPositions masks the positions minification moves.
func normPositions(s string) string {
	s = normPos.ReplaceAllString(s, "$1:L")
	s = normLine.ReplaceAllString(s, "line L")
	return s
}

// sameUpToRenaming reports whether mini is normal with its variables renamed by one
// consistent injective map to short names. Text outside the variable tokens must be
// identical; a token that did not change is fine whatever its shape.
func sameUpToRenaming(normal, mini string) bool {
	nv := varToken.FindAllStringIndex(normal, -1)
	mv := varToken.FindAllStringIndex(mini, -1)
	if len(nv) != len(mv) {
		return false
	}
	forward := map[string]string{}
	backward := map[string]string{}
	prevN, prevM := 0, 0
	for i := range nv {
		if normal[prevN:nv[i][0]] != mini[prevM:mv[i][0]] {
			return false
		}
		from, to := normal[nv[i][0]:nv[i][1]], mini[mv[i][0]:mv[i][1]]
		if from != to && !shortVar.MatchString(to) {
			return false
		}
		if seen, ok := forward[from]; ok && seen != to {
			return false
		}
		if seen, ok := backward[to]; ok && seen != from {
			return false
		}
		forward[from], backward[to] = to, from
		prevN, prevM = nv[i][1], mv[i][1]
	}
	return normal[prevN:] == mini[prevM:]
}

// compareResults pairs the two runs test by test and classifies each difference as
// allowed (it disappears once variable names and positions are normalized away) or
// real.
func compareResults(normal, minified []loader.TestResult) (real, allowed []testDiff) {
	if len(normal) != len(minified) {
		reason := fmt.Sprintf("the runs ran different numbers of tests: %d vs %d",
			len(normal), len(minified))
		real = append(real, testDiff{name: "(suite)", reason: reason})
		return real, allowed
	}
	for i := range normal {
		a, b := normal[i], minified[i]
		if a.Name != b.Name {
			real = append(real, testDiff{name: a.Name, reason: fmt.Sprintf(
				"test %d became %q", i, b.Name)})
			continue
		}
		for _, d := range resultDiffs(&a, &b) {
			if isAllowed(d) {
				allowed = append(allowed, testDiff{name: a.Name, reason: d.reason()})
			} else {
				real = append(real, testDiff{name: a.Name, reason: d.reason()})
			}
		}
	}
	return real, allowed
}

// fieldDiff is one differing field of one test's result. lenient marks the fields
// that may carry error text (errors, failures, warnings, and the output of a run that
// errored, since YAGPDB appends the error to it): only there can a difference be
// allowed. A successful run's output must match exactly.
type fieldDiff struct {
	field   string // what differed
	normal  string
	mini    string
	lenient bool
}

func (d fieldDiff) reason() string {
	return fmt.Sprintf("%s differs: %q vs %q", d.field, d.normal, d.mini)
}

// isAllowed reports whether a lenient field's two texts differ only by moved
// positions and consistently renamed variables.
func isAllowed(d fieldDiff) bool {
	return d.lenient && sameUpToRenaming(normPositions(d.normal), normPositions(d.mini))
}

// resultDiffs lists every field of the two results that differs.
func resultDiffs(a, b *loader.TestResult) []fieldDiff {
	var diffs []fieldDiff
	if a.Passed != b.Passed {
		diffs = append(diffs, fieldDiff{field: "passed",
			normal: fmt.Sprint(a.Passed), mini: fmt.Sprint(b.Passed)})
	}
	if (a.Error == nil) != (b.Error == nil) {
		av, bv := "<nil>", "<nil>"
		if a.Error != nil {
			av = a.Error.Error()
		}
		if b.Error != nil {
			bv = b.Error.Error()
		}
		diffs = append(diffs, fieldDiff{field: "error", normal: av, mini: bv})
	} else if a.Error != nil && a.Error.Error() != b.Error.Error() {
		diffs = append(diffs, fieldDiff{field: "error",
			normal: a.Error.Error(), mini: b.Error.Error(), lenient: true})
	}
	diffs = append(diffs, stringSliceDiffs("failure", a.Failures, b.Failures)...)
	diffs = append(diffs, stringSliceDiffs("warning", a.Warnings, b.Warnings)...)
	if a.Output != b.Output {
		diffs = append(diffs, fieldDiff{field: "output",
			normal: a.Output, mini: b.Output, lenient: a.Error != nil})
	}
	return diffs
}

// stringSliceDiffs pairs two lists of strings and returns the differing pairs.
func stringSliceDiffs(field string, a, b []string) []fieldDiff {
	var diffs []fieldDiff
	if len(a) != len(b) {
		return []fieldDiff{{field: field, normal: fmt.Sprintf("%d entries", len(a)),
			mini: fmt.Sprintf("%d entries", len(b))}}
	}
	for i := range a {
		if a[i] != b[i] {
			diffs = append(diffs, fieldDiff{
				field:   fmt.Sprintf("%s %d", field, i),
				normal:  a[i],
				mini:    b[i],
				lenient: true,
			})
		}
	}
	return diffs
}
