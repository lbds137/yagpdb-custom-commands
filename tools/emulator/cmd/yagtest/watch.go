package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Directories never worth watching: large, generated, or not ours.
var skipDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true, "bin": true, "__snapshots__": true, ".idea": true,
}

var watchedExts = map[string]bool{".gohtml": true, ".yaml": true, ".yml": true, ".json": true}

func watchCommand(args []string) {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	var opts testOptions
	addTestFlags(fs, &opts)
	watchDirs := fs.String("watch", ".", "Comma-separated directories to watch")
	interval := fs.Duration("interval", 500*time.Millisecond, "How often to check for changes")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: test file or directory required")
		os.Exit(1)
	}
	opts.path = fs.Arg(0)

	dirs := strings.Split(*watchDirs, ",")
	for i, d := range dirs {
		dirs[i] = strings.TrimSpace(d)
		if _, err := os.Stat(dirs[i]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot watch %s: %v\n", dirs[i], err)
			os.Exit(1)
		}
	}

	last := scanMtimes(dirs)
	rerun := func(reason string) {
		fmt.Print("\033[H\033[2J") // clear the screen
		fmt.Printf("%s[%s] %s%s\n\n", colorCyan, time.Now().Format("15:04:05"), reason, colorReset)
		runTests(opts)
		fmt.Printf("\n%sWatching %s for changes (Ctrl+C to stop)%s\n", colorCyan, strings.Join(dirs, ", "), colorReset)
	}
	rerun("Starting")

	for {
		time.Sleep(*interval)
		current := scanMtimes(dirs)
		if changed := changedFile(last, current); changed != "" {
			last = current
			rerun("Changed: " + changed)
		}
	}
}

// scanMtimes records the modification time of every watched file under dirs.
func scanMtimes(dirs []string) map[string]time.Time {
	mtimes := map[string]time.Time{}
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // a file vanished mid-walk; the next scan sees the result
			}
			if d.IsDir() {
				if path != dir && skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if !watchedExts[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			if info, err := d.Info(); err == nil {
				mtimes[path] = info.ModTime()
			}
			return nil
		})
	}
	return mtimes
}

// changedFile returns a file that was added, removed or modified, or "" if none was.
func changedFile(before, after map[string]time.Time) string {
	for path, t := range after {
		if old, ok := before[path]; !ok || !old.Equal(t) {
			return path
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			return path + " (removed)"
		}
	}
	return ""
}
