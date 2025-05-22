package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type LintResult struct {
	File     string
	Line     int
	Column   int
	Rule     string
	Message  string
	Severity string
}

type Linter struct {
	rules   []Rule
	results []LintResult
}

type Rule interface {
	Name() string
	Check(filename string, lines []string) []LintResult
}

func main() {
	var (
		dir     = flag.String("dir", ".", "Directory to lint")
		verbose = flag.Bool("v", false, "Verbose output")
		fix     = flag.Bool("fix", false, "Attempt to auto-fix issues")
	)
	flag.Parse()

	linter := NewLinter()
	
	err := filepath.Walk(*dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if strings.HasSuffix(path, ".gohtml") {
			if *verbose {
				fmt.Printf("Linting: %s\n", path)
			}
			linter.LintFile(path)
		}
		return nil
	})
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
		os.Exit(1)
	}
	
	linter.PrintResults(*verbose)
	
	if len(linter.results) > 0 {
		os.Exit(1)
	}
}

func NewLinter() *Linter {
	return &Linter{
		rules: []Rule{
			&HeaderRule{},
			&ConfigLoadingRule{},
			&PermissionCheckRule{},
			&ErrorHandlingRule{},
			&TriggerDeletionRule{},
			&VariableNamingRule{},
			&RegexPatternRule{},
			&DatabaseOperationRule{},
		},
	}
}

func (l *Linter) LintFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", filename, err)
		return
	}
	defer file.Close()
	
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filename, err)
		return
	}
	
	for _, rule := range l.rules {
		results := rule.Check(filename, lines)
		l.results = append(l.results, results...)
	}
}

func (l *Linter) PrintResults(verbose bool) {
	if len(l.results) == 0 {
		fmt.Println("✅ All files passed linting!")
		return
	}
	
	errorCount := 0
	warningCount := 0
	
	for _, result := range l.results {
		switch result.Severity {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		}
		
		icon := "⚠️"
		if result.Severity == "error" {
			icon = "❌"
		}
		
		fmt.Printf("%s %s:%d:%d [%s] %s\n", 
			icon, result.File, result.Line, result.Column, result.Rule, result.Message)
	}
	
	fmt.Printf("\n📊 Summary: %d errors, %d warnings\n", errorCount, warningCount)
}

// HeaderRule checks for proper command headers
type HeaderRule struct{}

func (r *HeaderRule) Name() string { return "header" }

func (r *HeaderRule) Check(filename string, lines []string) []LintResult {
	var results []LintResult
	
	if len(lines) == 0 {
		return results
	}
	
	// Check for header comment block
	if !strings.Contains(lines[0], "{{- /*") {
		results = append(results, LintResult{
			File:     filename,
			Line:     1,
			Column:   1,
			Rule:     "header-missing",
			Message:  "Missing header comment block",
			Severity: "error",
		})
		return results
	}
	
	// Extract header content
	headerLines := []string{}
	inHeader := false
	headerEndLine := 0
	
	for i, line := range lines {
		if strings.Contains(line, "{{- /*") {
			inHeader = true
		}
		if inHeader {
			headerLines = append(headerLines, line)
		}
		if strings.Contains(line, "*/ -}}") {
			headerEndLine = i + 1
			break
		}
	}
	
	headerContent := strings.Join(headerLines, "\n")
	
	// Check for required fields
	requiredFields := []string{"Author:", "Trigger type:", "Trigger:"}
	for _, field := range requiredFields {
		if !strings.Contains(headerContent, field) {
			results = append(results, LintResult{
				File:     filename,
				Line:     headerEndLine,
				Column:   1,
				Rule:     "header-missing-field",
				Message:  fmt.Sprintf("Missing required header field: %s", field),
				Severity: "error",
			})
		}
	}
	
	// Check for proper author format
	authorRegex := regexp.MustCompile(`Author:\s*(.+)`)
	if match := authorRegex.FindStringSubmatch(headerContent); match != nil {
		author := strings.TrimSpace(match[1])
		if !strings.Contains(author, "Vladlena Costescu") && !strings.Contains(author, "@lbds137") {
			results = append(results, LintResult{
				File:     filename,
				Line:     headerEndLine,
				Column:   1,
				Rule:     "header-author-format",
				Message:  "Author should include 'Vladlena Costescu (@lbds137)'",
				Severity: "warning",
			})
		}
	}
	
	return results
}

