package loader

import (
	"os"
	"path/filepath"
	"testing"
)

// guild.roles' managed reaches .Managed (yaml → AvailableRoles), through getRole and
// through a slash role option's resolved value
func TestManagedRoleFixturePlumbsThrough(t *testing.T) {
	dir := t.TempDir()
	suite := `
name: "managed"
tests:
  - name: "getRole"
    template_source: |
      {{ (getRole 5).Managed }} {{ (getRole 6).Managed }}
    context:
      guild:
        id: 1
        roles:
          - { id: 1, name: "@everyone" }
          - { id: 5, name: "Bot", managed: true }
          - { id: 6, name: "Plain" }
    expected:
      output_equals: "true false"
  - name: "a slash role option"
    template_source: |
      {{/*
        Trigger type: ` + "`Slash Command`" + `
        Trigger: ` + "`probe`" + `
        Slash description: ` + "`probe`" + `
        Slash option: ` + "`role role the role`" + `
      */}}
      {{ .Options.role.Managed }}
    context:
      guild:
        id: 1
        roles:
          - { id: 1, name: "@everyone" }
          - { id: 5, name: "Bot", managed: true }
      interaction: { type: slash, options: { role: 5 } }
    expected:
      output_equals: "true"
`
	path := filepath.Join(dir, "managed_tests.yaml")
	if err := os.WriteFile(path, []byte(suite), 0o644); err != nil {
		t.Fatal(err)
	}
	tests, err := LoadTestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range tests {
		res := NewRunner(RunnerConfig{BaseDir: dir}).RunTest(tc)
		if res.Error != nil || !res.Passed {
			t.Errorf("%s: error=%v failures=%q output=%q", tc.Name, res.Error, res.Failures, res.Output)
		}
	}
}
