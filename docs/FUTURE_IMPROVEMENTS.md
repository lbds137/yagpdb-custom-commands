# Future Improvement Ideas

This document tracks potential enhancements for the YAGPDB custom commands project.

## Known command bugs

The snapshot audit's list (2026-09-25) is fixed (see Completed Improvements). Each fix
gets a failing test first.
- The Global "ExecCC Limit" setting (bootstrap default 10) is trusted as is: YAGPDB allows
  10 immediate execCC calls per run on premium (1 on a free server), so a setting above
  that makes rules, contrasts, hugemoji and pyramid fail at the 11th call instead of
  skipping.
  Harmless at the default; clamp it to 10 in those commands if the setting is ever
  raised (read, not run).
- Ruled out: gematria_bootstrap lists `Â`/`â` twice. The table is the Romanian letters
  (Ă Â Î Ș Ț) merged with the French ones (À Â Ç ...), which share Â; the repeated key has
  the same value (lines 59 and 63), so it is a no-op, and an edit would only cost a paste
  and a bootstrap rerun.

## Emulator Enhancements

### Remaining emulator gaps
- `exec`/`execAdmin` record the command line (`execs:`, snapshots) but don't run the bot
  command: the call returns "", where YAGPDB returns the command's response ("Unknown
  command" for a name it doesn't have, "Error: ..." when it fails), and execAdmin's
  "Failed fetching member" isn't modelled. The recording mock of embed_exec
  (testdata/templates) keeps the title, description, fields, color, image and thumbnail,
  but not embed_exec's author, its author-color fallback, its description cut or its
  DeleteResponse.
- The limit tables in docs/API_REFERENCE.md and .claude/skills/yagpdb-templates.md
  weren't written from the vendored source (the 10-second timeout was wrong); check each
  remaining row (embed description 2,048, "ExecCC concurrent calls typically 10-20", ...)
  against vendor/yagpdb and Discord's limits.
- directory (`exec "Clean" ...`) and ticket_adduser_exec (`exec "ticket adduser"`) have no
  test that checks their exec lines (screen_user's test maps ticket_adduser_exec to a
  recording mock).
- `yagtest watch` takes one path, and `-stop-on-fail` with several test paths stops only
  within the current one.
- Values holding Discord objects (a member, a message, a `cembed`, a whole `dbGet`
  entry) serialize as the emulator's types, so their size differs from YAGPDB's. A value
  whose overflow past 100000 bytes is only whitespace is stored whole; YAGPDB stores it
  cut off, so reading it back fails. Deferred (2026-09-25): none of the 26 dbSet calls in
  utility/ and staff_utility/ stores a Discord object, and the largest stored dict comes
  from a 7.4 KB source. Promote when a command stores a Discord object or a value nears
  100000 bytes; the fix is storing the msgpack bytes and decoding them with a copy of
  YAGPDB's newDecoder, with the emulator's Discord types shaped like discordgo's.
- An immediate `execCC` runs inline, before the caller goes on; YAGPDB starts it in a
  goroutine, so it races with the rest of the caller (a `dbGet` right after an `execCC`
  that writes the key may read the old value in production). Scheduled runs are recorded,
  not run: test the scheduled command on its own with the recorded `exec_data` (a test's
  exec_data is a plain map, while the real run gets an `*sdict` with `.Get`/`.Set`).
- `command_map` is only the commands a test runs, so an unmapped command may exist in
  production: execCC of one warns (`[execcc]`) and runs nothing, and a delayed run or
  scheduleUniqueCC of one is scheduled. YAGPDB's disabled-command and disabled-group
  errors aren't modelled.
- Channels have only an ID and a name: no types, so threads, voice channels and DMs
  aren't told apart (YAGPDB's name lookup skips some types, and getMessage/editMessage
  refuse DMs), and getChannel has no other fields. A test that declares no channels
  treats any channel ID as existing, with a `[channel]` warning per ID. Names are looked
  up in declared order, standing in for Discord's positions. getTargetPermissionsIn and
  sendTemplate ignore their channel (YAGPDB's sendTemplate errors "unknown channel").
- Reactions: an emoji is refused only when it isn't a string ("<int Value>"); an unknown
  or misspelled emoji, which Discord refuses (10014), is recorded. Reacting to a message
  the emulator doesn't know is Discord's 10008 refusal, though the message may exist in
  production. addReactions in an interval run reacts to the stand-in message (ID 0), which
  Discord refuses; that the error is 10008 is inferred, not probed.
- Without -strict, a function over its call limit warns "YAGPDB stops the command here"
  and runs on, even inside `{{try}}`, where YAGPDB's error would go to `{{catch}}` instead.
- A fixed clock (`clock:`) stands still for the whole run, where YAGPDB's moves on by
  milliseconds (sleep moves it on). The database's entry times follow the calling run's
  clock, so a sleep inside an execCC child doesn't move them, and a setup template's sleeps
  aren't carried into the test (its entries can look newer than the test's clock). A bad `clock:`
  value's error names neither the field nor the line (yaml.v3's time parse error), and a
  fractional `seed:` is truncated silently (1.5 is 1).
