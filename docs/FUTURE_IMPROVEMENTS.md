# Future Improvement Ideas

This document tracks potential enhancements for the YAGPDB custom commands project.

## Emulator Enhancements

### Seed gematria tests from the real bootstrap
The gematria tests seed 4 of the 9 tables `gematria_bootstrap.gohtml` builds, so numerals and
Thoth names come out empty. Running the bootstrap first (a test option that executes a setup
template before the test) would test gematria against the real data.

### Remaining emulator gaps
- `.ValueSize` of database entries is an estimate, not YAGPDB's msgpack size.
- Discord functions are mocks: `cembed` keeps the dict instead of building a Discord embed
  (so Discord's field limits aren't checked), `editMessage` and the role/reaction calls only
  record, `sendTemplate` is a no-op, and there is no `sendMessageNoEscape`, components or threads yet.
- Missing standard functions that need Discord data: `snowflakeToTime`, `humanize*`,
  `roleAbove`, `sanitizeText`, `adjective`/`noun`/`verb`.

### Smoke test noise
`scripts/test-all-templates.sh` runs every command with no arguments and an empty database,
so every command that needs input or config "fails" (26 of 45 on 2026-09-24), which hides
real failures.

**Implementation approach:**
- Treat a `parseArgs` usage error as a pass, or give each command a default argument set
  and seed the database from `tools/emulator/testdata/initial_db.json`

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
- [x] Emulator runs on YAGPDB's own template engine and standard functions, with the
      operation limit, value_num, LIKE patterns, and members/roles/messages (2026-09-24)
- [x] Fixed crashes in admit_user, reject_user, screen_user, archive, message_link and
      guest (malformed links, deleted messages, departed users) (2026-09-24)
- [x] Cookbook of tested recipes (`docs/COOKBOOK.md`) (2026-09-24)
- [x] GoLand live templates (`tools/ide/`) (2026-09-24)
- [x] File upload support in emulator (complexMessage with "file"/"filename")
- [x] `db dump` operation for exporting database entries
- [x] Direct array append syntax for `db add`
- [x] Array remove operation for `db remove`
- [x] Comprehensive test coverage for db operations
