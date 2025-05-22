#!/usr/bin/env python3
"""
YAGPDB Custom Commands Linter

A linter for .gohtml template files used in YAGPDB custom commands.
Checks for common patterns, errors, and style issues.
"""

import argparse
import os
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import List, Tuple


@dataclass
class LintResult:
    file: str
    line: int
    column: int
    rule: str
    message: str
    severity: str


class Rule:
    """Base class for linting rules"""
    
    def name(self) -> str:
        raise NotImplementedError
    
    def check(self, filename: str, lines: List[str]) -> List[LintResult]:
        raise NotImplementedError


class HeaderRule(Rule):
    """Checks for proper command headers"""
    
    def name(self) -> str:
        return "header"
    
    def check(self, filename: str, lines: List[str]) -> List[LintResult]:
        results = []
        
        if not lines:
            return results
        
        # Check for header comment block
        if "{{- /*" not in lines[0]:
            results.append(LintResult(
                file=filename,
                line=1,
                column=1,
                rule="header-missing",
                message="Missing header comment block",
                severity="error"
            ))
            return results
        
        # Extract header content
        header_lines = []
        in_header = False
        header_end_line = 0
        
        for i, line in enumerate(lines):
            if "{{- /*" in line:
                in_header = True
            if in_header:
                header_lines.append(line)
            if "*/ -}}" in line:
                header_end_line = i + 1
                break
        
        header_content = "\n".join(header_lines)
        
        # Check for required fields
        required_fields = ["Author:", "Trigger type:"]
        for field in required_fields:
            if field not in header_content:
                results.append(LintResult(
                    file=filename,
                    line=header_end_line,
                    column=1,
                    rule="header-missing-field",
                    message=f"Missing required header field: {field}",
                    severity="error"
                ))
        
        # Check for trigger specification (Trigger:, Interval:, or None trigger type)
        has_trigger = "Trigger:" in header_content
        has_interval = "Interval:" in header_content
        has_none_trigger = "Trigger type: `None`" in header_content or 'Trigger type: "None"' in header_content
        
        if not has_trigger and not has_interval and not has_none_trigger:
            results.append(LintResult(
                file=filename,
                line=header_end_line,
                column=1,
                rule="header-missing-trigger",
                message="Missing trigger specification (should have 'Trigger:', 'Interval:', or 'Trigger type: None')",
                severity="error"
            ))
        
        # Check for proper author format
        author_match = re.search(r"Author:\s*(.+)", header_content)
        if author_match:
            author = author_match.group(1).strip()
            if "Vladlena Costescu" not in author and "@lbds137" not in author:
                results.append(LintResult(
                    file=filename,
                    line=header_end_line,
                    column=1,
                    rule="header-author-format",
                    message="Author should include 'Vladlena Costescu (@lbds137)'",
                    severity="warning"
                ))
        
        return results


class ConfigLoadingRule(Rule):
    """Checks for consistent configuration loading patterns"""
    
    def name(self) -> str:
        return "config-loading"
    
    def check(self, filename: str, lines: List[str]) -> List[LintResult]:
        results = []
        
        has_global_dict = False
        has_commands_dict = False
        has_embed_exec = False
        
        for i, line in enumerate(lines):
            # Check for global dictionary loading
            if '(dbGet 0 "Global").Value' in line:
                has_global_dict = True
            
            # Check for commands dictionary loading
            if '(dbGet 0 "Commands").Value' in line:
                has_commands_dict = True
            
            # Check for embed_exec loading
            if "$embed_exec" in line and "toInt" in line:
                has_embed_exec = True
            
            # Check for proper variable naming in config loading
            config_pattern = re.search(r'\$(\w+) := toInt \(\$\w+\.Get "([^"]+)"\)', line)
            if config_pattern:
                var_name = config_pattern.group(1)
                config_key = config_pattern.group(2)
                
                # Variable should end with appropriate suffix
                if "Role" in config_key and not var_name.endswith("RoleID"):
                    results.append(LintResult(
                        file=filename,
                        line=i + 1,
                        column=1,
                        rule="config-naming",
                        message=f"Role variable '{var_name}' should end with 'RoleID'",
                        severity="warning"
                    ))
                
                if "Channel" in config_key and not var_name.endswith("ChannelID"):
                    results.append(LintResult(
                        file=filename,
                        line=i + 1,
                        column=1,
                        rule="config-naming",
                        message=f"Channel variable '{var_name}' should end with 'ChannelID'",
                        severity="warning"
                    ))
        
        # Check if file uses embed_exec but doesn't load it properly
        uses_embed_exec = any("execCC $embed_exec" in line for line in lines)
        is_bootstrap = "bootstrap.gohtml" in filename
        
        # Bootstrap is special - it gets embed_exec ID from user args and stores it in DB
        if uses_embed_exec and not has_embed_exec and not is_bootstrap:
            results.append(LintResult(
                file=filename,
                line=1,
                column=1,
                rule="config-missing-embed-exec",
                message="File uses embed_exec but doesn't load it from Commands dictionary",
                severity="error"
            ))
        
        return results


