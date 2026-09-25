# YAGPDB Template Emulator

## Quick Commands

Run from the repo root. Needs Go (mise pins 1.27 on the dev machine).

```bash
make test              # template tests in tools/emulator/testdata/, with db_schema.yaml
make test-go           # go vet + Go unit tests
make ci                # everything CI runs (.github/workflows/test.yml): test-go, test, lint, gofmt
make watch             # rerun template tests on changes
make update-snapshots  # accept an intended change in snapshot output (also prunes)
make prune-snapshots   # only remove the entries of renamed/deleted tests (make ci and CI
                       # fail while any are left; a plain make test warns)

./bin/yagtest run -args "get,Global" -verbose utility/db.gohtml
./bin/yagtest run -message "#ff8800" utility/hex_to_int.gohtml # whole message (Regex triggers need it)
./bin/yagtest run -no-premium -strict utility/db.gohtml   # free-server limits, fail on breach
./bin/yagtest check utility/*.gohtml                      # parse + static warnings
./bin/yagtest test tools/emulator/testdata/db_tests.yaml tools/emulator/testdata/pings_tests.yaml  # several suites
make test-templates                # smoke-run every command with no args on a bootstrapped DB (in make ci)
./scripts/find-missing-functions.sh
```

Flags go before the file. `run` and `test` take `-strict` and `-schema <file>`.

## Test Case Format

```yaml
- name: "Test description"            # unique within the file (snapshots are keyed by name)
  template: "../../../utility/example.gohtml"   # or template_source: |
  strict: true                        # optional: fail on YAGPDB limits
  snapshot: true                      # optional: compare with __snapshots__/<file>.snap.yaml
  context:
    args: ["arg1", "arg two"]         # after the trigger in the header: the message is `-example arg1 "arg two"`
    premium: false                    # optional, default true
    clock: 2026-01-02T03:04:05Z       # stops the clock there (currentTime, timestamps, db
                                      # entry times), for a snapshot of time-dependent output
    seed: 1                           # seeds randInt, shuffle, adjective, noun, verb
    user: { id: 1, roles: [111] }
    guild: { roles: [{ id: 111, name: "Staff", color: 3447003, position: 2 }] }  # if set, unknown roles are nil/errors
    # guild.bot_mention_everyone: false -> only roles with mentionable: true ping, never @everyone/@here
    # guild.channels: [{ id: 9, name: "staff-log" }]: if set, channel arguments accept only these
    # (and the test's channel), by ID or name; else any ID, with a [channel] warning.
    # A channel also takes type (0 text, 2 voice, 4 category, 5 announcement, 15 forum),
    # parent_id, position, topic, nsfw; .Guild.Channels is sorted by position
    # guild.owner_id: .Guild.OwnerID (default: the triggering user); guild.prefix (default "-")
    members: [1, 2]                   # if set, anyone else has left (getMember/userArg nil)
    member_roles: { 2: [111] }        # other members' roles (takeRoleID only takes a role they have)
    member_nicks: { 1: "Nick" }       # nicknames (.Member and getMember), the triggering user's too
    member_joined_ago: { 2: 12h }     # join time before the run (default 30 days; JoinedAt.Parse)
    messages: [{ id: 7, channel_id: 9, author_id: 2, content: "hi" }]  # getMessage finds these, sent messages and the trigger (not a reaction/interval run's)
                                      # (give channel_id: without it the message is in channel 0)
                                      # embeds: [{ title: "T", fields: [{ name: n, value: v }] }]
                                      # (cembed's keys; templates read .Title, .Author.Name)
    message_content: "text"           # or the whole message, trigger included (Regex triggers need it);
                                      # with exec_data/reaction it is only .Message (no arguments)
    reaction: { emoji: "🎮", message_id: 5, added: true }   # reaction-triggered run; its .Message is
                                      # the messages entry with that id in the run's channel
    # an interval/cron header: no .Message, .User or .Member; a None command keeps a message
    # a header line  Case sensitive: `true`  makes the trigger case-sensitive (default: not);
    # Show errors: `false` and Redirect errors: `<channel ID>` set how a failed run's error
    # is posted (default: a sent message in its own channel; with Show errors false, the
    # partial output as the response). Header keys are case-insensitive; a bad value fails
    # the test
  setup_db:
    - { user_id: 0, key: "Global", value: { Delete Trigger Delay: 5 } }
  setup_templates: ["../../../staff_utility/gematria_bootstrap.gohtml"]  # run first, same DB
  command_map: { 1: "../../../utility/embed_exec.gohtml" }  # execCC targets; a target's
                                      # output is a sent message in its channel, as in YAGPDB
                                      # (a failed one: YAGPDB's error message, unless its
                                      # header turns Show errors off). Map the real command
                                      # where you can; a missing file is an error, an
                                      # unmapped execCC warns [execcc], a failed child fails
                                      # the test (unless warning_contains expects it). IDs are ints: store
                                      # them as the bootstrap does (strings) and toInt them
  expected:
    output_contains: "..."            # also output_equals ("" = no output), output_matches, error_contains
                                      # (with error_contains, the other checks still run)
    warning_contains: "..."
  assertions:
    db_checks: [{ user_id: 0, key: "K", value_equals: 1 }]   # or value_contains, not_exists
    sent_messages: [{ channel_id: 9, embed_title: "Title" }]  # or embed_contains: "`441`";
                                      # nth: 2 = the second message there (default: the first)
    edited_messages: [{ channel_id: 9, content_equals: "x" }] # edits (as edited); "" = empty
    role_changes: [{ user_id: 1, role_id: 111, action: "add" }]  # delay: 90s for a scheduled one
    no_role_changes: true             # give/take of a role they have/lack, or a non-member, change nothing
    response_pings: { everyone: true, users: [1], roles: [111] }  # exactly who the output notifies
    # (sent/edited message checks take pings: too). Typed <@&id>/@everyone ping only through
    # mentionRole*/mentionEveryone, a complexMessage's allowed_mentions, or the NoEscape functions.
    # A complexMessage "reply" pings the replied-to author when replied_user is on (NoEscape
    # turns it on); the trigger's .Message.ID is 234567890; replying to a message that isn't
    # the trigger, a sent one or in messages: warns [message]
    reactions: [{ action: add, emoji: "👋", message_id: 7 }, { action: remove_all }]
    # exactly the reaction changes, in order: action = add|remove|remove_emoji|remove_all;
    # response: true = on the response (addResponseReactions); reacting to a message that isn't
    # declared, sent or the run's own is Discord's 10008 (an error with -strict or inside {{try}}, else a warning)
    execs: [{ line: 'kick 5 "spam"', admin: true }]
    # exactly the exec/execAdmin calls, in order ([] for none), with the command line as
    # YAGPDB builds it (strings quoted, switches and numbers not); recorded, not run: the
    # call returns the context's exec_responses entry for the line, or "" with a warning
    # if it isn't declared there. context: { exec_responses: { 'kick 5 "spam"': "Kicked" } }
    # declares what a line returns ("" is allowed, and silences the warning)
    deletions: [{ of: trigger, delay: 5s }, { of: message, channel_id: 9, message_id: 7 }]
    # exactly the deletions asked for, in order ([] for none): of = trigger|message|response;
    # unset fields match anything, delay: 0s = at once. Snapshots record deletions too
    scheduled_runs: [{ cc_id: 5, channel_id: 9, delay: 90s, key: "k", exec_data_contains: '"n":1' }]
    # exactly the runs execCC with a delay / scheduleUniqueCC left, in the order scheduled
    # (a replaced one moves last; [] for none). They aren't run: test that command separately
    # (its exec_data there is a plain map; the real run gets an *sdict)
```

