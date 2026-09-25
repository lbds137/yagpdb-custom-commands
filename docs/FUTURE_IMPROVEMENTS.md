# Future Improvement Ideas

This document tracks potential enhancements for the YAGPDB custom commands project.

## Emulator Enhancements

### Remaining emulator gaps
- Values holding Discord objects (a member, a message, a `cembed`, a whole `dbGet`
  entry) serialize as the emulator's types, so their size differs from YAGPDB's. A value
  whose overflow past 100000 bytes is only whitespace is stored whole; YAGPDB stores it
  cut off, so reading it back fails.
- An immediate `execCC` runs inline, before the caller goes on; YAGPDB starts it in a
  goroutine, so it races with the rest of the caller (a `dbGet` right after an `execCC`
  that writes the key may read the old value in production). Scheduled runs are recorded,
  not run: test the scheduled command on its own with the recorded `exec_data` (a test's
  exec_data is a plain map, while the real run gets an `*sdict` with `.Get`/`.Set`).
- `execCC` and `scheduleUniqueCC` don't check that the command exists (YAGPDB errors
  "Couldn't find custom command" before its "Unknown channel"): an unmapped command is
  skipped, since `command_map` is only the commands a test runs.
- Channels have only an ID and a name: no types, so threads, voice channels and DMs
  aren't told apart (YAGPDB's name lookup skips some types, and getMessage/editMessage
  refuse DMs), and getChannel has no other fields. A test that declares no channels
  treats any channel ID as existing, with a `[channel]` warning per ID. Names are looked
  up in declared order, standing in for Discord's positions. deleteMessage,
  addMessageReactions, deleteAllMessageReactions, getTargetPermissionsIn and sendTemplate
  ignore their channel (YAGPDB's deleteAllMessageReactions prints "non-existing channel"
  for an unknown one, and sendTemplate errors "unknown channel").
- Discord functions are mocks: reaction calls only record, role changes don't update the
  members' roles within the run (as in YAGPDB, whose state updates later), `sendTemplate`
  is a no-op, and there are no components or threads yet.
- Pings follow the allowed mentions, but Discord also lets a role ping only when the role is
  mentionable or the bot may mention everyone, and @everyone/@here only with that
  permission; the emulator assumes the bot has it. A complexMessage `reply` isn't modelled,
  so the replied-to author's ping (NoEscape, or `replied_user: true`) isn't recorded.
- A failed execCC child's show_errors message isn't checked against Discord's
  2000-character limit. That it pings no one is read from the code, not probed.
  With `-strict`, a child over the source-length or time limit sends that error as the
  message; YAGPDB wouldn't save such a command, and has no time-limit error there.
  `deleteResponse`, `deleteMessage` and `deleteTrigger` record no deletions.
  `editMessageNoEscape` is `editMessage` (edits notify
  no one either way).
- `editMessage` gaps: a stored message keeps only its first embed and no file, so edits
  of multi-embed or file messages can differ; message builders read keys as a map, so a
  repeated key (two `"embed"`s) counts once. A test message is the bot's to edit only
  with `author_id: 1234567890`.
- A LIKE pattern is matched against the rows the query's other conditions select, so the
  trailing-escape error comes only from a row whose match reaches the escape. Postgres
  also runs LIKE while planning, on the key column's statistics (every server's keys),
  for a pattern that isn't an exact match: there the emulator warns (`[db]`) instead of
  guessing. Keys that aren't valid UTF-8 or hold a NUL byte are stored; Postgres
  rejects them.
- `parseArgs` resolves `user`, `member` and `role` arguments through the mocks (a test that
  declares no guild roles accepts any role). Its `role` argument uses the role functions' lookup;
  dcmd's RoleArg matches names case-sensitively and falls back from a numeric ID to a name.
- Role gaps: a test that declares no guild roles treats any role ID as existing, with a
  `[role]` warning per ID (a stale ID would be nil in production).
- Component and modal triggers aren't modelled (YAGPDB's `.Message` there is the
  interaction's message with the clicker as author), nor is a join message's `ctx.Msg` (a
  blank message from the joining member, which an execCC from it would inherit).

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
- [x] The role functions follow YAGPDB's, all three forms each (plain, ID, Name): its
      FindRole input rules per form, `mentionRole*` of an unknown role is "", and
      give/take/add/remove change nothing for an unknown role, a non-member, or a member
      who already has (or lacks) the role; a delay schedules the change (2026-09-25)
- [x] `snowflakeToTime`, `humanizeDuration*`, `humanizeTimeSinceDays`, `sanitizeText`
      (YAGPDB's confusables tables), `adjective`/`noun`/`verb` (its word lists) and
      `roleAbove` are copied from YAGPDB; test roles take a `position` (2026-09-25)
- [x] Output is what YAGPDB sends: trimmed, with the 2k notice under -strict, and a
      failed run keeps what it printed before the error (`yagtest run` prints it). A test
      that expects an error still checks its other assertions (2026-09-25)
- [x] Template output goes through YAGPDB's LimitWriter (now shared in `yagstd` with the
      database serializer): leading whitespace doesn't count, and output past 25k fails
      unless the rest is whitespace, with YAGPDB's error; outside -strict a shadow writer
      turns the same verdict into a warning (2026-09-25)
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
- [x] `execCC` with a delay and `scheduleUniqueCC` record a scheduled run instead of running
      it now, their data through YAGPDB's msgpack round trip ("ExecData is too big" over
      1000000 bytes for execCC); a unique key replaces, `cancelScheduledUniqueCC` removes;
      a third level of immediate execCC is YAGPDB's error; `scheduled_runs` assertion
      (2026-09-25)
