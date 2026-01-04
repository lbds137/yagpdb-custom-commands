# Future Improvement Ideas

This document tracks potential enhancements for the YAGPDB custom commands project.

## Emulator Enhancements

### Watch Mode
Auto-rerun tests when `.gohtml` files change. Would improve development workflow significantly.

**Implementation approach:**
- Use `fsnotify` or similar file watcher in Go
- Trigger test re-run on file save
- Clear terminal and show results immediately

### Schema Validation
Warn when database values don't match expected types. Helps catch data corruption early.

**Implementation approach:**
- Define `schema.yaml` with expected types for each db key
- Validate on `dbSet` calls in emulator
- Show warnings (not errors) for type mismatches

### Query Efficiency Warnings
Detect N+1 database access patterns (e.g., `dbGet` inside loops).

**Implementation approach:**
- Track db calls during execution
- Flag patterns like: loop iteration count matches db call count
- Suggest `dbGetPattern` or batch fetching

### Rate Limit Simulation
YAGPDB has strict execution limits. Emulator could enforce these in "strict mode".

**Implementation approach:**
- Track operation count, execution time, output size
- Configurable limits matching YAGPDB's actual limits
- Option to run in permissive vs strict mode

## Testing Improvements

### CI/CD Integration
Run emulator tests in GitHub Actions on push/PR.

**Implementation approach:**
- Add `.github/workflows/test.yml`
- Build emulator and run test suite
- Fail PR if tests fail

### Snapshot Testing
Capture command output and compare against saved snapshots.

**Implementation approach:**
- Save embed structure, text output as JSON snapshots
- Compare on test run, flag differences
- Easy update command for intentional changes

## IDE Integration

### GoLand Plugin
Syntax highlighting and snippets for YAGPDB templates in GoLand.

**Features:**
- Highlight YAGPDB-specific functions
- Snippets for common patterns (parseArgs, embedbuilding)
- Inline documentation on hover
- Error highlighting for common mistakes

**Note:** This would be a JetBrains plugin, not VS Code.

## Documentation

### Interactive Cookbook
Recipe-based documentation showing common patterns:
- Currency/economy system
- Moderation logging
- Welcome messages with roles
- Reaction roles
- Leveling system

### Error Message Improvements
When emulator crashes, link to relevant documentation.

**Implementation approach:**
- Parse error messages for function names
- Map functions to doc URLs
- Include helpful suggestions in error output

---

## Completed Improvements

- [x] File upload support in emulator (complexMessage with "file"/"filename")
- [x] `db dump` operation for exporting database entries
- [x] Direct array append syntax for `db add`
- [x] Array remove operation for `db remove`
- [x] Comprehensive test coverage for db operations
