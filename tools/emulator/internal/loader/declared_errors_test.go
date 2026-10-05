package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadSuiteSource(t *testing.T, src string) ([]*TestCase, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s_tests.yaml")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return LoadTestFile(path)
}

// command_status merges like command_map: the suite's entries reach each test, the test's win.
func TestCommandStatusMergesSuiteIntoTests(t *testing.T) {
	tests, err := loadSuiteSource(t, `
command_status:
  7: disabled
  8: missing
tests:
  - name: inherits
    template_source: x
  - name: overrides
    template_source: x
    command_status:
      7: group_disabled
`)
	if err != nil {
		t.Fatal(err)
	}
	if tests[0].CommandStatus[7] != "disabled" || tests[0].CommandStatus[8] != "missing" {
		t.Errorf("inherits: %v", tests[0].CommandStatus)
	}
	if tests[1].CommandStatus[7] != "group_disabled" || tests[1].CommandStatus[8] != "missing" {
		t.Errorf("overrides: %v", tests[1].CommandStatus)
	}
}

func TestUnknownCommandStatusIsALoaderError(t *testing.T) {
	for name, src := range map[string]string{
		"test level": `
tests:
  - name: bad
    template_source: x
    command_status:
      7: gone
`,
		"suite level": `
command_status:
  7: gone
tests:
  - name: bad
    template_source: x
`,
		"single test": `
name: single
template_source: x
command_status:
  7: gone
`,
	} {
		if _, err := loadSuiteSource(t, src); err == nil || !strings.Contains(err.Error(), `unknown status "gone"`) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestExecLineInBothResponsesAndErrorsIsALoaderError(t *testing.T) {
	_, err := loadSuiteSource(t, `
tests:
  - name: both
    template_source: x
    context:
      exec_responses:
        "kick 5": "ok"
      exec_errors:
        "kick 5": "nope"
`)
	if err == nil || !strings.Contains(err.Error(), "is in both exec_responses and exec_errors") {
		t.Errorf("got %v", err)
	}
	if _, err = loadSuiteSource(t, `
tests:
  - name: fine
    template_source: x
    context:
      exec_errors:
        "kick 5": "nope"
`); err != nil {
		t.Errorf("exec_errors alone should load: %v", err)
	}
}

// End to end: command_status and exec_errors reach the run.
func TestDeclaredErrorsRunEndToEnd(t *testing.T) {
	tests, err := loadSuiteSource(t, `
tests:
  - name: both
    template_source: '{{try}}{{execCC 9 nil 0 nil}}{{catch}}a={{.Error}};{{end}}{{try}}{{exec "kick" 5}}{{catch}}b={{.Error}}{{end}}'
    command_status:
      9: disabled
    context:
      exec_errors:
        "kick 5": "Missing Permissions"
    expected:
      output_equals: "a=custom command is disabled;b=exec/execadmin, run: Missing Permissions"
`)
	if err != nil {
		t.Fatal(err)
	}
	res := NewRunner(RunnerConfig{BaseDir: t.TempDir()}).RunTest(tests[0])
	if res.Error != nil || len(res.Failures) != 0 {
		t.Errorf("err=%v failures=%q", res.Error, res.Failures)
	}
}
