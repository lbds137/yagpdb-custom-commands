package runtime

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The browser deploy (deploy/deploy.js) re-implements the header reader's panel-facing
// half in JS: the defer mode, the slash description and the slash subcommand and option
// rows. This test writes what this package reads from every commands/**/*.gohtml header,
// plus the full message for each rejected header, to deploy/testdata/headers.golden.json;
// deploy/deploy.test.js parses the same files and sources with deploy.js and asserts it
// gets the same. Regenerate after a header change or a grammar change with
//
//	cd tools/emulator && go test ./internal/runtime -run TestHeaderGolden -update
var updateHeaderGolden = flag.Bool("update", false, "rewrite deploy/testdata/headers.golden.json")

const (
	repoRoot         = "../../../.."
	headerGoldenPath = repoRoot + "/deploy/testdata/headers.golden.json"
)

type goldenOption struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type goldenSubcommand struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Options     []goldenOption `json:"options"`
}

type goldenSlash struct {
	Subcommands []goldenSubcommand `json:"subcommands"`
	Options     []goldenOption     `json:"options"`
}

type goldenHeader struct {
	DeferMode        int          `json:"deferMode"`
	Interval         *int         `json:"interval"`
	SlashDescription *string      `json:"slashDescription"`
	Slash            *goldenSlash `json:"slash"`
}

type goldenError struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Error  string `json:"error"`
}

// goldenSample is a valid header the committed commands don't cover (a defer mode other
// than None, a description, flat and subcommand rows) with what the emulator reads of it.
type goldenSample struct {
	Name   string       `json:"name"`
	Source string       `json:"source"`
	Header goldenHeader `json:"header"`
}

type headerGolden struct {
	Commands map[string]goldenHeader `json:"commands"`
	Samples  []goldenSample          `json:"samples"`
	Errors   []goldenError           `json:"errors"`
}

func goldenSamples(t *testing.T) []goldenSample {
	sources := []struct{ name, src string }{
		{"flat rows of every type", deployableProbeHeader},
		{"subcommands", kvHeader},
		{"ephemeral defer and a description", slashHeaderSource("x",
			"  Slash description: `Color tools`\n  Defer mode: `Ephemeral Message Response`\n")},
		{"message defer, unicode option name", slashHeaderSource("x",
			"  Defer mode: `Message Response`\n  Slash option: `מפתח string! the key`\n")},
		{"a component with a defer mode",
			"{{/*\n  Trigger type: `Message Component`\n  Trigger: `^x:`\n  Defer mode: `Update Message Response`\n*/}}"},
		{"a context menu entry", "{{/*\n  Trigger type: `User Context Menu`\n  Trigger: `View avatar`\n*/}}"},
		{"an hourly interval", "{{/*\n  Trigger type: `Hourly interval`\n  Group: `Utility`\n  Interval: `12`\n*/}}"},
		{"a minute interval", "{{/*\n  Trigger type: `Minute interval`\n  Group: `Utility`\n  Interval: `15`\n*/}}"},
		{"an Interval line on a command is ignored",
			"{{/*\n  Trigger type: `Command`\n  Trigger: `x`\n  Interval: `3`\n*/}}"},
	}
	var out []goldenSample
	for _, s := range sources {
		out = append(out, goldenSample{s.name, s.src, goldenOf(t, s.name, s.src)})
	}
	return out
}

func goldenOptions(opts []SlashOption) []goldenOption {
	out := make([]goldenOption, 0, len(opts))
	for _, o := range opts {
		out = append(out, goldenOption{o.Name, o.FormType, o.Description, o.Required})
	}
	return out
}

func goldenOf(t *testing.T, path, source string) goldenHeader {
	t.Helper()
	if err := ValidateHeader(source); err != nil {
		t.Errorf("%s: %v", path, err)
	}
	g := goldenHeader{DeferMode: int(ReadDeferMode(source))}
	if tr, ok := ReadTrigger(source); ok && tr.IntervalTriggered() {
		if n, ok := ReadInterval(source); ok {
			g.Interval = &n
		}
	}
	if d, ok := ReadSlashDescription(source); ok {
		g.SlashDescription = &d
	}
	if tr, ok := ReadTrigger(source); ok && tr.SlashTriggered() {
		def, err := ReadSlashCommand(source)
		if err != nil {
			t.Errorf("%s: %v", path, err)
		}
		s := &goldenSlash{Subcommands: []goldenSubcommand{}, Options: goldenOptions(def.Options)}
		for _, sub := range def.Subcommands {
			s.Subcommands = append(s.Subcommands,
				goldenSubcommand{sub.Name, sub.Description, goldenOptions(sub.Options)})
		}
		g.Slash = s
	}
	return g
}

func TestHeaderGolden(t *testing.T) {
	got := headerGolden{Commands: map[string]goldenHeader{}}
	commands := filepath.Join(repoRoot, "commands")
	err := filepath.WalkDir(commands, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".gohtml") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoRoot, p)
		got.Commands[filepath.ToSlash(rel)] = goldenOf(t, rel, string(src))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Commands) == 0 {
		t.Fatal("no commands found: wrong repoRoot?")
	}
	got.Samples = goldenSamples(t)
	errorCases := slashHeaderErrorCases()
	for _, c := range []struct{ name, src string }{
		{"hourly interval without Interval", "{{/*\n  Trigger type: `Hourly interval`\n  Group: `Utility`\n*/}}"},
		{"minute interval without Interval", "{{/*\n  Trigger type: `Minute interval`\n  Group: `Utility`\n*/}}"},
		{"interval of zero", "{{/*\n  Trigger type: `Hourly interval`\n  Interval: `0`\n*/}}"},
		{"interval negative", "{{/*\n  Trigger type: `Minute interval`\n  Interval: `-5`\n*/}}"},
		{"interval not a number", "{{/*\n  Trigger type: `Hourly interval`\n  Interval: `daily`\n*/}}"},
		{"hourly interval above 744", "{{/*\n  Trigger type: `Hourly interval`\n  Interval: `745`\n*/}}"},
		{"minute interval below 5", "{{/*\n  Trigger type: `Minute interval`\n  Interval: `4`\n*/}}"},
		{"minute interval above 44640", "{{/*\n  Trigger type: `Minute interval`\n  Interval: `44641`\n*/}}"},
		{"minute interval of whole hours", "{{/*\n  Trigger type: `Minute interval`\n  Interval: `120`\n*/}}"},
		{"interval fractional", "{{/*\n  Trigger type: `Hourly interval`\n  Interval: `1.5`\n*/}}"},
	} {
		errorCases = append(errorCases, struct{ name, src, want string }{c.name, c.src, ""})
	}
	for _, c := range errorCases {
		err := ValidateHeader(c.src)
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		got.Errors = append(got.Errors, goldenError{c.name, c.src, err.Error()})
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(got); err != nil {
		t.Fatal(err)
	}
	if *updateHeaderGolden {
		if err := os.MkdirAll(filepath.Dir(headerGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(headerGoldenPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(headerGoldenPath)
	if err != nil {
		t.Fatalf("%v (run with -update to write it)", err)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		t.Errorf("deploy/testdata/headers.golden.json is stale: run `cd tools/emulator && go test " +
			"./internal/runtime -run TestHeaderGolden -update`")
	}
}
