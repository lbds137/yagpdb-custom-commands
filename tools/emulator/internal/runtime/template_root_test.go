package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An execCC child's warning names the command as the test's command_map wrote it, not the
// path under TemplateRoot it was read from, so a normal run and a run against minified
// copies print the same name (yagmin's prove compares them).
func TestChildWarningNamesTheUnremappedPath(t *testing.T) {
	const rel = "commands/x/child.gohtml"
	const child = `{{range seq 0 3}}{{dbSet 1 "k" .}}{{end}}`
	base, root := t.TempDir(), t.TempDir()
	for _, dir := range []string{base, root} {
		if err := os.MkdirAll(filepath.Join(dir, "commands", "x"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, rel), child)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Rel(wd, filepath.Join(base, rel))
	if err != nil {
		t.Fatal(err)
	}

	warn := func(templateRoot string) string {
		ctx := newCtx(false, true)
		ctx.TemplateBaseDir = base
		ctx.TemplateRoot = templateRoot
		ctx.CommandIDMap = map[int64]string{7: rel}
		if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}`); err != nil {
			t.Fatal(err)
		}
		msgs := kinds(ctx, KindLoopDB)
		if len(msgs) != 1 {
			t.Fatalf("TemplateRoot %q: loop-db warnings %q", templateRoot, msgs)
		}
		return msgs[0]
	}

	plain, remapped := warn(""), warn(root)
	if !strings.Contains(plain, want+":1:") {
		t.Errorf("normal run's warning %q doesn't name %s", plain, want)
	}
	if remapped != plain {
		t.Errorf("TemplateRoot run's warning differs:\n got %q\nwant %q", remapped, plain)
	}
	if strings.Contains(remapped, root) || strings.Contains(remapped, filepath.Base(root)) {
		t.Errorf("warning names the remapped root %s: %q", root, remapped)
	}
}
