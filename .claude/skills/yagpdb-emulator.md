# YAGPDB Template Emulator

## Quick Commands

```bash
# Build emulator
cd tools/emulator && go build -o bin/yagtest ./cmd/yagtest

# Run single template
./bin/yagtest run ../../utility/timestamp.gohtml

# Run test suite
./bin/yagtest test testdata/command_tests.yaml

# Run all templates (check for parse errors)
./scripts/test-all-templates.sh

# Find missing functions
./scripts/find-missing-functions.sh
```

## Test Case Format

```yaml
- name: "Test description"
  template: "../../../utility/example.gohtml"
  context:
    args: ["arg1", "arg2"]
    cmd_args: ["arg1", "arg2"]
  # Optional: override database values
  setup_db:
    - user_id: 0
      key: "Global"
      value:
        Delete Trigger Delay: 5
```

## Project Structure

```
tools/emulator/
├── cmd/yagtest/        # CLI entry point
├── internal/
│   ├── context/        # Mock Discord context (users, channels, guilds)
│   ├── funcs/          # Template function implementations
│   ├── runtime/        # Template engine and execution
│   └── test/           # Test runner and YAML parsing
├── testdata/
│   ├── command_tests.yaml    # Main test suite
│   └── templates/            # Mock templates for execCC
└── bin/                # Built binaries
```

## Adding Missing Functions

1. Check if function exists in `internal/funcs/` or `internal/runtime/engine.go`
2. Add implementation to appropriate file
3. Register in `engine.go` FuncMap
4. Update `scripts/find-missing-functions.sh` IMPLEMENTED array
5. Rebuild and test

## Key Files

| File | Purpose |
|------|---------|
| `engine.go` | Template FuncMap and execution |
| `funcs/standard.go` | Core template functions |
| `funcs/database.go` | dbGet/dbSet implementations |
| `context/mock.go` | Discord context mocking |

## Common Issues

**"function X not defined"**: Add to FuncMap in `engine.go`

**"no args for template"**: Template expects `.CmdArgs` - add to test context

**"nil pointer"**: Check mock context has required fields

## When to Use This Skill

- Running local template tests
- Debugging emulator issues
- Adding new function implementations
- Creating test cases for templates
