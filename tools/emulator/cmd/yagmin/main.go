// yagmin: the CLI front end. See minify.go for what minification does.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}
	switch args[0] {
	case "batch":
		batch(args[1:])
	case "check":
		check(args[1:])
	case "prove":
		prove(args[1:])
	case "help", "-h", "--help":
		usage()
	default:
		minifyFile(args)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `yagmin - parser-based minifier for free-tier command size

Usage:
    yagmin [-o <out>] <file.gohtml>   Minify one file; print to stdout, or write to <out>
    yagmin batch <flags>              Minify a tree of commands
    yagmin check <flags>              Fail when a tree of copies is not what batch
                                      would write (stale, missing or extra files)
    yagmin prove <flags>              Equivalence proof: run the whole YAML suite on
                                      minified copies of every command, and compare

Batch and check flags:
    -src <dir>   Tree to minify (default commands)
    -dst <dir>   Where the copies go (batch writes there, check reads; required);
                 mirrors src's layout
    -over <n>    Only files over n runes (0 = every file)

Prove flags:
    -commands <dir>  Command tree (default commands)
    -testdata <dir>  YAML suite directory (default tools/emulator/testdata)
    -schema <file>   Type schema for stored values (default db_schema.yaml)
    -scratch <dir>   Where to put the minified copies (default: a temp dir, removed
                     when the run ends; a directory given here is kept)
`)
}

// minifyFile handles the one-file form: yagmin [-o out] file.gohtml.
func minifyFile(args []string) {
	fs := flag.NewFlagSet("yagmin", flag.ExitOnError)
	out := fs.String("o", "", "Write the minified output here instead of stdout")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: yagmin [-o <out>] <file.gohtml>")
		os.Exit(1)
	}
	path := fs.Arg(0)
	source, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading %s: %v\n", path, err)
		os.Exit(1)
	}
	minified, err := minify(filepath.Base(path), string(source))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
		os.Exit(1)
	}
	if *out == "" {
		fmt.Print(minified)
		return
	}
	if err := os.WriteFile(*out, []byte(minified), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "writing %s: %v\n", *out, err)
		os.Exit(1)
	}
}

// batch handles: yagmin batch -src DIR -dst DIR [-over N]. Every .gohtml under src
// whose rune count is over the threshold is minified into dst under the same
// relative path. Batch never deletes anything; files that fall under the threshold
// keep their stale copy until someone removes it.
func batch(args []string) {
	fs := flag.NewFlagSet("batch", flag.ExitOnError)
	src := fs.String("src", "commands", "Tree to minify")
	dst := fs.String("dst", "", "Where to write the minified copies (required)")
	over := fs.Int("over", 0, "Only files over this many runes (0 = every file)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *dst == "" {
		fmt.Fprintln(os.Stderr, "batch: -dst is required")
		os.Exit(1)
	}
	if err := batchRun(*src, *dst, *over); err != nil {
		fmt.Fprintf(os.Stderr, "batch: %v\n", err)
		os.Exit(1)
	}
}

// batchRun is batch's body, separate so prove and tests can reuse it.
func batchRun(src, dst string, over int) error {
	written := 0
	err := eachMinified(src, over, func(rel, path string, source, minified []byte) error {
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, minified, 0o644); err != nil {
			return err
		}
		written++
		fmt.Printf("%s: %d -> %d runes\n", path, utf8.RuneCount(source), utf8.RuneCount(minified))
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("minified %d file(s) into %s\n", written, dst)
	return nil
}

// check handles: yagmin check -src DIR -dst DIR [-over N], the freshness check behind
// make minify-check. Nothing is written: every file batch would write is minified in
// memory and compared with dst's copy, and any other file under dst is reported as
// extra (batch never deletes). Exit 1 lists each problem with the fix.
func check(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	src := fs.String("src", "commands", "Tree to minify")
	dst := fs.String("dst", "", "Tree of copies to check (required)")
	over := fs.Int("over", 0, "Only files over this many runes (0 = every file)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *dst == "" {
		fmt.Fprintln(os.Stderr, "check: -dst is required")
		os.Exit(1)
	}
	problems, err := checkRun(*src, *dst, *over)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check: %v\n", err)
		os.Exit(1)
	}
	if len(problems) > 0 {
		fmt.Printf("❌ %s is out of date\n", *dst)
		for _, p := range problems {
			fmt.Printf("   %s\n", p)
		}
		fmt.Println("   Fix: make minify   (and remove any file it no longer writes; it never deletes)")
		os.Exit(1)
	}
	fmt.Printf("✅ %s is up to date\n", *dst)
}

// checkRun is check's body: one line per stale, missing or extra file under dst, or
// one line when dst itself is missing.
func checkRun(src, dst string, over int) ([]string, error) {
	if info, err := os.Stat(dst); os.IsNotExist(err) || (err == nil && !info.IsDir()) {
		return []string{fmt.Sprintf("missing: %s is not a directory; nothing minified yet", dst)}, nil
	} else if err != nil {
		return nil, err
	}
	var problems []string
	expected := map[string]bool{}
	err := eachMinified(src, over, func(rel, path string, source, minified []byte) error {
		expected[rel] = true
		current, err := os.ReadFile(filepath.Join(dst, rel))
		switch {
		case os.IsNotExist(err):
			problems = append(problems, fmt.Sprintf("missing: %s (from %s)", filepath.Join(dst, rel), path))
		case err != nil:
			return err
		case string(current) != string(minified):
			problems = append(problems, fmt.Sprintf("stale: %s (from %s)", filepath.Join(dst, rel), path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	copies, err := listFiles(dst, func(string) bool { return true })
	if err != nil {
		return nil, err
	}
	for _, path := range copies {
		rel, err := filepath.Rel(dst, path)
		if err != nil {
			return nil, err
		}
		if !expected[rel] {
			problems = append(problems,
				fmt.Sprintf("extra: %s (no command over the threshold writes it)", path))
		}
	}
	return problems, nil
}

// eachMinified minifies every .gohtml under src over the threshold, in path order,
// and hands each one to visit with its path relative to src.
func eachMinified(src string, over int,
	visit func(rel, path string, source, minified []byte) error) error {
	files, err := commandFiles(src)
	if err != nil {
		return err
	}
	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if over > 0 && utf8.RuneCount(source) <= over {
			continue
		}
		minified, err := minify(filepath.Base(path), string(source))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if err := visit(rel, path, source, []byte(minified)); err != nil {
			return err
		}
	}
	return nil
}

// commandFiles lists the .gohtml files under root, sorted.
func commandFiles(root string) ([]string, error) {
	return listFiles(root, func(path string) bool { return strings.HasSuffix(path, ".gohtml") })
}

// listFiles lists the files under root that keep says, sorted.
func listFiles(root string, keep func(path string) bool) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && keep(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
