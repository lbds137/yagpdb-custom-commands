# Future Improvement Ideas

This document tracks potential enhancements for the YAGPDB custom commands project.

## Command file layout: topic folders (DECIDED, Lila 2026-09-27; SHIPPED the same day)

Reverses the earlier "folders = panel groups" layout (new information: slash roots will
be topics, and several topics span both groups). One top-level `commands/` tree with a
folder per topic, groups mixed: bump/ (bump_check, bump_remind, bump_reset), rules/
(rule, rules, rule_edit), gematria/ (gematria, gematria_bootstrap), hebrew/ (alefbet,
atbash, pyramid, rand_hebrew), color/, db/ (db, db_get_embed, db_get_text,
simple_db_edit, simple_db_lookup), members/ (hiatus, unhiatus, inactivity, staff_roles,
role_ping…), channels/ (channel_activity + pager, channel_tracker, directory,
channel_link, message_pointer), plumbing/ (embed_exec, message_link, log_user,
ticket_clean, bootstrap)… — final placement of the rest decided in the unit's spec, with
folders matching the planned slash roots. The panel group moves into each header as a
`Group:` line, checked by CI and reported as drift by the deploy tool like the trigger.
Everything that names paths moves with it: panel.json, test YAML template:/command_map
paths, snapshots' keys if path-based, Makefile/scripts (COMMAND_DIRS), deploy manifest,
README, CLAUDE.md, skills, docs. Tell the Night House site session when it lands.

## Interactive UX (Lila 2026-09-27)

Order: (1) UX audit, (2) emulator components, (3) channel_activity buttons + approved audit
picks, then the minifier.
- channel_activity (shipped 2026-09-27 as main 86, with channel_tracker main 85, a tracker
  because templates can't read a channel's last message) is too long to scroll. DECIDED:
  a browse view. Summary counts (active <7d / quiet 7-30d / stale 30d+ / never seen),
  bucket buttons, 15 per page with prev/next, edited in place by a new Component-trigger
  handler command (created DISABLED until deployed). State lives in the button's custom
  ID (e.g. `ca:stale:2`); the handler rebuilds the page from it. Patterns from Tzurot's
  .claude/rules/04-discord.md: browse, not dump; summary first; ephemeral for staff tools
  where the trigger allows it; acknowledge within 3 s; last-page "next" disabled.
