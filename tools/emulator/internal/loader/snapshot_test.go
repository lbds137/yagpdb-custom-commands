package loader

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func snapshotTest(dir, source string) *TestCase {
	tc := &TestCase{
		Name:           "greets",
		TemplateSource: source,
		Snapshot:       true,
		SourceFile:     filepath.Join(dir, "suite.yaml"),
	}
	tc.applyDefaults()
	return tc
}

func TestSnapshotLifecycle(t *testing.T) {
	dir := t.TempDir()
	src := `{{dbSet 0 "count" 1}}{{sendMessage nil (cembed "title" "Hi")}}` +
		`{{sendMessage nil (complexMessage "file" "file body" "filename" "notes")}}hello`

	// CI refuses to invent a missing snapshot
	ci := NewRunner(RunnerConfig{BaseDir: dir, CI: true})
	if res := ci.RunTest(snapshotTest(dir, src)); res.Passed {
		t.Fatal("CI run without a snapshot should fail")
	}

	// First local run writes it
	r := NewRunner(RunnerConfig{BaseDir: dir})
	res := r.RunTest(snapshotTest(dir, src))
	if !res.Passed || !res.SnapshotWritten {
		t.Fatalf("first run should pass and write: %+v", res)
	}
	data, err := os.ReadFile(filepath.Join(dir, "__snapshots__", "suite.snap.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"greets:", "output: hello", `"title": "Hi"`, "key: count", "filename: notes.txt", "content: file body"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("snapshot missing %q:\n%s", want, data)
		}
	}

	// Same output matches, in CI too
	if res := ci.RunTest(snapshotTest(dir, src)); !res.Passed || res.SnapshotWritten {
		t.Fatalf("unchanged run should pass: %+v", res)
	}

	// A change fails with a diff
	changed := strings.Replace(src, "hello", "goodbye", 1)
	res = r.RunTest(snapshotTest(dir, changed))
	if res.Passed || len(res.Failures) != 1 {
		t.Fatalf("changed output should fail: %+v", res)
	}
	if f := res.Failures[0]; !strings.Contains(f, "- output: hello") || !strings.Contains(f, "+ output: goodbye") {
		t.Errorf("failure should show the diff:\n%s", f)
	}

	// -update-snapshots accepts the change
	u := NewRunner(RunnerConfig{BaseDir: dir, UpdateSnapshots: true})
	if res := u.RunTest(snapshotTest(dir, changed)); !res.Passed || !res.SnapshotWritten {
		t.Fatalf("update should pass and write: %+v", res)
	}
	if res := r.RunTest(snapshotTest(dir, changed)); !res.Passed {
		t.Fatalf("after update the change should match: %+v", res.Failures)
	}
}

// A failed run with no output posts "\nAn error caused...": yaml.v3 alone writes that, in
// a list, as a block scalar it can't read back
func TestSnapshotStringsReadBack(t *testing.T) {
	dir := t.TempDir()
	src := `{{sendMessage nil "\nAn error:\nline two"}}`
	r := NewRunner(RunnerConfig{BaseDir: dir})
	if res := r.RunTest(snapshotTest(dir, src)); !res.Passed || !res.SnapshotWritten {
		t.Fatalf("first run should pass and write: %+v", res)
	}
	if res := r.RunTest(snapshotTest(dir, src)); !res.Passed || res.SnapshotWritten {
		t.Fatalf("the saved snapshot should read back and match: %+v", res.Failures)
	}

	// Every string of up to 3 of the characters YAML treats specially
	chars := []string{"\n", "\t", " ", "\r", "x", "#", ":", "-", "'", `"`, "|", "\u00a0"}
	strs := []string{""}
	for n := 0; n < 3; n++ {
		for _, prefix := range strs {
			if len([]rune(prefix)) == n {
				for _, c := range chars {
					strs = append(strs, prefix+c)
				}
			}
		}
	}
	for _, str := range strs {
		s := snapText(str)
		snaps := map[string]Snapshot{"t": {Output: s, Messages: []SnapshotMessage{{Content: s, Embed: s}},
			Files: []SnapshotFile{{Filename: s, Content: s}}, DB: []SnapshotEntry{{Key: s, Value: s}}}}
		data, err := encodeSnapshots(snaps)
		if err != nil {
			t.Errorf("%q: %v", s, err)
			continue
		}
		// Checked apart from encodeSnapshots' own check
		var back map[string]Snapshot
		if err := yaml.Unmarshal(data, &back); err != nil || !reflect.DeepEqual(back, snaps) {
			t.Errorf("%q doesn't read back: %v\n%s", s, err, data)
		}
	}
}

