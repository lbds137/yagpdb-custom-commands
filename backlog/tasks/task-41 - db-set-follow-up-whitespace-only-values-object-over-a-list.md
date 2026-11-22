---
id: TASK-41
title: '/db set follow-up: whitespace-only values; object over a list'
status: To Do
assignee: []
created_date: '2026-11-22T17:59:00Z'
updated_date: '2026-11-22T18:03:00Z'
labels:
  - commands
dependencies: []
priority: medium
ordinal: 39000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
From PR #32's passing re-review (2026-11-22). (1) Regression from the trim: $valueCheck runs on the raw value, so '/db set Key "   "' (or a whitespace execCC Value) passes 'You must provide a value!', trims to "" and is stored; add appends "". Before PR #32, set refused it (jsonToSdict "" threw). Fix: refuse a value that is empty after trimming, set and add. (2) Lila 2026-11-22 (AskUserQuestion): refuse a JSON object set over an existing LIST with the 'holds a list or a group' message (an object over a group still replaces it). Tests for both, red first.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
PR #32 final review (PASS, 2026-11-22) adds: (3) the [ refusal text points to /db add, which can't start a top-level list: reword to put the list inside a group, e.g. {"Tags": ["a"]}. (4) trimming stored plain strings (my widening beyond Lila's 'trim before the { check') breaks /db remove on items stored with spaces: trim only to detect/parse JSON, keep plain strings verbatim in set/add/remove. Decided engineering-only, back to her literal ruling.
<!-- SECTION:NOTES:END -->