- channel_activity polish (found live 2026-09-27, first run: 217 never seen, 0 tracked
  eligible — the tracker had only seen Lila's own command in the excluded #yagpdb; a
  deleted trigger message still counts): (1) open on the first non-empty bucket instead
  of an empty Stale page; (2) show "tracking since <date>" (earliest Last Active
  created_at, or a date stored by channel_tracker's first run) so "never seen" reads as
  "not yet". Pager is at 9,913 of 10,000 chars: needs the minifier or a trim first.
- Add to channel_activity's browse view (Lila 2026-09-27, relayed: "a download button to
  save channel stats"): a "Download" button whose handler replies with a CSV file
  (complexMessage "file"/"filename", text/plain, max 100,000 chars, vendor general.go:
  314-325): channel id, name, category, last message (ISO date), days quiet, bucket. Up
  to 200 tracked channels (2 dbBottomEntries), about 12 KB. Ephemeral if the click
  allows (it's an interaction). Format unconfirmed with Lila: propose CSV when building.
  Shipped 2026-09-27 (channel_activity_pager, main 87): `ca:csv` replies ephemerally with
  `channel_activity.txt` (YAGPDB forces .txt), rows dropped past the 100,000-byte cap.
- Deferred (found 2026-09-27, building the CSV download): the emulator's YAML tests can't
  assert an uploaded file's content (`file`/`filename` in complexMessage): only a snapshot
  pins it (`files:` in the .snap.yaml). Add a `files_contains:` (or `files:` list with
  `filename`/`content_contains`) check to sent_messages / interaction_responses, mirroring
  `components_contains`. Trigger: the next command that uploads a file, or a CSV format
  change that a snapshot diff would hide in noise.
- UX audit done 2026-09-27 (all 39 commands, vendor c579722; most are fine as is). Lila
  approved ALL of these, build order: (1) channel_activity browse; (2) rules browse, same
  pager, jump-to-rule select, public rulebook posting kept; (3) `/edit` slash opening a
  modal for rule_edit and simple_db_edit, deletes behind a danger confirm button; (4) db
  as a slash command with ephemeral replies via sendResponse (bypass embed_exec, which
  always posts publicly with sendMessage), 7 subcommands — IMPLEMENTED 2026-09-30 as
  commands/db/db_slash.gohtml (spec docs/design/db-slash.md; awaiting the panel command's
  creation + deploy, see its §Rollout); (5) user/message context-menu
  entries for avatar_viewer and hugemoji (text triggers kept) — IMPLEMENTED 2026-10-01 as
  commands/general/avatar_menu.gohtml ("View Avatar", user menu, panel 94) +
  emoji_menu.gohtml ("Expand Emoji", message menu, panel 95), deploy pending; adds 1 to
  each per-type cap (5 free / 15 premium, customcommands.go:963-964); (6) slash with
  role/user pickers for staff_roles, inactivity (kick behind a confirm), role_ping.
  Shared blocks: one pager handler (Component trigger `^pg:(\w+):(\w+):(\d+)$`, page logic
  inside it; the slash command execCCs it for page 1, inheriting the Interaction,
  tmplextensions.go:240-242), one confirm handler (`confirm:<action>:<id>`). Slot budget:
  6 of 10 free slash CCs (50 premium), 2 of 5 context-menu CCs (customcommands.go:950-964).
  Smaller wins not picked (fine as is for now): contrasts as one embed (also frees the
  free tier's 1 execCC), rand_hebrew / simple_db_lookup / bootstrap as slash.
- /edit entry's `new:` prefix is UI cruft (Lila 2026-10-01, first live use: "works but
  messy"). Inherited verbatim from simple_db_edit, where a text command needed an
  in-band create signal; its real job is typo protection (a stray `Admib` must not
  silently become a category), so the fix moves that protection to a typed control
  rather than dropping it. Leading candidate: a `create` true/false slash option on
  `/edit entry` — the modal keeps its free-text Category field (prefill needs it),
  and the handler refuses unknown categories unless `create` is set. `/edit delete`'s
  unknown-category refusal stays as is (delete never creates). A dynamic category
  picker would need /setup-style machinery (YAGPDB's string_menu choices are static,
  set in the panel form). Pick up with the next /edit polish round or with the
  pickers (roadmap item 4).
- Staff-facing tools are still techy (Lila 2026-10-01: "my regular staff might have a
  hard time understanding how to use them" — pre-existing; the modal UX is "a ton
  better" but the vocabulary stayed). The techiness is storage shape leaking into
  tasks: users type Category/Key/value and colon-nesting where they think in jobs
  ("change the greeting", "edit rule 3"). Candidate pickup: a comprehension audit
  with a staff lens — first question, what IS the regular-staff surface (/edit and
  /db may be admin-only in practice, which shrinks the problem); then per command:
  what job is a moderator doing, and where does storage vocabulary (categories,
  keys, `Rule #N`, title-casing) surface in the UI. Fixes likely overlap the pickers
  / /setup (roadmap item 4) and the new: cleanup; the cheap teaching layer (option
  descriptions, modal placeholders) already shipped with the slash work.


- Deferred, trigger: the six picks above shipped. A slash-first pass over the rest (Lila
  2026-09-27: slash commands are "a game changer"). The main server's text prefix is
  already `/` (members type `/gematria` as text), so a real slash CC of the same name
  replaces the fake one without changing what people type. No clash (Lila 2026-09-27, from
  use): Discord's client turns a typed "/" into the slash picker; only a leading space
  sends "/name" as plain text. So convert in place; no need to keep both (a CC has ONE
  trigger type, so both would be two panel commands). Also pick a free-tier top 10 (free servers get 10 slash CCs, premium
  50; live panel lists Slash Command / User and Message Context Menu types, checked
  2026-09-27).
  Group related commands into one slash command with subcommands where they belong
  together (Lila 2026-09-27), e.g. `/hebrew atbash|alefbet|pyramid|random` with
  `/gematria` on its own or under a broader root (gematria is multi-script: digits,
  Hebrew, Phoenician, Greek incl. polytonic, Latin, Arabic abjad —
  staff/gematria_bootstrap.gohtml; so not under /hebrew),
  `/color contrast|contrasts|random|hex`, staff tools under a few roots. Constraints:
  YAGPDB supports ONE level of subcommands (handle_slashcommand.go:125-140; no
  subcommand groups), up to 10 per command free / 25 premium (customcommands.go:
  958-963); a slash CC is one template, so a root either holds all its subcommands'
  logic (10k/20k-char cap) or dispatches each subcommand to the existing command by
  execCC (one execCC per run on free: fine, a run is one subcommand). Grouping also
  saves slash slots (10 free), so it helps the free-tier top 10.
- Prerequisite (met by unit 4, 2026-09-27): the emulator models interactions: Slash
  (.Options, .CmdArgs, subcommands), Component and Modal triggers (.CustomID, .Values,
  .IsButton/.IsMenu), context-menu (.TargetUser/.TargetMember/.Message), sendResponse/
  updateMessage/sendModal/ephemeralResponse, the 1-per-run interaction response. Design and unit order:
  docs/design/emulator-interactions.md (f). Unit 1 (component builders) shipped
  2026-09-27: cbutton/cmenu, complexMessage(+Edit) buttons/menus/components as action
  rows with YAGPDB's custom-ID prefixing and every vendor error text, components on sent,
  edited and read-back messages, `components_contains` and snapshot `components:`
  (tools/emulator/internal/runtime/components.go, types/components.go). Unit 2 (the
  interaction core + Component trigger) shipped 2026-09-27: `Trigger type: \`Message
  Component\`` matched as YAGPDB matches it (a regex on the prefix-stripped custom ID),
  `Defer mode:` header, the handler's data keys (.Interaction, .InteractionData,
  .CustomID, .Cmd/.CmdArgs/.StrippedID, .IsButton/.IsMenu/.MenuType/.Values, .Message =
  the clicked message by the clicker), sendResponse*/updateMessage*/ephemeralResponse
  with the vendor errors and the one-response rule, the output routed as response /
  followup / deferred edit, `context.interaction: { type: component }` and the
  `interaction_responses:` assertion (tools/emulator/internal/runtime/interactions.go;
  the skill has a click recipe). Unit 3 (slash commands + context menus) shipped
  2026-09-27: `Trigger type: \`Slash Command\`` with `Slash subcommand:` / `Slash
  option:` header lines validated as the panel validates its rows (customcommands.go:
  610-741 texts), typed options resolved from the test's users/channels/roles as
  discordgo decodes them, the handler's keys (.Options/.SubCommand/.Args/.CmdArgs with
  absent optionals skipped, blank .Message), `User Context Menu` / `Message Context
  Menu` triggers with .TargetUser/.TargetMember/.Author/.CommandType and no .User/
  .Member (sendDM sends nothing, as YAGPDB's does without a member), `context.interaction`
  types slash / user_menu / message_menu (tools/emulator/internal/runtime/slash.go). Not
  captured from Discord (INF): its refusal of `updateMessage` on a slash/context-menu
  interaction, which the emulator stands in for with a 50035 "Invalid Form Body"
  warning. Unit 4 (modals) shipped 2026-09-27, the last of the four: cmodal/
  modalBuilder/ctextInput/clabel with every vendor error text and quirk (a `fields`
  slice keeps its first 5 silently; `components` discards ModalBuilder.Set's error, so
  a bad entry silently ends the list), sendModal in vendor check order, the `Modal
  Submission` trigger with .IsModal/.Values (field order)/.ModalValues/.CustomID and
  .Message = the source message or a blank one, `context.interaction: { type: modal }`
  with ordered `fields:`, and `{ kind: modal, title, custom_id, fields }` in
  `interaction_responses:` (tools/emulator/internal/runtime/modals.go; the skill has a
  two-test recipe). Not captured from Discord (INF): its refusals of a modal after a
  deferral (40060) or in answer to a modal submission (stood in for with 50035), and of
  a deferred update of a modal no message opened (then its output and any followup,
  stood in for with 10015, aren't delivered); of text inputs and labels in a message
  (50035); and which form a submission comes back in (action rows for a `fields` modal,
  labels for a clabel one: a test's `form: label`); all warnings. Not modelled: a clabel
  around a modal select menu, checkbox or radio group (an emulator error; design (e)),
  and whether Discord accepts a followup after a modal response (the emulator records
  the printed output as a followup).
- Deferred (unit 2 review, 2026-09-27): `.Interaction.MessageComponentData` (discordgo's
  method on the embedded Interaction, lib/discordgo/interactions.go:271) isn't modelled:
  a YAGPDB template can call it, the emulator's `.Interaction` has no such method (use
  `.InteractionData`). Promote when a command calls it.
- Emulator divergence to know (unit 2): YAGPDB runs an execCC child in a goroutine
  (tmplextensions.go:249) sharing the caller's Interaction pointer, so a caller that both
  execCCs and prints races the child for the interaction's one response. The emulator
  runs the child inline: the child's sendResponse always wins and the caller's output is
  always the followup. Commands must never mix the two (either the child responds, or the
  caller does); a test can't tell you which one Discord would have picked.

## Free-tier compatibility (Lila 2026-09-27: the suite should work on free servers too)

Free per run: 1 execCC, 10 DB interactions; commands ≤10,000 runes
(vendor tmplextensions.go:184/394, customcommands.go:164-176, 357-360).
- Planned: a parser-based minifier (drop comments, rename variables per scope, re-print),
  proven equivalent by running the whole emulator suite on the minified output; output
  committed as `dist/free/`, used by the deploy manifest only for commands over a free
  server's limit (panel.json gets a per-server tier). Needed by db (17,538) and gematria
  (11,514): comment/indent stripping alone leaves them at ~10.5k/10.8k; variable renaming
  gets both to ~7.7k (measured 2026-09-27).
- Known defect (found in the config_sync review 2026-09-27): bootstrap makes 18 DB calls
  (9 dbGets, 9 dbSets), so on a free server it dies at the 11th, the Rules dbSet, and
  never writes Roles, Channels, Admin, Knowledge, Directory, Inactivity Prune or Staff
  (`yagtest run -no-premium -strict -db tools/emulator/testdata/initial_db.json
  commands/plumbing/bootstrap.gohtml`). Fix before a free server bootstraps: e.g. split it
  in two runs, or set only the dicts a fresh server lacks.
- Ruled out: passing embed_exec's Global fields through ExecData to save its one dbGet.
  It stays within limits on both tiers (each run has its own counters) and would couple
  ~30 callers to embed_exec's internals.
- Ruled out (audit error): "multi-word pyramid exceeds the execCC nesting cap". Its
  per-word execCC is delayed (1s); only immediate execCC sets StackDepth
  (tmplextensions.go:237), so each scheduled word starts at depth 0.
- Ruled out (sweep error, 2026-09-27): an and/or sweep (YAGPDB's and/or are eager,
  vendor lib/template/funcs.go:342/357; 128 call sites) claimed four crashes: db.gohtml's
  `and $entry $entry.Value`, bootstrap/staff_roles' `or (dbGet 0 "X").Value sdict`, and
  `or $user.String ...` on a nil userArg in message_link and avatar_viewer. None crash:
  `.Field` on an untyped nil evaluates to nothing (lib/template/exec.go:794-799), probed in
  the emulator, whose engine is YAGPDB's copy (only op-limit and call-count hooks
  differ). Only a typed nil pointer's field errors (exec.go:803-806, 863-865), and
  message_link already guards getMember for that. The eager-and hazard that is real:
  eq/lt on mismatched types (the simple_db_edit rewrite hit it and guards it).
- Ruled out (checked 2026-09-27, vendor code-reading): staff/directory.gohtml:98's bare
  `exec "Clean"` can't post text. Clean returns a `*dcmd.TemporaryResponse`
  (moderation/commands.go:810), which exec's type switch doesn't match, so exec returns ""
  (commands/tmplexec.go:202-215). Its failures come back as errors that end the run, and
  capturing the result in `$silent` wouldn't catch those either.

## Watch: tickets may move to Dyno

- FYI (Lila 2026-09-27, relayed by the Night House site session; a separate project she
  wants investigated first): she's considering Dyno Premium tickets instead of YAGPDB's
  for better UX. If it happens, everyone/general/ticket_clean (nags about the YAGPDB
  `tickets open` syntax in #ticket-submission) becomes a candidate for retirement. Nothing to do until she decides.

## Known command bugs

The snapshot audit's list (2026-09-25) is fixed (see Completed Improvements). Each fix
gets a failing test first.
- Found 2026-10-01 (grounding the context-menu unit), Lila's call — the color is
  user-visible: avatar_viewer.gohtml's role-color loop (lines 170-178) computes `$color`
  from the TARGET member's top colored role but never passes it to embed_exec, so the
  embed always takes the INVOKER's color (embed_exec derives it from AuthorID). Either
  pass `"Color" $color` (avatar embeds adopt the viewed person's color) or delete the
  loop (invoker-colored, today's live behavior). The context-menu twin avatar_menu
  shipped without the loop (invoker-colored).
- Found 2026-10-01 (same grounding): avatar_viewer's guild-icon branch builds
  `https://cdn.discordapp.com/icons/<id>/.png` — a 404 — when the guild has no icon:
  `.Guild.Icon` is empty and the `or $avatarURL $defaultAvatarURL` fallback can't catch
  non-empty garbage. Fix shape: fall back to `$defaultAvatarURL` when `.Guild.Icon` is
  empty, mirroring embed_exec's own guild branch. (The same `or` fallback IS live in the
  text command for a typed unresolvable snowflake: userArg nil → "(Unknown)" +
  Default Avatar; the context-menu twin dropped it as dead code — resolveSlashUser never
  returns nil and AvatarURL never returns empty, vendor handle_slashcommand.go:231-238 +
  user.go:162-189.)
- Known quirk, candidate improvement (found 2026-09-30 building db_slash, probed against
  db.gohtml): `db set` (text and slash alike) only accepts JSON OBJECT values — the set
  path runs jsonToSdict on every string, so `db set Key hello` fails with "Invalid value
  provided!"; a plain string must be sent as `"hello"` (a JSON string literal), which
  jsonToSdict also refuses (objects only). Kept for parity in the slash port (tests pin
  it). Improvement would be: accept a plain string as-is when it doesn't start with `{`,
  storing it as a string. Lila's call whether the convenience is worth the behavior
  change.
- Fixed 2026-09-27, access control (found 2026-09-27 by the Night House site session,
  confirmed by code-reading): everyone/services/db_get_embed.gohtml (and db_get_text,
  same shape) took any all-digit first argument as the target user ID with no staff
  check, and both ran for every member (Utility group), so `/db_get_embed 0 Admin`
  showed the server-wide Admin dict (also Staff, "Inactivity Prune", Roles, Channels…:
  every config dict is user_id 0, db_schema.yaml), and any member could read another
  member's per-user entries. Now gated behind hasRoleID like db.gohtml: a digit argument
  other than the caller's own ID is refused ("Only staff can look up another member's or
  the server's entries.") unless the caller has the Staff role; own ID and the ExecData
  service path (trusted callers) are unaffected.
- Fixed 2026-09-27, from the command review (`docs/COMMAND_REVIEW.md`, "Bugs found"): a
  deleted middle rule hid the last rule, simple_db_lookup couldn't read `Commands`,
  `db dump` skipped `Staff` and `Inactivity Prune` (it now lists every stored key with one
  `dbGetPattern`, so it dumps at most 100 keys per user), and contrasts dropped colors
  silently.
- Fixed 2026-09-27, staff_roles' three data/crash bugs (found by its first tests): a
  missing `Roles` or `Staff` dictionary now falls back to an empty sdict, house style;
  an unset `Roles > Staff` is skipped instead of crashing `reFind` on nil; and when every
  given role ID is invalid, the command sends an error embed instead of wiping the
  configured staff roles with an empty list.
- Fixed 2026-09-27: the Global "ExecCC Limit" setting (bootstrap default 10) was trusted as
  is, and YAGPDB allows 10 execCC calls per run on premium (1 on a free server), counted
  together with scheduleUniqueCC, so a setting above that made rules, contrasts, hugemoji
  and pyramid fail at the 11th call instead of skipping. Each now clamps its own read of
  the setting to `min(setting, 10)` before using it. (Race fixes, 2026-09-27: rules no
  longer execCC's `rule` at all, so it no longer reads or clamps "ExecCC Limit"; it now
  packs its own embeds into as few messages as Discord's 10-embed / 6,000-character
  limits allow instead. contrasts and hugemoji are unaffected, and still clamp it.)
- Deferred: `define` folds accents with a hand-written map (Latin-1 vowels, ñ, ç, ý/ÿ,
  macron vowels), while the site's slugify strips every combining mark after NFD. A term
  with any other mark (č, ş, ő, ą...) gets a hyphen where the site drops the mark, so its
  anchor misses. Today's 67 terms are all covered (only Ásatrú and Santería are
  accented, checked 2026-09-27). Promote when a glossary term with another accented
  letter is added to the-night-house `src/content/glossary.md`: add its letter to the map.
- Ruled out: gematria_bootstrap lists `Â`/`â` twice. The table is the Romanian letters
  (Ă Â Î Ș Ț) merged with the French ones (À Â Ç ...), which share Â; the repeated key has
  the same value (lines 59 and 63), so it is a no-op, and an edit would only cost a paste
  and a bootstrap rerun.
- Ruled out (2026-09-30, tripped Lila once): only `db dump` defaults staff to the global
  row (userID 0); get/delete/set/add/remove need the explicit `operation:0` form (db.gohtml
  63-73, 161-169). Deliberate shape left as is: for destructive operations, naming the row
  is the safety catch; dump defaults to 0 because it only reads. Documented here so nobody
  "fixes" it into a fat-fingered global delete.
- Deferred (found in the rules browse review 2026-09-30, pre-existing pattern shared with
  rules.gohtml): the rules family scans `seq 1..maxRuleNumber` linearly, so a pathological
  key like "Rule #1000000" (rule_edit's number argument is free-form) would exhaust the
  ops budget on the next `rule`/`rules`/browse run. Bound the number in rule_edit —
  natural home: the planned /edit modal unit (validate 1..9999 there). Trigger: that
  unit, or a real typo incident.
- Stale key noted 2026-09-30 (config_sync verification dump): the Commands dict still
  held `ticket_adduser_exec` "45" from the command retired 2026-09-25 — config_sync
  merges without owning the dict, so pre-existing keys survive. Inert (nothing deployed
  reads it; the emulator's admission tests use their own test-local dict). DELETED by
  Lila 2026-09-30 via `/db delete:0 Commands:ticket_adduser_exec`; the dict now holds
  exactly the managed set.

## Emulator Enhancements

- Found 2026-09-30 deploying db_slash: the panel/validator counts every newline TWICE
  against the response limit (form arrives CRLF server-side; runes + newline-count vs
  20,000 premium — customcommands-editcmd.html updateCCLength, web/validation.go
  ValidateTemplateField), so a template can pass every emulator test and still be
  REFUSED at save time (db_slash was: 19,881 runes + 449 lines). The emulator's
  premium char check should model this (its limit check counts runes only). Guard in
  place meanwhile: the linter's `response-length` rule (tools/linter/yagpdb_lint.py)
  errors on any command file whose panel count exceeds 20,000.

### Remaining emulator gaps
- Deferred, unverified: embed_exec cuts titles and descriptions by code point, and the
  emulator's limit checks count code points too (limits.go, `utf8.RuneCountInString`). The
  PR 7 review suggested that Discord may count characters outside the BMP (many emoji) as
  two UTF-16 units. If so, an emoji-heavy title or description near the limit would still
  be rejected, and the emulator couldn't show it. Promote on a live rejection of an
  emoji-heavy embed, or on a Discord doc or source that settles how it counts.
- A modal interaction response records its shape only — kind, title, custom_id, field
  ids (runtime SnapshotResponse) — not each text input's value/placeholder/required/
  style. /edit's prefill behavior (edit_tests.yaml) is therefore pinned by modal shape
  alone; a prefill regression passes the suite. Promote before any unit needs to
  assert a modal field's value or placeholder: record the fields in the response and
  expose them to yaml assertions and snapshots (expect snapshot churn in modal tests).
- The mock user factory (runtime/context.go:644) hardcodes a no-avatar user
  (MockUser/0), so avatar URL assertions only exercise the computed default-avatar path
  (`embed/avatars/N.png?size=1024` — size passthrough pinned by avatar_menu_tests); a
  real `cdn/avatars/<id>/<hash>.png` URL is unreachable in tests. Promote when a unit
  needs to assert a real avatar URL: give the factory an optional avatar field.
- Discord's error bodies are written as `{"message": "...", "code": N}` (errors.go
  discordError): the spacing is Discord's usual, not captured from a live response, and
  a 50035 Invalid Form Body body also lists the fields at fault, which the emulator's
  leaves out. A `{{catch}}` comparing the whole text could differ; one checking for the
  code or message won't. Promote by capturing a real response. Also unprobed: which
  error Discord gives first when a call has two problems (the emulator checks a
  reaction's emoji before its message, and an edit's target before its form body);
  promote if a command's catch tells those errors apart.
- A `bot_cannot_send` channel refuses sends only: `editMessage` there still succeeds,
  though YAGPDB's ChannelMessageEditComplex error would surface too (context_funcs.go
  tmplEditMessage). Deferred (2026-09-27): channel_link edits only messages it just
  posted; promote when a command edits in a channel it didn't just post to.
- `editMessageNoEscape` runs editMessage's code, so its warnings name `editMessage`.
  Promote when a command uses editMessageNoEscape (none does today).
- Left as is (2026-09-25): a test whose name starts with a newline can't be
  snapshotted. The name is a YAML map key, which yaml.v3 can't write for such text; the
  write is refused with an error, so nothing is corrupted. Promote if a test needs it.
- `cembed` returns the emulator's map (JSON keys: "title", "author"), where YAGPDB's
  returns a *discordgo.MessageEmbed, so `(cembed "title" "x").Title` reads nothing here.
  A message read back holds discordgo-shaped embeds already (types.MessageEmbed). No
  command reads `(cembed ...).Field` directly (git grep; a cembed kept in a variable and
  read later wasn't traced); promote when a command reads one.
- `sendDM` turns any argument into text, where YAGPDB's takes an embed, a list of embeds
  or a complexMessage (context_funcs.go tmplSendDM). No command calls sendDM (git grep,
  2026-09-25); promote when one does.
- Ruled out (2026-09-25): YAGPDB's `LimitWriter` drops leading whitespace bytes, and
  `serializeValue` passes msgpack through it, but no value's encoding starts with one.
  msgpack v4.0.4 (YAGPDB's and the emulator's) writes one-byte fixints only with compact
  encoding, which `serializeValue` doesn't turn on; ints start with 0xd3, and maps,
  arrays and strings with header bytes outside 0x09-0x0d and 0x20. In the emulator,
  `dbSet` of 32 and 10 read back as 32 and 10.
- `exec`/`execAdmin` record the command line (`execs:`, snapshots) but don't run the bot
  command: a test declares what a call returns per line (`exec_responses:`; an
  undeclared line returns "" and warns `[exec]`). The command's own checks aren't
  modelled: errors that fail the run ("exec/execadmin, run: ...", a parse error,
  execAdmin's "Failed fetching member", a guild cooldown), nor the text YAGPDB returns
  ("Unknown command", "Error: ...") unless a test declares it.
- Values holding Discord objects (a member, a message, a `cembed`, a whole `dbGet`
  entry) serialize as the emulator's types, so their size differs from YAGPDB's. A value
  whose overflow past 100000 bytes is only whitespace is stored whole; YAGPDB stores it
  cut off, so reading it back fails. Deferred (2026-09-25): none of the 26 dbSet calls in
  everyone/ and staff/ stores a Discord object, and the largest stored dict comes
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
- `.Guild.Channels` items (types.ChannelState) lack dstate.ChannelState's thread
  metadata, permission overwrites, DefaultThreadRateLimitPerUser and forum fields (tags,
  default reaction, sort order, layout), so reading one errors here where YAGPDB gives
  the value. Promote when a command reads one.
- `.User`, `.Member` and `.Message` are values here, where YAGPDB's are pointers
  (`&c.MS.User`, `DgoMember()`, `*discordgo.Message`): a pointer-receiver method on them
  isn't reachable, and `printf "%T"` and printing differ. `.Channel` and `.Guild` are
  pointers already. Promote when a command calls such a method or prints one of them.
- Channels: a declared channel has a type, parent, position, topic and NSFW flag, and no
  DMs (getMessage/editMessage refuse them) or permission overwrites. A test that declares
  no channels treats any channel ID as existing, with a `[channel]` warning per ID, and
  its .Guild.Channels is empty. getTargetPermissionsIn and sendTemplate ignore their
  channel (YAGPDB's sendTemplate errors "unknown channel"). Threads (a `guild.channels`
  entry of type 10/11/12): resolved by ChannelArg/getChannelOrThread and by name
  (text-like channels tried first, then threads), the way baseChannelArg does
  (vendor common/templates/context_funcs.go); absent from .Guild.Channels/ChannelOrder,
  the way dstate.GuildSet.Channels holds no thread state; getChannel errors "channel not
  in state" for one, the way GS.GetChannel (channels only) does (2026-09-27,
  tools/emulator/internal/runtime/channels_test.go's TestThreadNotInGuildState,
  TestGetChannelOrThreadResolvesAThread, TestSendMessageToThreadByID,
  TestThreadNameLookupOrder). Not modelled: `getThread`'s live-Discord-API fallback for a
  thread not already declared (getThread isn't implemented at all — no emulator function
  calls it); `ChannelArgNoDMNoThread` (only vendor's `editChannelTopic` uses it, and the
  emulator doesn't implement `editChannelTopic`); and a thread's own
  ThreadMetadata/message-count/parent-forum-tag fields (a `guild.channels` thread entry
  has the same type/parent/position/topic/NSFW shape as any other declared channel).
  Promote the `getThread`/`editChannelTopic` gaps when a command calls either; promote the
  thread-metadata gap when a command reads a thread's own metadata.
- Reactions: an emoji is refused only when it isn't a string ("<int Value>"); an unknown
  or misspelled emoji, which Discord refuses (10014), is recorded. Reacting to a message
  the emulator doesn't know is Discord's 10008 refusal, though the message may exist in
  production. addReactions in an interval run reacts to the stand-in message (ID 0), which
  Discord refuses; that the error is 10008 is inferred, not probed.
- A fixed clock (`clock:`) stands still for the whole run, where YAGPDB's moves on by
  milliseconds (sleep moves it on). The database's entry times follow the calling run's
  clock, so a sleep inside an execCC child doesn't move them, and a setup template's sleeps
  aren't carried into the test (its entries can look newer than the test's clock).
- `printf "%T"` of the emulator's Discord types prints their Go names (`types.CtxMessage`,
  `types.Timestamp`), not discordgo's (`*discordgo.Message`, `discordgo.Timestamp`); a
  command comparing those names would behave differently. None does today (the `%T`
  comparisons in db, db_get_text and db_get_embed are against `*templates.SDict` and
  `string` only).
- Discord functions are mocks: role changes don't update the
  members' roles within the run (as in YAGPDB, whose state updates later), `sendTemplate`
  is a no-op, and there are no components yet (threads are modelled as of 2026-09-27,
  see the Channels gap above for what's still missing).
- Pings: the bot's "Mention @everyone, @here, and All Roles" permission is one setting for
  the whole server (Discord checks it per channel), and it defaults to granted. A role the
  test doesn't declare is never mentionable. A reply to a message the emulator doesn't know
  (not the trigger, a test's `messages` or a sent one) is assumed to exist: it warns
  (`[message]`) and records no author ping. Discord refuses a reply to a message that
  doesn't exist by default (fail_if_not_exists; unverified here, YAGPDB doesn't set it).
  A `silent` message still counts its pings, though Discord sends no notification for it.
- That a failed run's show_errors message pings no one is read from the code, not probed.
  With `-strict`, a child over the source-length limit sends that error as the message;
  YAGPDB wouldn't save such a command.
  Deletions are recorded, not made: a deleted message stays findable by getMessage for the
  rest of the run (YAGPDB deletes from a goroutine or a scheduled event, so usually after
  the run ends, but a short delay can land mid-run). `editMessageNoEscape` is
  `editMessage` (edits notify no one either way).
- An execCC child works on a copy of the messages, so the caller's later getMessage never
  sees what the child sent or edited. YAGPDB starts the child in a goroutine (tmplRunCC's
  `go ExecuteCustomCommand`), so the caller most likely reads first (inferred, not
  probed), but a caller that sleeps can see the child's messages there.
- `editMessage` gaps: a stored message keeps its embeds but no file, so edits of file
  messages can differ. A test message is the bot's to edit only with
  `author_id: 1234567890`, and no message is ever `.Pinned` (pins aren't modelled).
  `editMessage` counts any sticker or forward key of a complexMessage as keeping the edit
  non-empty, though YAGPDB's ToMessageEdit drops them (only components carry over).
- Deferred (found 2026-09-27): `sendDM` takes its message as a string only (engine.go
  `sendDM`, ToString), but c579722's tmplSendDM runs it through parseMessageInput
  (context_funcs.go:74-86): it takes an embed or a complexMessage, and sends nothing for a
  message with no content, embed, file or components. Promote when a command sendDMs an
  embed or a complexMessage (none does today).
- A LIKE pattern is matched against the rows the query's other conditions select, so the
  trailing-escape error comes only from a row whose match reaches the escape. Postgres
  also runs LIKE while planning, on the key column's statistics (every server's keys),
  for a pattern that isn't an exact match: there the emulator warns (`[db]`) instead of
  guessing. Keys that aren't valid UTF-8 or hold a NUL byte are stored; Postgres
  rejects them.
- `parseArgs` resolves `user` and `member` arguments through the mocks.
- Role gaps: a test that declares no guild roles treats any role ID as existing, with a
  `[role]` warning per ID (a stale ID would be nil in production).
- A join message's `ctx.Msg` isn't modelled (a blank message from the joining member,
  which an execCC from it would inherit). (Component and modal triggers now are, with
  YAGPDB's `.Message`: the interaction's message with the clicker as author.)

## Tooling defects

- Forum prompts are POSSIBLE with the vendored YAGPDB (CORRECTED 2026-10-01 evening —
  an earlier entry here claimed the vendor was stale and `createForumPost` absent; that
  was a grep miss: the absence grep's `head -8` truncated before
  context_funcs.go:1532). Facts: `createForumPost` is registered in our vendor
  (context.go:916) and posts a forum thread WITH a first message
  (`ForumThreadStartComplex`, context_funcs.go:1532); content can be a complexMessage,
  so `allowed_mentions` rides on the message. Whether forum first-messages actually
  NOTIFY is still to be runtime-verified at /prompt's smoke (the same
  content-vs-pings lesson as the role ping). What's actually missing: the EMULATOR
  knows the name (yagpdb_funcs.go known-funcs list) but has no implementation — a
  /prompt unit ports `tmplCreateForumPost` from the vendor first (fidelity rule: copy,
  don't reimplement). `make vendor-drift` (added 2026-10-01) guards vendor-vs-upstream
  drift; its first run is what exposed the false staleness claim.
- Upstream stance research (2026-10-01, for any future submission): botlabs-gg/yagpdb
  CONTRIBUTING.md says NOTHING about AI-assisted contributions (silent, not hostile);
  MIT license; PRs target `dev`, not master. No known upstream issue/PR covers forum
  thread creation (it shipped). Openly-AI-labelled contribution remains Lila's call
  if one ever happens.
- `scripts/test-all-templates.sh` asserts each bare run's exit code only, and the yaml
  runner refuses bare runs of Slash/Modal/Component triggers by design
  (loader/runner.go:374-383) — so the one-line usage messages those runs print (e.g.
  /edit's three) are pinned nowhere: a wording regression in a bare line cannot redden
  any gate. Shape: the script (or a generated fixture) asserts each bare output against
  the file's expected line.
- DECIDED (Lila 2026-09-27, both parts; part 1 after the channel_activity + db_get
  deploys, part 2 with the slash work). Shape for part 1: a generated
  commands/plumbing/config_sync.gohtml (Hourly interval) with panel.json's ids baked in
  (as shipped: by a generator, the file committed, since deploys paste committed bytes), dbSet-ing the Commands dict each run (idempotent, self-healing); its own
  id also lives in panel.json. Keep config in a few DB dicts (one dbGet each fits the free tier's
  10 DB calls per run), but stop typing IDs by hand. (1) The `Commands` dict comes from
  deploy/panel.json, which already holds every command's panel id: the deploy tool
  generates a one-shot config command (dbSet "Commands" …) from it after each deploy,
  replacing bootstrap's embed_exec/db arguments and every `simple_db_edit Commands …`
  step (today: channel_activity_pager). (2) Role and channel IDs come from a `/setup`
  slash command with Discord's role/channel pickers once slash commands land (resolving
  by name at runtime breaks on a rename, so IDs stay stored).
  Part 1 shipped 2026-09-27: `scripts/gen-config-sync.py` (`make config-sync`; `make ci`
  runs its `--check`) generates commands/plumbing/config_sync.gohtml (Hourly interval,
  Staff Utility) from panel.json, one id map per server chosen by `.Guild.ID`, merged into
  the Commands dict as strings (1 dbGet + 1 dbSet, no output); bootstrap lost its
  embed_exec/db arguments (usage `bootstrap [staff role ID]`, and an omitted role no
  longer sets Roles "Staff" to nil; the old three-ID form is refused, since parseArgs
  would fold it into the role). Main's config_sync is panel id 88. An interval command
  runs only with a context channel set: without one (or once that channel is deleted)
  YAGPDB neither runs nor reschedules it, silently (handle_timed.go:165-172). Part 2
  (`/setup`) stays open, with the slash work.
- Deferred (trigger: Lure of the Void is used again): lotv has no config_sync (no panel
  id in panel.json, so it isn't deployed there and the generated lotv branch is unused),
  and its bootstrap no longer writes embed_exec/db. Revive it with a config_sync created
  there (disabled, a channel set), its id in panel.json, `make config-sync`, deploy.

- Shipped (8aa3b70): deploy/deploy.test.js no longer pins the sha256 of real command
  files; the parity test computes the Python-normalized hash at test time (spawnSync
  python3) and compares it with deploy.js's.
- Fix next: port the vendor refresh. `vendor/yagpdb` was refreshed 2026-09-27 (Lila's
  yes) from 0cf2ec5 (2025-12-18) to c579722 (2026-09-27); the emulator still copies the old
  code. `git -C vendor/yagpdb diff 0cf2ec5 c579722 -- common/templates lib/template
  customcommands lib/dstate lib/discordgo/structs.go lib/discordgo/components.go
  lib/discordgo/message.go lib/discordgo/interactions.go` (58 files, +7.7k/−2.3k with
  voice/mls noise). Notable: lib/template exec.go/parse.go/template.go changed (the
  emulator's internal/yagtemplate is that copy); new template functions
  `hasAnyPermissions`, `targetHasAnyPermissions`, `memberAbove`, `memberAboveRole`;
  components.go grew. Do this BEFORE the emulator components unit ("Interactive UX").
  Analysis 2026-09-27 (read-only, c579722 line numbers) split the port into units after
  the lib/template re-sync (shipped 9e12156): (1) message builders (shipped): complexMessageEdit =
  complexMessage (general.go:263, returns *MessageSend, error text "send message builder"),
  editMessage on parseMessageInput (context_funcs.go:32-70, 457; accepts MessageSend, no
  null check, an embed-only edit sends content "" — pin with a test), a repeated "embed" key
  replaces, CtxMessage.Pinned (dstate interface.go:425); (2) small fixes (shipped): toInt/
  ToInt64 optional base (general.go:1219/1240), .BotUser without a member (context.go:365),
  execCC/scheduleUniqueCC also refuse Role-trigger CCs with the new error text
  (tmplextensions.go:202-210, 309-317), runtime/yagpdb_funcs.go regenerated from c579722
  (run scripts/gen-yagpdb-funcs.sh from the main checkout: worktrees have no vendor/); (3) new
  functions hasAnyPermissions/targetHasAnyPermissions (need a real permission model; the
  permission family is a stub today) and memberAbove/memberAboveRole (context_funcs.go
  771-847, 1051-1087) — DEFERRED 2026-09-27, promote when a command calls any of
  hasPermissions/targetHasPermissions/hasAnyPermissions/targetHasAnyPermissions/
  getTargetPermissionsIn/memberAbove/memberAboveRole (`git grep` over everyone/ staff/
  found none; the emulator never implemented hasPermissions either). Not modelled, record only: pin/pin-count limits, editChannel* 10-min
  cooldown, createThread/createForumPost raw errors, exec cooldown text, group
  RedirectErrorsChannel, nil ChannelOrThreadParent when the parent isn't in state.
- Deferred (found 2026-09-27): `scripts/lint-all.sh` passes a positional file argument to
  `yagpdb_lint.py`, which only accepts `--dir`; every file it hands over "fails" as a
  result. Fix next time the script is touched.
- Deferred (found 2026-09-27): `scripts/save-lint-report.py` passes an absolute path to
  `--dir` although its own header says `--dir .`, so `reports/latest_lint.txt` bakes in
  machine-local paths; that report is also stale (it still has pre-restructure paths).
- Deferred (found 2026-09-27): `tools/linter/main.go` doesn't build (unused vars
  `fix`/`hasGlobalDict`/`hasCommandsDict`), isn't `gofmt`-clean, and nothing references it —
  dead code, a candidate to delete (Lila's call).

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

- [x] admit_user, archive, guest, reject_user, screen_user and ticket_adduser_exec moved
      from staff/ to retired/: Discord's server join applications replaced guest
      screening, so the owner deleted them from YAGPDB. Kept, unmaintained, for anyone who
      wants the flow; still covered by admission_tests.yaml (2026-09-25)

- [x] A failed run's show_errors message is now checked against Discord's 2000-character
      limit too (YAGPDB's ChannelMessageSend error is discarded, so an over-limit message
      silently never posts); non-strict warns and still records it, strict warns and drops
      it, and applies to a top-level run and an execCC child alike (2026-09-25)

- [x] `SentMessage.Embeds` records every embed of a sent or edited message, not only the
      first; `has_embed`, `embed_contains` and `embed_title` match any of them, and
      snapshots store a message's embeds as an `embeds:` list (2026-09-25)

- [x] The six suites that execCC embed_exec (admission, avatar_viewer, command,
      dice_roll, gematria, message_link) now map it to the real
      `everyone/services/embed_exec.gohtml` instead of the recording mock, so their snapshots show
      the real embeds (the author line, the guild's "Embed Color", empty image and
      thumbnail) and DeleteResponse's deletions; none sends a description long enough to
      be cut (2026-09-25)

- [x] A test can declare what `exec`/`execAdmin` return per command line
      (`context: { exec_responses: { '<line>': '<response>' } }`, inherited by execCC
      children); an undeclared call still returns "" but now warns `[exec]` instead of
      silently doing so (2026-09-25)

- [x] `.Guild.Channels` holds YAGPDB's dstate.ChannelState shape, not `.Channel`'s: no
      IsThread or IsForum (reading one errors, as in production), IsPrivate and Mention
      as methods, and Icon, Bitrate, UserLimit, RateLimitPerUser, Flags and OwnerID read
      as zero. `.Channel`, `.Guild` and `.Server` are pointers, as YAGPDB's are, and
      `.Channel.Mention` works; `.server` and `.ChannelOrThreadParent` exist (2026-09-25)

- [x] getMessage returns a copy, as YAGPDB's fetches the message from Discord on each
      call: an editMessage after the fetch no longer shows through the fetched message's
      content or embeds (2026-09-25)

- [x] avatar_viewer: it never recognized real `whois` output (its field list lacked
      "Roles", which whois always adds, and the tracking-off "Usernames"/"Nicknames");
      it read only 5 of the mod log's actions (not warnings, timeouts, role changes);
      it took a mod log ID from anywhere in the text; and a link it couldn't read sent
      nothing (an empty ID field made Discord refuse the embed), where it now says no
      avatar was found (Lila's call). Its first tests use whois and mod log embeds as
      YAGPDB builds them (logs/plugin_bot.go, moderation/modlog.go) (2026-09-25)

- [x] A message's embeds read back as discordgo's (`.Title`, `.Author.Name`, `.Fields`
      with `.Name`/`.Value`, `.Image.URL`), not the emulator's maps; test messages take
      `embeds:` (cembed's keys, a typo refused), and message_link's quoted-embed branch
      has tests, including its 1024 cut and re-linking a Message Link. A read embed can
      be sent again (sendMessage, editMessage, complexMessage, cembed take it, as
      YAGPDB's do); a sent message keeps all its embeds, empty ones dropped (2026-09-25)

- [x] Function errors carry YAGPDB's text, which is what a `{{catch}}`'s `.Error` and a
      failed run's message show: "too many calls to this function" alone, and Discord's
      refusals as discordgo's `HTTP 404 Not Found, {"message": "Unknown Message", "code":
      10008}`. The emulator's explanation (which function, which limit, why Discord
      refuses) is a warning, with -strict too (2026-09-25)

- [x] A bad `clock:` value's error names the field and line (a mapping is refused
      rather than read as year 1), and a float `seed:` (1.5, 1.0, 1e3) is an error
      instead of being truncated (2026-09-25)

- [x] Without -strict, YAGPDB's function errors (call limits, Discord refusing a call or
      a message, reaction limits) were warnings even inside `{{try}}`, so the run went
      on where YAGPDB's `{{catch}}` would run. Inside `{{try}}` they are now returned (and
      still warned about); the engine's `OnCall` patch says whether a call is inside one
      (2026-09-25)

- [x] `yagtest watch` runs every test path given, as `test` does, and `-stop-on-fail`
      skips the paths after a failing one (2026-09-25)

- [x] Snapshot files always read back: yaml.v3 v3.0.1 wrote text starting with "\n"
      (or "\t\n") as a block scalar that lost the newline or couldn't be parsed, so
      snapshotting a failed run ("\nAn error caused...") corrupted the file. Such text
      is now double-quoted, and a write that wouldn't read back is refused (2026-09-25)

- [x] `embed_exec` cut every description to 1,998 characters; it now cuts only past
      Discord's 4,096, or less when the title, fields and author would push the whole
      embed over 6,000 (Lila's call). `db`'s own cut follows (2026-09-25)

- [x] The limit tables in docs/API_REFERENCE.md and the templates skill are rewritten
      from vendor/yagpdb and Discord's documented limits: embed description 4,096 (not
      2,048); execCC 1 call per run, 10 on premium, 2 levels deep (not "concurrent,
      typically 10-20"); DB entries, key cut, value size and calls per run; API calls
      per run (2026-09-25)

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
- [x] `.Guild.Channels` holds the declared channels, sorted by position as YAGPDB's state
      tracker sorts them (an unstable sort.Sort, copied); a declared channel's `type`,
      `parent_id`, `position`, `topic` and `nsfw` reach `.Channel`, getChannel and
      `.Guild.Channels`, and a name finds only text, voice, announcement and forum
      channels. directory and ticket_adduser_exec have their first tests (2026-09-25)
- [x] A declared channel's `bot_cannot_send: true` makes a send there fail with Discord's
      403 Missing Permissions (50013), nothing recorded; channel_link catches it and says
      it couldn't post (it aborted with no message) (2026-09-27)
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
- [x] A sent message records how far into the run's sleeps it went out (whole seconds, an
      execCC child's messages counting its caller's sleeps too); a `sent_messages` check's
      `sent_after_seconds: N` asserts it exactly, and a snapshot's message record carries it
      only when it's nonzero. Pins today's sleeps: hugemoji's NSFW-refusal and limit
      warnings, contrasts' dropped-colors warning, rules' per-rule messages (2026-09-27).
      The race fixes the same day removed those sleeps; the tests now pin their warnings
      at 0 s, sent before the first execCC
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
