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
Free-tier top 10 (lotv's deployed set): /color, /hebrew, /gematria, /timestamp, plus
/db and /edit if lotv adopts them — 6, under the cap. avatar_viewer / hugemoji keep their
text form and context menus (no slash root: the menus already are the discoverable form).

## Unit order

1. `/color` (smallest chain: no gematria; proves the embed_exec shared block).
2. `/hebrew` + `/gematria` (gematria's renderer check; pyramid's delayed self-execCC
   targets the root with ExecData).
3. `/rule`, `/timestamp`, `/define` (single-option roots).
4. `/hiatus` + `/unhiatus` (Night House message: member commands going real slash).
5. `/staff` (largest; may split if over 10 free subcommands).
Then config_sync part 2 (`/setup` pickers) and minifier part 2 (free-tier deploy manifest,
needed before lotv gets any root over 10k runes).

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
- Size: 8.7k runes as built → under the free cap, so lotv needs no minifier for it.

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
