package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A guild.channels entry's available_tags reach the channel's details, so a forum post
// can apply them by name (yaml → ChannelDetails → createForumPost's tagIDFromName)
func TestForumAvailableTagsPlumbThrough(t *testing.T) {
	dir := t.TempDir()
	suite := `
name: "forum tags"
tests:
  - name: "a post applies a declared tag"
    template_source: |
      {{ $p := createForumPost 77 "t" "x" "tags" "news" }}{{ $p.AppliedTags }}
    context:
      guild:
        channels:
          - { id: 77, name: "forum", type: 15, available_tags: [{id: 101, name: "news"}] }
    expected:
      output_equals: "[101]"
`
	path := filepath.Join(dir, "forum_tags_tests.yaml")
	if err := os.WriteFile(path, []byte(suite), 0o644); err != nil {
		t.Fatal(err)
	}
	tests, err := LoadTestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res := NewRunner(RunnerConfig{BaseDir: dir}).RunTest(tests[0])
	if res.Error != nil || !res.Passed {
		t.Errorf("want the post's tag applied, got error=%v failures=%q warnings=%q",
			res.Error, res.Failures, res.Warnings)
	}
	if !strings.Contains(res.Output, "[101]") {
		t.Errorf("output %q should list the applied tag", res.Output)
	}
}
