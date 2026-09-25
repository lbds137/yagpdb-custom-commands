// Package loader provides test running capabilities for the YAGPDB emulator.
package loader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
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
		// Fixture maps stand in for sdicts a command stored, and keys are cut as dbSet cuts them
		key := funcs.LimitString(entry.Key, 256)
		if _, err := db.Set(entry.UserID, key, types.FixtureForStorage(entry.Value)); err != nil {
			result.Error = fmt.Errorf("setup_db %q: %w", entry.Key, err)
			return result
		}
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
	if err := runtime.ValidateHeader(source); err != nil {
		result.Error = err
		return result
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
		// An expected error goes on to the other assertions: YAGPDB keeps what the run did
		// and sends what it printed before the error
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
	failures = r.checkMessages(ctx.SentMessages, tc.Assertions.SentMessages, "sent")
	if f := checkPings(ctx.ResponsePings, tc.Assertions.ResponsePings); f != "" {
		failures = append(failures, "response "+f)
	}
	failures = append(failures, r.checkMessages(ctx.EditedMessages, tc.Assertions.EditedMessages, "edited")...)
	result.Failures = append(result.Failures, failures...)

	result.Failures = append(result.Failures, checkScheduledRuns(ctx.ScheduledRuns(), tc.Assertions.ScheduledRuns)...)

	// Check role changes
	failures = r.checkRoleChanges(ctx.RoleChanges, tc.Assertions.RoleChanges)
	if tc.Assertions.NoRoleChanges && len(ctx.RoleChanges) > 0 {
		failures = append(failures, fmt.Sprintf("expected no role changes, got %+v", ctx.RoleChanges))
	}
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

	if b := tc.Context.Guild.BotMentionEveryone; b != nil {
		ctx.BotCannotMentionEveryone = !*b
	}
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
	ctx.MemberNicks = tc.Context.MemberNicks
	if len(tc.Context.MemberJoinedAgo) > 0 {
		ctx.MemberJoinedAgo = make(map[int64]time.Duration, len(tc.Context.MemberJoinedAgo))
		for id, ago := range tc.Context.MemberJoinedAgo {
			ctx.MemberJoinedAgo[id] = time.Duration(ago)
		}
	}
	for _, ch := range tc.Context.Guild.Channels {
		ctx.Channels[ch.ID] = ch.Name
		ctx.ChannelOrder = append(ctx.ChannelOrder, ch.ID)
	}
	if _, ok := ctx.Channels[ctx.ChannelID]; len(ctx.Channels) > 0 && !ok {
		ctx.Channels[ctx.ChannelID] = ctx.ChannelName // the test's channel, after the declared ones
		ctx.ChannelOrder = append(ctx.ChannelOrder, ctx.ChannelID)
	}
	for _, role := range tc.Context.Guild.Roles {
		ctx.AvailableRoles[role.ID] = types.CtxRole{ID: role.ID, Name: role.Name, Color: role.Color, Position: role.Position, Mentionable: role.Mentionable}
	}
	if len(ctx.AvailableRoles) > 0 {
		// Every guild has @everyone, whose ID is the guild's
		if _, ok := ctx.AvailableRoles[ctx.GuildID]; !ok {
			ctx.AvailableRoles[ctx.GuildID] = types.CtxRole{ID: ctx.GuildID, Name: "@everyone"}
		}
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
		ctx.NoMessage, ctx.NoMember = t.Scheduled(), t.Scheduled()
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

	if expected.OutputEquals != nil {
		expectedNorm := strings.TrimSpace(*expected.OutputEquals)
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
		entry := db.Get(check.UserID, funcs.LimitString(check.Key, 256)) // as dbGet looks it up

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

// checkMessages checks sent or edited messages (how says which); each check matches the
// first message in its channel.
func (r *Runner) checkMessages(messages []runtime.SentMessage, checks []MessageCheck, how string) []string {
	var failures []string

	for i, check := range checks {
		// The nth message (the first by default) in the check's channel, or of any channel
		var found *runtime.SentMessage
		n := max(check.Nth, 1)
		for j := range messages {
			if check.ChannelID == 0 || messages[j].ChannelID == check.ChannelID {
				if n--; n == 0 {
					found = &messages[j]
					break
				}
			}
		}

		if found == nil {
			which := "no message"
			if check.Nth > 1 {
				which = fmt.Sprintf("no message #%d", check.Nth)
			} else if check.ChannelID == 0 {
				which = "no messages"
			}
			where := ""
			if check.ChannelID != 0 {
				where = fmt.Sprintf(" in channel %d", check.ChannelID)
			}
			failures = append(failures, fmt.Sprintf("%s message check %d: %s %s%s", how, i, which, how, where))
			continue
		}

		if check.ContentEquals != nil && found.Content != *check.ContentEquals {
			failures = append(failures,
				fmt.Sprintf("message check %d: content mismatch:\n  expected: %q\n  got:      %q",
					i, *check.ContentEquals, found.Content))
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

		if f := checkPings(found.Pings, check.Pings); f != "" {
			failures = append(failures, fmt.Sprintf("message check %d: %s", i, f))
		}
	}

	return failures
}

// checkPings compares who a message notifies with the expected pings, if any.
func checkPings(got runtime.Pings, check *PingsCheck) string {
	if check == nil {
		return ""
	}
	users := slices.Compact(slices.Sorted(slices.Values(check.Users)))
	roles := slices.Compact(slices.Sorted(slices.Values(check.Roles)))
	if got.Everyone == check.Everyone && slices.Equal(got.Users, users) && slices.Equal(got.Roles, roles) {
		return ""
	}
	want := runtime.Pings{Everyone: check.Everyone, Users: users, Roles: roles}
	return fmt.Sprintf("pings mismatch:\n  expected: %s\n  got:      %s", want, got)
}

// checkRoleChanges verifies role change assertions.
func (r *Runner) checkRoleChanges(changes []runtime.RoleChange, checks []RoleCheck) []string {
	var failures []string

	for i, check := range checks {
		found := false
		for _, change := range changes {
			if change.UserID == check.UserID && change.RoleID == check.RoleID && change.Action == check.Action &&
				(check.Delay == 0 || change.Delay == time.Duration(check.Delay)) {
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

// checkScheduledRuns compares the scheduled runs with the expected list, one by one.
func checkScheduledRuns(runs []runtime.ScheduledRun, checks *[]ScheduledRunCheck) []string {
	if checks == nil {
		return nil
	}
	if len(runs) != len(*checks) {
		return []string{fmt.Sprintf("expected %d scheduled runs, got %d: %s", len(*checks), len(runs), describeRuns(runs))}
	}
	var failures []string
	for i, check := range *checks {
		run := runs[i]
		data := compactJSON(run.ExecData)
		if (check.CCID != 0 && run.CCID != check.CCID) ||
			(check.ChannelID != 0 && run.ChannelID != check.ChannelID) ||
			(check.Delay != 0 && run.Delay != time.Duration(check.Delay)) ||
			(check.Key != nil && (run.Key == nil || *run.Key != *check.Key)) ||
			!strings.Contains(data, check.ExecDataContains) {
			want := fmt.Sprintf("cc %d, channel %d, delay %s", check.CCID, check.ChannelID, time.Duration(check.Delay))
			if check.Key != nil {
				want += fmt.Sprintf(", key %q", *check.Key)
			}
			if check.ExecDataContains != "" {
				want += fmt.Sprintf(", data containing %s", check.ExecDataContains)
			}
			failures = append(failures, fmt.Sprintf("scheduled run %d doesn't match (%s; 0 = any): %s", i, want, describeRuns(runs[i:i+1])))
		}
	}
	return failures
}

func describeRuns(runs []runtime.ScheduledRun) string {
	var parts []string
	for _, r := range runs {
		s := fmt.Sprintf("cc %d in channel %d after %s", r.CCID, r.ChannelID, r.Delay)
		if r.Key != nil {
			s += fmt.Sprintf(" (key %q)", *r.Key)
		}
		parts = append(parts, s+" with "+compactJSON(r.ExecData))
	}
	return "[" + strings.Join(parts, "; ") + "]"
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
