package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupTemplatesRunFirstOnTheSameDatabase(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("seed.gohtml", `{{dbSet 0 "seeded" "yes"}}{{sendMessage nil "from setup"}}`)
	write("broken.gohtml", `{{index (cslice) 3}}`)

	tc := &TestCase{
		Name:           "reads the seed",
		TemplateSource: `{{(dbGet 0 "seeded").Value}}`,
		SetupTemplates: []string{"seed.gohtml"},
		Expected:       ExpectedResult{OutputEquals: "yes"},
		Assertions:     Assertions{SentMessages: []MessageCheck{{ContentEquals: "from test"}}},
	}
	tc.applyDefaults()
	r := NewRunner(RunnerConfig{BaseDir: dir})
	res := r.RunTest(tc)
	// The setup's message isn't the test's: the only message check fails, nothing else
	if res.Error != nil || len(res.Failures) != 1 || !strings.Contains(res.Failures[0], "no messages sent") {
		t.Errorf("want only the message check to fail: err=%v failures=%q", res.Error, res.Failures)
	}

	tc.SetupTemplates = []string{"broken.gohtml"}
	if res := r.RunTest(tc); res.Error == nil || !strings.Contains(res.Error.Error(), "setup template broken.gohtml") {
		t.Errorf("a failing setup template should fail the test and name itself: %v", res.Error)
	}
}
