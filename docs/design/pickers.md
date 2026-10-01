# Pickers: select menus for staff_roles, inactivity prune, role_ping

Design of record (2026-10-01). UX picks 2026-09-27 (Lila): "Role/user pickers for
staff_roles, inactivity (kick behind confirm), role_ping". Three panels spawned by the
existing text commands; three NEW Message Component handlers. No retirements — the text
paths stay (they are the spawners and keep the cases a select can't do).

## Mechanics (vendor-cited)

- Build: `cmenu "type" "user"|"role"|"string" "custom_id" … "min_values" … "max_values"
  … "placeholder" … ["default_values" …]` — CreateSelectMenu (common/templates/
  components.go:256-329) maps `user`/`role`/`mentionable`/`channel` to Discord's
  auto-populated selects (no option enumeration); string selects need 1-25 options with
  unique values. `default_values` entries are `sdict "id" … "type" "user"|"role"`
  (discordgo components.go:288-300, only for auto-populated types).
- Layout: a select always occupies its own action row; passing it in a flat
  `"components"` cslice is enough (distribute, components.go:1052-1054).
- Receive: Message Component trigger (same trigger type as buttons);
  `custom_id` matched against the panel's trigger regex; selections arrive as
  `.Data.Values` ([]string — snowflake STRINGS for user/role selects)
  (customcommands/handle_component.go:294-308).
- Role pings from interaction responses: TWO DIFFERENT PATHS (review finding
  2026-10-01, runtime-verified). The plain-output path (template text /
  Context.SendResponse METHOD) allowlists via `CurrentFrame.MentionRoles`
  (context_funcs.go:2377-2382 → context.go:593 → 663-672) — that's the text-path
  idiom. The `sendResponse` template FUNCTION instead uses only the complexMessage's
  own AllowedMentions (which CreateComplexMessage defaults to users-only,
  general.go:275-279; context_interactions.go:307-372), so a mention built with
  `mentionRoleID` RENDERS but notifies NOBODY. In an interaction response, pass
  `"allowed_mentions" (sdict "roles" (cslice $roleID))` explicitly (parseAllowedMentions,
  general.go:512-563; complexMessage key general.go:326-335). Pin with a
  `response_pings` assertion — content_contains alone cannot see the difference.
- Panel update: pager idiom — `updateMessage $msg` inside an interaction,
  `sendMessage nil $msg` otherwise (channel_activity_pager.gohtml:230-234). execCC
  into a component CC is fine from a TEXT command; `.ExecData` carries the handoff
  (pager lines 130-135). CAVEAT: `.Interaction` is nil in the child only when the
  PARENT run had no interaction — vendor propagates the parent's interaction into an
  immediate execCC child (tmplextensions.go:238-241), sharing RespondedTo state.
  Never execCC a picker from a component CC expecting a fresh frame.

## Emulator support (verified present — no gaps this unit)

`cmenu` (runtime/engine.go:135), "menus" key + row distribution (engine.go:712-838),
select rendering in snapshots (snapshot_test.go:261), select submit simulation
`interaction: { type: component, custom_id: …, values: […] }` (loader/testcase.go:208,
347; interaction_test.go:59-66).

## 1. staff_roles → Staff Roles panel (role select)

- Spawn: **bare `staff_roles`** (no role IDs) posts the panel instead of today's
  append-and-save [Staff-role] clobber (DECIDED Lila 2026-10-01: panel replaces the
  clobber; reverses the 2026-09-27 fix-spec line "keep today's behavior" — the
  reset-to-Staff-role semantic survives via the panel: pick only the Staff role, save.
  Write the reversal back into .claude/dispatch/staff-roles-fixes.md). Text path with
  IDs byte-identical.
- Panel: embed "Staff Roles Configuration" (current roles listed) + role select
  `sr:<spawner snowflake>` — min_values 1 (parity: text path requires ≥1), max_values
  25, placeholder "Pick the staff roles…", default_values = current Staff.Roles.
- Handler `^sr:\d+$` → commands/members/staff_roles_pick.gohtml, panel **97**,
  Staff Utility, Defer None.
- Gate: staff only (Roles dict "Staff"; non-staff → ephemeral ⚠️, dismiss idiom).
- Submit: `Staff.Roles` = `.Data.Values` (strings in, strings stored — parity with
  today's regex strings) → updateMessage (embed + refreshed default_values). The
  visible panel change is the confirmation; no extra ack.

## 2. role_ping → Role Ping panel (string select)

- Spawn: bare `role_ping` (today: parseArgs usage error) posts the panel. Text path
  (name/nil + optional message) byte-identical — the panel is the quick-ping surface
  and does not compose messages.
- Panel: embed "Role Ping" + string select `rp:<spawner snowflake>` — options = Roles
  dict keys (label = key, value = key; ≤25, today ~8), placeholder "Pick a role to
  ping…". Zero usable keys → no select; embed_exec error embed instead (a string select
  cannot have 0 options, components.go:309).
- Handler `^rp:\d+$` → commands/members/role_ping_pick.gohtml, panel **98**,
  Staff Utility, Defer None.
- Gate: staff only.
- Submit: public interaction response whose content is `(mentionRoleID (Roles-dict
  value))` — the ping is the display (mechanics above). Panel not edited; it stays for
  the next ping.

## 3. inactivity prune → Prune panel with kick confirm (user select)

- Spawn: `inactivity prune` with NO user param (today: "Invalid User Parameter" error)
  posts the panel.
- Panel: embed "Inactivity Prune" (state: prev/next dates) + user select
  `ip:<spawner snowflake>` — max_values 1, placeholder "Pick a member to prune…".
- Handler `^ip:\d+(:\w+)?$` → commands/members/inactivity_prune_pick.gohtml, panel
  **99**, Staff Utility, Defer None. Four modes:
  - select submit → verdict now (isActive/isInactive, text-path rules): inactive →
    confirm state (embed: member, avatar, "will be kicked"); active → verdict embed
    with the Kick button DISABLED + "not inactive" note.
  - Kick button `ip:<spawner>:kick` (red) → RE-CHECK at press time (stale select):
    member gone or no longer inactive → verdict embed; else `exec "kick"` (same reason
    string) → public result embed exactly like the text path's (Title, Reason field,
    thumbnail) → panel back to picker state.
  - Cancel button `ip:<spawner>:cancel` (secondary, no confirm) → picker state.
  - execCC mode (from the TEXT path, DECIDED Lila 2026-10-01: confirm gates both
    paths): text `inactivity prune @user` stops kicking directly and execCCs this
    handler with ExecData {user, spawner} to post the same confirm state. One confirm
    mechanism, no bypass.