## Project Structure

```
tools/emulator/
├── cmd/yagtest/          # CLI: main.go (run/test/check), watch.go
├── internal/
│   ├── yagtemplate/      # YAGPDB's text/template fork, copied (try/catch, while, return,
│   │                     # execTemplate, built-ins, op limit); one EMULATOR PATCH (OnMaxOps)
│   ├── yagstd/           # `package templates` (YAGPDB's name, so %T prints *templates.SDict),
│   │                     # imported as yagstd: standard functions, sdict/dict/cslice, regex, sort, copied
│   ├── runtime/          # engine.go (Discord/database mocks, Execute), context.go, limits.go
│   │                     # (YAGPDB call counters), loopcheck.go, hints.go
│   ├── funcs/            # database functions, parseArgs, conversion helpers for the mocks
│   ├── loader/           # YAML tests, runner, snapshots
│   ├── schema/           # db_schema.yaml checks
│   ├── state/            # mock database (raw value + value_num, Postgres LIKE)
│   └── types/            # context structs; SDict/Dict/Slice aliases of yagstd's
└── testdata/             # *.yaml suites, templates/ for execCC mocks, __snapshots__/
```

## Adding Missing Functions

Copy YAGPDB's code rather than rewriting it (fetch its source with `vendor/update-yagpdb.sh`):

1. A pure function (no Discord or bot state) from `common/templates/general.go` or
   `context_funcs.go`: copy it into `internal/yagstd/` and list it in `StandardFuncs()`
   (`funcmap.go`) or `Context.Funcs()` (`contextfuncs.go`).
2. A function that needs Discord: add a mock on the Engine in `internal/runtime/engine.go`,
   registered in `BuildFuncMap`, that follows YAGPDB's argument handling and return types
   (nil pointers for missing things, errors where YAGPDB errors).
3. If YAGPDB limits it (`IncreaseCheckCallCounter` in its source), add it to
   `limitedFuncs` in `limits.go`.
4. Update the IMPLEMENTED array in `scripts/find-missing-functions.sh`.
5. After updating vendor/yagpdb, rerun `scripts/gen-yagpdb-funcs.sh` (function list for
   hints), and see `internal/yagtemplate/README.md` to update the engine copy.

## Fidelity Notes

- Templates run on YAGPDB's own engine and standard functions (copied, not reimplemented),
  so try/catch, while, return, eq/index/len, and/or (which evaluate every argument), the
  1M/2.5M operation limit and the 10-regex cache limit behave as in production.
- Database values are copied in and out like YAGPDB's serialization: dbGet returns sdicts,
  dicts and cslices as pointers (*SDict ...); changing one doesn't persist without dbSet.
  A stored number reads back as a float64 (value_num), so `eq (dbGet 0 "n").Value 5` is an
  error; compare with `5.0` or use `toInt`.
- getMessage/getMember/getRole return nil pointers for missing things (field access then
  errors); dbGet and userArg return untyped nil (field access reads as no value).
- Discord functions are mocks. Output is not whitespace-trimmed like YAGPDB's response;
  assertions trim it.

## When to Use This Skill

- Running local template tests
- Debugging emulator issues
- Adding new function implementations
- Creating test cases for templates
