# Browser deploy (Claude-in-Chrome)

YAGPDB has no API for custom commands, but the control panel is a plain form. This skill has
Claude paste each command's exact committed bytes into it from inside Lila's logged-in
browser tab, via the Claude-in-Chrome `javascript_tool` -- no command text passes through the
model. `deploy/deploy.js` fetches each file from `raw.githubusercontent.com` at the pinned
commit sha in the manifest (the repo is public, so that origin sends
`access-control-allow-origin: *`) and hashes it against the manifest before and after writing,
so a deploy can only ever write the exact bytes that were committed.

The manual paste (`make changed-since-deploy` / `make mark-deployed`, in this repo's
CLAUDE.md) stays as the fallback when Claude-in-Chrome isn't available.

## Steps

1. Push the branch: the manifest refuses if HEAD isn't reachable from any `origin/*` branch
   (raw.githubusercontent.com URLs for an unpushed commit would 404).
2. `make deploy-manifest SERVER=<main|lotv>` and capture its one line of JSON as `<manifest>`.
   A refusal or a stderr warning (a file with no id for this server) means stop and report it,
   not push through.
3. Load the Claude-in-Chrome tools (`ToolSearch` for `mcp__claude-in-chrome__*` if they're not
   already loaded).
4. Open a new tab on `https://yagpdb.xyz/manage/<guild>/customcommands/` (the guild id is in
   the manifest). Confirm it's Lila's logged-in session, not a login page.
5. Paste the full contents of `deploy/deploy.js` into `javascript_tool` to define
   `window.yagDeploy`.
6. Dry run: `await yagDeploy.run(<manifest>, {dryRun: true})`. Report only statuses and drift
   to Lila -- never paste command code back into the conversation (the browser tool's content
   filter blocks some of it anyway, and there's no reason to route it through the model twice).
   Show her the `would-update` list and any `drift` entries (header vs. live type/trigger --
   informational only; this script never changes a trigger or type).
7. On her go-ahead, run for real: `await yagDeploy.run(<manifest>, {dryRun: false})`.
8. Every command should come back `"updated"` or `"same"`. Anything else (`missing`,
   `skipped-multi`, or a `failed: ...` status) -- stop, report the statuses, and do not retry
   blindly; a retry without understanding the failure can compound it. A `note` field on a
   result (an `.alert-danger`/`.alert.alert-error` seen on the page) is informational only:
   the panel shows site-wide notice banners too, so it rides along on an `"updated"` result
   as often as on a failed one. Success/failure is decided solely by the read-back hash.
9. If `SERVER=main` and everything came back clean, `make mark-deployed`.

## Before retiring or deleting a command

Check the live panel for callers first: it can have commands the repo doesn't (panel-only
commands like `log_user`, or test commands). Do a read-only pass over the panel tab, the same
tab procedure this skill already uses, searching live command code for the command's name and
for its `Commands`-dict key. Only once that comes up empty is it safe to retire or delete.

## Out of scope for this script (stay manual)

- Deleting a command in the panel, or creating a new one -- list these for Lila instead of
  acting on them. Once she creates a new command in the panel, add its id to
  `deploy/panel.json` (both `main` and, if it exists there, `lotv`) so the next run picks
  it up.
- Changing a live command's trigger type or trigger text -- `drift` in the results is a report,
  never an instruction to the script to fix it.

## Landmines

- The manifest's `raw` URLs point at a specific commit sha, not a branch -- if HEAD moves
  before you run the deploy, regenerate the manifest.
- The panel's edit form carries fields YAGPDB's own Go struct doesn't know about
  (`role_trigger_mode`, `role_context_channel`, `slash_command_description`,
  `slash_use_subcommands`, ...). `deploy.js` always resubmits the page's own form with just
  `responses` swapped in, never a rebuilt one, so those fields survive untouched.
- `deploy.js` never navigates, clicks, or triggers a dialog -- if a command needs a structural
  change (new argument, renamed trigger), that's still a manual panel edit.
