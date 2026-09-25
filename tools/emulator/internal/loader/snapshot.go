package loader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/runtime"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
)

// Snapshot records what a test run produced: its output and side effects.
type Snapshot struct {
	Output      string            `yaml:"output"`
	Messages    []SnapshotMessage `yaml:"messages,omitempty"`
	Edits       []SnapshotMessage `yaml:"edits,omitempty"`
	RoleChanges []string          `yaml:"role_changes,omitempty"`
	DB          []SnapshotEntry   `yaml:"db,omitempty"`
}

// SnapshotMessage is a sent message. Embeds are stored as indented JSON.
type SnapshotMessage struct {
	ChannelID int64  `yaml:"channel_id"`
	Content   string `yaml:"content,omitempty"`
	Embed     string `yaml:"embed,omitempty"`
}

// SnapshotEntry is a database entry after the run. Values are stored as JSON.
type SnapshotEntry struct {
	UserID int64  `yaml:"user_id"`
	Key    string `yaml:"key"`
	Value  string `yaml:"value"`
}

// SnapshotPath is where snapshots for tests from sourceFile are kept:
// __snapshots__/<file>.snap.yaml next to the test file.
func SnapshotPath(sourceFile string) string {
	base := strings.TrimSuffix(filepath.Base(sourceFile), filepath.Ext(sourceFile))
	return filepath.Join(filepath.Dir(sourceFile), "__snapshots__", base+".snap.yaml")
}

func takeSnapshot(output string, ctx *runtime.ExecutionContext, db *state.MockDB) Snapshot {
	snap := Snapshot{Output: strings.TrimSpace(output)}
	snap.Messages = snapshotMessages(ctx.SentMessages)
	snap.Edits = snapshotMessages(ctx.EditedMessages)
	for _, rc := range ctx.RoleChanges {
		snap.RoleChanges = append(snap.RoleChanges, fmt.Sprintf("%s role %d for user %d", rc.Action, rc.RoleID, rc.UserID))
	}
	entries := db.GetAll()
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].UserID != entries[j].UserID {
			return entries[i].UserID < entries[j].UserID
		}
		return entries[i].Key < entries[j].Key
	})
	for _, e := range entries {
		snap.DB = append(snap.DB, SnapshotEntry{UserID: e.UserID, Key: e.Key, Value: compactJSON(e.Value)})
	}
	return snap
}

func snapshotMessages(messages []runtime.SentMessage) []SnapshotMessage {
	var out []SnapshotMessage
	for _, msg := range messages {
		sm := SnapshotMessage{ChannelID: msg.ChannelID, Content: msg.Content}
		if msg.Embed != nil {
			sm.Embed = readableJSON(msg.Embed)
		}
		out = append(out, sm)
	}
	return out
}

func compactJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<cannot encode as JSON: %v>", err)
	}
	return string(data)
}

// readableJSON is indented JSON without HTML escaping, so mentions stay "<@id>".
func readableJSON(v interface{}) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Sprintf("<cannot encode as JSON: %v>", err)
	}
	return strings.TrimRight(buf.String(), "\n")
}

func readSnapshots(path string) (map[string]Snapshot, error) {
	snaps := map[string]Snapshot{}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return snaps, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &snaps); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return snaps, nil
}

func writeSnapshot(path, name string, snap Snapshot) error {
	snaps, err := readSnapshots(path)
	if err != nil {
		return err
	}
	snaps[name] = snap
	return saveSnapshots(path, snaps)
}

