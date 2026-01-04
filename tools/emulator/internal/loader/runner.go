// Package loader provides test running capabilities for the YAGPDB emulator.
package loader

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/runtime"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// TestResult represents the result of running a single test.
type TestResult struct {
	Name     string
	Passed   bool
	Error    error
	Failures []string
	Output   string
	Duration string
}

// RunnerConfig configures the test runner.
type RunnerConfig struct {
	BaseDir   string // Base directory for resolving template paths
	Verbose   bool   // Show detailed output
	StopOnFail bool  // Stop on first failure
}

// Runner executes test cases.
type Runner struct {
	config RunnerConfig
}

// NewRunner creates a new test runner.
func NewRunner(config RunnerConfig) *Runner {
	return &Runner{config: config}
}

// RunTest executes a single test case.
func (r *Runner) RunTest(tc *TestCase) *TestResult {
	result := &TestResult{
		Name: tc.Name,
	}

	// Get template source
	source, err := tc.GetTemplateSource(r.config.BaseDir)
	if err != nil {
		result.Error = err
		return result
	}

	// Set up database
	db := state.NewMockDB(tc.Context.Guild.ID)
	for _, entry := range tc.SetupDB {
		value := entry.Value
		// Convert map[string]interface{} to SDict
		if m, ok := value.(map[string]interface{}); ok {
			value = types.SDict(m)
		}
		db.Set(entry.UserID, entry.Key, value)
	}

	// Create execution context
	ctx := runtime.NewExecutionContext(tc.Context.Guild.ID, db)
	ctx.GuildName = tc.Context.Guild.Name
	ctx.ChannelID = tc.Context.Channel.ID
	ctx.ChannelName = tc.Context.Channel.Name
	ctx.UserID = tc.Context.User.ID
	ctx.Username = tc.Context.User.Username
	ctx.Discriminator = tc.Context.User.Discriminator
	ctx.UserRoles = tc.Context.User.Roles

	// Set up args
	if len(tc.Context.Args) > 0 {
		ctx.Args = make([]interface{}, len(tc.Context.Args))
		for i, a := range tc.Context.Args {
			ctx.Args[i] = a
		}
	}
	if len(tc.Context.CmdArgs) > 0 {
		ctx.CmdArgs = make([]interface{}, len(tc.Context.CmdArgs))
		for i, a := range tc.Context.CmdArgs {
			ctx.CmdArgs[i] = a
		}
	} else if len(tc.Context.Args) > 0 {
		// Default CmdArgs to Args if not specified
		ctx.CmdArgs = ctx.Args
	}

	// Set ExecData if provided
	if tc.Context.ExecData != nil {
		ctx.ExecData = types.SDict(tc.Context.ExecData)
	}

	// Set command map for execCC
	if tc.CommandMap != nil {
		ctx.CommandIDMap = tc.CommandMap
	}
	ctx.TemplateBaseDir = r.config.BaseDir

	// Execute template
	engine := runtime.NewEngine(ctx)
	output, execErr := engine.Execute(source)
	result.Output = output

	// Check for expected errors
	if tc.Expected.ErrorContains != "" {
		if execErr == nil {
			result.Failures = append(result.Failures,
				fmt.Sprintf("expected error containing %q but got no error", tc.Expected.ErrorContains))
		} else if !strings.Contains(execErr.Error(), tc.Expected.ErrorContains) {
			result.Failures = append(result.Failures,
				fmt.Sprintf("expected error containing %q but got: %v", tc.Expected.ErrorContains, execErr))
		}
		// If we expected an error and got one, don't check other assertions
		if execErr != nil && strings.Contains(execErr.Error(), tc.Expected.ErrorContains) {
			result.Passed = true
			return result
		}
	} else if execErr != nil {
		result.Error = execErr
		return result
	}

	// Check output assertions
	failures := r.checkOutput(output, tc.Expected)
	result.Failures = append(result.Failures, failures...)

	// Check database assertions
	failures = r.checkDatabase(db, tc.Assertions.DBChecks)
	result.Failures = append(result.Failures, failures...)

	// Check sent messages
	failures = r.checkMessages(ctx.SentMessages, tc.Assertions.SentMessages)
	result.Failures = append(result.Failures, failures...)

	// Check role changes
	failures = r.checkRoleChanges(ctx.RoleChanges, tc.Assertions.RoleChanges)
	result.Failures = append(result.Failures, failures...)

	result.Passed = len(result.Failures) == 0 && result.Error == nil
	return result
}

// checkOutput verifies output assertions.
func (r *Runner) checkOutput(output string, expected ExpectedResult) []string {
	var failures []string

	// Normalize output (trim whitespace)
	output = strings.TrimSpace(output)

	if expected.OutputEquals != "" {
		expectedNorm := strings.TrimSpace(expected.OutputEquals)
		if output != expectedNorm {
			failures = append(failures,
				fmt.Sprintf("output mismatch:\n  expected: %q\n  got:      %q", expectedNorm, output))
		}
	}

	if expected.OutputContains != "" {
		if !strings.Contains(output, expected.OutputContains) {
			failures = append(failures,
				fmt.Sprintf("output should contain %q but got: %q", expected.OutputContains, output))
		}
	}

	if expected.OutputMatches != "" {
		re, err := regexp.Compile(expected.OutputMatches)
		if err != nil {
			failures = append(failures,
				fmt.Sprintf("invalid output regex %q: %v", expected.OutputMatches, err))
		} else if !re.MatchString(output) {
			failures = append(failures,
				fmt.Sprintf("output should match %q but got: %q", expected.OutputMatches, output))
		}
	}

	return failures
}