- `printf "%T"` of the emulator's Discord types prints their Go names (`types.CtxMessage`,
  `types.Timestamp`), not discordgo's (`*discordgo.Message`, `discordgo.Timestamp`); a
  command comparing those names would behave differently. None does today (the `%T`
  comparisons in db, db_get_text and db_get_embed are against `*templates.SDict` and
  `string` only).
- Discord functions are mocks: role changes don't update the
  members' roles within the run (as in YAGPDB, whose state updates later), `sendTemplate`
  is a no-op, and there are no components or threads yet.
- Pings: the bot's "Mention @everyone, @here, and All Roles" permission is one setting for
  the whole server (Discord checks it per channel), and it defaults to granted. A role the
  test doesn't declare is never mentionable. A reply to a message the emulator doesn't know
  (not the trigger, a test's `messages` or a sent one) is assumed to exist: it warns
  (`[message]`) and records no author ping. Discord refuses a reply to a message that
  doesn't exist by default (fail_if_not_exists; unverified here, YAGPDB doesn't set it).
  A `silent` message still counts its pings, though Discord sends no notification for it.
- A failed execCC child's show_errors message isn't checked against Discord's
  2000-character limit. That it pings no one is read from the code, not probed.
  With `-strict`, a child over the source-length limit sends that error as the message;
  YAGPDB wouldn't save such a command.
  Deletions are recorded, not made: a deleted message stays findable by getMessage for the
  rest of the run (YAGPDB deletes from a goroutine or a scheduled event, so usually after
  the run ends, but a short delay can land mid-run). `editMessageNoEscape` is
  `editMessage` (edits notify no one either way).
- getMessage of a test's or a sent message returns the stored message itself, so a later
  editMessage shows through it (`{{$m := getMessage nil $id}}{{editMessage nil $id "b"}}
  {{$m.Content}}` is "b"; YAGPDB's earlier fetch keeps the old content). An execCC child
  works on a copy of the messages, so its edits aren't seen by the caller's later getMessage.
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
- `parseArgs` resolves `user` and `member` arguments through the mocks.
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
      length; a time limit too, removed 2026-09-25 as unsourced), warnings by default and
      failures with `-strict` (2026-09-24)
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
- [x] `printf "%T"` gives YAGPDB's type names: the copied standard library keeps YAGPDB's
      package name, `templates`, so a stored dict is `*templates.SDict` (it was
      `*yagstd.SDict`, which made the db, db_get_embed and db_get_text dict paths dead in
      tests). A user prints as username#discriminator and its default avatar follows
      discordgo (discriminator % 5, or (id >> 22) % 6 on the new system); AvatarURL takes
      its size argument as there. Mock users have the new system's discriminator "0".
      Snapshots record attached files. command_tests and db_tests are snapshot tests
      (rand_hebrew and db dump since the fixed clock and seed) (2026-09-25)
- [x] execCC, scheduleUniqueCC and cancelScheduledUniqueCC take the command as an `int`,
      as in YAGPDB (a string or float variable is "wrong type for value"), and follow
      tmplRunCC's order: the command is looked up (an Interval or Crontab command refused)
      before the channel. An unmapped immediate execCC warns instead of doing nothing
      silently, and a command_map file that can't be read is an error. That turned up
      command_tests' four `mock_*` targets, which never existed (their children never
      ran): they now map to the real commands, and the suites' Commands dicts hold string
      IDs as the bootstrap stores them, plus the `contrast` and `db` entries. A failed
      execCC child now fails its test unless the test expects it with warning_contains
      (to YAGPDB's caller it's only a log line) (2026-09-25)
- [x] Reactions are recorded (YAGPDB's tmplAddReactions, tmplAddResponseReactions,
      tmplAddMessageReactions, tmplDelMessageReaction, tmplDelAllMessageReactions copied,
      with their argument checks, early returns and printed "non-existing channel/user",
      and their call counting per emoji as they go instead of all up front).
      addResponseReactions' reactions go on the response once it's sent. Tests assert
      them with `reactions:`, and snapshots record them (2026-09-25)
- [x] Deletions are recorded (YAGPDB's tmplDelTrigger/tmplDelMessage/tmplDelResponse):
      deleteTrigger deletes the run's message (the reacted-to one in a reaction run, the
      caller's in an execCC child; an interval run's stand-in, ID 0, deletes nothing), deleteMessage skips an
      unknown channel, delays default to 10s, cap at a day and run at once under 1, and
      deleteResponse deletes the response only when one is sent (an execCC child's by its
      message ID). Tests assert them with `deletions:`, and snapshots record them
      (2026-09-25)
