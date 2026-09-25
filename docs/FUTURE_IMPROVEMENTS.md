# Future Improvement Ideas

This document tracks potential enhancements for the YAGPDB custom commands project.

## Emulator Enhancements

### Remaining emulator gaps
- Values holding Discord objects (a member, a message, a `cembed`, a whole `dbGet`
  entry) serialize as the emulator's types, so their size differs from YAGPDB's. A value
  whose overflow past 100000 bytes is only whitespace is stored whole; YAGPDB stores it
  cut off, so reading it back fails.
- A NaN value_num (`dbSet` of "NaN") sorts unpredictably in dbTopEntries, dbRank and
  dbDelMultiple; Postgres puts NaN above every number. setup_db keys aren't cut to 256
  bytes, so a longer fixture key can't be read back.
- `execCC` with a delay passes its data as is; YAGPDB msgpack-encodes it (so types change
  and over 1000000 bytes fails with "ExecData is too big").
- The output limit doesn't drop leading whitespace as YAGPDB's LimitWriter does, so a
  response that is mostly leading spaces can fail the 25k limit in the emulator only.
- Discord functions are mocks: the role/reaction calls only record, `sendTemplate` is a no-op, and there is no `sendMessageNoEscape`, components or threads yet.
- Missing standard functions that need Discord data: `snowflakeToTime`, `humanize*`,
  `roleAbove`, `sanitizeText`, `adjective`/`noun`/`verb`.
- Role lookups accept IDs, mentions and names everywhere; YAGPDB's `FindRole` accepts a
  different set per function.
- `editMessage` gaps: the channel argument is read as a number (YAGPDB also takes channel
  names and refuses floats and unknown channels up front); a stored message keeps only its
  first embed and no file, so edits of multi-embed or file messages can differ; edits don't
  set `EditedTimestamp`; message builders read keys as a map, so a repeated key (two
  `"embed"`s) counts once. A test message is the bot's to edit only with `author_id:
  1234567890`.
- Message assertions match the first message per channel, and `content_equals: ""` checks
  nothing, so an emptied content can't be asserted.
- A trailing backslash in a LIKE pattern is ignored; Postgres errors.
- `parseArgs` resolves `user`, `member` and `role` arguments through the mocks (a test that
  declares no guild roles accepts any role), and accepts any channel ID, since the
  emulator has no channel list.
- A command run by `execCC` from a reaction-triggered command sees the test's
  `message_content` as `.Message`; YAGPDB passes on the reacted-to message.
- Interval and None runs get a `.Message` with empty content; YAGPDB gives them none.
- Triggers are always case-insensitive (YAGPDB's default); a header can't say otherwise.

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
- [x] Smoke test (`make test-templates`, part of `make ci`) runs on the database a fresh
      bootstrap leaves; asking for arguments passes and regex-trigger commands are skipped,
      so a failure is a real error (2026-09-25)
- [x] Messages have `.Link`; sent messages get unique IDs and `getMessage` finds them (a nil
      channel is the current one, as in YAGPDB); tests can set `guild.owner_id` (bump_remind's owner ping is tested) (2026-09-25)
- [x] The triggering message is built like YAGPDB's: tests give the arguments after the
      trigger their template's header names (or the whole message), and `.Args`, `.Cmd`,
      `.CmdArgs` and `.StrippedMsg` come from YAGPDB's `CheckMatch`, so `.Args` starts with
      the trigger. A message that doesn't match the trigger is an error, and Regex triggers
      need `message_content`. Without a message trigger (execCC, reaction, interval) those
      fields are unset and `parseArgs` parses nothing, as in YAGPDB. `guild.prefix` sets
      `.ServerPrefix`; `yagtest run -message` gives a whole message; commands run by
      `execCC` see the caller's `.Message` (2026-09-25)
- [x] Members have nicknames and join times (`member_nicks`, `member_joined_ago`; default
      30 days ago); `.Member` and `getMember` build the same member, and `JoinedAt` is
      discordgo's `Timestamp` string, formatted as Discord sends it. guest's grace-period
      path is tested. A suite that doesn't parse reports its own error (2026-09-25)
- [x] `dbCount`, `dbRank` and `dbDelMultiple` are ported with YAGPDB's query dict
      (`userID`, `pattern`, `reverse`) and its errors; keys and patterns are cut to 256
      bytes as YAGPDB cuts them; the database functions declare YAGPDB's parameter types,
      so a float user ID fails as in production (2026-09-25)
- [x] Database values are serialized as YAGPDB serializes them (msgpack v4 with its
      sdict/dict/cslice extensions, through its LimitWriter): `.ValueSize` is exact, and
      a value over 100000 bytes fails with YAGPDB's "short write", unless the overflow
      is only whitespace (2026-09-25)
- [x] `editMessage` edits the message: someone else's or a missing one is refused as
      Discord refuses it, the edited message gets the send checks, and `complexMessageEdit`
      is ported (with YAGPDB's "both content and embed cannot be null"). Tests assert with
      `edited_messages`; channel_link is tested (2026-09-25)
- [x] Hand-written files are parsed strictly (test YAML, `-schema` YAML, `-context` and
      `-db` JSON): an unknown key, a second YAML document or trailing JSON is an error, and a
      suite's defaults can't set args, exec_data, message_content or reaction. A misspelled
      assertion can't pass silently (none were found). Free-form values (exec_data,
      setup_db values) are still unchecked (2026-09-25)
- [x] `parseArgs` is ported from YAGPDB and dcmd: the last argument takes the rest of the
      message, quotes group words, types and bounds are checked with dcmd's errors, and a
      command run by execCC or a reaction parses nothing (2026-09-25)
- [x] `sendMessage`/`sendDM` check Discord's message limits (embed title, description,
      fields, footer, author, 6000 in total, blank field names/values, 2000-character
      content, empty messages): a warning, or with `-strict` Discord's 400 error from
      `sendMessage` and a silently dropped DM from `sendDM`; `cembed` itself doesn't check,
      as in production (2026-09-25)
- [x] Gematria tests run on the real bootstrap (`setup_templates`) and check the computed
      values (`embed_contains`) (2026-09-25)
- [x] File upload support in emulator (complexMessage with "file"/"filename")
- [x] `db dump` operation for exporting database entries
- [x] Direct array append syntax for `db add`
- [x] Array remove operation for `db remove`
- [x] Comprehensive test coverage for db operations
