// Package loader provides test case loading and parsing for the YAGPDB emulator.
package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/runtime"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// TestCase represents a single test definition.
type TestCase struct {
	Name           string     `yaml:"name"`
	Template       string     `yaml:"template"`        // Path to template file
	TemplateSource string     `yaml:"template_source"` // Inline template source
	Context        ContextDef `yaml:"context"`
	SetupDB        []DBEntry  `yaml:"setup_db"`
	// SetupTemplates run in order before the test, on the same database (e.g. a bootstrap)
	SetupTemplates []string         `yaml:"setup_templates"`
	CommandMap     map[int64]string `yaml:"command_map"` // Maps command IDs to template paths
	Expected       ExpectedResult   `yaml:"expected"`
	Assertions     Assertions       `yaml:"assertions"`
	Strict         bool             `yaml:"strict"`   // Fail on YAGPDB execution limits (like -strict)
	Snapshot       bool             `yaml:"snapshot"` // Compare results with the saved snapshot

	SourceFile string `yaml:"-"` // YAML file the test came from (for snapshots)
}

// ContextDef defines the execution context for a test.
type ContextDef struct {
	User    UserDef    `yaml:"user"`
	Channel ChannelDef `yaml:"channel"`
	Guild   GuildDef   `yaml:"guild"`
	// Args are the arguments after the trigger; the message is the trigger followed by them
	Args     []string               `yaml:"args"`
	ExecData map[string]interface{} `yaml:"exec_data"`
	Premium  *bool                  `yaml:"premium"`  // Default true
	Reaction *ReactionDef           `yaml:"reaction"` // Makes this a reaction-triggered run
	Messages []MessageDef           `yaml:"messages"` // Messages getMessage can find
	// MessageContent is the whole triggering message, trigger included (instead of args);
	// with exec_data or reaction, the message .Message is
	MessageContent string  `yaml:"message_content"`
	Members        []int64 `yaml:"members"` // If set, the only users getMember finds
	// MemberRoles gives other members' roles (the triggering user's are user.roles)
	MemberRoles map[int64][]int64 `yaml:"member_roles"`
	// MemberNicks gives members' nicknames, the triggering user's included
	MemberNicks map[int64]string `yaml:"member_nicks"`
	// MemberJoinedAgo gives how long before the run members joined, like "12h" (default 30 days)
	MemberJoinedAgo map[int64]Duration `yaml:"member_joined_ago"`
}

// MessageDef is an existing Discord message.
type MessageDef struct {
	ID        int64  `yaml:"id"`
	ChannelID int64  `yaml:"channel_id"`
	AuthorID  int64  `yaml:"author_id"`
	Content   string `yaml:"content"`
}

// ReactionDef describes the reaction that triggered a command.
type ReactionDef struct {
	Emoji     string `yaml:"emoji"`      // Unicode emoji, or a custom emoji's name
	EmojiID   int64  `yaml:"emoji_id"`   // Custom emoji ID (0 for Unicode emoji)
	MessageID int64  `yaml:"message_id"` // Message that was reacted to
	Added     *bool  `yaml:"added"`      // false for a removed reaction (default true)
}

// UserDef defines user context.
type UserDef struct {
	ID            int64   `yaml:"id"`
	Username      string  `yaml:"username"`
	Discriminator string  `yaml:"discriminator"`
	Roles         []int64 `yaml:"roles"`
}

// ChannelDef defines channel context.
type ChannelDef struct {
	ID   int64  `yaml:"id"`
	Name string `yaml:"name"`
}

// GuildDef defines guild/server context.
type GuildDef struct {
	ID    int64     `yaml:"id"`
	Name  string    `yaml:"name"`
	Roles []RoleDef `yaml:"roles"` // If set, getRole and targetHasRole know only these
	// Channels, if set, are the server's other channels (the test's channel always exists):
	// channel arguments then accept only these, by ID or by name
	Channels []ChannelDef `yaml:"channels"`
	// OwnerID is .Guild.OwnerID (default: the triggering user)
	OwnerID int64  `yaml:"owner_id"`
	Prefix  string `yaml:"prefix"` // The command prefix (default: YAGPDB's "-")
	// BotMentionEveryone is whether the bot has Discord's "Mention @everyone, @here, and
	// All Roles" permission (default true). Without it @everyone and @here never ping, and
	// a role mention pings only a mentionable role.
	BotMentionEveryone *bool `yaml:"bot_mention_everyone"`
}

