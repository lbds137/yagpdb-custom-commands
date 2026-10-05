package runtime

import (
	"os"
	"path/filepath"
	"strings"
)

// RemapTemplateRoot rewrites a test's relative template path so that a path under the
// repository's commands/ tree (however many .. it climbs out of the testdata dir) is read
// from under root instead, as an absolute path. Anything else — local fixture paths,
// absolute paths, retired/ and docs/ — is left for the caller to resolve as usual.
// root empty reports ok=false, so callers only need one call site.
//
// yagmin's equivalence proof uses this to run the whole YAML suite against minified
// copies of every command while the rest of the paths resolve as normal.
func RemapTemplateRoot(root, rel string) (string, bool) {
	if root == "" || filepath.IsAbs(rel) {
		return "", false
	}
	p := filepath.Clean(rel)
	sep := string(filepath.Separator)
	for strings.HasPrefix(p, ".."+sep) {
		p = strings.TrimPrefix(p, ".."+sep)
	}
	if p != "commands" && !strings.HasPrefix(p, "commands"+sep) {
		return "", false
	}
	abs, err := filepath.Abs(filepath.Join(root, p))
	if err != nil {
		return "", false
	}
	return abs, true
}

// DisplayPath returns a template path relative to the working directory, so warnings
// print as clickable file:line locations. A relative template is joined with baseDir
// first. It takes the path as the test wrote it, before RemapTemplateRoot, so a run
// against minified copies names the same file as a normal run.
func DisplayPath(baseDir, template string) string {
	if template == "" {
		return ""
	}
	p := template
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, p)
	}
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, p); err == nil {
			return rel
		}
	}
	return p
}