- Gate: staff at every mode.
- Red = Kick only (member removed from guild = entity); Cancel secondary. Design
  language ✓.
- IMPLEMENTED DEVIATION (2026-10-01, driver-accepted): the Kick button recovers its
  target from the confirm embed's description (`reFind "\d{5,}"`), not from the
  custom_id — the two-segment trigger regex `^ip:\d+(:\w+)?$` was kept (a
  target-bearing custom_id would change the panel regex Lila creates). The embed is
  this handler's own output (closed-loop); degradation on a mismatch is the verdict
  state, never a wrong kick (the press-time re-check still gates).

## Panels for Lila (create DISABLED; enable after deploy; then Run-now config_sync)

| id | file | type | trigger | group | defer |
|----|------|------|---------|-------|-------|
| 97 | commands/members/staff_roles_pick.gohtml | Message Component | `^sr:\d+$` | Staff Utility | None |
| 98 | commands/members/role_ping_pick.gohtml | Message Component | `^rp:\d+$` | Staff Utility | None |
| 99 | commands/members/inactivity_prune_pick.gohtml | Message Component | `^ip:\d+(:\w+)?$` | Staff Utility | None |

Unit also edits deploy/panel.json (97/98/99), regenerates config_sync (`make
config-sync`), and adds the three files to the unmanaged-exclusion check flow as mapped.

## Test plan (per file; red-on-HEAD where behavior changes)

- staff_roles: panel spawn snapshot (select shape + default_values), submit saves
  Staff.Roles + updates panel, non-staff ephemeral ⚠️, text-path tests untouched;
  NEW: bare-run posts panel (asserts the clobber is gone — red on HEAD).
- role_ping: panel spawn snapshot (options from seeded Roles dict), select → public
  message content_contains `<@&ID>`, gate test, zero-keys branch (error embed, no
  select), bare-run posts panel.
- inactivity: panel spawn snapshot (user select), select → confirm embed (inactive:
  Kick enabled; active: Kick disabled), kick execs + result embed + picker restored,
  stale-select re-check (state changed between select and press), cancel restores,
  text-path prune now posts confirm instead of kicking (red on HEAD when Lila approves
  part 2).

## Scope guard

NOT in this unit: slash conversions (fleet pass, Next #3), minifier (#4), /edit polish,
text-twin retirements (none here — these files ARE the spawners).
