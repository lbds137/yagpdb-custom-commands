// Package main provides the CLI entry point for yagtest.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/loader"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/runtime"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/schema"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

const version = "0.1.0"

func main() {
	// Global flags
	versionFlag := flag.Bool("version", false, "Print version and exit")

	// Parse flags
	flag.Parse()

	if *versionFlag {
		fmt.Printf("yagtest version %s\n", version)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "run":
		runCommand(cmdArgs)
	case "check":
		checkCommand(cmdArgs)
	case "test":
		testCommand(cmdArgs)
	case "watch":
		watchCommand(cmdArgs)
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`yagtest - YAGPDB Template Emulator

Usage:
    yagtest <command> [options] <arguments>

Commands:
    run     Execute a template file
    test    Run test cases from YAML files
    watch   Rerun tests whenever a template or test file changes
    check   Validate a template without executing
    help    Show this help message

Run Options:
    -context <file>   JSON file with context data
    -db <file>        JSON file with initial database state
    -premium          Use premium limits (default: true)
    -no-premium       Use non-premium limits
    -args <args>      Command arguments after the trigger (comma-separated)
    -message <text>   The whole triggering message (needed for Regex triggers)
    -strict           Fail on YAGPDB execution limits instead of warning
    -schema <file>    Warn when stored values don't match the schema's types
    -verbose          Show detailed output

Test Options:
    -verbose          Show detailed output for each test
    -stop-on-fail     Stop on first test failure
    -base-dir <dir>   Base directory for resolving template paths
    -strict           Fail on YAGPDB execution limits instead of warning
    -schema <file>    Warn when stored values don't match the schema's types
    -update-snapshots Rewrite the snapshots of tests marked snapshot: true
                      (a missing snapshot is written, or fails when CI is set)

Watch Options:
    Same as test, plus:
    -watch <dirs>     Comma-separated directories to watch (default: .)
    -interval <dur>   How often to check for changes (default: 500ms)

Examples:
    yagtest run utility/db.gohtml
    yagtest run -args "get,Global" utility/db.gohtml
    yagtest run -message "#ff8800" utility/hex_to_int.gohtml
    yagtest run -db initial_db.json -context context.json utility/db.gohtml
    yagtest test testdata/simple_tests.yaml
    yagtest test testdata/
    yagtest test -strict -schema db_schema.yaml testdata/
    yagtest watch -watch tools/emulator/testdata,utility testdata/
    yagtest check utility/*.gohtml

Note: Flags must come before the file/directory path.`)
}