- [x] getMessage and editMessage find the triggering message (an execCC child its caller's)
      as well as a test's and sent messages, as YAGPDB, which asks Discord, does; editing
      it is Discord's "authored by another user" refusal. A reaction run's reacted message
      and an interval run have only what the test declares (2026-09-25)
- [x] Pings follow what Discord lets the bot ping: a guild's `bot_mention_everyone: false`
      (the bot lacks "Mention @everyone, @here, and All Roles") stops @everyone/@here and
      every role that isn't `mentionable`, in sends, responses and execCC children. A
      complexMessage `reply` pings the replied-to author when the allowed mentions'
      replied_user is set (the NoEscape functions set it); `reply` of 0 or less is
      YAGPDB's error. The triggering message has an ID (it was 0) (2026-09-25)
- [x] parseArgs' role argument is YAGPDB's RoleArg (copied): a mention or ID matches a
      role's ID or, as text, its exact (case-sensitive) name, the first role in guild order
      winning; a mention's last character is cut whatever it is; a mention that isn't a
      number panics, as there, so no try catches it. With no roles declared an ID is an
      assumed role (2026-09-25)
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
- [x] Command bugs from the snapshot audit, each with a test that fails on the old
      command (2026-09-25):
      - `db add`/`remove` on a stored array work (`kindOf ... true` looks through the
        *templates.Slice), and a JSON array appended to one appends (also to an array
        inside stored JSON, a plain slice).
      - `db add`/`remove` report a missing key, a missing array item or dictionary key,
        and text given for a dictionary, instead of a false success. A nested dictionary
        target is changed itself, not its parent.
      - Nested keys reach dictionaries inside JSON stored by `db set` (plain maps, which
        the key walk turns into sdicts in their parent).
      - `db get`/`delete` find a stored `false`, `0` or `""`. A path through a missing
        key or a value that isn't a dictionary finds nothing, and `set` refuses it.
      - `db` cuts a long value to fit embed_exec's description with its code fence, so
        the fence survives.
      - `contrast #ffffff` gives embed color 16777215 (was doubled).
      - `contrasts` takes each word (split on anything but `#` and hex digits) that is a
        whole color or a role ID (the regexes are anchored), so a role ID is no longer
        read as 6-digit colors. An 8-digit `#rrggbbaa` is refused (it was read as its
        first 6 digits).
      - `message_pointer` answers a non-link with "Invalid Message Link" (was "index out
        of range").
      - Long text is cut between characters, not bytes, and only when it is over the
        limit: db, embed_exec, message_link, message_pointer, directory.
      - `role_ping` title-cases the role name without a `:` too, and an unknown role sends
        only the error (the message went out unpinged).
      - `admit_user` with no Welcome Message skips the welcome; it stopped there with
        "invalid value; expected string", before the admission record.
- [x] A snapshot entry no test has any more (a renamed or deleted test) is listed after
      the results: a warning from `make test`, a failure in `make ci` and CI; `make
      prune-snapshots` removes only those, `make update-snapshots` rewrites too
      (2026-09-25)
- [x] `sleep` is YAGPDB's tmplSleep without the wait: under 1 second or over 60 in all is
      "can sleep for max 60 seconds combined", and the run's clock moves on by the
      seconds slept (an execCC child starts its own 60 on its caller's clock). The strict
      "10-second" run limit is gone: the vendored YAGPDB (commit 0cf2ec5) has no time
      limit (no deadline in common/templates, lib/template or customcommands;
      Context.Execute's timing is commented out; commands.CommandExecTimeout wraps only
      built-in commands). A run is bounded by its operation count, sleep's 60 seconds and
      25k of output (2026-09-25)
- [x] exec and execAdmin record the command line as YAGPDB builds it (`execs:`,
      snapshots), so the kicks in guest, reject_user and inactivity are pinned; they
      were silent no-ops (2026-09-25)
- [x] db_get_text and db_get_embed walk nested keys as db does: a stored `false`, `0` or
      `""` is a value, a dictionary inside JSON stored by `db set` is reached, a key past
      a value that isn't a dictionary finds nothing, and a missing nested key keeps its
      title (it was blank) (2026-09-25)
- [x] A test's `clock:` fixes the run's clock (currentTime, humanizeTimeSinceDays,
      message timestamps, members' join times, database entry times and expiry) and
      `seed:` seeds randInt, shuffle, adjective, noun and verb, so db dump and rand_hebrew
      are snapshot tests. `.Message.Timestamp` and `.EditedTimestamp` are discordgo's
      Timestamp strings with `.Parse` (they were time.Time), and a test's `messages:`
      have the time their ID holds (2026-09-25)
- [x] The recording embed_exec mock keeps color, image and thumbnail; an `embed_title`
      check on a message without an embed fails (it passed) (2026-09-25)
- [x] Test coverage for db operations (it passed only because the tests never reached the
      global data; rewritten 2026-09-25)
