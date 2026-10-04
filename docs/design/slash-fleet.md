# Slash-first fleet pass

Design of record (2026-10-04). Decided by Lila: one-level subcommands; gematria NOT
under /hebrew; free-tier top 10; each unit retires its own text twin LAST; context menus
are additive (text avatar_viewer/hugemoji stay); the Tzurot design language (errors and
settings ephemeral, results public). Root map APPROVED by Lila 2026-10-04 (start with /color).

## Mechanics (vendor c579722)

- One level of subcommands, ≤10 per root free / 25 premium; 10 slash CCs free / 50
  premium (handle_slashcommand.go:125-140, customcommands.go:950-964).
- A CC has ONE group, so a root is one permission scope: Utility roots and Staff Utility
  roots never share subcommands (hiatus is Staff Utility, unhiatus Utility on purpose).
- Immediate execCC nests at most 2 deep (tmplextensions.go:231-232) and the child shares
  the caller's Interaction pointer (:238-241). So root → child → gematria → embed_exec
  (depth 3) is refused: **a root can't dispatch to today's text commands**; it holds its
  subcommands' logic inline and execCCs only the renderer it already used (gematria,
  embed_exec) at depth 1-2.
- A run with empty output sends nothing (context.go:647-650), so a root that execCCs its
  renderer and prints nothing leaves the one interaction response to the child. Never
  mix: either the child responds or the root does (emulator divergence, FUTURE "unit 2").
- Interaction response deletes cap at 10 s (context.go:1011-1020): text commands'
  "Delete Response" errors become **ephemeral** responses instead of timed deletes.
- Defer mode `None` (as /prompt, /role_ping): the root's few DB reads and one execCC fit
  the 3 s ack.

## Shared block: embed_exec answers interactions (unit 0, ships inside unit 1)

embed_exec is the fleet's renderer (~30 callers). When `.Interaction` is set and not yet
responded to, it sends its embed with `sendResponse` instead of `sendMessage`;
`DeleteResponse` maps to `"ephemeral" true` (no timed delete). Without an interaction it
is byte-for-byte today's path, so text callers are untouched. gematria needs the same
check where it renders, for /hebrew and /gematria (unit 2).

## Root map (approved 2026-10-04)