// ConfigLoadingRule checks for consistent configuration loading patterns
type ConfigLoadingRule struct{}

func (r *ConfigLoadingRule) Name() string { return "config-loading" }

func (r *ConfigLoadingRule) Check(filename string, lines []string) []LintResult {
	var results []LintResult
	
	hasGlobalDict := false
	hasCommandsDict := false
	hasEmbedExec := false
	
	for i, line := range lines {
		// Check for global dictionary loading
		if strings.Contains(line, `(dbGet 0 "Global").Value`) {
			hasGlobalDict = true
		}
		
		// Check for commands dictionary loading
		if strings.Contains(line, `(dbGet 0 "Commands").Value`) {
			hasCommandsDict = true
		}
		
		// Check for embed_exec loading
		if strings.Contains(line, `$embed_exec`) && strings.Contains(line, `toInt`) {
			hasEmbedExec = true
		}
		
		// Check for proper variable naming in config loading
		configPattern := regexp.MustCompile(`\$(\w+) := toInt \(\$\w+\.Get "([^"]+)"\)`)
		if match := configPattern.FindStringSubmatch(line); match != nil {
			varName := match[1]
			configKey := match[2]
			
			// Variable should end with appropriate suffix
			if strings.Contains(configKey, "Role") && !strings.HasSuffix(varName, "RoleID") {
				results = append(results, LintResult{
					File:     filename,
					Line:     i + 1,
					Column:   1,
					Rule:     "config-naming",
					Message:  fmt.Sprintf("Role variable '%s' should end with 'RoleID'", varName),
					Severity: "warning",
				})
			}
			
			if strings.Contains(configKey, "Channel") && !strings.HasSuffix(varName, "ChannelID") {
				results = append(results, LintResult{
					File:     filename,
					Line:     i + 1,
					Column:   1,
					Rule:     "config-naming",
					Message:  fmt.Sprintf("Channel variable '%s' should end with 'ChannelID'", varName),
					Severity: "warning",
				})
			}
		}
	}
	
	// Check if file uses embed_exec but doesn't load it properly
	usesEmbedExec := false
	for _, line := range lines {
		if strings.Contains(line, "execCC $embed_exec") {
			usesEmbedExec = true
			break
		}
	}
	
	if usesEmbedExec && !hasEmbedExec {
		results = append(results, LintResult{
			File:     filename,
			Line:     1,
			Column:   1,
			Rule:     "config-missing-embed-exec",
			Message:  "File uses embed_exec but doesn't load it from Commands dictionary",
			Severity: "error",
		})
	}
	
	return results
}

// PermissionCheckRule checks for proper permission validation
type PermissionCheckRule struct{}

func (r *PermissionCheckRule) Name() string { return "permission-check" }

func (r *PermissionCheckRule) Check(filename string, lines []string) []LintResult {
	var results []LintResult
	
	// Check if file is in staff_utility but doesn't check permissions
	if strings.Contains(filename, "staff_utility/") {
		hasPermissionCheck := false
		
		for _, line := range lines {
			if strings.Contains(line, "hasRoleID") || strings.Contains(line, "permissionCheck") {
				hasPermissionCheck = true
				break
			}
		}
		
		if !hasPermissionCheck {
			results = append(results, LintResult{
				File:     filename,
				Line:     1,
				Column:   1,
				Rule:     "permission-missing",
				Message:  "Staff utility command should include permission checks",
				Severity: "error",
			})
		}
	}
	
	return results
}

// ErrorHandlingRule checks for proper error handling patterns
type ErrorHandlingRule struct{}

func (r *ErrorHandlingRule) Name() string { return "error-handling" }

func (r *ErrorHandlingRule) Check(filename string, lines []string) []LintResult {
	var results []LintResult
	
	for i, line := range lines {
		// Check for Discord API calls without try-catch
		discordAPICalls := []string{
			"addMessageReactions",
			"deleteAllMessageReactions",
			"deleteMessage",
			"sendMessage",
			"giveRoleID",
			"takeRoleID",
		}
		
		for _, apiCall := range discordAPICalls {
			if strings.Contains(line, apiCall) {
				// Check if this line is within a try block
				inTryBlock := false
				for j := i; j >= 0 && j >= i-10; j-- {
					if strings.Contains(lines[j], "{{ try }}") {
						inTryBlock = true
						break
					}
				}
				
				if !inTryBlock {
					results = append(results, LintResult{
						File:     filename,
						Line:     i + 1,
						Column:   1,
						Rule:     "error-no-try-catch",
						Message:  fmt.Sprintf("Discord API call '%s' should be wrapped in try-catch block", apiCall),
						Severity: "warning",
					})
				}
			}
		}
	}
	
	return results
}