func saveSnapshots(path string, snaps map[string]Snapshot) error {
	data, err := yaml.Marshal(snaps)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// checkSnapshot compares a run with its saved snapshot. A missing snapshot is written,
// except in CI, where it fails. With UpdateSnapshots, the snapshot is always rewritten.
func (r *Runner) checkSnapshot(tc *TestCase, output string, ctx *runtime.ExecutionContext, db *state.MockDB) ([]string, bool) {
	if tc.SourceFile == "" {
		return []string{"snapshot: the test has no source file to store its snapshot next to"}, false
	}
	if r.duplicateNames[snapshotKey(tc)] {
		return []string{fmt.Sprintf("snapshot: another test in %s is also named %q; snapshots are stored by name, so rename one",
			tc.SourceFile, tc.Name)}, false
	}
	path := SnapshotPath(tc.SourceFile)
	got := takeSnapshot(output, ctx, db)

	snaps, err := readSnapshots(path)
	if err != nil {
		return []string{fmt.Sprintf("snapshot: %v", err)}, false
	}
	want, exists := snaps[tc.Name]

	if r.config.UpdateSnapshots || !exists {
		if !exists && r.config.CI && !r.config.UpdateSnapshots {
			return []string{fmt.Sprintf("snapshot: none saved for %q in %s (run yagtest test -update-snapshots and commit it)", tc.Name, path)}, false
		}
		if err := writeSnapshot(path, tc.Name, got); err != nil {
			return []string{fmt.Sprintf("snapshot: writing %s: %v", path, err)}, false
		}
		return nil, true
	}

	wantYAML, _ := yaml.Marshal(want)
	gotYAML, _ := yaml.Marshal(got)
	if string(wantYAML) == string(gotYAML) {
		return nil, false
	}
	return []string{fmt.Sprintf("snapshot mismatch (%s); if the change is intended, rerun with -update-snapshots\n%s",
		path, lineDiff(string(wantYAML), string(gotYAML)))}, false
}

// lineDiff returns the changed lines between two texts, "-" for removed and "+" for added.
func lineDiff(a, b string) string {
	x := strings.Split(strings.TrimRight(a, "\n"), "\n")
	y := strings.Split(strings.TrimRight(b, "\n"), "\n")

	// Longest common subsequence table
	lcs := make([][]int, len(x)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(y)+1)
	}
	for i := len(x) - 1; i >= 0; i-- {
		for j := len(y) - 1; j >= 0; j-- {
			if x[i] == y[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	const maxLines = 40
	var out []string
	i, j := 0, 0
	for (i < len(x) || j < len(y)) && len(out) < maxLines {
		switch {
		case i < len(x) && j < len(y) && x[i] == y[j]:
			i++
			j++
		case j < len(y) && (i == len(x) || lcs[i][j+1] >= lcs[i+1][j]):
			out = append(out, "    + "+y[j])
			j++
		default:
			out = append(out, "    - "+x[i])
			i++
		}
	}
	if len(out) == maxLines {
		out = append(out, "    ...")
	}
	return strings.Join(out, "\n")
}

func snapshotKey(tc *TestCase) string {
	return tc.SourceFile + "\x00" + tc.Name
}

// findDuplicateNames marks snapshot tests that share a name with another test in the
// same file.
func findDuplicateNames(tests []*TestCase) map[string]bool {
	seen := map[string]int{}
	for _, tc := range tests {
		seen[snapshotKey(tc)]++
	}
	dups := map[string]bool{}
	for key, n := range seen {
		if n > 1 {
			dups[key] = true
		}
	}
	return dups
}

// PruneSnapshots removes saved snapshots whose test no longer exists or no longer has
// snapshot: true, for every test file among tests. Returns how many were removed.
func PruneSnapshots(tests []*TestCase) (int, error) {
	keep := map[string]map[string]bool{}
	for _, tc := range tests {
		if tc.SourceFile == "" {
			continue
		}
		if keep[tc.SourceFile] == nil {
			keep[tc.SourceFile] = map[string]bool{}
		}
		if tc.Snapshot {
			keep[tc.SourceFile][tc.Name] = true
		}
	}

	removed := 0
	for source, names := range keep {
		path := SnapshotPath(source)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		snaps, err := readSnapshots(path)
		if err != nil {
			return removed, err
		}
		before := len(snaps)
		for name := range snaps {
			if !names[name] {
				delete(snaps, name)
			}
		}
		if len(snaps) == before {
			continue
		}
		removed += before - len(snaps)
		if err := saveSnapshots(path, snaps); err != nil {
			return removed, err
		}
	}
	return removed, nil
}