| Root | Group | Subcommands (replaces) |
|---|---|---|
| `/color` | Utility | contrast (contrast), contrasts (contrasts), random (rand_color), hex (adds; hex_to_int's chat regex stays) |
| `/hebrew` | Utility | atbash, alefbet, pyramid, random (rand_hebrew) |
| `/gematria` | Utility | none: text option (gematria) |
| `/rule` | Utility | none: number option (rule) |
| `/timestamp` | Utility | none (timestamp) |
| `/define` | Utility | none (define) |
| `/hiatus` | Staff Utility | none (hiatus) |
| `/unhiatus` | Utility | none (unhiatus) |
| `/staff` | Staff Utility | roles, inactivity, activity, rules, pointer, bump_reset, delrep, bootstrap |

Existing: /db, /edit, /prompt, /role_ping. Main (premium): 13 slash CCs of 50.
Free-tier top 10 (for a free server; lotv turned out to be premium, 2026-10-04): /color,
/hebrew, /gematria, /timestamp, plus /db and /edit — 6, under the cap. avatar_viewer / hugemoji keep their
text form and context menus (no slash root: the menus already are the discoverable form).

## Unit order

1. `/color` (smallest chain: no gematria; proves the embed_exec shared block).
2. `/hebrew` + `/gematria` (gematria's renderer check; pyramid's delayed self-execCC
   targets the root with ExecData).
3. `/rule`, `/timestamp`, `/define` (single-option roots).
4. `/hiatus` + `/unhiatus` (Night House message: member commands going real slash).
5. `/staff` (largest; may split if over 10 free subcommands).
Then config_sync part 2 (`/setup` pickers) and minifier part 2 (free-tier deploy manifest,
needed before a free server gets any root over 10k runes; main, lotv and The Rose are
all premium).

## Unit 1: /color

- New `commands/color/color_slash.gohtml`, Slash Command `color`, Utility, Defer `None`.
  Subcommands and options:
  - `contrast`: `color string` (hex, `#` optional) OR `role role` — one required; both or
    neither → ephemeral usage error.
  - `contrasts`: `colors string!` (hex codes and/or role IDs/mentions, ≤10 analyzed,
    today's dropped-count warning folded into the response).
  - `random`: no options.
  - `hex`: `color string!`.
- Logic inlined from contrast.gohtml (luminance, WCAG ratios), contrasts (loop, one
  embed per color in ONE response, ≤10 embeds: one execCC on free), rand_color + hex_to_int
  (hex → int). Output: one execCC to embed_exec (results public, errors ephemeral).
  contrasts needs a multi-embed path: embed_exec takes `Embeds` (a list) or the root
  renders contrasts itself with sendResponse — implementer picks the smaller diff, cites it.
- Tests: emulator slash interaction per subcommand, error paths ephemeral, text paths of
  embed_exec unchanged (existing suites green, snapshots unchanged).
- panel.json: new id (Lila creates the panel command DISABLED); config_sync regen.
- Retired 2026-10-04 after Lila's live check: all three disabled on main by the deploy;
  contrasts moved to retired/; contrast and rand_color stay in commands/color/ because
  lotv still runs them (FUTURE: lotv /color, then retire them). hex_to_int stays (chat
  regex, and /color hex shares no code with it after inlining).
- Size: 8.7k runes as built → under the free cap, so even a free server needs no
  minifier for it.

### /color panel rows (command 103, Lila at the panel, 2026-10-04)

Slash description `Color tools`; tick "use subcommands"; then (header lines' text):

| row | type | required | description |
|---|---|---|---|
| sub contrast | — | — | WCAG contrast of a color or role against dark and light |
| · color | string | — | hex code, # optional (use this OR role) |
| · role | role | — | a role whose color to check (use this OR color) |
| sub contrasts | — | — | contrast of up to 10 colors or roles at once |
| · colors | string | ✓ | hex codes and/or role IDs or mentions, up to 10 |
| sub random | — | — | a random color |
| sub hex | — | — | a hex code's integer value |
| · color | string | ✓ | hex code, # optional |

## Unit 2: /hebrew + /gematria

Files: `commands/hebrew/hebrew_slash.gohtml` (`/hebrew`, 15,201 runes; minified 11,319 in
`dist/free/`, still over the free 10k cap: a free server can't take it),
`commands/gematria/gematria_slash.gohtml` (`/gematria`, 963 runes), and `gematria.gohtml`
(11,784 runes) grows by passing `Respond` through (set only when the caller passed it, so
every other caller's output is byte-identical). Both roots: Utility, Defer `None`. The text
twins (alefbet, atbash, pyramid, rand_hebrew, gematria's text trigger) are untouched; their
retirement is a later step after the live check. panel.json ids come from creating the
panel commands (DISABLED), then `make config-sync`.

Results are public; refusals are ephemeral sendResponses of the root with no execCC. A
result is ONE execCC to gematria with `Respond`, which passes it to embed_exec, which
answers through sendResponse: root (0) -> gematria (1) -> embed_exec (2), within YAGPDB's
immediate execCC depth of 2 (customcommands/tmplextensions.go:231-232). The root prints
nothing on that path (an empty run sends no response: common/templates/context.go:647-650).

### /gematria panel rows

Slash description `Gematria of Hebrew text`, no subcommands: option `text` string
required, `the text to calculate`.

### /hebrew panel rows

Slash description `Hebrew tools`; tick "use subcommands"; then (header lines' text):

| row | type | required | description |
|---|---|---|---|
| sub atbash | — | — | atbash cipher of Hebrew, Greek, Arabic, Latin, runes and digits |
| · text | string | ✓ | the text to encipher |
| sub alefbet | — | — | Paleo-Hebrew and Arabic letters converted to Hebrew |
| · text | string | ✓ | the text to convert |
| sub pyramid | — | — | gematria of a word's pyramid, one embed per word |
| · text | string | ✓ | one or more words |
| sub random | — | — | random Hebrew letters, with their gematria |
| · letters | integer | ✓ | letters per group |
| · groups | integer | — | how many groups (default 1) |

`random`: groups < 1, letters < 1 or letters x groups > 1000 are refused; the groups are
joined with single spaces (rand_hebrew's group mode joins with a leading space, which makes
gematria count an empty first word; the slash form has none).

### The multi-word pyramid

A word is one execCC to gematria. Several words cannot be answered by the delayed runs
alone: nobody would answer the interaction within Discord's 3 seconds. So the run answers
FIRST with a public sendResponse (`Pyramid gematria of **N** words follows, one embed per
word.`, plus pyramid.gohtml's skipped-words warning as a second line when `ExecCC Limit`
cut words), THEN schedules one delayed self-execCC (delay 1) per kept word to its own CC
(Commands key `hebrew_slash`), with ExecData `Description`, `AuthorID`, `ChannelID`.

- A delayed execCC (and scheduleUniqueCC) stores the caller's CurrentFrame, its Interaction
  and RespondedTo serialized at SCHEDULE time (tmplextensions.go:256-282; :344 for
  scheduleUniqueCC). Because the response went first, each stored frame has
  RespondedTo=true.
- The delayed run restores it (customcommands/handle_timed.go:102-117): `.Interaction` is
  set, but NOT `.IsSlashCommand`, `.SubCommand` or `.Options` (only set in
  handle_slashcommand.go:118-160). The root therefore dispatches on `.ExecData.Description`
  BEFORE `.IsSlashCommand`.
- The run's sendResponse follows tokenArg (common/templates/context_interactions.go:
  414-450): already responded -> followup, so each word's embed is a followup of the
  interaction.
- Cap: one immediate execCC on free servers, 10 on premium (the runcc counter,
  tmplextensions.go:185): the delayed execCCs are counted in the root's run, so the kept
  words are clamped to 10 as in pyramid.gohtml.

Emulator support (this unit): `scheduled_runs` takes `interaction: none|pending|responded`
(the state at schedule time), and a test runs the delayed half with
`interaction: { type: slash, delayed: true, responded_to: true }` plus `exec_data`.
- Retired on main 2026-10-04 after Lila's live check: atbash (68), alefbet (83), pyramid
  (75) and rand_hebrew (65) disabled by the deploy; the files stay in commands/hebrew/
  because lotv and The Rose still run them. gematria (52) stays: it is the roots' renderer,
  and its text trigger is the one Lila uses most. A pyramid line of 2+ letters now ends in
  its final form (display only: the Gematria Values give final letters their plain values).

## Unit 3: /rule, /timestamp, /define

Files: `commands/rules/rule_slash.gohtml` (`/rule`, 2,890 runes),
`commands/general/timestamp_slash.gohtml` (`/timestamp`, 2,852),
`commands/knowledge/define_slash.gohtml` (`/define`, 2,480; the slug and kb-alias block is
copied from define.gohtml (since retired), identical modulo leading whitespace, and the
header keeps the note that it must match the-night-house's `slugify`), and `commands/rules/rules_pager.gohtml`
(9,528 runes, was 8,689). All three roots:
Utility, Defer `None`, single option, no subcommands. Every new file is under the free 10k
cap. panel.json gets no ids until the panel commands are created (DISABLED), then
`make config-sync`.

After Lila's live check (2026-10-04) the text twins rule and define were retired
(`retired/rule.gohtml`, `retired/define.gohtml`; neither exists on another server) and
timestamp was disabled on main (its id dropped from panel.json); timestamp.gohtml stays on
lotv and The Rose, where it also got the `\d{16,19}` snowflake fix.

Results are public: ONE execCC to embed_exec with `Respond`. Refusals are ephemeral
sendResponses of the root with no execCC; the root prints nothing on an execCC path. Each
root that execCCs embed_exec checks first that its Commands id is above 0 (config_sync
hasn't picked it up otherwise) and refuses ephemerally with `⚠️ This command isn't set up
on this server yet.` The refusal tests are snapshotted: an ephemeral response is itself a
sent message in the emulator, so `sent_messages: []` can't pin "no stray channel message",
and the snapshot lists every sent message.

### /rule: Lila's decision (2026-10-04)

`/rule` with no number opens the browse view; with a number it shows that rule. A number
with no such rule (or 0, or any number when no rules exist) is an ephemeral refusal
(`Could not find rule N. Use /rule without a number to browse the rules.`, or `No rules are
configured yet.`; no range of valid numbers, since a deleted middle rule would make it
lie). The check is `HasKey
"number"`, not truthiness, so a given 0 refuses instead of opening the browse view. Without
`rules_pager` in the Commands dict the browse path refuses ephemerally.

### rules_pager answers a slash first render

An immediate execCC child inherits the caller's Interaction pointer (customcommands/
tmplextensions.go:223-247), and a slash run has no CustomID, so rules_pager's old
`{{ if .Interaction }}` branch would have split an empty CustomID and updateMessage'd a slash
interaction. The first render is now identified by `.ExecData.Page` (set only by `rule` and
`rule_slash`), not by `.Interaction`:

| caller | `.Interaction` | `.ExecData.Page` | parse | send |
|---|---|---|---|---|
| `rule browse` (text) | none | 1 | ExecData | `sendMessage` (byte-identical) |
| `/rule` (slash) | set | 1 | ExecData | `sendResponse nil $msg`: public, components ride through |
| button or select click | set | unset | CustomID | `updateMessage` (unchanged) |

`sendResponse` carries components: common/templates/context_interactions.go:307-368
(parseMessageInput) into lib/discordgo/message.go:722-734 (ToInteractionResponseData copies
Components). The opener stays `.User.ID`, the invoker. Existing rules_browse tests and
snapshots are unchanged; the slash first render is tested in `rule_slash_tests.yaml`.

### /timestamp

`id` omitted decodes the invoker. Given: trimmed, then a bare snowflake (`\A\d{16,19}\z`: a 20-digit
number would overflow int64 and decode as MaxInt64) or a mention (`<@id>`, `<@!id>`,
`<@&id>`, `<#id>`, same bound, digits extracted); anything else is an ephemeral refusal
that echoes the input with backticks turned into `'` (so it can't break the code span),
cut to 100 characters (the text twin silently falls back to the user; design language:
errors are ephemeral).

### /define

`term` omitted links the whole glossary; otherwise the same slug, kb aliases and
Title/Description as define.

### Panel rows (Lila at the panel, header lines' text)

| root | slash description | option | type | required | description |
|---|---|---|---|---|---|
| `/rule` | View a server rule, or browse them all | number | integer | — | the rule to view (omit to browse all) |
| `/timestamp` | When a Discord ID was created (default: you) | id | string | — | a Discord ID or mention (default: you) |
| `/define` | Link a Night House glossary term | term | string | — | the term to look up (omit for the whole glossary) |

## Unit 4: /hiatus, /unhiatus

Files: `commands/members/hiatus_slash.gohtml` (`/hiatus`, Staff Utility) and
`commands/members/unhiatus_slash.gohtml` (`/unhiatus`, Utility, for the reason in the root
map). No options. After Lila's live check (2026-10-04) the text twins hiatus (main 81) and
unhiatus (82) were disabled and moved to `retired/`.

Every reply is to the invoker alone, so both roots use Defer mode `Ephemeral Message
Response` (customcommands/handle_component.go:172-174), the fleet's first: the ack goes
out before any work, so the role calls (one GuildMemberRoleRemove/Add HTTP call per role,
context_funcs.go:2465-2520 and 2580-2612) never race the 3 s window, and the reply is
written only after the work is done. The reply is the run's PRINTED OUTPUT, which a
deferred run sends as the edit of the deferred response (context.go:604-607, 684-692:
EditOriginalInteractionResponse, ephemeral from the defer; the emulator records
`deferred_edit`). Not `sendResponse`: after a defer that is a followup
(context_interactions.go:350-363, 446-448), which leaves the "thinking..." message unfilled.

Success order: refusal checks; roles taken (hiatus, held roles from `hasRoleID`, the
interaction member's own roles, no API call) or given (unhiatus); `Staff` dict written;
ONE execCC to embed_exec WITHOUT `Respond` posting the record to Mod Log (the current
channel when Mod Log is unset, as today); then the ephemeral reply naming the roles
removed or restored (`<@&id>` mentions; an ephemeral message notifies nobody). Refusals
(no staff roles configured, holds none of them, not on hiatus, embed_exec not in
Commands) are ephemeral replies with no execCC and no writes.

`/unhiatus` restores only the recorded roles still in `Staff.Roles` (Lila, 2026-10-04: it
is in Utility, so anyone can run it, and someone removed from staff while on hiatus must
not get the roles back). The reply names any recorded role it skipped; when none is still
a staff role it refuses and keeps the entry, so re-adding a role to Staff.Roles makes it
restorable. The match is by `toString`, since YAGPDB's `in` doesn't compare int with uint
(general.go ~606). The execCC child runs in a goroutine sharing the interaction (tmplextensions.go:240-248),
so the parent's reply is safe only while embed_exec's non-`Respond` path prints nothing
(its whitespace is trimmed and dropped, bot.go:762, context.go:648); a `print` there
would race the parent for the deferred edit. YAGPDB swallows role-call failures (giveRole/takeRole return "" on
every error path), so a failure can't be detected or reported; the retired text twins behaved the
same.

### Panel rows

| root | slash description |
|---|---|
| `/hiatus` | Step away from staff duties (removes your staff roles) |
| `/unhiatus` | Return from a staff hiatus (restores your staff roles) |
