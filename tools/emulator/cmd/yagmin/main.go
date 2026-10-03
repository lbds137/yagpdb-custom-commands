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
    yagmin prove <flags>              Equivalence proof: run the whole YAML suite on
                                      minified copies of every command, and compare

Batch flags:
    -src <dir>   Tree to minify (default commands)
    -dst <dir>   Where to write the copies (required); mirrors src's layout
    -over <n>    Only files over n runes (0 = every file)

Prove flags:
    -commands <dir>  Command tree (default commands)
    -testdata <dir>  YAML suite directory (default tools/emulator/testdata)
    -schema <file>   Type schema for stored values (default db_schema.yaml)
    -scratch <dir>   Where to put the minified copies (default: a fresh temp dir,
                     kept; yagmin never deletes)
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
	var files []string
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".gohtml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)
	written := 0
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
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(minified), 0o644); err != nil {
			return err
		}
		written++
		fmt.Printf("%s: %d -> %d runes\n", path, utf8.RuneCount(source), utf8.RuneCount([]byte(minified)))
	}
	fmt.Printf("minified %d file(s) into %s\n", written, dst)
	return nil
}
