# /db slash — spec

Status: drafted 2026-09-30, GLM 5.3 session. Decisions of record: Lila 2026-09-27
(docs/FUTURE_IMPROVEMENTS.md "Interactive UX", pick 4): db as a slash command with
ephemeral replies, 7 subcommands. Vocabulary and interaction design per the Tzurot
consult of 2026-09-30 (design language of record). Staff-only → no Night House
/commands/ change.

## Current behavior

- Text `/db` (`commands/db/db.gohtml`, main 15, lotv 3 dead): Command trigger, Group
  `Utility`, 7 operations via fake-slash args — `keys`, `get`, `set`, `delete`, `add`,
  `remove`, `dump`. Staff target rows via the colon hack (`db get:123 Key`), non-staff
  are pinned to their own row. Every result posts PUBLICLY via embed_exec
  (title `Database Operation: \`op\``, code-fenced value, User ID / Key / Result fields).
- **`inactivity` execCCs INTO db** (commands/members/inactivity.gohtml:78, `date`
  action): ExecData sdict `ChannelID/UserID 0/Operation "add"/Key "Inactivity Prune"/
  Value (sdict …)/Title`, and db answers through the embed_exec path. Text /db cannot
  be retired until inactivity repoints (a slash-trigger CC may be execCC'd; only
  Role-trigger CCs are refused — vendor tmplextensions.go:202-210).

## Design

### New command: `commands/db/db_slash.gohtml`

A NEW panel command (a CC has one trigger type; deploy.js cannot change it). Lila
creates it DISABLED with the panel rows in §Panel; we deploy the body; she enables.
Text `/db` stays live untouched during the transition (her call when it retires).

Header (the emulator parses these rows; they document the panel config):

```
Trigger type: `Slash Command`
Trigger: `db`            (lowercase, enforced by the panel's name regex)
Group: `Utility`
Defer mode: `None`
Slash subcommand: `view <description>`      … 7 lines, then option rows
Slash option: `view.key string! <description>`
```

Subcommands (old → new names; Tzurot vocabulary): `get`→**view**, `keys`→**browse**,
`set`→**set**, `add`→**add**, `remove`→**remove**, `delete`→**delete**, `dump`→**export**.
`set/add/remove/delete` keep their names. The colon-hack dies: `user` is a typed option.

| subcommand | option rows |
|---|---|
| view | `key string!` · `user user` |
| browse | `key string!` · `user user` |
| set | `key string!` · `value string!` · `user user` |
| add | `key string!` · `value string!` · `user user` |
| remove | `key string!` · `value string!` · `user user` |
| delete | `key string!` · `user user` |
| export | `key string` (optional, empty = all) · `user user` |

Option names: short lowercase nouns (Tzurot). Descriptions ≤100 chars, e.g.
key = "database key, nest with :" — final wording below in §Panel (identical rows for
the panel form and the header).

### Inputs (vendor handle_slashcommand.go:106-170, emulator runtime/slash.go)

- `$operation := .SubCommand` (Discord delivers the chosen subcommand; the picker makes
  unknown subcommands impossible).
- `.Options.key` / `.Options.value` — strings; `.Options.user` — a resolved user
  object, absent when not provided. Absent optional options are unset in `.Options`.
- `.Member.User.ID` is the invoker (vendor builds a minimal member-carrying message,
  :153-166), so `hasRoleID` works: staff = `Roles` dict "Staff", as today.
