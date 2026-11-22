---
id: TASK-45
title: 'Deploy skill: PR #34 round-3 review lows'
status: Done
assignee: []
created_date: '2026-11-22T19:04:00Z'
updated_date: '2026-11-22T19:05:00Z'
labels:
  - deploy
dependencies: []
priority: medium
ordinal: 43000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
PR #34's passing claude-review (2026-11-22, 38a09ee) left five Lows on .claude/skills/yagpdb-deploy/SKILL.md: (1) step 1 must force-fetch the deployed tag and check it equals origin's (git fetch --force origin tag deployed; git rev-parse deployed vs git ls-remote origin refs/tags/deployed): a stale local tag widens the expected set, failing open; applied by hand in the 11-22 round. (2) define 'a command this round created' as ids added to panel.json since deployed, since create + its ids PR precede step 1. (3) enable ids are per server (panel.json main vs rose). (4) rewrap New-command lines over 100 chars; step 9 indent. (5) keep unrelated backlog tasks out of focused PRs (process note, no edit).
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Items 1-4 applied to SKILL.md (tag force-fetch + origin check in step 1; 'created this round' = id added to panel.json since deployed; per-server enable ids; rewrap/indent). Item 5 is a process note: no edit.
<!-- SECTION:NOTES:END -->
