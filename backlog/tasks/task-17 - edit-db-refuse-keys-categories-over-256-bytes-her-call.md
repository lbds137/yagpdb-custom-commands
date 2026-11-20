---
id: TASK-17
title: '/edit + /db keys over 256 bytes: leave as is (ruled out 11-19)'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
updated_date: '2026-11-20T02:43:00Z'
labels:
  - commands
dependencies: []
ordinal: 17000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred (her call; recommendation: leave it).** YAGPDB cuts every DB key to 256 bytes (vendor limitString; the emulator models it), so a /edit category or /db key over 256 bytes stores under its first 256, and two such names sharing that prefix would share one entry. Consistent on read and write, reachable only by typing two 256-byte names that agree for 256 bytes. Fix shape: a refusal branch in /edit and /db. Trigger: her word, or any real >256-byte name. Found 2026-10-04 (A-F5 unit).
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Needs Lila: refuse >256-byte keys/categories, or leave as is? Recommendation: leave it.

Decision (Lila, 2026-11-19 21:43 EST, AskUserQuestion): leave as is - RULED OUT. Read and write stay consistent, and the collision needs two names agreeing for all 256 bytes; revisit only if a real over-length name turns up.
<!-- SECTION:NOTES:END -->