// TriggerDeletionRule checks for proper trigger deletion
type TriggerDeletionRule struct{}

func (r *TriggerDeletionRule) Name() string { return "trigger-deletion" }

func (r *TriggerDeletionRule) Check(filename string, lines []string) []LintResult {
	var results []LintResult
	
	hasTriggerDeletion := false
	for _, line := range lines {
		if strings.Contains(line, "deleteTrigger") {
			hasTriggerDeletion = true
			break
		}
	}
	
	// Skip check for embed_exec (trigger type: None)
	isEmbedExec := false
	for _, line := range lines {
		if strings.Contains(line, `Trigger type: "None"`) {
			isEmbedExec = true
			break
		}
	}
	
	if !hasTriggerDeletion && !isEmbedExec {
		results = append(results, LintResult{
			File:     filename,
			Line:     len(lines),
			Column:   1,
			Rule:     "trigger-deletion-missing",
			Message:  "Command should include deleteTrigger call",
			Severity: "warning",
		})
	}
	
	return results
}

// VariableNamingRule checks for consistent variable naming
type VariableNamingRule struct{}

func (r *VariableNamingRule) Name() string { return "variable-naming" }

func (r *VariableNamingRule) Check(filename string, lines []string) []LintResult {
	var results []LintResult
	
	for i, line := range lines {
		// Check for dictionary variable naming
		dictPattern := regexp.MustCompile(`\$(\w+) := \(dbGet 0 "(\w+)"\)\.Value`)
		if match := dictPattern.FindStringSubmatch(line); match != nil {
			varName := match[1]
			dbKey := match[2]
			
			expectedName := strings.ToLower(dbKey) + "Dict"
			if varName != expectedName {
				results = append(results, LintResult{
					File:     filename,
					Line:     i + 1,
					Column:   1,
					Rule:     "variable-naming-dict",
					Message:  fmt.Sprintf("Dictionary variable should be named '%s', got '%s'", expectedName, varName),
					Severity: "warning",
				})
			}
		}
	}
	
	return results
}

// RegexPatternRule checks for common regex patterns
type RegexPatternRule struct{}

func (r *RegexPatternRule) Name() string { return "regex-pattern" }

func (r *RegexPatternRule) Check(filename string, lines []string) []LintResult {
	var results []LintResult
	
	for i, line := range lines {
		// Check for unescaped regex patterns
		if strings.Contains(line, "reFind") || strings.Contains(line, "reReplace") {
			// Common patterns that should be escaped
			problematicPatterns := []string{
				`"d"`, // Should be `"\\d"`
				`"s"`, // Should be `"\\s"`
				`"w"`, // Should be `"\\w"`
			}
			
			for _, pattern := range problematicPatterns {
				if strings.Contains(line, pattern) {
					results = append(results, LintResult{
						File:     filename,
						Line:     i + 1,
						Column:   1,
						Rule:     "regex-pattern-escape",
						Message:  fmt.Sprintf("Regex pattern may need escaping: %s", pattern),
						Severity: "warning",
					})
				}
			}
		}
	}
	
	return results
}

// DatabaseOperationRule checks for proper database operations
type DatabaseOperationRule struct{}

func (r *DatabaseOperationRule) Name() string { return "database-operation" }

func (r *DatabaseOperationRule) Check(filename string, lines []string) []LintResult {
	var results []LintResult
	
	for i, line := range lines {
		// Check for direct dbSet operations on user ID 0 without proper validation
		if strings.Contains(line, "dbSet 0") {
			// This should be in staff utilities or bootstrap
			isStaffUtility := strings.Contains(filename, "staff_utility/")
			isBootstrap := strings.Contains(filename, "bootstrap.gohtml")
			
			if !isStaffUtility && !isBootstrap {
				results = append(results, LintResult{
					File:     filename,
					Line:     i + 1,
					Column:   1,
					Rule:     "database-global-write",
					Message:  "Global database writes (user ID 0) should be in staff utilities",
					Severity: "warning",
				})
			}
		}
	}
	
	return results
}