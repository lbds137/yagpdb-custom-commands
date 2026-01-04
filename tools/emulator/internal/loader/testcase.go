// Package loader provides test case loading and parsing for the YAGPDB emulator.
package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// TestCase represents a single test definition.
type TestCase struct {
	Name           string            `yaml:"name"`
	Template       string            `yaml:"template"`        // Path to template file
	TemplateSource string            `yaml:"template_source"` // Inline template source
	Context        ContextDef        `yaml:"context"`
	SetupDB        []DBEntry         `yaml:"setup_db"`
	CommandMap     map[int64]string  `yaml:"command_map"` // Maps command IDs to template paths
	Expected       ExpectedResult    `yaml:"expected"`
	Assertions     Assertions        `yaml:"assertions"`
}

// ContextDef defines the execution context for a test.
type ContextDef struct {
	User    UserDef    `yaml:"user"`
	Channel ChannelDef `yaml:"channel"`
	Guild   GuildDef   `yaml:"guild"`
	Args    []string   `yaml:"args"`
	CmdArgs []string   `yaml:"cmd_args"`
	ExecData map[string]interface{} `yaml:"exec_data"`
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
	ID   int64  `yaml:"id"`
	Name string `yaml:"name"`
}

// DBEntry represents a database entry for setup.
type DBEntry struct {
	UserID int64       `yaml:"user_id"`
	Key    string      `yaml:"key"`
	Value  interface{} `yaml:"value"`
}

// ExpectedResult defines expected output.
type ExpectedResult struct {
	OutputEquals   string `yaml:"output_equals"`   // Exact match
	OutputContains string `yaml:"output_contains"` // Substring match
	OutputMatches  string `yaml:"output_matches"`  // Regex match
	ErrorContains  string `yaml:"error_contains"`  // Expected error
}

// Assertions defines post-execution checks.
type Assertions struct {
	DBChecks     []DBCheck     `yaml:"db_checks"`
	SentMessages []MessageCheck `yaml:"sent_messages"`
	RoleChanges  []RoleCheck   `yaml:"role_changes"`
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
	ChannelID       int64  `yaml:"channel_id"`
	ContentEquals   string `yaml:"content_equals"`
	ContentContains string `yaml:"content_contains"`
	HasEmbed        bool   `yaml:"has_embed"`
	EmbedTitle      string `yaml:"embed_title"`
}

// RoleCheck defines a role change assertion.
type RoleCheck struct {
	UserID int64  `yaml:"user_id"`
	RoleID int64  `yaml:"role_id"`
	Action string `yaml:"action"` // "add" or "remove"
}

// TestSuite represents a collection of test cases.
type TestSuite struct {
	Name       string           `yaml:"name"`
	Tests      []TestCase       `yaml:"tests"`
	Defaults   ContextDef       `yaml:"defaults"`    // Default context values
	SetupDB    []DBEntry        `yaml:"setup_db"`    // Shared database setup
	CommandMap map[int64]string `yaml:"command_map"` // Shared command ID mapping
}

// LoadTestCase loads a single test case from a YAML file.
func LoadTestCase(filename string) (*TestCase, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading test file: %w", err)
	}

	var tc TestCase
	if err := yaml.Unmarshal(data, &tc); err != nil {
		return nil, fmt.Errorf("parsing test YAML: %w", err)
	}

	// Apply defaults
	tc.applyDefaults()

	return &tc, nil
}

// LoadTestSuite loads a test suite from a YAML file.
func LoadTestSuite(filename string) (*TestSuite, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading test suite file: %w", err)
	}

	var ts TestSuite
	if err := yaml.Unmarshal(data, &ts); err != nil {
		return nil, fmt.Errorf("parsing test suite YAML: %w", err)
	}

	// Apply defaults to all tests
	for i := range ts.Tests {
		ts.Tests[i].mergeDefaults(ts.Defaults, ts.SetupDB, ts.CommandMap)
	}

	return &ts, nil
}

// LoadTestsFromDir loads all test files from a directory.
func LoadTestsFromDir(dir string) ([]*TestCase, error) {
	var tests []*TestCase

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Load .yaml and .yml files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Try loading as test suite first
		ts, err := LoadTestSuite(path)
		if err == nil && len(ts.Tests) > 0 {
			for i := range ts.Tests {
				tests = append(tests, &ts.Tests[i])
			}
			return nil
		}

		// Try loading as single test case
		tc, err := LoadTestCase(path)
		if err != nil {
			return fmt.Errorf("loading %s: %w", path, err)
		}

		tests = append(tests, tc)
		return nil
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

	// Merge guild defaults
	if tc.Context.Guild.ID == 0 {
		tc.Context.Guild.ID = defaults.Guild.ID
	}
	if tc.Context.Guild.Name == "" {
		tc.Context.Guild.Name = defaults.Guild.Name
	}

	// Prepend shared DB entries
	if len(sharedDB) > 0 {
		tc.SetupDB = append(sharedDB, tc.SetupDB...)
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
