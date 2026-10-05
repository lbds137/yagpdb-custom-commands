# /edit modal — spec

Status: drafted 2026-10-01, overnight GLM 5.3 session. Superseded in one part
(2026-10-04): the `new:` category prefix below became `/edit entry`'s `create` option
(docs/FUTURE_IMPROVEMENTS.md has the shipped note); the rest stands. Decisions of record: Lila's
pick 3 of 2026-09-27 ("`/edit` slash opening a modal for rule_edit and
simple_db_edit, deletes behind a danger confirm button") and the Tzurot design
language of 2026-09-30. Staff-only → no Night House /commands/ change.

## Current behavior

- Text `/rule_edit` (`commands/rules/rule_edit.gohtml`, staff): sets/adds a rule's
  text; `(nil)` deletes. Posts a PUBLIC embed via embed_exec to the YAGPDB channel.
- Text `/simple_db_edit` (`commands/db/simple_db_edit.gohtml`, staff): sets/adds a
  category › key entry; `new:` creates the category; `(nil)` deletes. Same public
  embed_exec ack. Both delete by typing the magic string `(nil)` — no confirm.

Nothing execCCs INTO either command (checked: no `execCC` references to their ids;
`embed_exec` is their dependency, not the reverse), so both can retire cleanly when
Lila calls it. Until then the text commands stay live, untouched. (Retired 2026-10-01,
9c7fd87: both now live in `retired/`.)

## Design

Three NEW panel commands in a new `commands/edit/` folder (the planned slash root).
YAGPDB's modal flow needs them: a slash run opens the modal (its one interaction
response), Discord delivers the submission to a Modal Submission trigger, and the
delete confirm's buttons to a Message Component trigger. A CC has one trigger type.

### custom_id grammar — the contract between the three

| id | written by | means |
|---|---|---|
| `edit:rule:<n>:<stamp>` | /edit rule | rule `<n>`'s text form |
| `edit:entry:<stamp>` | /edit entry | the category/key/value form |
| `editdel:<uid>:<stamp>:go` | /edit delete | confirm button |
| `editdel:<uid>:<stamp>:no` | /edit delete | dismiss button |

- `<stamp>` = `currentTime.Unix` at spawn (unix seconds, digits only). Handlers
  refuse anything older than **1 hour** (`lt (sub (currentTime.Unix) $stamp) 3600`)
  — a backstop; Discord expires interaction UI long before. `snowflakeToTime`
  exists (vendor general.go:1399) but unix seconds is simpler and needs no decode.
- `<uid>` = the spawner's `.Member.User.ID`; the delete record lives on their row.
- Segments never contain `:` (design language). Ids stay ≤ ~55 chars; Discord caps
  custom ids at 100 INCLUDING the `templates-` prefix YAGPDB adds (10 chars,
  vendor components.go:1102-1117). The prefix is stripped before trigger matching,
  so trigger regexes are written against the bare ids (the `^ca:` pattern).
