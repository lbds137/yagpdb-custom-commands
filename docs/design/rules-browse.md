# Rules browse — spec

Status: drafted 2026-09-30, GLM session. Decisions of record: Lila 2026-09-27
(docs/FUTURE_IMPROVEMENTS.md "Interactive UX", audit pick 2): rules browse, same pager
pattern, jump-to-rule select, public rulebook posting kept. Member-visible (the Night
House /commands/ page plans to list it).

## Current behavior

- `rules` (Staff Utility, main 48): posts EVERY rule as packed embeds via embed_exec.
  Stays unchanged — the public rulebook posting.
- `rule` (Utility, main 1): one rule by number via embed_exec. Members' current entry.
- Rules live in the `Rules` dict, keys `Rule #N` (N ≥ 1, gaps allowed: rule_edit deletes
  by key; rules.gohtml walked `seq(start..end)` with HasKey checks — since 2026-10-04
  both readers collect the existing N ≥ 1 and `sort` them, as `seq` refuses > 10,000).

## Design

### New command: `commands/rules/rules_pager.gohtml`

- Trigger type `Message Component`, Trigger `^rules:` (a regex on the prefix-stripped
  custom ID, as the panel matches; the emulator does the same).
- Group: **Utility**, not Staff Utility — members click these buttons, and the panel
  group gates every run of the Component CC (the reason unhiatus sits in Utility).
  This is the one place the channel_activity_pager pattern MUST NOT be copied blindly
  (that one is Staff Utility on purpose).
- Stateless: every click recomputes from the `Rules` dict; all state lives in the custom
  ID, like channel_activity_pager.
- Custom IDs (all explicit and distinct — Discord refuses a message whose components
  share one, and the emulator enforces it too):
  - `rules:<page>:<opener>` — ◀ / ▶ buttons
  - `rules:noop` — the disabled "Page X/Y" indicator button
  - `rules:jump:<opener>` — the select menu; `.Values` holds the chosen rule number as a
    string
  - `rules:close:<opener>` — Dismiss, honored only for the opener
  - `<opener>` (added 2026-10-04, GLM audit A-F3) is the snowflake of the member who ran
    `rule browse`, carried through every re-render so paging by others can't take over
    Dismiss; old ids without it (`rules:<page>`, `rules:jump`) fall back to the clicker.
- Defer mode None (channel_activity_pager's rationale applies: an updateMessage under
  Update Message Response is refused, 40060).
- No .Interaction (entry via execCC) → sendMessage the page normally; with
  .Interaction → updateMessage in place.

### Page layout

One rule per page (rules are long; single-embed scope). Embed:
- Title: `Rule #N` — or `Rules` when the dict holds exactly one rule TOTAL (keep
  `rule`'s title rule, rules.gohtml:69-71).
- Description first line (summary-first, Tzurot 04-discord.md): `N rules — page X of Y`;
  then the rule text, cut to Discord's 4,096 description cap with `…` when over
  (embed_exec did this cutting for `rule`; the pager has no embed_exec, so it cuts —
  cut code points, not bytes, as the other commands do after the snapshot-audit fixes).
- Components, two rows:
  - row 1: the jump select (cmenu): options `Rule #1` … `Rule #N` in numeric order,
    value = N. Discord caps a select at 25 options; the server has 13. A 26th rule is
    the trigger to switch to range-grouped selects (filed, out of scope).
  - row 2: ◀ (`rules:<page-1>:<opener>`, disabled on page 1) · Page X/Y (`rules:noop`,
    disabled) · ▶ (`rules:<page+1>:<opener>`, disabled on last page) · Dismiss.
- Pages enumerate EXISTING keys sorted numerically (skipping deleted middles), NOT
  seq(1..max): the page count and the select options come from the real key list.
- An out-of-range or missing page (rule deleted between renders) clamps to the nearest
  real page; a dict with zero rules shows one page saying rules aren't configured yet.
- Color: Global "Embed Color", like every view.

### Entry point: `rule browse`

- `rule` (Utility) accepts an optional literal `browse` as its argument: `rule browse`
  opens page 1 (the page of the LOWEST existing rule). `rule <number>` keeps working
  exactly as today. Bare `rule` keeps today's usage error. (Opening browse on bare
  `rule` was considered and declined: it changes what an existing typo does.)
- Path: `rule` execCCs the pager with ExecData `{Page: 1}` (or the jump target) instead
  of embed_exec on the browse path — 1 execCC on the free tier either way. The pager's
  own output path (sendMessage with components) replaces embed_exec here; embed_exec
  can't carry components.
- `deleteTrigger` still runs on `rule`'s browse path (the trigger message goes away; the
  posted browse message stays, as today's `rule` reply stays).

## Panel / deploy

- New panel command, created DISABLED by Lila, deploy, then she enables (the 2026-09-27
  `.*` placeholder lesson — the pager's trigger `^rules:` is message-component, so the
  enable is safe, but the DISABLED-first order stays).
- panel.json gains `commands/rules/rules_pager.gohtml` (main id assigned at creation) →
  `make config-sync` regenerates config_sync → CI --check must pass before commit.
- Utility group → member command: message the Night House site session at deploy time
  (old → new invocation: `/rule browse`; buttons member-clickable) for /commands/.

## Budgets

Free tier: 1 execCC on the entry path; DB interactions per run ≤ 3 (Rules, Global on
the pager; Global + Rules + Commands on `rule`'s normal path — unchanged); the pager
landed at 6,620 chars (review noted the spec's ≤6,000 soft target missed by ~10%;
accepted as is — the binding cap is the 10,000 free-tier limit, 3,380 headroom, and the
bytes are header/comments. channel_activity_pager is 9,913/10,000 with CSV + buckets;
this one has neither). Premium server unaffected.

## Tests (emulator; interactions units 1–4 all shipped)

1. Entry: `rule browse` execCCs the pager, page 1 = lowest rule; trigger deleted;
   snapshot pins the browse embed + components.
2. Click ▶/◀: paging across multiple rules, disabled state at both ends, page indicator.
3. Jump select: `.Values` → that rule's page (assert via updateMessage/edit or a second
   click's render).
4. Gap handling: dict with Rule #1, #3 (no #2) → 2 pages, select offers 2 options.
5. Over-4096 rule cut with `…`, cut on code points not bytes.
6. Zero rules → the not-configured page.
7. One-rule dict → title "Rules".
8. Out-of-range click (`rules:99`) clamps to the last page.
9. Duplicate custom IDs refused (the emulator already errors; the suite must not trip it).

## Out of scope (filed, with triggers)

- A generic shared `^pg:` pager handler (audit's shared-block idea): a THIRD browse view
  is the trigger; two exist after this unit.
- Bare `rule` opening browse: declined for now (behavior change).
- >25-rule select grouping: trigger is the 26th rule.
- Slash conversion of `rule`/`rules`: rides the later slash-first pass (Lila's order).

## Open questions for Lila (ride the round report; do not block)

- Entry phrase `rule browse` vs another word.
- Jump select labels: `Rule #N` vs a preview of the rule's first words (values stay N).