func runCommand(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	contextFile := fs.String("context", "", "JSON file with context data")
	dbFile := fs.String("db", "", "JSON file with initial database state")
	premium := fs.Bool("premium", true, "Use premium limits")
	noPremium := fs.Bool("no-premium", false, "Use non-premium limits")
	cmdArgs := fs.String("args", "", "Command arguments after the trigger (comma-separated)")
	message := fs.String("message", "", "The whole triggering message (instead of -args)")
	strict := fs.Bool("strict", false, "Fail on YAGPDB execution limits")
	schemaFile := fs.String("schema", "", "Schema file with expected database value types")
	verbose := fs.Bool("verbose", false, "Show detailed output")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: template file required")
		os.Exit(1)
	}

	templatePath := fs.Arg(0)

	// Read template file
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading template: %v\n", err)
		os.Exit(1)
	}

	// Initialize database
	guildID := int64(123456789012345678)
	db := state.NewMockDB(guildID)

	// Load initial database state if provided
	if *dbFile != "" {
		if err := loadDatabaseState(db, *dbFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading database state: %v\n", err)
			os.Exit(1)
		}
	}

	// Create execution context
	ctx := runtime.NewExecutionContext(guildID, db)

	// Load context from file if provided
	var contextArgs []string
	if *contextFile != "" {
		var err error
		if contextArgs, err = loadContextFromFile(ctx, *contextFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading context: %v\n", err)
			os.Exit(1)
		}
	}

	// Set premium/non-premium
	if *noPremium {
		ctx.SetNonPremium()
	} else if !*premium {
		ctx.SetNonPremium()
	}

	ctx.Strict = *strict
	ctx.Schema = mustLoadSchema(*schemaFile)
	ctx.SourceName = templatePath

	// The triggering message: -message, or the trigger the header names (or the file name)
	// followed by the args
	trigger, ok := runtime.ReadTrigger(string(templateContent))
	if !ok {
		trigger = runtime.Trigger{Type: "Command", Text: strings.TrimSuffix(filepath.Base(templatePath), ".gohtml")}
	}
	parts := contextArgs
	if *cmdArgs != "" {
		parts = nil
		for _, p := range strings.Split(*cmdArgs, ",") {
			parts = append(parts, strings.TrimSpace(p))
		}
	}
	msg := *message
	switch {
	case !trigger.MessageTriggered():
		if len(parts) > 0 || msg != "" {
			fatalf("arguments and -message need a message trigger; the template's is %q", trigger.Type)
		}
	case msg != "" && len(parts) > 0:
		fatalf("give arguments or -message, not both")
	case msg == "" && trigger.Type == "Regex":
		fatalf("a Regex trigger needs -message (the whole message)")
	default:
		if msg == "" {
			msg = runtime.TriggerMessage(ctx.Prefix, trigger, parts)
		}
		if err := ctx.SetTriggerMessage(trigger, msg); err != nil {
			fatalf("%v", err)
		}
	}

	// Execute template
	engine := runtime.NewEngine(ctx)
	output, err := engine.Execute(string(templateContent))

	// Print results
	if *verbose {
		fmt.Println("=== Execution Results ===")
		fmt.Printf("Template: %s\n", templatePath)
		fmt.Printf("Premium: %v\n", ctx.IsPremium)
		fmt.Printf("Args: %v\n", ctx.Args)
		fmt.Println()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Execution Error: %v\n", err)
		printHint(err)
		printDiagnostics(ctx.Diagnostics)
		os.Exit(1)
	}

	// Print output
	if output != "" {
		if *verbose {
			fmt.Println("=== Template Output ===")
		}
		fmt.Println(output)
	}

	// Print sent messages
	if len(ctx.SentMessages) > 0 {
		if *verbose {
			fmt.Println("\n=== Sent Messages ===")
		}
		for i, msg := range ctx.SentMessages {
			if *verbose {
				fmt.Printf("Message %d (Channel: %d):\n", i+1, msg.ChannelID)
			}
			if msg.Content != "" {
				fmt.Println(msg.Content)
			}
			if msg.Embed != nil {
				embedJSON, _ := json.MarshalIndent(msg.Embed, "", "  ")
				fmt.Println(string(embedJSON))
			}
		}
	}

	// Print role changes
	if len(ctx.RoleChanges) > 0 && *verbose {
		fmt.Println("\n=== Role Changes ===")
		for _, change := range ctx.RoleChanges {
			fmt.Printf("User %d: %s role %d\n", change.UserID, change.Action, change.RoleID)
		}
	}

	// Print database state if verbose
	if *verbose {
		entries := db.GetAll()
		if len(entries) > 0 {
			fmt.Println("\n=== Database State ===")
			for _, entry := range entries {
				valueJSON, _ := json.MarshalIndent(entry.Value, "", "  ")
				fmt.Printf("User %d, Key '%s': %s\n", entry.UserID, entry.Key, string(valueJSON))
			}
		}
	}

	printDiagnostics(ctx.Diagnostics)

	if *verbose {
		fmt.Println("\n=== Execution Complete ===")
	}
}

func mustLoadSchema(filename string) *schema.Schema {
	if filename == "" {
		return nil
	}
	s, err := schema.Load(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return s
}

func printHint(err error) {
	if hint := runtime.Hint(err); hint != "" {
		fmt.Fprintf(os.Stderr, "%sHint:%s %s\n", colorCyan, colorReset, hint)
	}
}

func printDiagnostics(diags []runtime.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintf(os.Stderr, "%s⚠ %s%s\n", colorYellow, d, colorReset)
	}
}

func checkCommand(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: at least one template file required")
		os.Exit(1)
	}

	// Expand glob patterns
	var files []string
	for _, pattern := range fs.Args() {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error expanding pattern %s: %v\n", pattern, err)
			continue
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "No files matched")
		os.Exit(1)
	}

	// Check each file
	hasErrors := false
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", file, err)
			hasErrors = true
			continue
		}

		// Create minimal context for parsing
		db := state.NewMockDB(1)
		ctx := runtime.NewExecutionContext(1, db)
		ctx.SourceName = file
		engine := runtime.NewEngine(ctx)

		// Try to parse (not execute)
		_, parseErr := engine.Execute(string(content))
		if parseErr != nil {
			// Check if it's a parse error vs execution error
			if strings.Contains(parseErr.Error(), "template parse error") {
				fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", file, parseErr)
				printHint(parseErr)
				hasErrors = true
			} else {
				// Execution error is okay for check - template is syntactically valid
				fmt.Printf("OK   %s\n", file)
			}
		} else {
			fmt.Printf("OK   %s\n", file)
		}
		// Static findings only: runtime warnings depend on arguments check doesn't have
		for _, d := range ctx.Diagnostics {
			switch {
			case d.Kind == runtime.KindLoopDB: // the message starts with file:line
				fmt.Fprintf(os.Stderr, "%sWARN %s%s\n", colorYellow, d.Message, colorReset)
			case strings.Contains(d.Message, "refuses to save"):
				fmt.Fprintf(os.Stderr, "%sWARN %s: %s%s\n", colorYellow, file, d.Message, colorReset)
			}
		}
	}

	if hasErrors {
		os.Exit(1)
	}
}

