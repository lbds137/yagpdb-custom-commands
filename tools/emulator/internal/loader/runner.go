// Package loader provides test running capabilities for the YAGPDB emulator.
package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/runtime"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/schema"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/state"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// TestResult represents the result of running a single test.
type TestResult struct {
	Name     string
	Passed   bool
	Error    error
	Failures []string
	Warnings []string
	Output   string
	Duration string

	SnapshotWritten bool // A new or updated snapshot was saved
}

// RunnerConfig configures the test runner.
type RunnerConfig struct {
	BaseDir         string         // Base directory for resolving template paths
	Verbose         bool           // Show detailed output
	StopOnFail      bool           // Stop on first failure
	Strict          bool           // Fail on YAGPDB execution limits in every test
	Schema          *schema.Schema // Expected database value types
	UpdateSnapshots bool           // Rewrite snapshots instead of comparing
	CI              bool           // A missing snapshot fails instead of being written
}

// Runner executes test cases.
type Runner struct {
	config         RunnerConfig
	duplicateNames map[string]bool // snapshot keys used by more than one test
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
		// Fixture maps stand in for sdicts a command stored
		db.Set(entry.UserID, entry.Key, types.FixtureForStorage(entry.Value))
	}

	for _, path := range tc.SetupTemplates {
		if err := r.runSetupTemplate(tc, db, path); err != nil {
			result.Error = err
			return result
		}
	}

	ctx := r.newContext(tc, db)
	ctx.SourceName = displayPath(r.config.BaseDir, tc.Template)
	if ctx.SourceName == "" {
		ctx.SourceName = fmt.Sprintf("inline template of %q", tc.Name)
	}
	if err := setTriggerMessage(tc, source, ctx); err != nil {
		result.Error = err
		return result
	}

	// Execute template
	engine := runtime.NewEngine(ctx)
	output, execErr := engine.Execute(source)
	result.Output = output
	for _, d := range ctx.Diagnostics {
		result.Warnings = append(result.Warnings, d.String())
	}

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

	if want := tc.Expected.WarningContains; want != "" && !containsAny(result.Warnings, want) {
		result.Failures = append(result.Failures,
			fmt.Sprintf("expected a warning containing %q but got: %q", want, result.Warnings))
	}

	if tc.Snapshot {
		failures, written := r.checkSnapshot(tc, output, ctx, db)
		result.Failures = append(result.Failures, failures...)
		result.SnapshotWritten = written
	}

	result.Passed = len(result.Failures) == 0 && result.Error == nil
	return result
}