- [x] Pings: `sendMessageNoEscape`(`RetID`) ported; sent messages and the response record
      who they notify, from YAGPDB's allowed mentions (users only by default; roles and
      @everyone via mentionRole*/mentionEveryone/mentionHere, a complexMessage's
      `allowed_mentions`, or NoEscape); `pings` and `response_pings` assertions (2026-09-25)
- [x] An execCC child's response (its trimmed output) is a sent message in its channel,
      with its pings; the over-2k notice names the child's number. A failed child sends
      YAGPDB's error message (formatCustomCommandRunErr copied: CC number, line, row, the
      source lines around it). Children's templates are named "CC #<n>", and errors carry
      YAGPDB's "Failed parsing/executing template" prefixes (2026-09-25)
- [x] Channel arguments follow YAGPDB's baseChannelArg for sendMessage, getMessage,
      editMessage, getChannel, execCC and scheduleUniqueCC: an int is an ID, a string an ID
      or a name (any case), anything else (a float) no channel, and the channel must exist
      (tests declare `guild.channels`; each function fails as YAGPDB does). parseArgs'
      channel argument is dcmd's. sendMessageRetID returns "" when nothing was sent (it
      used to return the previous message's ID after a refused send). Edits set
      EditedTimestamp (2026-09-25)
- [x] Message checks take `nth` (the nth message in the channel, or of all; 1 = first, the
      default), and an explicit `""` in content_equals or output_equals asserts emptiness
      (2026-09-25)
- [x] A command's header can set its error settings: "Show errors: `false`" (a failed run
      then sends its partial output as a normal response) and "Redirect errors: `<channel
      ID>`" (where its error message goes). A failed run, top-level or execCC, posts the
      show_errors message as a sent message and its response pings no one. Header lines
      are read from the leading comment only, keys in any case, and a value the emulator
      can't read fails the test (2026-09-25)
- [x] A header line "Case sensitive: `true`" makes the trigger case-sensitive, as the
      control panel's checkbox drops CheckMatch's (?i) (2026-09-25)
- [x] `.Message` follows what started the run: an interval or cron run has none, and no
      `.User`, `.Member` or `.BotUser` (its children neither); a reaction run's is the
      test's message with the reacted-to ID in the run's channel (its content and author);
      an execCC child gets its caller's message as YAGPDB keeps it (a reaction run's with
      the reactor as author, a blank one from the bot after an interval run). A None
      command, which only execCC runs, keeps a message (2026-09-25)
- [x] getRole* over the API-call limit fail with YAGPDB's "too many calls to this
      function" (2026-09-25)
- [x] `deleteResponse` with a delay under 1 sends no response (the output is empty, pings
      no one and gets a `[response]` note, for execCC children too), as YAGPDB's
      SendResponse skips it; a failed run's error message (show_errors on) still carries
      the output (2026-09-25)
- [x] DB patterns use a port of Postgres's MatchText (like_match.c), checked against
      Postgres 15 on 24,000 random cases: a trailing backslash is "LIKE pattern must not
      end with escape character" when matching reaches it, and a failed dbDelMultiple
      deletes nothing. Query errors carry sqlboiler's wrapping where YAGPDB selects through
      it (dbGetPattern*, dbTop/BottomEntries, dbDelMultiple); dbCount and dbRank return
      lib/pq's error as it is. NaN value_num sorts above every number (and equals NaN).
      setup_db, `yagtest run --db` and db_checks keys are cut to 256 bytes as dbSet and
      dbGet cut them (2026-09-25)
- [x] `.Guild.Roles` is in YAGPDB's order (its state tracker sorts roles as dstate.Roles:
      highest position first, the lower ID on a tie) instead of Go's random map order, and
      holds @everyone when a test declares no roles; a role name lookup takes the first
      match in that order, as YAGPDB's findRoleByName does (2026-09-25)
- [x] A role lookup that assumes an undeclared role exists warns (`[role]`), and the
      suites declare the roles they use; with no roles declared, the guild ID is
      @everyone (2026-09-25)
- [x] `.ExecData` is set as in YAGPDB: always in an immediate execCC child, even for nil
      data (so `.ExecData.Key` is a "nil pointer evaluating" error there), and elsewhere
      only when there is data (a test's `exec_data`, like a delayed run). Without it
      `.ExecData.Key` and `{{.ExecData}}` are `<no value>` and `index .ExecData "Key"`
      fails. An immediate child has `.StackDepth` (1 for the first level) (2026-09-25)
- [x] Gematria tests run on the real bootstrap (`setup_templates`) and check the computed
      values (`embed_contains`) (2026-09-25)
- [x] File upload support in emulator (complexMessage with "file"/"filename")
- [x] `db dump` operation for exporting database entries
- [x] Direct array append syntax for `db add`
- [x] Array remove operation for `db remove`
- [x] Comprehensive test coverage for db operations
