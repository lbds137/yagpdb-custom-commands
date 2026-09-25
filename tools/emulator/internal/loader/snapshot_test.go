package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	src := `{{dbSet 0 "count" 1}}{{sendMessage nil (cembed "title" "Hi")}}hello`

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
	for _, want := range []string{"greets:", "output: hello", `"title": "Hi"`, "key: count"} {
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
