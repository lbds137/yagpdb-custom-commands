# Twin retirement — unit-5 handlers, /pointer, fleet servers

GROUNDED 2026-11-04 from repo state (staff_slash.gohtml dispatch table, deploy/panel.json,
docs/design/slash-fleet.md §Retirement, docs/FUTURE_IMPROVEMENTS.md). Execution is gated on
Lila's live pass; nothing here is built yet.

**What would stale this spec:** a failed u5 live check on a handler (its fix lands before
retirement and may change the file); a different Rose/lotv ruling from Lila (changes §4/§3
only); u5-pointer-3 failing (delays §2).

## Gates (live-checks page, 7 sections / 55 items, 16 ticked 2026-11-04)

- **u5 section: 9/19** — open: u5-act-1, u5-ann-1…6, u5-delrep-2, u5-pointer-3, u5-roles-2.
  The handler switch (§1) waits for the whole section: a failing check means the handler
  changes before it retires.
- **message_pointer disable (§2) waits only on u5-pointer-3** (u5-pointer-1, -2 ticked).
- **lotv/Rose twins (§3) are NOT gated on this pass** — the pass has no lotv or Rose
  section; their live check still needs to be created.

## 1. The seven router handlers (main)

Dispatch table (staff_slash.gohtml:45-53): `roles`→staff_roles (80), `announce`/`prune`/
`prune_date`→inactivity (40), `activity`→channel_activity (86), `rules`→rules (48),
`bump_reset`→bump_reset (54), `delrep`→batch_delrep (76), `bootstrap`→bootstrap (78).

Retirement shape is already decided (slash-fleet.md §Retirement): each handler's panel
trigger becomes type **None** — *not* disabled, YAGPDB refuses execCC to a disabled
command (tmplextensions.go:198-200) — and the text-only branch is removed from the file.
The header's `Trigger type:` line changes (the browser deploy sets the panel trigger from
it, like `Group:`); None commands keep their `Group:` line (embed_exec is the model).

| handler | panel | file (lines) | text-path suite today |
|---|---|---|---|
| staff_roles | 80 | commands/members/staff_roles.gohtml (149) | staff_roles_tests.yaml |
| inactivity | 40 | commands/members/inactivity.gohtml (212) | inactivity_tests.yaml |
| channel_activity | 86 | commands/channels/channel_activity.gohtml (78) | channel_activity_tests.yaml |
| rules | 48 | commands/rules/rules.gohtml (182) | rules_browse_tests.yaml |
| bump_reset | 54 | commands/bump/bump_reset.gohtml (32) | bump_tests.yaml |
| batch_delrep | 76 | commands/members/batch_delrep.gohtml (79) | batch_delrep_tests.yaml |
| bootstrap | 78 | commands/plumbing/bootstrap.gohtml (121) | — |

**Bootstrap is the exception** (FUTURE "Known command bugs", unit-5a): `/staff bootstrap`
routes through Commands.bootstrap, which only hourly config_sync fills — on a brand-new
server the slash path can't reach bootstrap until the first hour tick, and trigger None
would leave no path at all. KEEP main 78 as `Command` trigger `bootstrap`: it is
Staff Utility (staff-only surface, nothing exposed), and the text path is the one thing
that works before config_sync lands. The six others switch to None.

Caller census (grep 2026-11-04): every reference outside staff_slash.gohtml and
config_sync is execCC-shaped (staff_roles_pick→staff_roles, inactivity_prune_pick→
inactivity, the pagers→channel_activity/rules) or a DB dict name. No command tells
members to type any handler's trigger, so no redirect line breaks.

Suite adaptation: the six text-path suites run their command as a top-level text
command today. After None, they run it the way the slash suites do (execCC via
command_map, as staff_slash_5b_tests.yaml already does end-to-end). Text-only assertions
(usage text, redirect lines) drop with the branch.

## 2. message_pointer (main 74)

/pointer (112) inlined message_pointer's logic (2.8k runes) and carries the
View-Channel guard; nothing execCCs 74 (census: config_sync and a comment in
pointer_slash only). After u5-pointer-3 passes: **disable panel 74** (not None — it has
no execCC caller). The file stays in commands/channels/: lotv 21 and rose 19 still run
the text twin, which still carries the View-Channel hole FUTURE documents. Move the file
to retired/ only when both fleet copies retire (§3).

## 3. lotv / The Rose text twins

The fleet policy (slash-fleet.md, FUTURE_IMPROVEMENTS §Interactive UX): the text twins
retire **after a live check on those servers**. No such check exists yet — the current
page has no lotv/Rose section. Deployed text twins today (panel.json 2026-11-04; both
servers already run the matching slash roots):

| file | lotv | rose |
|---|---|---|
| color/contrast | 18 | 17 |
| color/rand_color | 16 | 15 |
| general/timestamp | 13 | 12 |
| hebrew/atbash | 17 | 16 |
| hebrew/alefbet | 14 | 13 |
| hebrew/pyramid | 22 | 20 |
| hebrew/rand_hebrew | 39 | 22 |
| channels/message_pointer | 21 | 19 |

gematria (52/9/10) is already trigger None (the roots' renderer) — not a twin, stays.

Proposed check: add a **"Fleet servers (lotv + The Rose)"** section to the live-checks
page — one item per twin pair (run the text twin and the matching slash root's
subcommand, confirm parity) plus two smokes (config_sync's hourly id map present;
`/staff bootstrap` answers on Commands.bootstrap). Regenerate and install BOTH copies
(Charter: tailnet + claude.ai). After Lila's ticks: disable the twins' panel copies on
both servers (the hiatus 81/82 pattern — disabled, ids kept, nothing deleted without her
word), then move files to retired/ where no server runs them any more. Note the earlier
slash-fleet FUTURE line "lotv /color, then retire contrast/rand_color" is already
satisfied: color_slash is live on lotv 25 and rose 27.

## 4. The Rose leftover (Lila's call)

Unit-5 build note (slash-fleet.md): The Rose's `/staff` (33) and `/pointer` (34) were
created disabled; The Rose has only bootstrap of the seven handlers (rose 8), so 7 of 8
`/staff` subcommands would answer "isn't set up" if enabled. Options:

- **(a) keep both disabled** — status quo; /staff is dead weight on that server.
- **(b) enable /pointer only** — fully functional there (it needs no handler), and it is
  the fixed path (rose's message_pointer 19 has the View-Channel hole).
- **(c) enable both** — bootstrap subcommand works; the rest refuse ephemerally by design.

Lean (b): /pointer is Utility and useful; /staff without its handlers invites
"isn't set up" reports. Verify actual panel enablement state on the next deploy round's
dry run before acting — panel.json records ids, not enabled flags.

## 5. Work breakdown when the gates open

- **Unit A** (after the u5 section passes): six handlers — text-branch removal, suite
  adaptation, header `Trigger type:` → None, bootstrap exception kept. >5 lines →
  yagpdb-delegation (one worker; staff_slash itself does not change). Fresh-context
  review, `make ci`, then the deploy batch on Lila's go.
- **Unit B** (after u5-pointer-3): disable panel 74 — a panel edit only, no file change.
- **Unit C** (any time): the §3 live-check section (page regenerate + both installs),
  then the fleet-twin disable round on her go.
- Deploy mechanics: A+B ride one batch; trigger changes flow through the browser deploy
  from the headers; config_sync regenerates only if ids change (they don't here);
  `make changed-since-deploy` will list the six handler files.
