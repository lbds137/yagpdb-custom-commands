---
name: yagpdb-emulator
description: Running the YAGPDB template emulator (yagtest) - commands, test YAML shapes, assertions, snapshots, gotchas
---

# YAGPDB Template Emulator

## Quick Commands

Run from the repo root. Needs Go (mise pins 1.27 on the dev machine).

```bash
make test              # template tests in tools/emulator/testdata/, with db_schema.yaml
make test-go           # go vet + Go unit tests
make ci                # everything CI runs (.github/workflows/test.yml): test-go, test,
                       # test-templates, lint, test-deploy, minify-check, test-minified, gofmt
make watch             # rerun template tests on changes
make update-snapshots  # accept an intended change in snapshot output (also prunes)
make prune-snapshots   # only remove the entries of renamed/deleted tests (make ci and CI
                       # fail while any are left; a plain make test warns)

./bin/yagtest run -args "get,Global" -verbose retired/db.gohtml
./bin/yagtest run -message "#ff8800" commands/color/hex_to_int.gohtml # whole message (Regex triggers need it)
./bin/yagtest run -no-premium -strict retired/db.gohtml   # free-server limits, fail on breach
./bin/yagtest check commands/*/*.gohtml                   # parse + static warnings
./bin/yagtest test tools/emulator/testdata/db_tests.yaml tools/emulator/testdata/pings_tests.yaml  # several suites
make test-templates                # smoke-run every command with no args on a bootstrapped DB (in make ci)
./scripts/find-missing-functions.sh
```

Flags go before the file. `run` and `test` take `-strict` and `-schema <file>`.

## Test Case Format

```yaml
- name: "Test description"            # unique within the file (snapshots are keyed by name)
  template: "../../../commands/general/example.gohtml"   # or template_source: |
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
    # parent_id, position, topic, nsfw, bot_cannot_send (a send there fails as Discord's
    # 403 Missing Permissions would, nothing recorded); .Guild.Channels is sorted by position
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
    interaction: { type: component, custom_id: "pg:stale:2", message_id: 7 }  # a button click on
                                      # messages entry 7 (in the run's channel) for a `Message
                                      # Component` trigger; component: string_menu (or user_menu,
                                      # role_menu, mentionable_menu, channel_menu) with values: ["3"]
                                      # for a menu. See Button and menu clicks
    # interaction: { type: slash, subcommand: get, options: { key: "Global", who: 5 } }  # a slash
                                      # command (`Slash Command` trigger); options typed by the
                                      # header's Slash option lines. See Slash commands
    # interaction: { type: user_menu, target: 5 } / { type: message_menu, message_id: 7 }  # a
                                      # context menu entry used on user 5 / messages entry 7
                                      # (`User Context Menu` / `Message Context Menu` trigger)
    # interaction: { type: modal, custom_id: "edit:rule:3", message_id: 7,
    #                fields: { rule_text: "new text", reason: "typo" } }  # a modal submitted
                                      # (`Modal Submission` trigger), fields in the modal's
                                      # order; message_id optional; form: label for a
                                      # modal of clabels. See Modals
    # an interval/cron header: no .Message, .User or .Member; a None command keeps a message
    # a header line  Case sensitive: `true`  makes the trigger case-sensitive (default: not);
    # Show errors: `false` and Redirect errors: `<channel ID>` set how a failed run's error
    # is posted (default: a sent message in its own channel; with Show errors false, the
    # partial output as the response). Header keys are case-insensitive; a bad value fails
    # the test
  setup_db:
    - { user_id: 0, key: "Global", value: { Delete Trigger Delay: 5 } }
  setup_templates: ["../../../commands/gematria/gematria_bootstrap.gohtml"]  # run first, same DB
  command_map: { 1: "../../../commands/plumbing/embed_exec.gohtml" }  # execCC targets; a target's
                                      # output is a sent message in its channel, as in YAGPDB
                                      # (a failed one: YAGPDB's error message, unless its
                                      # header turns Show errors off). Map the real command
                                      # where you can; a missing file is an error, an
                                      # unmapped execCC warns [execcc], a failed child fails
                                      # the test (unless warning_contains expects it). IDs are ints: store
                                      # them as config_sync does (strings) and toInt them
  expected:
    output_contains: "..."            # also output_equals ("" = no output), output_matches, error_contains
                                      # (with error_contains, the other checks still run)
    warning_contains: "..."
    no_trigger: true                  # asserts message_content does NOT match the header trigger: the
                                      # test passes without running the template, or fails (naming the
                                      # trigger) if it does match. Rejected (a load-time error) combined
                                      # with any other expected: field or with assertions:, since the
                                      # template never runs
  assertions:
    db_checks: [{ user_id: 0, key: "K", value_equals: 1 }]   # or value_contains, not_exists
    sent_messages: [{ channel_id: 9, embed_title: "Title" }]  # or embed_contains: "`441`";
                                      # both match any of the message's embeds, not just the first
                                      # nth: 2 = the second message there (default: the first)
                                      # sent_after_seconds: 10 = exactly this many whole seconds
                                      # of sleep had elapsed in the run when it was sent
                                      # absent = unchecked; [] = assert the run sent no messages
                                      # at all (like execs/deletions/scheduled_runs/reactions)
                                      # components_contains: "templates-pg:2" = substring of any
                                      # of the message's action rows as JSON (see Buttons and menus)
                                      # ephemeral: true = an interaction response only the clicker sees
    edited_messages: [{ channel_id: 9, content_equals: "x" }] # edits (as edited); "" = empty
                                      # content; same absent/[] rule as sent_messages: absent =
                                      # unchecked, [] = assert the run edited no messages at all
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
    interaction_responses: [{ kind: update, embed_contains: "Page 2" }, { kind: followup, ephemeral: true }]
    # exactly the run's answers to its interaction, in order ([] for none): kind =
    # message|followup|deferred_edit|update|modal, ephemeral, and content_contains/embed_title/
    # embed_contains/components_contains as sent_messages has them (see Button and menu clicks);
    # a modal's title, custom_id (exact, no templates- prefix) and fields: [ids in order]
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
│   │                     # (YAGPDB call counters), components.go, interactions.go (clicks,
│   │                     # sendResponse/updateMessage), modals.go (cmodal, sendModal, the
│   │                     # Modal trigger), slash.go, trigger.go (header), loopcheck.go, hints.go
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