func loadDatabaseState(db *state.MockDB, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var entries []struct {
		UserID int64       `json:"user_id"`
		Key    string      `json:"key"`
		Value  interface{} `json:"value"`
	}

	if err := types.StrictJSON(data, &entries); err != nil {
		return err
	}

	for _, entry := range entries {
		// Fixture maps stand in for sdicts a command stored
		if _, err := db.Set(entry.UserID, entry.Key, types.FixtureForStorage(entry.Value)); err != nil {
			return fmt.Errorf("%q: %w", entry.Key, err)
		}
	}

	return nil
}

// loadContextFromFile sets the context the file gives, and returns its args.
func loadContextFromFile(ctx *runtime.ExecutionContext, filename string) ([]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var contextData struct {
		GuildID     int64    `json:"guild_id"`
		GuildName   string   `json:"guild_name"`
		ChannelID   int64    `json:"channel_id"`
		ChannelName string   `json:"channel_name"`
		UserID      int64    `json:"user_id"`
		Username    string   `json:"username"`
		UserRoles   []int64  `json:"user_roles"`
		Args        []string `json:"args"` // After the trigger, like -args
		IsPremium   *bool    `json:"is_premium"`
	}

	if err := types.StrictJSON(data, &contextData); err != nil {
		return nil, err
	}

	if contextData.GuildID != 0 {
		ctx.GuildID = contextData.GuildID
	}
	if contextData.GuildName != "" {
		ctx.GuildName = contextData.GuildName
	}
	if contextData.ChannelID != 0 {
		ctx.ChannelID = contextData.ChannelID
	}
	if contextData.ChannelName != "" {
		ctx.ChannelName = contextData.ChannelName
	}
	if contextData.UserID != 0 {
		ctx.UserID = contextData.UserID
	}
	if contextData.Username != "" {
		ctx.Username = contextData.Username
	}
	if contextData.UserRoles != nil {
		ctx.UserRoles = contextData.UserRoles
	}
	if contextData.IsPremium != nil && !*contextData.IsPremium {
		ctx.SetNonPremium()
	}

	return contextData.Args, nil
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// testOptions are the flags shared by test and watch.
type testOptions struct {
	path            string
	verbose         bool
	stopOnFail      bool
	baseDir         string
	strict          bool
	schemaFile      string
	updateSnapshots bool
}

func addTestFlags(fs *flag.FlagSet, opts *testOptions) {
	fs.BoolVar(&opts.verbose, "verbose", false, "Show detailed output for each test")
	fs.BoolVar(&opts.stopOnFail, "stop-on-fail", false, "Stop on first test failure")
	fs.StringVar(&opts.baseDir, "base-dir", "", "Base directory for resolving template paths")
	fs.BoolVar(&opts.strict, "strict", false, "Fail on YAGPDB execution limits")
	fs.StringVar(&opts.schemaFile, "schema", "", "Schema file with expected database value types")
	fs.BoolVar(&opts.updateSnapshots, "update-snapshots", false, "Rewrite snapshots")
}

func testCommand(args []string) {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	var opts testOptions
	addTestFlags(fs, &opts)

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: test file or directory required")
		os.Exit(1)
	}
	opts.path = fs.Arg(0)

	os.Exit(runTests(opts))
}

