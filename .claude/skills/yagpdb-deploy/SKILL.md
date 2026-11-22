---
name: yagpdb-deploy
description: Browser deploy of committed command bytes into the YAGPDB control panel via Claude-in-Chrome; manual paste is the fallback
---

# Browser deploy (Claude-in-Chrome)

YAGPDB has no API for custom commands, but the control panel is a plain form. This skill has
Claude paste each command's exact committed bytes into it from inside Lila's logged-in
browser tab, via the Claude-in-Chrome `javascript_tool` -- no command text passes through the
model. `deploy/deploy.js` fetches each file from `raw.githubusercontent.com` at the pinned
commit sha in the manifest (the repo is public, so that origin sends
`access-control-allow-origin: *`) and hashes it against the manifest before and after writing,
so a deploy can only ever write the exact bytes that were committed.

The same committed header is the one source of the command's panel structure, so the deploy
also applies it: trigger type (`Trigger type:`), trigger text (`Trigger:`), group (`Group:`),
interaction defer mode (`Defer mode:`, absent = None), the panel's cosmetic command name
(the file's basename without `.gohtml`, from the manifest path, not a header line; `create`
sets it too), and for a `Slash Command` the
description (`Slash description:`, required, 1-100 characters: YAGPDB refuses a new slash
command without one) and the
`Slash subcommand:` / `Slash option:` rows (replaced wholesale; choices, min/max and channel
types are posted empty), and for an `Hourly interval` / `Minute interval` the interval
(`Interval:`, required, within the panel's 1-744 hours / 5-44640 minutes: the unit the
panel's field shows; a minute interval that is whole hours is refused, since the panel
shows it as hourly;
the structure diff calls it `interval`). Everything else on the form (channels, roles,
case sensitivity, show errors, context channels, ...) stays as the live form has it. Enabling and
disabling are separate, explicit options. Nothing is ever deleted.

The manual paste (`make changed-since-deploy` / `make mark-deployed`, in this repo's
CLAUDE.md) stays as the fallback when Claude-in-Chrome isn't available. Its paste list also
skips `panel.json`'s `"unmanaged"` files, printing a `skipped (unmanaged, ...)` line for each
instead of listing them, so the same overwrite risk noted below can't reach the manual path.

## Steps

1. Deploy from merged `main` only: `git switch main && git pull --ff-only`, HEAD equal to
   `origin/main`. A branch commit hasn't passed review, and automerge's rebase rewrites its
   sha, so a `deployed` tag on it would never be on main. (The manifest itself only refuses
   a HEAD that isn't reachable from any `origin/*` branch: raw.githubusercontent.com URLs
   for an unpushed commit would 404.)
2. `make deploy-manifest SERVER=<main|lotv|rose>` and capture its one line of JSON as `<manifest>`.
   A refusal or a stderr warning means stop and report it, not push through. A command
   with no id for this server is simply left out (panel.json is the per-server map).
   The manifest also refuses a command over the server's tier cap, and a free-tier server
   deploys the command's `dist/free` copy (each entry's `source` says which file).
3. Load the Claude-in-Chrome tools (`ToolSearch` for `mcp__claude-in-chrome__*` if they're not
   already loaded).
4. Open a new tab on `https://yagpdb.xyz/manage/<guild>/customcommands/` (the guild id is in
   the manifest). Confirm it's Lila's logged-in session, not a login page.
5. Paste the full contents of `deploy/deploy.js` into `javascript_tool` to define
   `window.yagDeploy`.
6. Dry run: `await yagDeploy.run(<manifest>, {dryRun: true, enable: [...ids], disable: [...ids]})`
   (both lists optional). Read back only statuses, `enabled`, `enabledWanted` and
   `structure` -- never paste command code back into the conversation (the browser tool's
   content filter blocks some of it anyway, and there's no reason to route it through the
   model twice). Each `structure` diff is `{field: {live, header}}` (type, name, trigger,
   group, deferMode, interval, slashDescription, slash). Each result carries the live
   `enabled` and `enabledWanted` (true/false from the `enable`/`disable` lists, null =
   untouched): the planned enable/disable is `enabledWanted` where it differs from `enabled`.
   The dry run also builds each post body, so a type or group the panel's selects lack shows
   as `failed: header ...` there, not at the real run. A header the emulator rejects comes
   back `failed: header: ...` and posts nothing. An id in both lists, or not in the
   manifest, refuses the whole run (`failed: options`) before anything is fetched.