// RoleDef is a role in the guild.
type RoleDef struct {
	ID       int64  `yaml:"id"`
	Name     string `yaml:"name"`
	Color    int    `yaml:"color"`
	Position int    `yaml:"position"` // Higher is above; roleAbove compares these
	// Mentionable lets anyone ping the role; see GuildDef.BotMentionEveryone
	Mentionable bool `yaml:"mentionable"`
}

// DBEntry represents a database entry for setup.
type DBEntry struct {
	UserID int64       `yaml:"user_id"`
	Key    string      `yaml:"key"`
	Value  interface{} `yaml:"value"`
}

// ExpectedResult defines expected output.
type ExpectedResult struct {
	OutputEquals    *string `yaml:"output_equals"`    // Exact match ("" asserts no output)
	OutputContains  string  `yaml:"output_contains"`  // Substring match
	OutputMatches   string  `yaml:"output_matches"`   // Regex match
	ErrorContains   string  `yaml:"error_contains"`   // Expected error
	WarningContains string  `yaml:"warning_contains"` // Expected diagnostic
}

// Assertions defines post-execution checks.
type Assertions struct {
	DBChecks     []DBCheck      `yaml:"db_checks"`
	SentMessages []MessageCheck `yaml:"sent_messages"`
	// EditedMessages check messages as editMessage left them
	EditedMessages []MessageCheck `yaml:"edited_messages"`
	RoleChanges    []RoleCheck    `yaml:"role_changes"`
	// NoRoleChanges asserts the run changed no roles (a give of a role the member has, say)
	NoRoleChanges bool `yaml:"no_role_changes"`
	// ResponsePings is exactly who the response (the template's output) notifies
	ResponsePings *PingsCheck `yaml:"response_pings"`
	// ScheduledRuns are exactly the runs execCC with a delay and scheduleUniqueCC left
	// scheduled, in order (`[]` for none)
	ScheduledRuns *[]ScheduledRunCheck `yaml:"scheduled_runs"`
}

// ScheduledRunCheck matches a scheduled run; unset fields match anything.
type ScheduledRunCheck struct {
	CCID             int64    `yaml:"cc_id"`
	ChannelID        int64    `yaml:"channel_id"`
	Delay            Duration `yaml:"delay"`
	Key              *string  `yaml:"key"`                // scheduleUniqueCC's key
	ExecDataContains string   `yaml:"exec_data_contains"` // substring of the data as JSON
}

// PingsCheck is exactly who a message notifies; unset fields expect no one.
type PingsCheck struct {
	Everyone bool    `yaml:"everyone"` // @everyone or @here
	Users    []int64 `yaml:"users"`
	Roles    []int64 `yaml:"roles"`
}

// DBCheck defines a database assertion.
type DBCheck struct {
	UserID        int64       `yaml:"user_id"`
	Key           string      `yaml:"key"`
	ValueEquals   interface{} `yaml:"value_equals"`
	ValueContains string      `yaml:"value_contains"`
	NotExists     bool        `yaml:"not_exists"`
}

// MessageCheck defines a sent message assertion.
type MessageCheck struct {
	ChannelID       int64   `yaml:"channel_id"`
	Nth             int     `yaml:"nth"`            // which message in the channel (or of all): 1 = first, the default
	ContentEquals   *string `yaml:"content_equals"` // "" asserts empty content
	ContentContains string  `yaml:"content_contains"`
	HasEmbed        bool    `yaml:"has_embed"`
	EmbedTitle      string  `yaml:"embed_title"`
	EmbedContains   string  `yaml:"embed_contains"` // Substring of the embed as JSON (title, fields, ...)
	// Pings is exactly who the message notifies (edits notify no one)
	Pings *PingsCheck `yaml:"pings"`
}

// RoleCheck defines a role change assertion.
type RoleCheck struct {
	UserID int64    `yaml:"user_id"`
	RoleID int64    `yaml:"role_id"`
	Action string   `yaml:"action"` // "add" or "remove"
	Delay  Duration `yaml:"delay"`  // if set, the change is scheduled this far ahead
}