// checkDatabase verifies database assertions.
func (r *Runner) checkDatabase(db *state.MockDB, checks []DBCheck) []string {
	var failures []string

	for _, check := range checks {
		entry := db.Get(check.UserID, check.Key)

		if check.NotExists {
			if entry != nil {
				failures = append(failures,
					fmt.Sprintf("db entry [user=%d, key=%s] should not exist but has value: %v",
						check.UserID, check.Key, entry.Value))
			}
			continue
		}

		if entry == nil {
			failures = append(failures,
				fmt.Sprintf("db entry [user=%d, key=%s] not found", check.UserID, check.Key))
			continue
		}

		if check.ValueEquals != nil {
			// Compare as JSON for complex values
			expectedJSON, _ := json.Marshal(check.ValueEquals)
			actualJSON, _ := json.Marshal(entry.Value)
			if string(expectedJSON) != string(actualJSON) {
				failures = append(failures,
					fmt.Sprintf("db entry [user=%d, key=%s] value mismatch:\n  expected: %s\n  got:      %s",
						check.UserID, check.Key, expectedJSON, actualJSON))
			}
		}

		if check.ValueContains != "" {
			valueStr := fmt.Sprintf("%v", entry.Value)
			valueJSON, _ := json.Marshal(entry.Value)
			if !strings.Contains(valueStr, check.ValueContains) && !strings.Contains(string(valueJSON), check.ValueContains) {
				failures = append(failures,
					fmt.Sprintf("db entry [user=%d, key=%s] should contain %q but has: %v",
						check.UserID, check.Key, check.ValueContains, entry.Value))
			}
		}
	}

	return failures
}

// checkMessages verifies sent message assertions.
func (r *Runner) checkMessages(messages []runtime.SentMessage, checks []MessageCheck) []string {
	var failures []string

	for i, check := range checks {
		// Find matching message
		var found *runtime.SentMessage
		for j := range messages {
			if check.ChannelID == 0 || messages[j].ChannelID == check.ChannelID {
				found = &messages[j]
				break
			}
		}

		if found == nil {
			if check.ChannelID != 0 {
				failures = append(failures,
					fmt.Sprintf("message check %d: no message sent to channel %d", i, check.ChannelID))
			} else {
				failures = append(failures,
					fmt.Sprintf("message check %d: no messages sent", i))
			}
			continue
		}

		if check.ContentEquals != "" && found.Content != check.ContentEquals {
			failures = append(failures,
				fmt.Sprintf("message check %d: content mismatch:\n  expected: %q\n  got:      %q",
					i, check.ContentEquals, found.Content))
		}

		if check.ContentContains != "" && !strings.Contains(found.Content, check.ContentContains) {
			failures = append(failures,
				fmt.Sprintf("message check %d: content should contain %q but got: %q",
					i, check.ContentContains, found.Content))
		}

		if check.HasEmbed && found.Embed == nil {
			failures = append(failures,
				fmt.Sprintf("message check %d: expected embed but none found", i))
		}

		if check.EmbedTitle != "" && found.Embed != nil {
			// Check embed title
			if embedMap, ok := found.Embed.(types.SDict); ok {
				if title, ok := embedMap["title"].(string); ok {
					if title != check.EmbedTitle {
						failures = append(failures,
							fmt.Sprintf("message check %d: embed title mismatch:\n  expected: %q\n  got:      %q",
								i, check.EmbedTitle, title))
					}
				} else {
					failures = append(failures,
						fmt.Sprintf("message check %d: embed has no title, expected %q", i, check.EmbedTitle))
				}
			}
		}
	}

	return failures
}

// checkRoleChanges verifies role change assertions.
func (r *Runner) checkRoleChanges(changes []runtime.RoleChange, checks []RoleCheck) []string {
	var failures []string

	for i, check := range checks {
		found := false
		for _, change := range changes {
			if change.UserID == check.UserID && change.RoleID == check.RoleID && change.Action == check.Action {
				found = true
				break
			}
		}

		if !found {
			failures = append(failures,
				fmt.Sprintf("role check %d: expected %s role %d for user %d but not found",
					i, check.Action, check.RoleID, check.UserID))
		}
	}

	return failures
}

// RunTests executes multiple test cases and returns results.
func (r *Runner) RunTests(tests []*TestCase) []*TestResult {
	var results []*TestResult

	for _, tc := range tests {
		result := r.RunTest(tc)
		results = append(results, result)

		if r.config.StopOnFail && !result.Passed {
			break
		}
	}

	return results
}