// runTests loads and runs the tests at opts.path and returns the exit code.
func runTests(opts testOptions) int {
	path := opts.path

	// Determine base directory
	resolvedBaseDir := opts.baseDir
	if resolvedBaseDir == "" {
		absPath, _ := filepath.Abs(path)
		info, err := os.Stat(absPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		if info.IsDir() {
			resolvedBaseDir = absPath
		} else {
			resolvedBaseDir = filepath.Dir(absPath)
		}
	}

	var sch *schema.Schema
	if opts.schemaFile != "" {
		var err error
		if sch, err = schema.Load(opts.schemaFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}

	// Load tests
	var tests []*loader.TestCase
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if info.IsDir() {
		tests, err = loader.LoadTestsFromDir(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading tests from dir: %v\n", err)
			return 1
		}
	} else {
		tests, err = loader.LoadTestFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading tests: %v\n", err)
			return 1
		}
	}

	if len(tests) == 0 {
		fmt.Fprintln(os.Stderr, "No tests found")
		return 1
	}

	// Create runner
	runner := loader.NewRunner(loader.RunnerConfig{
		BaseDir:         resolvedBaseDir,
		Verbose:         opts.verbose,
		StopOnFail:      opts.stopOnFail,
		Strict:          opts.strict,
		Schema:          sch,
		UpdateSnapshots: opts.updateSnapshots,
		CI:              os.Getenv("CI") != "",
	})

	// Run tests
	fmt.Printf("%s=== Running %d test(s) ===%s\n\n", colorBold, len(tests), colorReset)

	results := runner.RunTests(tests)

	pruned := 0
	if opts.updateSnapshots {
		var err error
		if pruned, err = loader.PruneSnapshots(tests); err != nil {
			fmt.Fprintf(os.Stderr, "Error pruning snapshots: %v\n", err)
			return 1
		}
	}

	// Print results
	passed := 0
	failed := 0
	errors := 0
	warned := 0
	snapshotsWritten := 0
	// Loop warnings come from reading the template, so they repeat for every test of
	// the same template; they are listed once after the results instead.
	var loopWarnings []string
	seenLoop := map[string]bool{}

	for _, result := range results {
		if result.Passed {
			passed++
			fmt.Printf("%s✓ PASS%s %s\n", colorGreen, colorReset, result.Name)
			if opts.verbose && result.Output != "" {
				fmt.Printf("  %sOutput:%s %s\n", colorCyan, colorReset, strings.TrimSpace(result.Output))
			}
		} else if result.Error != nil {
			errors++
			fmt.Printf("%s✗ ERROR%s %s\n", colorRed, colorReset, result.Name)
			fmt.Printf("  %s%v%s\n", colorRed, result.Error, colorReset)
			if hint := runtime.Hint(result.Error); hint != "" {
				fmt.Printf("  %sHint:%s %s\n", colorCyan, colorReset, hint)
			}
		} else {
			failed++
			fmt.Printf("%s✗ FAIL%s %s\n", colorRed, colorReset, result.Name)
			for _, failure := range result.Failures {
				fmt.Printf("  %s• %s%s\n", colorYellow, failure, colorReset)
			}
			if opts.verbose && result.Output != "" {
				fmt.Printf("  %sOutput:%s %s\n", colorCyan, colorReset, strings.TrimSpace(result.Output))
			}
		}
		if result.SnapshotWritten {
			snapshotsWritten++
			fmt.Printf("  %s✎ snapshot written%s\n", colorCyan, colorReset)
		}
		runtimeWarnings := 0
		for _, w := range result.Warnings {
			if strings.HasPrefix(w, "["+runtime.KindLoopDB+"]") {
				if !seenLoop[w] {
					seenLoop[w] = true
					loopWarnings = append(loopWarnings, fmt.Sprintf("%s (first seen in %q)", w, result.Name))
				}
				continue
			}
			runtimeWarnings++
			fmt.Printf("  %s⚠ %s%s\n", colorYellow, w, colorReset)
		}
		if runtimeWarnings > 0 {
			warned++
		}
	}

	if len(loopWarnings) > 0 {
		fmt.Printf("\n%s=== Database calls in loops ===%s\n", colorBold, colorReset)
		for _, w := range loopWarnings {
			fmt.Printf("%s⚠ %s%s\n", colorYellow, strings.TrimPrefix(w, "["+runtime.KindLoopDB+"] "), colorReset)
		}
	}

	// Print summary
	fmt.Printf("\n%s=== Summary ===%s\n", colorBold, colorReset)
	fmt.Printf("Total: %d | ", len(results))
	if passed > 0 {
		fmt.Printf("%sPassed: %d%s | ", colorGreen, passed, colorReset)
	} else {
		fmt.Printf("Passed: 0 | ")
	}
	if failed > 0 {
		fmt.Printf("%sFailed: %d%s | ", colorRed, failed, colorReset)
	} else {
		fmt.Printf("Failed: 0 | ")
	}
	if errors > 0 {
		fmt.Printf("%sErrors: %d%s", colorRed, errors, colorReset)
	} else {
		fmt.Printf("Errors: 0")
	}
	if warned > 0 {
		fmt.Printf(" | %sWith warnings: %d%s", colorYellow, warned, colorReset)
	}
	if snapshotsWritten > 0 {
		fmt.Printf(" | Snapshots written: %d", snapshotsWritten)
	}
	if pruned > 0 {
		fmt.Printf(" | Stale snapshots removed: %d", pruned)
	}
	fmt.Println()

	if failed > 0 || errors > 0 {
		return 1
	}
	return 0
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