## Buttons and menus

`cbutton`/`cmenu` and complexMessage's `buttons` (up to 40), `menus` (up to 5) and
`components` (built ones, flat or as rows) are YAGPDB's own code
(`internal/runtime/components.go`, vendor common/templates/components.go): buttons pack 5
to a row up to 5 rows, a string menu takes a row of its own, every custom ID gets the
`templates-` prefix (numbered `templates-0`, `templates-1`... when empty; over 90 chars
after the prefix is `custom id too long`), and a link button loses its ID. Two components
on one custom ID build fine in YAGPDB (it never checks) but Discord refuses the message, so
every send, edit, `sendResponse` and `updateMessage` refuses it too: a `[limit]` warning
naming the ID (nothing sent), or with `strict`/inside `{{try}}` the error `HTTP 400 Bad
Request, {"message": "Invalid Form Body", "code": 50035}`. Sent and edited
messages carry them: assert with `components_contains: "templates-pg:2"` (a substring of
any action row as JSON: custom_id, label, placeholder...), read them back with
`(getMessage nil $id).Components` (rows of `.Components`, each with `.CustomID`, `.Label`,
`.URL`, `.Options`), and snapshots list them under `components:` in Discord's shape. A
`complexMessageEdit` without a components key keeps a message's rows; `"components"
cslice` clears them.

## Button and menu clicks

A click runs a command whose header says `Trigger type: \`Message Component\`` (or
`Component`), with `Trigger:` a regex matched against the custom ID after YAGPDB's
`templates-` prefix is stripped (`(?m)`, `(?i)` unless `Case sensitive: \`true\``;
vendor customcommands/handle_component.go). To test one, declare the clicked message
under `messages:` in the run's channel and click it:

```yaml
- name: "Next page"
  template: "../../../commands/channels/pager.gohtml"
  context:
    messages: [{ id: 7, channel_id: 9, author_id: 1234567890, embeds: [{ title: "Page 1" }] }]
    channel: { id: 9 }
    interaction: { type: component, custom_id: "pg:stale:2", message_id: 7 }
  assertions:
    edited_messages: [{ channel_id: 9, embed_contains: "Page 2", components_contains: "pg:stale:3" }]
    interaction_responses: [{ kind: update, embed_contains: "Page 2" }]
```

The run sees the handler's keys: `.Interaction` (`.ID`, `.Token`, `.RespondedTo`,
`.Deferred`, `.Message`, `.Member`), `.InteractionData` (`.CustomID` with the prefix,
`.ComponentType`, `.Values`), `.CustomID` (stripped), `.Cmd`/`.CmdArgs`/`.StrippedID`/
`.StrippedMsg` from the regex match (the ID after the match, split on spaces), `.IsButton`
or `.IsMenu` + `.MenuType` (`string`/`user`/`role`/`mentionable`/`channel`) + `.Values`
(`component: string_menu`, `values: [...]`), and `.Message` = the clicked message with
the clicker as `.Author`/`.Member`, like `.Interaction.Message`, the same message
(getMessage of it still shows the bot). `.User`/
`.Member` are the clicker. A non-matching custom ID is a `no_trigger` case, like a
message. `interaction:` can't combine with args, message_content, reaction or exec_data.