7. Check the dry run against the round, then run for real with the same options and
   `dryRun: false`. Deploy rounds are Claude's (Lila's ruling, 2026-11-22: no go-ahead
   needed), so this check is the only gate before a live write. The round's expected set
   is `make changed-since-deploy`, run on the step-1 `main` checkout (after a `create`, its
   ids-and-`config_sync` PR has merged by then, so `config_sync` is in the set; the tag
   moves only in step 10), plus any command this round created; it
   covers both servers, since every round deploys main and rose together. Dry-run BOTH
   servers before writing to either. Stop, write nothing, and report the dry runs to Lila
   when either shows any of:
   - a `would-update` on a command outside the expected set (live code or structure that
     drifted from the repo: someone edited the panel, the last round didn't land, or a
     `deploy.js` / header-grammar change reads unchanged headers differently). The first
     round under these rules may stop here on rose drift left by the old "deploy whatever
     would-update" practice; report it, and a one-off rose reconcile is her call;
   - a `structure` diff on a command outside the expected set;
   - a `structure` field on an expected command that this round's header diff
     (`git diff deployed -- <file>`, its header lines) didn't change: a panel hand-edit
     the post would silently revert (a command created this round is exempt);
   - a `slash` diff whose live rows carry `extra: true` on any command: that option has
     panel-only choices or min/max, and the post would clear them;
   - an `enabledWanted` that differs from `enabled`, other than enabling a command this
     round created (disabling a live command needs her yes, below);
   - any status other than `would-update` or `same` (`missing`, `skipped-multi`,
     `failed: ...`).
   Otherwise run it. lotv deploys and panel deletes stay hers. The round's final report
   to Lila carries each server's dry-run and live-run statuses, with every `updated`
   command's `structure` diff and any enable change: a report, not a gate.
8. Every command should come back `"updated"` or `"same"`. Anything else (`missing`,
   `skipped-multi`, or a `failed: ...` status) -- stop, report the statuses, and do not retry
   blindly; a retry without understanding the failure can compound it. A failed status
   ending `(panel: <message>)` carries the panel's own refusal text (it renders danger
   alerts through `showAlerts("<json>")`, which deploy.js parses). A `note` field on a
   result (an `.alert-danger`/`.alert.alert-error` seen on the page) is informational only:
   the panel shows site-wide notice banners too, so it rides along on an `"updated"` result
   as often as on a failed one. Success/failure is decided solely by the read-back: the code
   hash matches, the header's structure matches (`failed: read-back structure <fields>`
   otherwise), and `is_enabled` is what was asked (`failed: read-back enabled ...`).
9. Every round deploys to BOTH `main` and `rose` (Lila, 2026-10-04: The Rose, her private
    server, stays in sync with the latest commands; she uses gematria there heavily). Order:
    steps 2-6 for main and for rose, then step 7's check over both dry runs, then steps 7-8
    for main and for rose; rose is checked against the same expected set. If main's live
    run stops at step 8, rose is not written. A new command
    that should exist on both gets a `create` on each. lotv is deployed only when Lila asks.
10. Only once main AND rose both came back clean, `make mark-deployed`. If either stopped
    or failed, leave the tag where it is, so the next round's expected set still covers
    what didn't land.

## New commands, enabling, retiring

- **New command:** the id comes from creating it, so the order is: add the file with its
  header (type, trigger, group, rows), then in the panel tab
  `await yagDeploy.create(<guild>, [{path, group}], {dryRun: true})` (every item must come
  back `would-create`; anything else stops the round), then `dryRun: false` (every item
  `created-disabled`, else stop as below). It POSTs `commands/new` in the header's group, then immediately posts the
  new command's own form with `is_enabled` cleared and reads back that it is disabled
  (`created-disabled`). Then: write the returned ids into `deploy/panel.json` (`main` and
  `rose`; `lotv` only when Lila asks), `make config-sync`, and land that as its own PR
  (review + automerge, like any change). The new commands stay disabled meanwhile. Once it
  has merged, run the round from `main` (steps 1-10) with `enable: [<new ids>]`. For the ~1 s between the two
  POSTs the new command is live and enabled with YAGPDB's placeholder response and an empty
  text trigger (default type Command, customcommands.go:74), so it answers nothing in
  practice; a `failed: created #N ... still ENABLED` status means disable it in the panel at
  once, and `failed: no new command id ... MAY have been created` means check the panel
  before retrying. `create` refuses a command whose name (file basename) already exists on
  any list page (`failed: exists as #N`), so a repeat can't duplicate it.
  An interval command also needs its channel set in the panel (the deploy never touches
  context channels): without one YAGPDB never runs it, silently. Its interval comes from
  the header's `Interval:` on the run after `create` (create leaves it 0, which YAGPDB
  refuses for an hourly command).