// TestSuite represents a collection of test cases.
type TestSuite struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Tests       []TestCase `yaml:"tests"`
	Defaults    ContextDef `yaml:"defaults"` // Default context values
	SetupDB     []DBEntry  `yaml:"setup_db"` // Shared database setup
	// SetupTemplates run before every test in the suite, ahead of the test's own
	SetupTemplates []string         `yaml:"setup_templates"`
	CommandMap     map[int64]string `yaml:"command_map"` // Shared command ID mapping
}

// LoadTestCase loads a single test case from a YAML file.
func LoadTestCase(filename string) (*TestCase, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading test file: %w", err)
	}

	var tc TestCase
	if err := types.StrictYAML(data, &tc); err != nil {
		return nil, fmt.Errorf("parsing test YAML: %w", err)
	}

	// Apply defaults
	tc.applyDefaults()
	tc.SourceFile = filename

	return &tc, nil
}

// LoadTestSuite loads a test suite from a YAML file.
func LoadTestSuite(filename string) (*TestSuite, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading test suite file: %w", err)
	}

	var ts TestSuite
	if err := types.StrictYAML(data, &ts); err != nil {
		return nil, fmt.Errorf("parsing test suite YAML: %w", err)
	}

	// A suite's defaults can't trigger or feed a command; each test says that for itself
	d := ts.Defaults
	if len(d.Args) > 0 || d.ExecData != nil || d.MessageContent != "" || d.Reaction != nil {
		return nil, fmt.Errorf("%s: defaults can't set args, exec_data, message_content or reaction; set them per test", filename)
	}

	// Apply defaults to all tests
	for i := range ts.Tests {
		ts.Tests[i].mergeDefaults(ts.Defaults, ts.SetupDB, ts.CommandMap)
		if len(ts.SetupTemplates) > 0 {
			ts.Tests[i].SetupTemplates = append(append([]string{}, ts.SetupTemplates...), ts.Tests[i].SetupTemplates...)
		}
		ts.Tests[i].SourceFile = filename
		// YAGPDB runs no custom command for a bot's message (customcommands/bot.go)
		if ts.Tests[i].Context.User.ID == runtime.BotUserID {
			return nil, fmt.Errorf("%s: test %q: user.id %d is the bot's; YAGPDB runs no custom command for a bot's message",
				filename, ts.Tests[i].Name, runtime.BotUserID)
		}
	}

	return &ts, nil
}

// LoadTestFile loads a test file: a suite if it has a top-level tests: key, otherwise a
// single test case. A suite that doesn't parse is an error, not a single test case.
func LoadTestFile(filename string) ([]*TestCase, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading test file: %w", err)
	}
	var keys map[string]interface{}
	if err := yaml.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filename, err)
	}
	if _, isSuite := keys["tests"]; !isSuite {
		tc, err := LoadTestCase(filename)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", filename, err)
		}
		return []*TestCase{tc}, nil
	}
	ts, err := LoadTestSuite(filename)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", filename, err)
	}
	tests := make([]*TestCase, len(ts.Tests))
	for i := range ts.Tests {
		tests[i] = &ts.Tests[i]
	}
	return tests, nil
}

// LoadTestsFromDir loads all test files from a directory.
func LoadTestsFromDir(dir string) ([]*TestCase, error) {
	var tests []*TestCase

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == "__snapshots__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Load .yaml and .yml files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		loaded, err := LoadTestFile(path)
		tests = append(tests, loaded...)
		return err
	})

	if err != nil {
		return nil, err
	}

	return tests, nil
}

// applyDefaults sets default values for unset fields.
func (tc *TestCase) applyDefaults() {
	if tc.Context.User.ID == 0 {
		tc.Context.User.ID = 987654321098765432
	}
	if tc.Context.User.Username == "" {
		tc.Context.User.Username = "TestUser"
	}
	if tc.Context.User.Discriminator == "" {
		tc.Context.User.Discriminator = "0001"
	}
	if tc.Context.Channel.ID == 0 {
		tc.Context.Channel.ID = 123456789012345678
	}
	if tc.Context.Channel.Name == "" {
		tc.Context.Channel.Name = "test-channel"
	}
	if tc.Context.Guild.ID == 0 {
		tc.Context.Guild.ID = 111222333444555666
	}
	if tc.Context.Guild.Name == "" {
		tc.Context.Guild.Name = "Test Server"
	}
}