An interaction takes ONE response (vendor context_interactions.go): `updateMessage`
(edits the clicked message in place: content, embeds AND components are replaced, so a
plain string clears the buttons), `sendResponse nil msg` (a new message; `complexMessage
"ephemeral" true` makes it private), or the template's printed output (private with
`ephemeralResponse`). A second `updateMessage`/response is YAGPDB's `cannot respond to an
interaction > 1 time; consider using a followup`; a `sendResponse` after the response
is a followup instead (`sendResponseRetID` returns its ID). Without an interaction:
`updateMessage` errors `no interaction data in context; consider editMessage or
editResponse`, `sendResponse` `invalid interaction token`, `ephemeralResponse` is a
no-op. A header `Defer mode: \`Message Response\`` / `\`Ephemeral Message Response\`` /
`\`Update Message Response\`` (the panel's labels; default `None`) answers the click
before the run, and the printed output then edits that deferred response
(`deferred_edit`: a new message, ephemeral for the ephemeral mode, or the clicked
message's content under Update, keeping its embeds and buttons); `updateMessage` after a
deferral is Discord's 40060 (a warning, or an error with `-strict`/inside `{{try}}`).
Warnings tell you when a run never answers ("The application did not respond") or a
deferred run prints nothing. An execCC child inherits `.Interaction` (not the other
keys): its `sendResponse` takes the response and the caller's output becomes a
followup, always, where YAGPDB's concurrent child would race (docs/FUTURE_IMPROVEMENTS.md).
Responses land in `sent_messages` (`ephemeral: true` to assert it) or, for an update,
`edited_messages`; snapshots list `interaction_responses:` and mark ephemeral messages.
Not modelled yet: editResponse/getResponse/deleteInteractionResponse:
docs/design/emulator-interactions.md. Modals: see Modals.

## Slash commands

A slash command's header says `Trigger type: \`Slash Command\``, `Trigger:` its name
(lowercase, as the panel requires) and one line per panel row: `Slash subcommand:
\`get read a key\`` (name, description) and `Slash option: \`[sub.]name type[!]
description\`` with the panel's type keys (string, string_menu, integer, integer_menu,
number, number_menu, boolean, user, channel, role, mentionable), `!` = required and
`sub.` = the subcommand it belongs to (a command with subcommands has no top-level
options). The header is validated as the panel validates the form (vendor
customcommands/customcommands.go:610-741, its error texts): lowercase names of 1-32
letters/numbers/dashes/underscores, descriptions of 1-100 characters, no duplicates,
at most 25 options, and no `Update Message Response` defer mode. To test one:

```yaml
- name: "Get a key"
  template: "../../../retired/db.gohtml"
  context:
    guild: { roles: [{ id: 111, name: "Staff" }], channels: [{ id: 9, name: "log" }] }
    interaction: { type: slash, subcommand: get, options: { key: "Global", who: 5, where: 9, rank: 111 } }
  assertions:
    interaction_responses: [{ kind: message, ephemeral: true, embed_title: "Global" }]
```

Each option is typed by its header line: a string, a whole number (integer), a number,
true/false (boolean), or an ID for user (any user; `.Options.who` is the user as
getMember gives it, `*User` with `.ID`, `.Username`), channel (the run's channel or a
`guild.channels` entry, `*Channel`) and role (a `guild.roles` entry, `*Role`);
mentionable is a role when the ID is one, else a user. A missing required option, a
value of the wrong shape, an unknown option or a bad subcommand is an error before the
run (Discord never sends those). The run sees the handler's keys (vendor
customcommands/handle_slashcommand.go:109-238): `.IsSlashCommand`, `.CommandName`/
`.Cmd` (the name), `.SubCommand` (`""` without one), `.Options` (an sdict by option
name; an absent optional isn't in it), `.Args` (the name first) and `.CmdArgs` (the
values in header order, absent optionals skipped; YAML option names match in any case,
as the handler lowercases them), `.Interaction` (no `.Message`; `.DataCommand` is the
command data), `.InteractionData` (the same: `.Name`, `.CommandType` 1, `.Options`), `.Message` = a blank
message (ID 0) with the invoker as `.Author`/`.Member`, and `.User`/`.Member` = the
invoker. The printed output, `sendResponse`, `ephemeralResponse` and `Defer mode:`
work as for a click; `updateMessage` is refused (no message to update). An execCC child
inherits `.Interaction` and the blank `.Message`.

## Context menus