class PermissionCheckRule(Rule):
    """Checks for proper permission validation"""
    
    def name(self) -> str:
        return "permission-check"
    
    def check(self, filename: str, lines: List[str]) -> List[LintResult]:
        results = []
        
        # Staff utility commands are protected at YAGPDB command set level
        # so they don't need individual permission checks.
        # This rule is now disabled but kept for potential future use.
        
        # Only check non-staff-utility commands that might need conditional staff permissions
        if "staff_utility/" not in filename:
            # Check for commands that do staff-specific operations without permission checks
            has_db_write_global = any("dbSet 0" in line for line in lines)
            has_permission_check = any(
                "hasRoleID" in line or "permissionCheck" in line 
                for line in lines
            )
            
            # If command writes to global database but isn't in staff_utility and has no permission check
            if has_db_write_global and not has_permission_check:
                results.append(LintResult(
                    file=filename,
                    line=1,
                    column=1,
                    rule="permission-conditional-staff",
                    message="Command performs staff operations but lacks permission checks",
                    severity="warning"
                ))
        
        return results


class ErrorHandlingRule(Rule):
    """Checks for proper error handling patterns"""
    
    def name(self) -> str:
        return "error-handling"
    
    def check(self, filename: str, lines: List[str]) -> List[LintResult]:
        results = []
        
        discord_api_calls = [
            "addMessageReactions",
            "deleteAllMessageReactions",
            "deleteMessage",
            "sendMessage",
            "giveRoleID",
            "takeRoleID",
        ]
        
        for i, line in enumerate(lines):
            for api_call in discord_api_calls:
                if api_call in line:
                    # Check if this line is within a try block
                    in_try_block = False
                    for j in range(max(0, i - 10), i + 1):
                        if "{{ try }}" in lines[j]:
                            in_try_block = True
                            break
                    
                    if not in_try_block:
                        results.append(LintResult(
                            file=filename,
                            line=i + 1,
                            column=1,
                            rule="error-no-try-catch",
                            message=f"Discord API call '{api_call}' should be wrapped in try-catch block",
                            severity="warning"
                        ))
        
        return results


class TriggerDeletionRule(Rule):
    """Checks for proper trigger deletion"""
    
    def name(self) -> str:
        return "trigger-deletion"
    
    def check(self, filename: str, lines: List[str]) -> List[LintResult]:
        results = []
        
        has_trigger_deletion = any("deleteTrigger" in line for line in lines)
        
        # Skip check for embed_exec (trigger type: None)
        is_embed_exec = any('Trigger type: "None"' in line for line in lines)
        
        if not has_trigger_deletion and not is_embed_exec:
            results.append(LintResult(
                file=filename,
                line=len(lines),
                column=1,
                rule="trigger-deletion-missing",
                message="Command should include deleteTrigger call",
                severity="warning"
            ))
        
        return results


class VariableNamingRule(Rule):
    """Checks for consistent variable naming"""
    
    def name(self) -> str:
        return "variable-naming"
    
    def check(self, filename: str, lines: List[str]) -> List[LintResult]:
        results = []
        
        for i, line in enumerate(lines):
            # Check for dictionary variable naming
            dict_pattern = re.search(r'\$(\w+) := \(dbGet 0 "(\w+)"\)\.Value', line)
            if dict_pattern:
                var_name = dict_pattern.group(1)
                db_key = dict_pattern.group(2)
                
                expected_name = db_key.lower() + "Dict"
                if var_name != expected_name:
                    results.append(LintResult(
                        file=filename,
                        line=i + 1,
                        column=1,
                        rule="variable-naming-dict",
                        message=f"Dictionary variable should be named '{expected_name}', got '{var_name}'",
                        severity="warning"
                    ))
        
        return results


