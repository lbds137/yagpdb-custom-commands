package loader

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// countingRunner returns a Runner whose snapshot parser counts its calls, so a test can
// assert parse-once behavior. The count is only touched under the Runner's cache lock
// (cachedSnapshots parses while holding it), so it is safe for concurrent reads.
func countingRunner(dir string) (*Runner, *int) {
	r := NewRunner(RunnerConfig{BaseDir: dir})
	calls := 0
	r.parseSnapshots = func(path string) (map[string]Snapshot, error) {
		calls++
		return readSnapshots(path)
	}
	return r, &calls
}

func writeSnapFile(t *testing.T, path string, snaps map[string]Snapshot) {
	t.Helper()
	if err := saveSnapshots(path, snaps); err != nil {
		t.Fatal(err)
	}
}

// Three reads of one unchanged file parse it once
func TestSnapshotCacheParsesOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "suite.snap.yaml")
	writeSnapFile(t, path, map[string]Snapshot{"one": {Output: "hi"}})

	r, calls := countingRunner(dir)
	for i := 0; i < 3; i++ {
		snaps, err := r.cachedSnapshots(path)
		if err != nil {
			t.Fatal(err)
		}
		if snaps["one"].Output != "hi" {
			t.Fatalf("read %d: got output %q, want \"hi\"", i, snaps["one"].Output)
		}
	}
	if *calls != 1 {
		t.Errorf("snapshot file parsed %d times for 3 reads, want 1", *calls)
	}
}

// A changed file re-parses: the stamp check invalidates the entry
func TestSnapshotCacheReparsesChangedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "suite.snap.yaml")
	writeSnapFile(t, path, map[string]Snapshot{"one": {Output: "hi"}})

	r, calls := countingRunner(dir)
	if _, err := r.cachedSnapshots(path); err != nil {
		t.Fatal(err)
	}
	writeSnapFile(t, path, map[string]Snapshot{"one": {Output: "changed"}})
	snaps, err := r.cachedSnapshots(path)
	if err != nil {
		t.Fatal(err)
	}
	if snaps["one"].Output != "changed" {
		t.Errorf("read after the file changed: got output %q, want \"changed\"", snaps["one"].Output)
	}
	if *calls != 2 {
		t.Errorf("snapshot file parsed %d times, want 2 (once per file version)", *calls)
	}
}

// A Runner's own write is visible to its next read
func TestSnapshotCacheWriteVisible(t *testing.T) {
	dir := t.TempDir()
	path := SnapshotPath(filepath.Join(dir, "suite.yaml"))
	r := NewRunner(RunnerConfig{BaseDir: dir})
	if _, err := r.cachedSnapshots(path); err != nil {
		t.Fatal(err)
	}
	if err := r.writeSnapshot(path, "late", Snapshot{Output: "written"}); err != nil {
		t.Fatal(err)
	}
	snaps, err := r.cachedSnapshots(path)
	if err != nil {
		t.Fatal(err)
	}
	if snaps["late"].Output != "written" {
		t.Errorf("read after writeSnapshot: got output %q, want \"written\"", snaps["late"].Output)
	}
}

// The lifecycle flow: a Runner keeps reading a snapshot file that another Runner
// (-update-snapshots) has rewritten, and must see the rewrite. The file exists before
// the reader's first read, so the reader holds a cache entry the rewrite must
// invalidate — the final read goes through the stamp check, not a plain cache miss.
func TestSnapshotCacheSeesAnotherRunnersWrite(t *testing.T) {
	dir := t.TempDir()
	path := SnapshotPath(filepath.Join(dir, "suite.yaml"))
	reader, calls := countingRunner(dir)
	writer := NewRunner(RunnerConfig{BaseDir: dir, UpdateSnapshots: true})

	writeSnapFile(t, path, map[string]Snapshot{"greets": {Output: "hello"}})
	if _, err := reader.cachedSnapshots(path); err != nil {
		t.Fatal(err)
	}
	if err := writer.writeSnapshot(path, "greets", Snapshot{Output: "goodbye"}); err != nil {
		t.Fatal(err)
	}
	snaps, err := reader.cachedSnapshots(path)
	if err != nil {
		t.Fatal(err)
	}
	if snaps["greets"].Output != "goodbye" {
		t.Errorf("the first Runner read stale output %q after another Runner rewrote the file",
			snaps["greets"].Output)
	}
	if *calls != 2 {
		t.Errorf("snapshot file parsed %d times, want 2 (once per file version)", *calls)
	}
}

// Concurrent reads, and reads racing a write, stay consistent (run under -race)
func TestSnapshotCacheConcurrentUse(t *testing.T) {
	dir := t.TempDir()
	path := SnapshotPath(filepath.Join(dir, "suite.yaml"))
	if err := NewRunner(RunnerConfig{}).writeSnapshot(path, "base", Snapshot{Output: "hi"}); err != nil {
		t.Fatal(err)
	}
	r := NewRunner(RunnerConfig{BaseDir: dir})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				snaps, err := r.cachedSnapshots(path)
				if err != nil {
					t.Errorf("read: %v", err)
					return
				}
				if snaps["base"].Output != "hi" {
					t.Errorf("read got output %q, want \"hi\"", snaps["base"].Output)
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 5; j++ {
			if err := r.writeSnapshot(path, "extra", Snapshot{Output: "x"}); err != nil {
				t.Errorf("write: %v", err)
				return
			}
		}
	}()
	wg.Wait()
}

// A corrupt snapshot file surfaces the parse error and is not cached: after the file
// is fixed, the next read parses it fresh
func TestSnapshotCacheErrorNotCached(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "suite.snap.yaml")
	if err := os.WriteFile(path, []byte("not: [valid: yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	r, calls := countingRunner(dir)
	if _, err := r.cachedSnapshots(path); err == nil {
		t.Fatal("reading a corrupt snapshot file returned no error")
	}
	writeSnapFile(t, path, map[string]Snapshot{"one": {Output: "hi"}})
	snaps, err := r.cachedSnapshots(path)
	if err != nil {
		t.Fatalf("reading the fixed snapshot file: %v", err)
	}
	if snaps["one"].Output != "hi" {
		t.Errorf("got output %q after fixing the file, want \"hi\"", snaps["one"].Output)
	}
	if *calls != 2 {
		t.Errorf("snapshot file parsed %d times, want 2 (a failed parse must not be cached)", *calls)
	}
}