A context menu entry's header says `Trigger type: \`User Context Menu\`` or `\`Message
Context Menu\`` and `Trigger:` the entry's name (spaces and capitals allowed, 1-32
characters; matched trimmed, any case; no `Update Message Response` defer mode). Test a
user entry with `interaction: { type: user_menu, target: 5 }` and a message entry with
`interaction: { type: message_menu, message_id: 7 }` (a `messages:` entry in the run's
channel). The run sees (vendor customcommands/handle_contextmenu.go:121-159)
`.IsContextMenuCommand`, `.CommandName`/`.Cmd`, `.CommandType` (`user`/`message`),
`.Author` (the invoker's user), `.TargetUser` (the target user / the message's author)
and `.TargetMember` (their member, or nil if they aren't in `members:`), and for the
message entry `.Message` = the message used on. There is NO `.User` or `.Member` (the
handler passes no member), and for the user entry no `.Message`; `sendDM` sends nothing
and warns (YAGPDB returns `""` without a member, so an entry can't DM its target). An
execCC child gets the bot's blank message (user entry) or the message used on.

## Modals

A modal is two runs, so test it as two tests. First the run that opens it (a click or a
slash command): `sendModal` answers the interaction with the modal, recorded as
`{ kind: modal, title, custom_id, fields: [ids in order] }` (IDs without `templates-`):

```yaml
- name: "Edit opens the modal"
  template: "../../../retired/rule_edit.gohtml"
  context:
    messages: [{ id: 7, channel_id: 9, author_id: 1234567890, content: "Rule 3" }]
    channel: { id: 9 }
    interaction: { type: component, custom_id: "edit:3", message_id: 7 }
  assertions:
    interaction_responses: [{ kind: modal, title: "Edit rule 3", custom_id: "edit:rule:3", fields: ["rule_text"] }]
- name: "The submission saves it"
  template: "../../../commands/rules/rule_save.gohtml"   # illustrative: a Modal Submission command
  context:
    messages: [{ id: 7, channel_id: 9, author_id: 1234567890, content: "Rule 3" }]
    channel: { id: 9 }
    interaction: { type: modal, custom_id: "edit:rule:3", message_id: 7, fields: { rule_text: "new" } }
  assertions:
    interaction_responses: [{ kind: update, content_contains: "new" }]
```

Build the modal with `cmodal` (keys `title`, `custom_id` (default `templates--0`),
`fields`: an sdict or a slice of sdicts, each a text input, short unless `"style" 2`; a
blank field ID is numbered by the fields before it; a slice keeps only its FIRST 5,
silently) or `components` (a slice of `clabel`s; its error is discarded, so a bad entry
silently ends the list), or with `modalBuilder "id" "title" (clabel ...)` (at most 5
labels; `.Set`/`.AddComponents`). `ctextInput "custom_id" "x" "label" ...` is a text
input and `clabel "label" "L" "component" (ctextInput ...)` wraps it; a bare
`ctextInput` isn't top-level (`invalid top level component passed to modal builder`). A
label around a button or a string is YAGPDB's `unsupported component in label`; only a
component YAGPDB does allow in a label, a modal select menu (`cmenu`), is the emulator's
not-modelled error. A text input or label sent in a message (a cmodal's `.Data`) is
refused as Discord refuses it (a warning; an error with `-strict`). Errors are
YAGPDB's (vendor common/templates/context_interactions.go): `no interaction data in
context` without an interaction, `cannot send multiple modals to the same interaction`,
the one-response error after `sendResponse`/`updateMessage`, `invalid modal passed to
sendModal`. Discord refuses a modal after a deferral (`Defer mode:`, 40060) and in answer
to a modal submission (a warning; an error with `-strict`).

The submission runs a command whose header says `Trigger type: \`Modal Submission\`` (or
`Modal`), matched like a click. `fields:` is a mapping in the modal's order (the order
`.Values` has), each ID non-empty and distinct. Discord sends a `fields` modal back as
action rows (the default) and a modal of labels (`modalBuilder`, cmodal `components`) as
labels: give `form: label` for that one, so `.InteractionData.Components` has its shape
(`.Values`/`.ModalValues` are the same either way). The run sees (vendor customcommands/handle_component.go:337-437)
`.IsModal`, `.CustomID` (stripped), `.Cmd`/`.CmdArgs`/`.StrippedID`, `.Values` (the texts
in field order), `.ModalValues` (by field ID: `.type` 4, `.value`, `.custom_id`),
`.InteractionData` (`.CustomID` with the prefix, `.Components`), and `.Message` = the
`message_id` message (whose button opened the modal; `.Interaction.Message` is the same
object) or, without one, a blank message (ID 0; `.Interaction.Message` is nil), either
way with the submitter as `.Author`/`.Member`. The printed output, `sendResponse`,
`ephemeralResponse` and `updateMessage` (of the source message; refused without one)
work as for a click. `Defer mode: \`Update Message Response\`` on a modal no message
opened fails on Discord's side: the output and any followup are not delivered (warnings).

## When to Use This Skill

- Running local template tests
- Debugging emulator issues
- Adding new function implementations
- Creating test cases for templates