// newContext builds the execution context a test describes, on the given database.
func (r *Runner) newContext(tc *TestCase, db *state.MockDB) *runtime.ExecutionContext {
	ctx := runtime.NewExecutionContext(tc.Context.Guild.ID, db)
	ctx.Strict = r.config.Strict || tc.Strict
	ctx.Schema = r.config.Schema
	if tc.Context.Premium != nil && !*tc.Context.Premium {
		ctx.SetNonPremium()
	}
	ctx.GuildName = tc.Context.Guild.Name
	ctx.OwnerID = tc.Context.Guild.OwnerID
	if tc.Context.Guild.Prefix != "" {
		ctx.Prefix = tc.Context.Guild.Prefix
	}
	ctx.ChannelID = tc.Context.Channel.ID
	ctx.ChannelName = tc.Context.Channel.Name
	ctx.UserID = tc.Context.User.ID
	ctx.Username = tc.Context.User.Username
	ctx.Discriminator = tc.Context.User.Discriminator
	ctx.UserRoles = tc.Context.User.Roles

	for _, m := range tc.Context.Messages {
		ctx.Messages = append(ctx.Messages, types.CtxMessage{
			ID:        m.ID,
			ChannelID: m.ChannelID,
			GuildID:   ctx.GuildID,
			Author:    types.DiscordUser{ID: m.AuthorID, Username: "MockUser"},
			Content:   m.Content,
		})
	}
	ctx.Members = tc.Context.Members
	ctx.MemberRoles = tc.Context.MemberRoles
	for _, role := range tc.Context.Guild.Roles {
		ctx.AvailableRoles[role.ID] = types.CtxRole{ID: role.ID, Name: role.Name, Color: role.Color}
	}
	if rd := tc.Context.Reaction; rd != nil {
		ctx.Reaction = &types.CtxReaction{
			UserID:    ctx.UserID,
			MessageID: rd.MessageID,
			ChannelID: ctx.ChannelID,
			GuildID:   ctx.GuildID,
			Emoji:     types.CtxEmoji{ID: rd.EmojiID, Name: rd.Emoji},
		}
		ctx.ReactionAdded = rd.Added == nil || *rd.Added
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
	return ctx
}

// setTriggerMessage gives the command the message that triggered it: the test's
// message_content, or its args after the trigger the template's header names. Commands
// run by execCC or a reaction have no trigger, but message_content can set their .Message;
// commands without a message trigger have neither.
func setTriggerMessage(tc *TestCase, source string, ctx *runtime.ExecutionContext) error {
	c := tc.Context
	if c.ExecData != nil || c.Reaction != nil {
		if len(c.Args) > 0 {
			return fmt.Errorf("args need a message trigger, not exec_data or reaction")
		}
		ctx.MessageContent = c.MessageContent
		return nil
	}
	t, ok := runtime.ReadTrigger(source)
	if !ok {
		// Inline templates and ones without a header are commands named after their file
		name := strings.TrimSuffix(filepath.Base(tc.Template), filepath.Ext(tc.Template))
		if tc.Template == "" {
			name = "test"
		}
		t = runtime.Trigger{Type: "Command", Text: name}
	}
	if !t.MessageTriggered() {
		if len(c.Args) > 0 || c.MessageContent != "" {
			return fmt.Errorf("args and message_content need a message trigger; the template's is %q", t.Type)
		}
		return nil
	}

	msg := c.MessageContent
	switch {
	case msg != "" && len(c.Args) > 0:
		return fmt.Errorf("give args or message_content, not both")
	case msg == "" && t.Type == "Regex":
		return fmt.Errorf("a Regex trigger needs message_content (the whole message)")
	case msg == "":
		msg = runtime.TriggerMessage(ctx.Prefix, t, c.Args)
	}
	return ctx.SetTriggerMessage(t, msg)
}

// runSetupTemplate runs a template the test lists under setup_templates, in the test's
// context and on its database, and discards what it sends.
func (r *Runner) runSetupTemplate(tc *TestCase, db *state.MockDB, path string) error {
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(r.config.BaseDir, path)
	}
	source, err := os.ReadFile(full)
	if err != nil {
		return fmt.Errorf("setup template: %w", err)
	}
	ctx := r.newContext(tc, db)
	ctx.SourceName = displayPath(r.config.BaseDir, path)
	ctx.ExecData = nil
	if _, err := runtime.NewEngine(ctx).Execute(string(source)); err != nil {
		return fmt.Errorf("setup template %s: %w", path, err)
	}
	return nil
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

		if check.EmbedContains != "" {
			if found.Embed == nil {
				failures = append(failures,
					fmt.Sprintf("message check %d: expected an embed containing %q but none found", i, check.EmbedContains))
			} else if embed := readableJSON(found.Embed); !strings.Contains(embed, check.EmbedContains) {
				failures = append(failures,
					fmt.Sprintf("message check %d: embed should contain %q but is:\n%s", i, check.EmbedContains, embed))
			}
		}

		if check.EmbedTitle != "" && found.Embed != nil {
			// Check embed title
			if embedMap, ok := found.Embed.(types.Embed); ok {
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

// displayPath returns a template path relative to the working directory, so warnings
// print as clickable file:line locations.
func displayPath(baseDir, template string) string {
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

func containsAny(items []string, substr string) bool {
	for _, item := range items {
		if strings.Contains(item, substr) {
			return true
		}
	}
	return false
}

// RunTests executes multiple test cases and returns results.
func (r *Runner) RunTests(tests []*TestCase) []*TestResult {
	var results []*TestResult
	r.duplicateNames = findDuplicateNames(tests)

	for _, tc := range tests {
		result := r.RunTest(tc)
		results = append(results, result)

		if r.config.StopOnFail && !result.Passed {
			break
		}
	}

	return results
}