func TestLineDiff(t *testing.T) {
	got := lineDiff("a\nb\nc\n", "a\nx\nc\n")
	if got != "    - b\n    + x" && got != "    + x\n    - b" {
		t.Errorf("got %q", got)
	}
}

func TestLoadTestsFromDirSkipsSnapshots(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "t.yaml"), []byte("tests:\n  - name: one\n    template_source: hi\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "__snapshots__"), 0o755)
	os.WriteFile(filepath.Join(dir, "__snapshots__", "t.snap.yaml"), []byte("one:\n  output: hi\n"), 0o644)

	tests, err := LoadTestsFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tests) != 1 || tests[0].SourceFile != filepath.Join(dir, "t.yaml") {
		t.Errorf("got %d tests: %+v", len(tests), tests)
	}
}

func TestSnapshotDuplicateNamesFail(t *testing.T) {
	dir := t.TempDir()
	r := NewRunner(RunnerConfig{BaseDir: dir})
	results := r.RunTests([]*TestCase{snapshotTest(dir, "one"), snapshotTest(dir, "two")})
	for _, res := range results {
		if res.Passed || !strings.Contains(strings.Join(res.Failures, ""), "also named") {
			t.Errorf("duplicate names should fail: %+v", res)
		}
	}
}

func TestUpdateSnapshotsWritesInCI(t *testing.T) {
	dir := t.TempDir()
	r := NewRunner(RunnerConfig{BaseDir: dir, CI: true, UpdateSnapshots: true})
	if res := r.RunTest(snapshotTest(dir, "hi")); !res.Passed || !res.SnapshotWritten {
		t.Errorf("an explicit update should write even in CI: %+v", res)
	}
}

func TestPruneSnapshots(t *testing.T) {
	dir := t.TempDir()
	r := NewRunner(RunnerConfig{BaseDir: dir})
	kept := snapshotTest(dir, "hi")
	old := snapshotTest(dir, "bye")
	old.Name = "renamed away"
	r.RunTests([]*TestCase{kept, old})

	optedOut := snapshotTest(dir, "x")
	optedOut.Name = "no longer a snapshot test"
	optedOut.Snapshot = false
	stale, err := StaleSnapshots([]*TestCase{kept, optedOut})
	want := SnapshotPath(kept.SourceFile) + ": renamed away"
	if err != nil || len(stale) != 1 || stale[0] != want {
		t.Fatalf("stale = %q, %v; want [%q]", stale, err, want)
	}
	if snaps, _ := readSnapshots(SnapshotPath(kept.SourceFile)); len(snaps) != 2 {
		t.Fatalf("StaleSnapshots changed the file: %v", snaps)
	}
	removed, err := PruneSnapshots([]*TestCase{kept, optedOut})
	if err != nil || removed != 1 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	snaps, _ := readSnapshots(SnapshotPath(kept.SourceFile))
	if _, ok := snaps["greets"]; !ok || len(snaps) != 1 {
		t.Errorf("want only the kept snapshot, got %v", snaps)
	}
}

// Staleness is per suite file: a name another file still has doesn't keep an entry
func TestStaleSnapshotsPerFile(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	r := NewRunner(RunnerConfig{})
	a, b := snapshotTest(dirA, "a"), snapshotTest(dirB, "b")
	gone := snapshotTest(dirA, "c")
	gone.Name = "gone"
	r.RunTests([]*TestCase{a, b, gone})

	onlyB := snapshotTest(dirB, "b")
	onlyB.Name = "gone" // the same name, in the other file
	stale, err := StaleSnapshots([]*TestCase{a, b, onlyB})
	want := SnapshotPath(a.SourceFile) + ": gone"
	if err != nil || len(stale) != 1 || stale[0] != want {
		t.Errorf("stale = %q, %v; want [%q]", stale, err, want)
	}
}

// An unreadable snapshot file is reported, and the other files are still checked
func TestStaleSnapshotsPastAnUnreadableFile(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	a, b := snapshotTest(dirA, "a"), snapshotTest(dirB, "b")
	gone := snapshotTest(dirB, "c")
	gone.Name = "gone"
	NewRunner(RunnerConfig{}).RunTests([]*TestCase{a, b, gone})
	os.WriteFile(SnapshotPath(a.SourceFile), []byte("x: [\n"), 0o644)

	for i := 0; i < 5; i++ { // map order must not decide what's found
		stale, err := StaleSnapshots([]*TestCase{a, b})
		want := SnapshotPath(b.SourceFile) + ": gone"
		if err == nil || len(stale) != 1 || stale[0] != want {
			t.Fatalf("stale = %q, %v; want [%q] and an error", stale, err, want)
		}
	}
}
