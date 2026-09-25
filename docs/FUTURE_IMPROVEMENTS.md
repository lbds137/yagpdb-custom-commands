# Future Improvement Ideas

This document tracks potential enhancements for the YAGPDB custom commands project.

## Emulator Enhancements

### Template operation limit
YAGPDB stops a template after 1M operations (2.5M with premium). Go's standard
`text/template` can't count operations, so `-strict` doesn't enforce this limit.

**Implementation approach:**
- Vendor YAGPDB's fork of `text/template` (`vendor/yagpdb/lib/template`, which has `MaxOps`)
  into the emulator instead of the standard library

### Smoke test noise
`scripts/test-all-templates.sh` runs every command with no arguments and an empty database,
so every command that needs input or config "fails" (26 of 45 on 2026-09-24), which hides
real failures.

### Malformed message links crash staff commands
`admit_user`, `reject_user`, `screen_user`, `archive` (and `message_link`) take a message link
but don't check that the regex matched, so a malformed link fails with "index out of range"
instead of a usage message.

**Implementation approach:**
- Treat a `parseArgs` usage error as a pass, or give each command a default argument set

## IDE Integration

### GoLand Plugin
Live templates are done (`tools/ide/`). A plugin would add what they can't:

**Features:**
- Highlight YAGPDB-specific functions
- Inline documentation on hover (the function list is in
  `tools/emulator/internal/runtime/yagpdb_funcs.go`)
- Error highlighting for common mistakes (the emulator's hints could be reused)

**Note:** This would be a JetBrains plugin (Kotlin/Gradle), not VS Code.

---

## Completed Improvements

- [x] Fixed the two failing database tests: `dbGet` returned stored strings and numbers
      wrapped in `TemplateValue` (2026-09-24)
- [x] CI: `.github/workflows/test.yml` runs `make ci` (2026-09-24)
- [x] Strict mode: YAGPDB's execution limits (call counters, output, response, template
      length, time), warnings by default and failures with `-strict` (2026-09-24)
- [x] Warnings for database calls inside `range` loops (2026-09-24)
- [x] Schema validation (`-schema db_schema.yaml`) (2026-09-24)
- [x] Snapshot testing (`snapshot: true`, `-update-snapshots`) (2026-09-24)
- [x] Watch mode (`yagtest watch`, `make watch`) (2026-09-24)
- [x] Error hints with typo suggestions and docs links (2026-09-24)
- [x] Cookbook of tested recipes (`docs/COOKBOOK.md`) (2026-09-24)
- [x] GoLand live templates (`tools/ide/`) (2026-09-24)
- [x] File upload support in emulator (complexMessage with "file"/"filename")
- [x] `db dump` operation for exporting database entries
- [x] Direct array append syntax for `db add`
- [x] Array remove operation for `db remove`
- [x] Comprehensive test coverage for db operations