- **Delete record**: `/edit delete` writes `dbSet <uid> "Edit Pending" (sdict
  "kind" "rule"|"entry" "rule" <n> "category" <cat> "key" <key> "stamp" <stamp>)`,
  where `<stamp>` is the same value as the buttons' id stamp (computed once). One record
  per user, overwritten by the next delete, so the confirm acts only when the button's
  stamp matches the record's: an older confirm's `go` is refused ("a newer /edit delete
  replaced this confirm"), and a record without a stamp (written before this) never
  matches. `go` on a match deletes and `dbDel`s the record. `no` (Dismiss) always closes
  the message ("Dismissed.", even with nothing pending) and `dbDel`s the record only on a
  match, so dismissing a stale confirm leaves the newer pending one alone. Abandoned
  records linger on the spawner's row until overwritten.
  The rule number could ride inline in the id, but one record shape keeps the
  confirm handler a single code path.

### New command 1: `commands/edit/edit_slash.gohtml` — the spawner

Header rows:

```
Trigger type: `Slash Command`
Trigger: `edit`
Group: `Staff Utility`
Defer mode: `None`
```

Defer None is mandatory, not a choice: `sendModal` IS the interaction's one
response (vendor context_interactions.go:271-273 — the shared response counter); a
deferral would eat the slot and every sendModal would error.

Subcommands (root carries the verb; subcommands name the object — `entry` replaces
the pick's "simple_db_edit" wording; `delete` is the verb exception, per the
design language's delete vocabulary):

| subcommand | option rows |
|---|---|
| rule | `rule integer! the rule's number` |
| entry | `category string category to prefill` · `key string key to prefill` |
| delete | `rule integer rule number` · `category string category` · `key string key` |

Flow, every branch:

1. Staff gate FIRST: `hasRoleID $staffRoleID` (Roles dict "Staff", the db_slash
   idiom). Refusal: one ephemeral `sendResponse` — "⚠️ Staff only." A slash CC
   cannot be hidden from non-staff in Discord's picker; the gate is the tool.
2. Bare-run guard (`make test-templates` smoke): `{{if .IsSlashCommand}} …
   {{else}}` prints `Use /edit as a slash command: rule · entry · delete`.

**`/edit rule <n>`** — validate `1 ≤ n ≤ 9999` (ephemeral refusal otherwise: "⚠️ Rule
numbers run from 1 to 9999."; the rulebook is meant to hold small numbers, and the
readers once `seq`ed over them — they sort the existing numbers since 2026-10-04, so the
bound is hygiene, not their only guard; `/edit entry` and `/db set` can still write any
`Rule #N` key. edit_modal bounds the same, and `/edit delete` stays unbounded so an
out-of-range rule can be repaired away). Look up
`Rule #<n>` in the Rules dict (`or (dbGet 0 "Rules").Value sdict`). sendModal a
cmodal:

- `title` `"Edit rule <n>"`, `custom_id` `edit:rule:<n>:<stamp>`
- one field: label `"Rule <n> text"`, `custom_id "rule_text"`, style 2
  (paragraph), `required true`, and:
  - rule exists, old is a string ≤ 4000 runes → `value` = old
  - old is a string > 4000 runes → no value; placeholder "current text is too
    long for this box; type a replacement" (never prefill-truncate: a silent cut
    would store the truncation on save)
  - old is not a string (a dict/array stored under the key) → no value;
    placeholder "current value isn't plain text; type a replacement"
  - rule missing → placeholder "new rule; type its text"

**`/edit entry [section] [name]`** (the options were `category`/`key` until 2026-10-05; the
modal's field ids and the stored data are unchanged; its labels are now Section / Name / New
text) — both options optional. The modal is
self-describing: category and key are editable FIELDS, so it doubles as the
create path (type `new:Widgets` in the Category field) and no delete record or
state-in-id is needed beyond the stamp. sendModal:

- `title` `"Edit entry"`, `custom_id` `edit:entry:<stamp>`
- fields (all `required true`):
  - `category` — label "Category", style 1, value = the raw given category,
    placeholder "e.g. Admin — new:Name creates one"
  - `key` — label "Key", style 1, value = the raw given key, placeholder
    "the entry's name"
  - `value` — label "Value", style 2, value = old text when it resolves (below),
    placeholder "the new text"

  Prefill resolution (fail-soft, any miss → empty value field): copy
  simple_db_edit's category resolution verbatim — `new:` strip, `title`, dbGet,
  plain-map→sdict promotion, dict check — then title-case the key (Commands
  exempt, same rule) and read the entry; prefill only when the old value is a
  string. Fields echo the user's RAW input (transformation at store time shows
  up in the ack's "Stored as" field, the text command's behavior).

**`/edit delete`** — exactly one form: `rule`, or `section` AND `name`. Anything
else → one ephemeral usage error naming both forms. Then:

- rule: `n ≥ 1`; missing rule → ephemeral "⚠️ There is no rule #<n> to delete."
  (wording from rule_edit). Found → write the delete record, then the confirm.
- entry: copy simple_db_edit's validation preamble verbatim (`new:` strip,
  `title`, category must exist and hold a dictionary, key title-cased with
  Commands exempt). Unknown category / non-dict / missing key → the text
  command's exact refusal wordings. Found → record, then the confirm.

The confirm is ONE ephemeral response (the slash run's):

```
sendResponse nil (complexMessage "ephemeral" true
  "embed" $embed
  "components" (cslice $row))
```

- embed: title `"Delete rule #<n>?"` / `"Delete <Category> › <Key>?"`; description
  = the current value (string → as-is; non-string → the fenced-json form; both cut
  at 4096 — the JSON branch is rule_edit's truncation block verbatim; the string-side
  cut is new here, so a >4096-char value can't make Discord refuse the confirm embed).
- buttons: `(cbutton "label" "Delete rule <n>"|"Delete entry" "style" "danger"
  "custom_id" (print "editdel:" $uid ":" $stamp ":go"))` and `… "Dismiss"
  "style" "secondary" … ":no"`. Text on every button; danger = deleting an entity;
  dismissing spawned UI = secondary, no confirm (design language).

### New command 2: `commands/edit/edit_modal.gohtml` — the submission

```
Trigger type: `Modal Submission`
Trigger: `^edit:(rule:\d+|entry):\d+$`
Group: `Staff Utility`
Defer mode: `None`
```

1. Bare guard: `{{if .IsModal}} … {{else}}` prints one line ("Reached by
   submitting an /edit form; use /edit as a slash command.").
2. Gates: staff (`hasRoleID`); freshness (stamp = last `:`-segment, ≤ 1h).
   Refusals are one ephemeral sendResponse ("⚠️ Staff only." /
   "⚠️ This form expired — it's older than an hour. Run /edit again.").
3. Read fields by id: `.ModalValues.rule_text.value` /
   `.ModalValues.category.value` / `.key.value` / `.value.value`.
4. **`(nil)` is refused**: value or rule text exactly `(nil)` → ephemeral
   "⚠️ To delete, use `/edit delete`." The magic-string delete convention dies
   here; storing a literal "(nil)" rule would be a trap.
5. Engines are COPIED from the text commands, not rewritten — delete branches
   removed:
   - rule branch: rule_edit.gohtml:36-107 minus `$isDelete` — n re-validated
     (≥ 1, same wording), Rules fetch, found/old/oldIsString/isUnchanged (the
     non-short-circuiting `and` comment included), Set + dbSet.
   - entry branch: simple_db_edit.gohtml:35-174 minus `$isDelete` — `new:`
     strip, missing-name refusal, category existence + dict checks, key
     title-casing (Commands exempt), `$storedKeyDiffers` "Stored as" field,
     Set + dbSet.
6. Ack: ONE ephemeral sendResponse with the same embed shapes the text commands
   build — rule: title `Rule #<n> added|updated|unchanged`, description the new
   text, "Was" field on update (truncation blocks copied verbatim — string cut
   at 1024, non-string fenced-json cut at 1024-8, cut-then-fence); entry: title
   `<Category> › <Key> added|updated|unchanged`, "Was" and "Stored as" fields
   per the text command. No embed_exec, no public post (see Out of scope).

### New command 3: `commands/edit/edit_confirm.gohtml` — the buttons

```
Trigger type: `Message Component`
Trigger: `^editdel:\d+:\d+:(go|no)$`
Group: `Staff Utility`
Defer mode: `None`
```

Defer None like the pager's: under a defer, updateMessage is Discord's 40060.

1. Bare guard (`{{if .Interaction}} … {{else}}` one line).
2. Gates: staff; freshness ≤ 1h; then `dbGet <uid> "Edit Pending"` — segments:
   [1] uid, [2] stamp, [3] go/no. `go` with no record → ephemeral-style refusal
   ("⚠️ Nothing pending — already handled or expired. Run /edit again."); `go` whose
   stamp differs from the record's → "⚠️ A newer /edit delete replaced this confirm —
   use that one." Use sendResponse for refusals (a non-actor shouldn't rewrite the
   confirm).
3. `no` (Dismiss): `updateMessage "Dismissed."` always — replaces the confirm in
   place, removing the buttons — and `dbDel` the record only when its stamp matches.
   The stamp is whole seconds: two `/edit delete` runs by one staffer within the same
   second would share it. Accepted (2026-10-04 review): that takes one person
   submitting two slash commands inside one second. If it ever matters, a millisecond
   stamp (currentTime.UnixMilli, expiry divided by 1000) still fits the `\d+` trigger.
   A pre-stamp record's button reads "replaced" for at most the 1h expiry.
4. `go`: re-check the target (it may have changed since the confirm was spawned —
   `/db` and `/edit entry` still write the same dicts):
   - rule: Rules fetch; missing → ack "Rule #<n> not found" (already gone).
     Found → Del + dbSet, `dbDel` record, `updateMessage` the ack embed
     (title `Rule #<n> removed`, description the old value, truncation block
     verbatim from rule_edit's delete echo).
   - entry: category resolution copied; missing key → ack
     "`<Key>` not found". Found → Del + dbSet, `dbDel` record, ack
     (`<Category> › <Key> removed`, echo as the text command). A category that
     stopped being a dictionary between confirm and click gets the generic
     "not found" ack, not simple_db_edit's "doesn't hold a dictionary" wording —
     the target is unreachable either way (conscious divergence, review round).
   - updateMessage takes the complexMessage embed; the confirm was ephemeral and
     the update stays ephemeral.

### Budgets

db ops per path (Roles staff-gate read and the ack's Global color read included):
/edit rule 1 + submit 4 · /edit entry 2 + submit 4 · /edit delete 4 + click `go` 6
(click `no` 3) — all under the free tier's 10 per run, far under premium's 50. execCC: 0 anywhere. File sizes
will sit near the small text commands, nowhere near the panel limit (the
response-length lint rule guards it anyway).

## Tests — `tools/emulator/testdata/edit_tests.yaml`

One file, three CCs. Seeds mirror db_slash_tests.yaml's world (guild + Staff role
888888888, invoker 111…, second member 555…; staff tests use db_slash_tests'
staff-context syntax). **Every id-pinning test sets `clock: 2026-01-02T15:04:05Z`**
(unix 1767366245 — the emulator freezes `currentTime` and the db clock, so stamped
custom_ids and snapshots are exact). The emulator records a modal response's shape
only (kind/title/custom_id/field ids), so the prefill cases pin the modal's shape;
value/placeholder assertions are a filed emulator gap (FUTURE_IMPROVEMENTS). Stale ids use stamp 1750000000. The
`templates-` prefix is added/stripped by the engine; yaml asserts bare ids.

Spawner:
- rule: existing rule → `kind: modal`, custom_id `edit:rule:3:1767363845`, fields
  `["rule_text"]`, snapshot pins the prefilled value.
- rule: missing rule → snapshot pins the "new rule" placeholder.
- rule: >4000-rune rule (seed with `printf >>`, never Write/Edit) → no value,
  placeholder "too long" (snapshot).
- rule: non-string rule value (seed a dict under Rule #4) → "isn't plain text"
  placeholder (snapshot).
- entry: category+key given, entry exists → snapshot with prefilled value;
  unknown key → empty value field; no options → all three blank (snapshot).
- delete rule: seeded rule → ephemeral message, `components_contains` both
  custom_ids; snapshot pins style danger + label "Delete rule 3".
- delete entry: seeded entry → same, label "Delete entry"; unknown rule/entry →
  the refusal wordings, no buttons; no form / both forms → usage error.
- non-staff /edit rule → "Staff only." ephemeral.
- bare run → the usage line, no error, nothing sent.

Modal submission:
- rule: update (Was field, old→new in `db_checks`), add, unchanged (no change);
  `(nil)` text → refusal + db unchanged; stale stamp → expired; `edit:rule:0:…` →
  invalid-number wording; every ack `ephemeral: true`.
- entry: add, update (Was + Stored as when casing differs), unchanged;
  `new:Widgets` create; unknown category; non-dict category; Commands-exempt
  casing (seed the Commands dict); `(nil)` value → refusal; stale → expired.
- non-staff submit (context without the role) → "Staff only." (defense; Discord
  won't produce this today).
- wrong-shape id `edit:foo:123` → `no_trigger: true`.
- bare run → one line, no error.

Confirm buttons (seed a clicked message, pager-style `message_id: 7`):
- `go` rule → Rules minus the rule (`db_checks`), record gone, `edited_messages`
  ack on the confirm message, `kind: update`.
- `go` entry → category dict minus the key, record gone, ack.
- `go` on an already-deleted target (record seeded, target removed) → "not found"
  ack; `no` → "Dismissed.", record gone, db unchanged.
- no record → "Nothing pending"; stale → expired; non-staff → "Staff only."
- wrong id (`editdel:1:2:maybe`) → `no_trigger: true`; bare run → one line.

## §Panel — Lila's creation rows (three DISABLED commands, main server)

**1. edit_slash** — Trigger type **Slash Command**, name **edit**, slash
description `Edit rules and database entries`, Group **Staff Utility**, Defer
mode **None**, disabled.

| row | type | required | description |
|---|---|---|---|
| sub rule | — | — | edit a rule's text |
| · rule | integer | ✓ | the rule's number |
| sub entry | — | — | edit a database entry |
| · category | string | — | category to prefill |
| · key | string | — | key to prefill |
| sub delete | — | — | delete a rule or entry |
| · rule | integer | — | rule number |
| · category | string | — | category |
| · key | string | — | key |

**2. edit_modal** — Trigger type **Modal Submission**, Trigger
`^edit:(rule:\d+|entry):\d+$`, Group **Staff Utility**, Defer mode **None**,
disabled. (No subcommand/option rows.)

**3. edit_confirm** — Trigger type **Message Component**, Trigger
`^editdel:\d+:\d+:(go|no)$`, Group **Staff Utility**, Defer mode **None**,
disabled. (No rows.)

## §Rollout — order matters

1. Lila creates the three disabled panel commands per §Panel (morning step),
   sends the ids.
2. Us: panel.json entries (`main` ids only — lotv is dead) → `make config-sync` →
   full `make ci` green → push. **Commits before the ids exist are LOCAL ONLY**
   (deploy_panel_check fails on unmapped files by design — skip test-deploy until
   then, run test-go + test + test-templates + lint).
3. Deploy the three bodies (browser skill if Claude-in-Chrome is present, else
   the manual paste list); she ENABLES them. /edit is live alongside the text
   commands.
4. Transition (her call, no schedule): when she's confident, rule_edit and
   simple_db_edit move to retired/, she deletes their panel commands,
   config-sync, ci, deploy.

## Out of scope (recorded, not here)

- Public audit of edits: the text commands post public embeds; /edit acks
  ephemeral per the design language (settings/errors/sensitive). If she wants a
  public log, a followup unit adds one embed_exec call to the two write paths.
- Danger-confirm on `/db delete` — db-slash.md's out-of-scope already parks it;
  this unit's confirm handler is the pattern to copy.
- Role/user pickers, context menus, slash-first fleet pass (separate picks).
- Retiring the text commands (her call, §Rollout step 4).
- Night House /commands listing: staff-only command, no change.
