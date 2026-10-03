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
// positions, which minification changes by design; anything else is a minifier bug.
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
	if scratch == "" {
		dir, err := os.MkdirTemp("", "yagmin-prove-")
		if err != nil {
			return err
		}
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
	fmt.Printf("scratch kept at %s (yagmin never deletes)\n", scratch)
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

// normText normalizes the differences minification is allowed to make: variable names
// (every variable is renamed) and line:col positions (comments are dropped and
// actions re-printed, so everything moves).
var (
	normVar  = regexp.MustCompile(`\$[A-Za-z_][A-Za-z0-9_]*`)
	normLine = regexp.MustCompile(`(?::\d+|\bline \d+\b)`)
)

func normText(s string) string {
	s = normVar.ReplaceAllString(s, "$V")
	s = normLine.ReplaceAllString(s, ":L")
	return s
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

// fieldDiff is one differing field of one test's result.
type fieldDiff struct {
	field  string // what differed
	normal string
	mini   string
}

func (d fieldDiff) reason() string {
	return fmt.Sprintf("%s differs: %q vs %q", d.field, d.normal, d.mini)
}

// isAllowed reports whether the two texts match once variable names and positions
// are normalized away.
func isAllowed(d fieldDiff) bool {
	return normText(d.normal) == normText(d.mini)
}

// resultDiffs lists every field of the two results that differs.
func resultDiffs(a, b *loader.TestResult) []fieldDiff {
	var diffs []fieldDiff
	if a.Passed != b.Passed {
		diffs = append(diffs, fieldDiff{"passed", fmt.Sprint(a.Passed), fmt.Sprint(b.Passed)})
	}
	if (a.Error == nil) != (b.Error == nil) {
		av, bv := "<nil>", "<nil>"
		if a.Error != nil {
			av = a.Error.Error()
		}
		if b.Error != nil {
			bv = b.Error.Error()
		}
		diffs = append(diffs, fieldDiff{"error", av, bv})
	} else if a.Error != nil && a.Error.Error() != b.Error.Error() {
		diffs = append(diffs, fieldDiff{"error", a.Error.Error(), b.Error.Error()})
	}
	diffs = append(diffs, stringSliceDiffs("failure", a.Failures, b.Failures)...)
	diffs = append(diffs, stringSliceDiffs("warning", a.Warnings, b.Warnings)...)
	if a.Output != b.Output {
		diffs = append(diffs, fieldDiff{"output", a.Output, b.Output})
	}
	return diffs
}

// stringSliceDiffs pairs two lists of strings and returns the differing pairs.
func stringSliceDiffs(field string, a, b []string) []fieldDiff {
	var diffs []fieldDiff
	if len(a) != len(b) {
		return []fieldDiff{{field, fmt.Sprintf("%d entries", len(a)), fmt.Sprintf("%d entries", len(b))}}
	}
	for i := range a {
		if a[i] != b[i] {
			diffs = append(diffs, fieldDiff{
				field:  fmt.Sprintf("%s %d", field, i),
				normal: a[i],
				mini:   b[i],
			})
		}
	}
	return diffs
}
