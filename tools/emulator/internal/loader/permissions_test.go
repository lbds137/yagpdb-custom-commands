package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// guild.roles' permissions and guild.channels' permission_overwrites reach
// getTargetPermissionsIn (yaml → AvailableRoles/ChannelOverwrites → the permission math)
func TestPermissionFixturesPlumbThrough(t *testing.T) {
	dir := t.TempDir()
	suite := `
name: "permissions"
tests:
  - name: "an overwrite by type"
    template_source: |
      {{ getTargetPermissionsIn 2 10 }} {{ getTargetPermissionsIn 3 10 }}
    context:
      user: { id: 2 }
      guild:
        id: 1
        owner_id: 9
        roles: [{ id: 1, name: "@everyone", permissions: 1024 }]
        channels:
          - id: 10
            name: "text"
            permission_overwrites:
              - { id: 1, deny: 1024 }
              - { id: 3, type: member, allow: 1024 }
    expected:
      output_equals: "0 1024"
`
	path := filepath.Join(dir, "permissions_tests.yaml")
	if err := os.WriteFile(path, []byte(suite), 0o644); err != nil {
		t.Fatal(err)
	}
	tests, err := LoadTestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	res := NewRunner(RunnerConfig{BaseDir: dir}).RunTest(tests[0])
	if res.Error != nil || !res.Passed {
		t.Errorf("got error=%v failures=%q output=%q", res.Error, res.Failures, res.Output)
	}
}

// An overwrite's type is "role" or "member"; anything else is refused at load
func TestOverwriteTypeIsValidatedAtLoad(t *testing.T) {
	dir := t.TempDir()
	suite := `
name: "permissions"
tests:
  - name: "a bad type"
    template_source: "x"
    context:
      guild:
        channels:
          - { id: 10, name: "text", permission_overwrites: [{ id: 1, type: user }] }
`
	path := filepath.Join(dir, "bad_tests.yaml")
	if err := os.WriteFile(path, []byte(suite), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTestFile(path); err == nil || !strings.Contains(err.Error(), `write "role" or "member"`) {
		t.Errorf("want the type refused at load, got %v", err)
	}
}
