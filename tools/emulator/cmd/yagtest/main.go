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
    check   Validate a template without executing
    help    Show this help message

Run Options:
    -context <file>   JSON file with context data
    -db <file>        JSON file with initial database state
    -premium          Use premium limits (default: true)
    -no-premium       Use non-premium limits
    -args <args>      Command arguments (comma-separated)
    -verbose          Show detailed output

Test Options:
    -verbose          Show detailed output for each test
    -stop-on-fail     Stop on first test failure
    -base-dir <dir>   Base directory for resolving template paths

Examples:
    yagtest run utility/db.gohtml
    yagtest run -args "get,Global" utility/db.gohtml
    yagtest run -db initial_db.json -context context.json utility/db.gohtml
    yagtest test testdata/simple_tests.yaml
    yagtest test testdata/
    yagtest check utility/*.gohtml

Note: Flags must come before the file/directory path.`)
}

func runCommand(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	contextFile := fs.String("context", "", "JSON file with context data")
	dbFile := fs.String("db", "", "JSON file with initial database state")
	premium := fs.Bool("premium", true, "Use premium limits")
	noPremium := fs.Bool("no-premium", false, "Use non-premium limits")
	cmdArgs := fs.String("args", "", "Command arguments (comma-separated)")
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
	if *contextFile != "" {
		if err := loadContextFromFile(ctx, *contextFile); err != nil {
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

	// Parse command arguments
	if *cmdArgs != "" {
		parts := strings.Split(*cmdArgs, ",")
		ctx.Args = make([]interface{}, len(parts))
		ctx.CmdArgs = make([]interface{}, len(parts))
		for i, p := range parts {
			ctx.Args[i] = strings.TrimSpace(p)
			ctx.CmdArgs[i] = strings.TrimSpace(p)
		}
	}

	// Set command name from filename
	ctx.Cmd = strings.TrimSuffix(filepath.Base(templatePath), ".gohtml")

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

	if *verbose {
		fmt.Println("\n=== Execution Complete ===")
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
		engine := runtime.NewEngine(ctx)

		// Try to parse (not execute)
		_, parseErr := engine.Execute(string(content))
		if parseErr != nil {
			// Check if it's a parse error vs execution error
			if strings.Contains(parseErr.Error(), "template parse error") {
				fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", file, parseErr)
				hasErrors = true
			} else {
				// Execution error is okay for check - template is syntactically valid
				fmt.Printf("OK   %s\n", file)
			}
		} else {
			fmt.Printf("OK   %s\n", file)
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

	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	for _, entry := range entries {
		// Convert map[string]interface{} to SDict
		if m, ok := entry.Value.(map[string]interface{}); ok {
			entry.Value = types.SDict(m)
		}
		db.Set(entry.UserID, entry.Key, entry.Value)
	}

	return nil
}

func loadContextFromFile(ctx *runtime.ExecutionContext, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var contextData struct {
		GuildID     int64    `json:"guild_id"`
		GuildName   string   `json:"guild_name"`
		ChannelID   int64    `json:"channel_id"`
		ChannelName string   `json:"channel_name"`
		UserID      int64    `json:"user_id"`
		Username    string   `json:"username"`
		UserRoles   []int64  `json:"user_roles"`
		Args        []string `json:"args"`
		CmdArgs     []string `json:"cmd_args"`
		IsPremium   *bool    `json:"is_premium"`
	}

	if err := json.Unmarshal(data, &contextData); err != nil {
		return err
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
	if contextData.Args != nil {
		ctx.Args = make([]interface{}, len(contextData.Args))
		for i, a := range contextData.Args {
			ctx.Args[i] = a
		}
	}
	if contextData.CmdArgs != nil {
		ctx.CmdArgs = make([]interface{}, len(contextData.CmdArgs))
		for i, a := range contextData.CmdArgs {
			ctx.CmdArgs[i] = a
		}
	}
	if contextData.IsPremium != nil && !*contextData.IsPremium {
		ctx.SetNonPremium()
	}

	return nil
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

func testCommand(args []string) {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "Show detailed output for each test")
	stopOnFail := fs.Bool("stop-on-fail", false, "Stop on first test failure")
	baseDir := fs.String("base-dir", "", "Base directory for resolving template paths")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: test file or directory required")
		os.Exit(1)
	}

	path := fs.Arg(0)

	// Determine base directory
	resolvedBaseDir := *baseDir
	if resolvedBaseDir == "" {
		absPath, _ := filepath.Abs(path)
		info, err := os.Stat(absPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if info.IsDir() {
			resolvedBaseDir = absPath
		} else {
			resolvedBaseDir = filepath.Dir(absPath)
		}
	}

	// Load tests
	var tests []*loader.TestCase
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if info.IsDir() {
		tests, err = loader.LoadTestsFromDir(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading tests from dir: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Try as test suite first
		suite, suiteErr := loader.LoadTestSuite(path)
		if suiteErr == nil && len(suite.Tests) > 0 {
			for i := range suite.Tests {
				tests = append(tests, &suite.Tests[i])
			}
		} else {
			// Try as single test
			tc, tcErr := loader.LoadTestCase(path)
			if tcErr != nil {
				fmt.Fprintf(os.Stderr, "Error loading tests: %v\n", tcErr)
				os.Exit(1)
			}
			tests = append(tests, tc)
		}
	}

	if len(tests) == 0 {
		fmt.Fprintln(os.Stderr, "No tests found")
		os.Exit(1)
	}

	// Create runner
	runner := loader.NewRunner(loader.RunnerConfig{
		BaseDir:    resolvedBaseDir,
		Verbose:    *verbose,
		StopOnFail: *stopOnFail,
	})

	// Run tests
	fmt.Printf("%s=== Running %d test(s) ===%s\n\n", colorBold, len(tests), colorReset)

	results := runner.RunTests(tests)

	// Print results
	passed := 0
	failed := 0
	errors := 0

	for _, result := range results {
		if result.Passed {
			passed++
			fmt.Printf("%s✓ PASS%s %s\n", colorGreen, colorReset, result.Name)
			if *verbose && result.Output != "" {
				fmt.Printf("  %sOutput:%s %s\n", colorCyan, colorReset, strings.TrimSpace(result.Output))
			}
		} else if result.Error != nil {
			errors++
			fmt.Printf("%s✗ ERROR%s %s\n", colorRed, colorReset, result.Name)
			fmt.Printf("  %s%v%s\n", colorRed, result.Error, colorReset)
		} else {
			failed++
			fmt.Printf("%s✗ FAIL%s %s\n", colorRed, colorReset, result.Name)
			for _, failure := range result.Failures {
				fmt.Printf("  %s• %s%s\n", colorYellow, failure, colorReset)
			}
			if *verbose && result.Output != "" {
				fmt.Printf("  %sOutput:%s %s\n", colorCyan, colorReset, strings.TrimSpace(result.Output))
			}
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
	fmt.Println()

	if failed > 0 || errors > 0 {
		os.Exit(1)
	}
}
