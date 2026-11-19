---
id: TASK-9
title: 'Free tier: bootstrap dies at the 11th DB call (18 calls)'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - free-tier
  - commands
dependencies: []
ordinal: 9000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** bootstrap makes 18 DB calls (9 dbGet + 9 dbSet); on a free server (10 DB interactions/run) it dies at the 11th, the Rules dbSet, and never writes Roles, Channels, Admin, Knowledge, Directory, Inactivity Prune or Staff. Repro: yagtest run -no-premium -strict -db tools/emulator/testdata/initial_db.json commands/plumbing/bootstrap.gohtml. Fix before a free server bootstraps: split it in two runs, or set only the dicts a fresh server lacks. Trigger: a free server bootstraps.
<!-- SECTION:DESCRIPTION:END -->
