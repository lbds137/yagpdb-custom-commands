---
id: TASK-21
title: 'Emulator: deferred stored-interaction response shape'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - emulator
dependencies: []
ordinal: 21000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** The delayed-interaction test shape (interaction: {type: slash, delayed: true, responded_to}) cannot express a DEFERRED stored interaction: YAGPDB sends such a run's plain output as an edit of the original response (context.go:605-607); the emulator models a followup. No command hits it (every root that schedules is defer mode None). Trigger: a deferred slash root that schedules runs. Found 2026-10-04 (unit 2 review).
<!-- SECTION:DESCRIPTION:END -->