- **Enable / disable:** only through the `enable` / `disable` options. Enabling a command
  the round created, as its header says, is part of the round (Lila's ruling). Disabling
  a live command (a retirement, below) still needs her explicit yes.
- **Retire:** `disable: [id]` in a run, then `git mv` the file to `retired/` and drop it from
  `deploy/panel.json`. Deleting the command in the panel is permanent and stays Lila's.

`deploy/panel.json`'s top-level `"unmanaged"` list names repo files (like `log_user`) that
mirror a live panel command without an id: the manifest and deploy tooling skip them entirely.

## Before retiring or deleting a command

Check the live panel for callers first: it can have commands the repo doesn't (test commands,
or an unmanaged file's own live copy holding data this repo's copy can't -- see `log_user`
above). Do a read-only pass over the panel tab, the same
tab procedure this skill already uses, searching live command code for the command's name and
for its `Commands`-dict key. Only once that comes up empty is it safe to retire or delete.

## Out of scope for this script (stay manual)

- Deleting a command in the panel (permanent): list it for Lila instead of acting on it.
- Anything the header doesn't express: channels, roles, case sensitivity, show
  errors, context channels, and an option's choices, min/max and channel types.

## Landmines

- The manifest's `raw` URLs point at a specific commit sha, not a branch -- if HEAD moves
  before you run the deploy, regenerate the manifest.
- The panel's update form carries many fields beyond the code (channels, roles,
  `role_trigger_mode`, `role_context_channel`, interval fields, `show_errors`, ... : the
  `CustomCommand` form struct, customcommands.go ~195-244), and the update handler saves
  whatever it receives. `deploy.js` therefore resubmits the page's own form
  (`new FormData(form)`) with only the header-managed fields and `responses` swapped, never
  a rebuilt one, so every field it doesn't manage survives untouched.
- The emulator's header validation (and so the JS port) rejects two shapes the panel stores
  differently, because a read-back could never match them: an optional option before a
  required one (the panel stores required first) and `*_menu` option types (shown back as
  the base type, without choices).
- The header grammar is parsed twice, in Go (the emulator) and in `deploy.js`; Go's
  `TestHeaderGolden` writes `deploy/testdata/headers.golden.json` and `make test-deploy`
  holds the JS port to it. After a header or grammar change, regenerate it with
  `cd tools/emulator && go test ./internal/runtime -run TestHeaderGolden -update`.
- Form names and values come from the vendored panel: `type` select values and the
  `interaction_defer_mode` radios (value 0-3 = None, Message Response, Ephemeral Message
  Response, Update Message Response) in `customcommands-editcmd.html`, the `slash_*` field
  names and option type keys in `customcommands.go` (`CustomCommand`,
  `SlashCommandSubcommandForm`, `SlashCommandOptionForm`). A checkbox (`required`,
  `slash_use_subcommands`, `is_enabled`) is absent from the post when off, `on` when on.
- A full-manifest run (~50 commands, 0.8s apart plus fetches) outlasts `javascript_tool`'s
  45s timeout, and the call errors though the run keeps going in the page. Start it as
  `yagDeploy.run(m, opts).then(r => { window.yagDry = r })`, then poll `window.yagDry` in a
  later call (a loop of 1s waits, under 45s). The real run can take just the dry run's
  `would-update` ids.
- Step 5 can load the committed script instead of pasting it: fetch `deploy/deploy.js`
  from raw.githubusercontent.com at the manifest's commit and `(0, eval)` it (the panel's
  page allowed it on 2026-10-04); check `typeof yagDeploy.run === "function"`.
- An expired panel login redirects the tab to Discord's "Authorize YAGPDB" page: that is
  an OAuth grant, so Lila clicks Authorize herself; then reload the panel URL.
- `deploy.js` never navigates, clicks, or triggers a dialog: it only fetches and posts forms.
