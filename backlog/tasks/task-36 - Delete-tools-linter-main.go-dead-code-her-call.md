---
id: TASK-36
title: Delete tools/linter/main.go dead code (her call)
status: Deferred
assignee: []
created_date: '2026-11-19T21:57:00Z'
updated_date: '2026-11-20T02:23:00Z'
labels:
  - tools
  - needs-lila
dependencies: []
ordinal: 36000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred — her call.** tools/linter/main.go does not build (unused vars fix/hasGlobalDict/hasCommandsDict), is not gofmt-clean, and nothing references it — dead code. The active linter is tools/linter/yagpdb_lint.py, wired via scripts/lint-all.sh and make lint. Delete or revive: her call. Found 2026-09-27.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Needs Lila: delete tools/linter/main.go or revive it? Recommendation: delete (dead, nothing references it; yagpdb_lint.py is the active linter).
<!-- SECTION:NOTES:END -->