- Target row, ONE rule on every subcommand (revised 2026-10-01 after Lila's catch —
  the first build defaulted staff to their own row, which a user picker can't steer
  to row 0, stranding every global dict): **staff without `user` → row 0 (global,
  the text command's `:0`); staff with `user` (self included) → that member's row;
  non-staff → own row, a `user` option silently ignored.** Export needs no special
  case under this rule (its old one collapsed into it).
- Value parsing ports unchanged: a string starting `{` goes through `jsonToSdict`;
  nested keys split on `:`; the whole nested-key engine (split/sdict walk, parent
  rewrite, plain-map→sdict promotion) is copied from db.gohtml, not rewritten.

### Response — one ephemeral `sendResponse`, no embed_exec

The slash path builds the same display inline (cembed, Global "Embed Color", title
`Database Operation: \`op\``, code-fenced value cut at Discord's 4,096-char description
cap by code points, fields User ID / Key / Result, ✅/⚠️ prefix) and sends:

```
{{sendResponse nil (complexMessage "ephemeral" true "embed" $embed)}}
```

- Ephemerality comes from the **`"ephemeral" true` key in complexMessage**
  (vendor general.go:374-378) — NOT `ephemeralResponse`, which only flags plain output.
  This is the ca:csv pattern, proven in prod with files (channel_activity_pager).
- export replies with the file instead of an embed (today's dump shape):
  `complexMessage "ephemeral" true "content" <"Exported N entries to file"> "file" $json
  "filename" db_dump_<uid>_<unix>` — sendResponse carries files
  (context_interactions.go:350-363). The summary line moves from an embed field to
  `content` beside the file, like ca:csv.
- `sendResponse` is the interaction's ONE response (vendor caps it at 1); never call it
  twice, never print after it.
- `deleteTrigger` is dropped on the slash path (no trigger message). The ExecData path
  keeps db.gohtml's exact behavior, `deleteTrigger` included (it runs there today).

### Defer mode: None — decision + contingency

All seven ops are single-run DB work (worst case export: 4 setup `dbGet`s + 1
`dbGetPattern` + marshal), well under Discord's 3-second window; ca:csv (a heavier
build) has lived with defer None since 2026-09-27. Defer would cost: the deferred edit
carries no files (context.go:665-671), so export would need a followup and leave the
deferral dangling — two patterns in one command. **Contingency:** if prod ever shows
"application did not respond" under queue load, the fix is documented here — set panel
Defer to `Ephemeral Message Response`, switch embed ops' `sendResponse` → `sendMessage
nil …` (edits the deferral, keeps `"ephemeral" true`), and export becomes a followup —
a small follow-up unit, decided on evidence, not now.

### Bare run (make test-templates smoke)

No interaction and no ExecData → print one line, e.g. `Use /db as a slash command:
view · browse · set · add · remove · delete · export`, and stop. (Bare `yagtest run`
of a Slash-trigger template executes with no interaction — verified 2026-09-30 — the
template must not error.)

### Free-tier shape

Slash path: 0 execCC, ≤7 DB interactions (4 setup reads + op's read/write; export = 5).
ExecData path unchanged (1 execCC). **Size vs the panel: YAGPDB counts every newline
twice** (the form arrives CRLF server-side; the panel counter and validator measure
runes + newline-count, customcommands-editcmd.html `updateCCLength` + web/validation.go
`ValidateTemplateField`) — the effective limit for an N-line file is 20,000 − N, which
refused the first build at 19,881 runes + 449 lines. The trimmed file is 18,484 runes +
431 lines = panel count 18,915 (headroom ~1,085; the linter's `response-length` rule now
guards every command file against this). The engine comments cut here live on in
db.gohtml. The minifier unit (backlog item 5) remains the fix for real char pressure.

### inactivity repoint (same unit, second commit step)

`commands/members/inactivity.gohtml`: `Get "db"` → `Get "db_slash"` (config_sync writes
that key from the panel.json basename; gen-config-sync.py keys by filename). Its tests
keep passing with db_slash mapped in the test `command_map`.

## Tests — `tools/emulator/testdata/db_slash_tests.yaml`

Shape: `interaction: {type: slash, subcommand: <op>, options: {…}}` (slash_tests.yaml
is the reference; defaults.user.id must NOT be 1234567890). Seed like db_tests.yaml
(Global/Commands/Channels/Roles) plus Global "Embed Color" and an invoker row and a
second member's row. Cover:

- view: top key, nested key, missing key (⚠️ "No value found!"), non-staff passing
  `user` → own row (silently).
- browse: dict keys listed; non-dict → "No dictionary found for the given key!".
- set: plain string; JSON object value; nested key.
- add / remove: dict merge, dict key removal, array append, array item removal,
  missing target → "No value found!".
- delete: top-level (value echoed in the reply), nested.
- export: staff default → row 0 seeded global (file pinned by SNAPSHOT — yaml has no
  files assertion yet); with `key`; non-staff → own row.
- staff `user` option targeting another member's row (view or set).
- Every reply asserted `ephemeral: true` in interaction_responses.
- ExecData path: inactivity-shaped exec (command_map db_slash→its own file? no — run
  db_slash with an exec context and embed_exec stubbed in command_map, as db_tests.yaml
  does) asserting the public embed_exec call still happens with the given Title.
- Quoting rules for embed needles crossing newlines per the role file's gotchas.

## §Panel — Lila's creation rows (DISABLED command)

Trigger type **Slash Command**, name **db**, slash description
`Database dictionary tools`, Group **Utility**, Defer mode **None**, disabled.
Subcommands and options (same text as the header lines):

| row | type | required | description |
|---|---|---|---|
| sub view | — | — | show one value |
| · key | string | ✓ | database key, nest with : |
| · user | user | — | another member (staff) |
| sub browse | — | — | list a dictionary's keys |
| · key | string | ✓ | dictionary key, nest with : |
| · user | user | — | another member (staff) |
| sub set | — | — | set a value |
| · key | string | ✓ | database key, nest with : |
| · value | string | ✓ | value, JSON for dicts |
| · user | user | — | another member (staff) |
| sub add | — | — | add to a dict or array |
| · key | string | ✓ | database key, nest with : |
| · value | string | ✓ | value, JSON for dicts |
| · user | user | — | another member (staff) |
| sub remove | — | — | remove from a dict or array |
| · key | string | ✓ | database key, nest with : |
| · value | string | ✓ | value, JSON for dicts |
| · user | user | — | another member (staff) |
| sub delete | — | — | delete a key |
| · key | string | ✓ | database key, nest with : |
| · user | user | — | another member (staff) |
| sub export | — | — | export entries as a JSON file |
| · key | string | — | empty = all entries |
| · user | user | — | another member (staff) |

(7 subcommand rows + 17 option rows = 24 form rows — one-time, at the panel.)

## §Rollout — order matters

1. Lila creates the disabled panel command with §Panel's rows, sends the id.
2. Us: panel.json entry `commands/db/db_slash.gohtml: {main: <id>}` (make ci's
   deploy_panel_check FAILS without it — that's why the body can't merge first);
   `make config-sync`; inactivity repoint; `make ci` green; push. Commits before step
   1's id are LOCAL ONLY.
3. Deploy the body (browser skill if Claude-in-Chrome is present, else the klipper
   manual path); she ENABLES the command. Slash /db is live alongside text /db.
4. Deploy config_sync, then inactivity — in that order, and paste inactivity only after
   config_sync's next hourly run has written `db_slash` into the live Commands dict
   (until then the old text db keeps serving `inactivity date`).
5. Transition (her call, no schedule): when she's confident in the slash /db, we move
   db.gohtml to retired/, she deletes panel commands 15 (main) and 3 (lotv, dead),
   config-sync regen, ci, push, deploy. inactivity no longer depends on it.

## Out of scope (recorded, not here)

- Danger-confirm on `/db delete` — entity-delete confirmation belongs to pick 3's
  modal work (rule_edit / simple_db_edit), applied fleet-wide afterwards.
- deploy.js auto-checking option/defer rows vs the header (panel fields are form POSTs
  deploy.js doesn't touch; the emulator pins the header in tests, the one-time form
  fill is Lila's §Panel table).
- `files_contains` yaml assertion (already filed in FUTURE_IMPROVEMENTS; snapshots pin
  export's file meanwhile).
