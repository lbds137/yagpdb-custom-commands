---
id: TASK-16
title: '/db set: accept plain strings (decided 11-19)'
status: Done
assignee: []
created_date: '2026-11-19T21:56:00Z'
updated_date: '2026-11-22T17:36:00Z'
labels:
  - commands
dependencies: []
priority: low
ordinal: 16000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred (her call; recommendation recorded).** `db set` (text and slash) accepts JSON OBJECT values only — the set path runs jsonToSdict on every string, so `db set Key hello` fails with "Invalid value provided!"; a plain string must be sent as `"hello"`, which jsonToSdict also refuses (objects only). Improvement: accept a plain string as-is when it does not start with `{`. Kept for parity in the slash port (tests pin it). Lila decides whether the convenience is worth the behavior change. Found 2026-09-30.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Needs Lila: should db set accept plain strings not starting with {? See the recommendation above.

Decision (Lila, 2026-11-19 21:43 EST, AskUserQuestion): accept plain strings - a value not starting with { stores as-is. Implementation task now; update the pinned parity tests with it.

Shipped 2026-11-22 on branch drain-decided-1122: set branch guards jsonToSdict on a leading {; pinned refusal test replaced; new tests for malformed { and a nested plain string. db_slash panel count 19,500 -> 19,621.
<!-- SECTION:NOTES:END -->
