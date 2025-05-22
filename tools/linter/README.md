# YAGPDB Custom Commands Linter

A specialized linter for `.gohtml` template files used in YAGPDB custom commands. This tool analyzes your command templates for common issues, style problems, and potential bugs.

## Features

The linter checks for:

- **Header Validation**: Ensures all commands have proper headers with required fields
- **Configuration Loading**: Validates consistent configuration loading patterns  
- **Permission Checks**: Verifies staff commands include proper permission validation
- **Error Handling**: Identifies Discord API calls that should be wrapped in try-catch blocks
- **Variable Naming**: Enforces consistent naming conventions
- **Database Operations**: Validates proper database operation patterns
- **Trigger Deletion**: Ensures commands clean up properly
- **Regex Patterns**: Checks for common regex pattern issues

## Installation

The linter is a Python script with no external dependencies. Python 3.6+ is required.

```bash
# Make the linter executable
chmod +x tools/linter/yagpdb_lint.py

# Or use the convenience runner
chmod +x lint.py
```

## Usage

### Basic Usage

```bash
# Lint all .gohtml files in current directory
python3 tools/linter/yagpdb_lint.py

# Or use the convenience runner
./lint.py
```

### Command Line Options

```bash
# Lint a specific directory
python3 tools/linter/yagpdb_lint.py --dir ./staff_utility

# Verbose output (shows which files are being processed)
python3 tools/linter/yagpdb_lint.py -v

# Help
python3 tools/linter/yagpdb_lint.py --help
```

### Using the Convenience Runner

```bash
# Default: lint current directory with verbose output
./lint.py

# Lint specific directory
./lint.py --dir ./utility

# All options work the same as the main script
./lint.py -v --dir ./guests
```

## Linting Rules

### Header Rules (`header`)

- **header-missing**: Command file must start with a header comment block
- **header-missing-field**: Headers must include `Author:`, `Trigger type:`, and `Trigger:` fields
- **header-author-format**: Author should include "Vladlena Costescu (@lbds137)"

### Configuration Loading Rules (`config-loading`)

- **config-missing-embed-exec**: Files using `embed_exec` must load it from Commands dictionary
- **config-naming**: Role variables should end with `RoleID`, channel variables with `ChannelID`

### Permission Rules (`permission-check`)

- **permission-missing**: Staff utility commands must include permission checks

### Error Handling Rules (`error-handling`)

- **error-no-try-catch**: Discord API calls should be wrapped in try-catch blocks

Monitored API calls:
- `addMessageReactions`
- `deleteAllMessageReactions` 
- `deleteMessage`
- `sendMessage`
- `giveRoleID`
- `takeRoleID`

### Trigger Deletion Rules (`trigger-deletion`)

- **trigger-deletion-missing**: Commands should include `deleteTrigger` call (except embed_exec)

### Variable Naming Rules (`variable-naming`)

- **variable-naming-dict**: Dictionary variables should follow pattern `{key}Dict` (e.g., `globalDict`)

### Regex Pattern Rules (`regex-pattern`)

- **regex-pattern-escape**: Common regex patterns that may need escaping

### Database Operation Rules (`database-operation`)

- **database-global-write**: Global database writes (user ID 0) should be in staff utilities

## Severity Levels

- **Error** (❌): Issues that likely cause functionality problems
- **Warning** (⚠️): Style issues or potential problems that should be reviewed

## Exit Codes

- `0`: No errors found (warnings are allowed)
- `1`: Errors found or linting failed

## Integration

### Pre-commit Hook

Add to `.git/hooks/pre-commit`:

```bash
#!/bin/bash
python3 tools/linter/yagpdb_lint.py --dir .
if [ $? -ne 0 ]; then
    echo "❌ Linting failed. Fix errors before committing."
    exit 1
fi
```

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Lint YAGPDB Commands
  run: python3 tools/linter/yagpdb_lint.py --dir .
```

### Makefile Integration

```makefile
lint:
	python3 tools/linter/yagpdb_lint.py --dir .

lint-verbose:
	python3 tools/linter/yagpdb_lint.py --dir . -v
```

## Example Output

```
🔍 Running linter...
Linting: utility/dice_roll.gohtml
Linting: staff_utility/admit_user.gohtml
❌ staff_utility/admit_user.gohtml:1:1 [permission-missing] Staff utility command should include permission checks
⚠️ utility/dice_roll.gohtml:38:1 [error-no-try-catch] Discord API call 'sendMessage' should be wrapped in try-catch block

📊 Summary: 1 errors, 1 warnings
```

## Extending the Linter

To add new rules:

1. Create a new class inheriting from `Rule`
2. Implement `name()` and `check()` methods
3. Add the rule to the `YAGPDBLinter` constructor

Example:

```python
class MyCustomRule(Rule):
    def name(self) -> str:
        return "my-custom-rule"
    
    def check(self, filename: str, lines: List[str]) -> List[LintResult]:
        results = []
        # Your checking logic here
        return results
```

## Contributing

When adding new rules:

1. Focus on common issues found in the codebase
2. Provide clear, actionable error messages
3. Use appropriate severity levels
4. Add examples to this documentation
5. Test the rule against existing files

## Limitations

- The linter performs static analysis only
- It cannot validate YAGPDB-specific template syntax
- Complex template logic may not be fully analyzed
- No auto-fix functionality yet implemented

## Future Enhancements

- Auto-fix functionality for common issues
- Integration with code editors (LSP)
- Additional rules for template syntax validation
- Performance optimizations for large codebases
- Configuration file support for custom rules