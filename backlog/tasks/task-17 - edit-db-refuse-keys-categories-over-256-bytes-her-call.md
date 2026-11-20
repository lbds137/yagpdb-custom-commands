---
id: TASK-17
title: '/edit + /db: refuse keys/categories over 256 bytes (her call)'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
updated_date: '2026-11-20T02:23:00Z'
labels:
  - commands
  - needs-lila
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
<!-- SECTION:NOTES:END -->
