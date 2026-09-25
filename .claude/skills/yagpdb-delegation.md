# Delegation in this repo

This repo uses the harness `delegation` skill (adopted 2026-09-25, Lila's yes): the driver
grounds and specs each unit, a `harness:implementer` worker edits in its own worktree, and
the driver reads the full diff, transfers it, runs the gates and commits. This file fills
the skill's PROJECT slots; the skill has the spec template and the transfer steps.

## Gates (whole repo, from the Makefile and .github/workflows/test.yml)

- `make ci >/dev/null 2>&1; echo "make ci exit $?"`: Go vet and tests, the YAML template
  tests (with the database schema and stale-snapshot check), the no-argument smoke run of
  every command, the linter, and `gofmt -l tools/emulator`. Judge it by the exit code
  only: a grepped summary hides gofmt failures below green totals.
- Focused runs while working: `cd tools/emulator && go test ./internal/<pkg>/ -run <Test>`,
  `./bin/yagtest test -schema db_schema.yaml tools/emulator/testdata/<file>.yaml`.

## Step 0 (a bare worktree)

Nothing to install: the Go module cache is shared, and `make ci` builds `bin/yagtest`
itself. YAGPDB's source (`vendor/yagpdb/`) is gitignored, so it is absent from a worktree:
read it at `/home/deck/Projects/yagpdb-custom-commands/vendor/yagpdb/`.

## Ceilings

- A command file must stay under YAGPDB's limit of 20,000 characters (premium; Lila's
  servers are premium). Report the size of any `.gohtml` command the unit grows.
- Discord: message content 2,000; embed description 4,096, field value 1,024, whole
  embed 6,000 (docs/API_REFERENCE.md has the sourced table).
- Lines under ~100 characters where possible.

## Landmines

- Fidelity comes from YAGPDB's code: copy the vendor function, and build test fixtures
  (embeds, messages, errors) from what YAGPDB's code produces, never invented shapes.
- A changed command file (`utility/`, `staff_utility/`) needs Lila to paste it into YAGPDB
  by hand; the driver reports the batch at the end of a round. A command fix gets a test
  that fails on the old command (red on HEAD). `retired/` isn't deployed, so its files
  never need a paste.
- A `-}}` alone on its own line doesn't parse in YAGPDB; put it on the previous line.
- `make ci` fails on stale snapshot entries: after removing or renaming a snapshotted test,
  run `make prune-snapshots`; after an intended output change, `make update-snapshots`,
  then read the snapshot diff.
- The Bash tool's `grep` is ugrep (`$` in a pattern is an anchor, counts can mislead):
  use `/usr/bin/grep` or `git grep` for anything a claim rests on.
- Interpreter rewrites of files (python heredocs) are blocked: edit with the Edit tool.
- Never `rm -r`/`rm -rf`, even a scratch directory: make a new directory each time.
- A typed nil in an `interface{}` is not `== nil` (template data and function returns).
- Mutation checks: confirm the suite is green first, and check each mutant compiles
  (a build failure isn't a kill).

## Local-only

Nothing needs secrets or live services. Pastes into YAGPDB and `make mark-deployed` are
Lila's. Pushing runs CI only and is approved; the driver commits and pushes.

## Dispatch folder

Spec record copies: `.claude/dispatch/` (excluded in `.git/info/exclude`, as is
`.claude/worktrees/`). Workers get the spec inline in the Agent prompt.