class RegexPatternRule(Rule):
    """Checks for common regex patterns"""
    
    def name(self) -> str:
        return "regex-pattern"
    
    def check(self, filename: str, lines: List[str]) -> List[LintResult]:
        results = []
        
        for i, line in enumerate(lines):
            # Check for unescaped regex patterns
            if "reFind" in line or "reReplace" in line:
                # Common patterns that should be escaped
                problematic_patterns = ['"d"', '"s"', '"w"']
                
                for pattern in problematic_patterns:
                    if pattern in line:
                        results.append(LintResult(
                            file=filename,
                            line=i + 1,
                            column=1,
                            rule="regex-pattern-escape",
                            message=f"Regex pattern may need escaping: {pattern}",
                            severity="warning"
                        ))
        
        return results


class DatabaseOperationRule(Rule):
    """Checks for proper database operations"""
    
    def name(self) -> str:
        return "database-operation"
    
    def check(self, filename: str, lines: List[str]) -> List[LintResult]:
        results = []
        
        for i, line in enumerate(lines):
            # Check for direct dbSet operations on user ID 0 without proper validation
            if "dbSet 0" in line:
                # Staff utilities and bootstrap are expected to write to global database
                is_staff_utility = "staff_utility/" in filename
                is_bootstrap = "bootstrap.gohtml" in filename
                
                # Some utility commands may legitimately need to write global data
                # (like bump tracking), so this is now just a warning
                if not is_staff_utility and not is_bootstrap:
                    results.append(LintResult(
                        file=filename,
                        line=i + 1,
                        column=1,
                        rule="database-global-write",
                        message="Global database write - verify this is intentional and properly secured",
                        severity="info"
                    ))
        
        return results


class YAGPDBLinter:
    """Main linter class"""
    
    def __init__(self):
        self.rules = [
            HeaderRule(),
            ConfigLoadingRule(),
            PermissionCheckRule(),
            ErrorHandlingRule(),
            TriggerDeletionRule(),
            VariableNamingRule(),
            RegexPatternRule(),
            DatabaseOperationRule(),
        ]
        self.results = []
    
    def lint_file(self, filename: str) -> None:
        """Lint a single file"""
        try:
            with open(filename, 'r', encoding='utf-8') as f:
                lines = f.readlines()
            
            # Remove newline characters
            lines = [line.rstrip('\n') for line in lines]
            
            for rule in self.rules:
                rule_results = rule.check(filename, lines)
                self.results.extend(rule_results)
                
        except Exception as e:
            print(f"Error reading file {filename}: {e}", file=sys.stderr)
    
    def lint_directory(self, directory: str, verbose: bool = False) -> None:
        """Lint all .gohtml files in a directory"""
        path = Path(directory)
        
        for gohtml_file in path.rglob("*.gohtml"):
            if verbose:
                print(f"Linting: {gohtml_file}")
            self.lint_file(str(gohtml_file))
    
    def print_results(self, verbose: bool = False) -> None:
        """Print linting results"""
        if not self.results:
            print("✅ All files passed linting!")
            return
        
        error_count = sum(1 for r in self.results if r.severity == "error")
        warning_count = sum(1 for r in self.results if r.severity == "warning")
        info_count = sum(1 for r in self.results if r.severity == "info")
        
        for result in self.results:
            if result.severity == "error":
                icon = "❌"
            elif result.severity == "warning":
                icon = "⚠️"
            else:  # info
                icon = "ℹ️"
            print(f"{icon} {result.file}:{result.line}:{result.column} [{result.rule}] {result.message}")
        
        summary_parts = []
        if error_count > 0:
            summary_parts.append(f"{error_count} errors")
        if warning_count > 0:
            summary_parts.append(f"{warning_count} warnings")
        if info_count > 0:
            summary_parts.append(f"{info_count} info")
            
        print(f"\n📊 Summary: {', '.join(summary_parts)}")
    
    def has_errors(self) -> bool:
        """Check if there are any errors (not just warnings)"""
        return any(r.severity == "error" for r in self.results)


def main():
    parser = argparse.ArgumentParser(description="YAGPDB Custom Commands Linter")
    parser.add_argument("--dir", default=".", help="Directory to lint")
    parser.add_argument("-v", "--verbose", action="store_true", help="Verbose output")
    parser.add_argument("--fix", action="store_true", help="Attempt to auto-fix issues (not implemented)")
    
    args = parser.parse_args()
    
    linter = YAGPDBLinter()
    linter.lint_directory(args.dir, args.verbose)
    linter.print_results(args.verbose)
    
    # Exit with error code if there are errors
    if linter.has_errors():
        sys.exit(1)


if __name__ == "__main__":
    main()