// mergeDefaults merges suite defaults into a test case.
func (tc *TestCase) mergeDefaults(defaults ContextDef, sharedDB []DBEntry, sharedCommandMap map[int64]string) {
	// Merge user defaults
	if tc.Context.User.ID == 0 {
		tc.Context.User.ID = defaults.User.ID
	}
	if tc.Context.User.Username == "" {
		tc.Context.User.Username = defaults.User.Username
	}
	if tc.Context.User.Discriminator == "" {
		tc.Context.User.Discriminator = defaults.User.Discriminator
	}
	if tc.Context.User.Roles == nil && defaults.User.Roles != nil {
		tc.Context.User.Roles = defaults.User.Roles
	}

	// Merge channel defaults
	if tc.Context.Channel.ID == 0 {
		tc.Context.Channel.ID = defaults.Channel.ID
	}
	if tc.Context.Channel.Name == "" {
		tc.Context.Channel.Name = defaults.Channel.Name
	}

	if tc.Context.Premium == nil {
		tc.Context.Premium = defaults.Premium
	}
	if tc.Context.Messages == nil {
		tc.Context.Messages = defaults.Messages
	}
	if tc.Context.Members == nil {
		tc.Context.Members = defaults.Members
	}
	if tc.Context.MemberRoles == nil {
		tc.Context.MemberRoles = defaults.MemberRoles
	}
	if tc.Context.MemberNicks == nil {
		tc.Context.MemberNicks = defaults.MemberNicks
	}
	if tc.Context.MemberJoinedAgo == nil {
		tc.Context.MemberJoinedAgo = defaults.MemberJoinedAgo
	}
	if tc.Context.Guild.Roles == nil {
		tc.Context.Guild.Roles = defaults.Guild.Roles
	}
	if tc.Context.Guild.Channels == nil {
		tc.Context.Guild.Channels = defaults.Guild.Channels
	}

	// Merge guild defaults
	if tc.Context.Guild.ID == 0 {
		tc.Context.Guild.ID = defaults.Guild.ID
	}
	if tc.Context.Guild.Name == "" {
		tc.Context.Guild.Name = defaults.Guild.Name
	}
	if tc.Context.Guild.Prefix == "" {
		tc.Context.Guild.Prefix = defaults.Guild.Prefix
	}
	if tc.Context.Guild.OwnerID == 0 {
		tc.Context.Guild.OwnerID = defaults.Guild.OwnerID
	}
	if tc.Context.Guild.BotMentionEveryone == nil {
		tc.Context.Guild.BotMentionEveryone = defaults.Guild.BotMentionEveryone
	}

	// Prepend shared DB entries
	if len(sharedDB) > 0 {
		tc.SetupDB = append(append([]DBEntry{}, sharedDB...), tc.SetupDB...)
	}

	// Merge command map (suite-level + test-level, test overrides suite)
	if len(sharedCommandMap) > 0 {
		if tc.CommandMap == nil {
			tc.CommandMap = make(map[int64]string)
		}
		for k, v := range sharedCommandMap {
			if _, exists := tc.CommandMap[k]; !exists {
				tc.CommandMap[k] = v
			}
		}
	}

	// Apply standard defaults
	tc.applyDefaults()
}

// GetTemplateSource returns the template source, loading from file if needed.
func (tc *TestCase) GetTemplateSource(baseDir string) (string, error) {
	if tc.TemplateSource != "" {
		return tc.TemplateSource, nil
	}

	if tc.Template == "" {
		return "", fmt.Errorf("no template specified")
	}

	// Resolve template path
	templatePath := tc.Template
	if !filepath.IsAbs(templatePath) {
		templatePath = filepath.Join(baseDir, templatePath)
	}

	data, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("reading template %s: %w", templatePath, err)
	}

	return string(data), nil
}

// Duration is a Go duration in YAML, like "12h" or "90m".
type Duration time.Duration

// UnmarshalYAML parses a duration string.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	parsed, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("line %d: %w (use h, m or s: 72h, not 3d)", node.Line, err)
	}
	if parsed < 0 {
		return fmt.Errorf("line %d: %q is negative, a join time in the future", node.Line, node.Value)
	}
	*d = Duration(parsed)
	return nil
}
