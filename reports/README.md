# Lint Reports

This directory contains linting reports for the YAGPDB custom commands.

## Files

- **`latest_lint.txt`** - Most recent lint report (tracked in git)
- **`lint_output_YYYYMMDD_HHMMSS.txt`** - Timestamped reports (not tracked in git)

## Usage

### Generate New Report
```bash
# Generate and save a new lint report
./save-lint-report.py

# Or use Makefile
make lint-report
```

### View Reports
```bash
# View latest report
make lint-latest

# List all available reports
make lint-history

# View specific report
cat reports/lint_output_20250521_235003.txt
```

## Report Format

Each report contains:
- Timestamp and git commit
- Full linter output with file locations
- Summary statistics

Example output:
```
⚠️ utility/dice_roll.gohtml:38:1 [error-no-try-catch] Discord API call 'sendMessage' should be wrapped in try-catch block
📊 Summary: 0 errors, 50 warnings, 3 info
```

## Integration

The latest report (`latest_lint.txt`) is tracked in git to maintain a history of code quality over time. Individual timestamped reports are ignored to avoid repository bloat.

Use this for:
- Tracking code quality improvements over time
- Reviewing specific issues before committing
- CI/CD quality gates
- Code review context