---
id: TASK-39
title: 'CI: -race job over the YAML suite, non-blocking (approved 11-19)'
status: To Do
assignee: []
created_date: '2026-11-19T21:57:00Z'
updated_date: '2026-11-20T02:43:00Z'
labels:
  - ci
dependencies: []
priority: medium
ordinal: 50
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Parked for Lila (CI budget call).** No standing race gate anywhere (no -race in Makefile/CI; host gcc lacks glibc headers, so -race runs only in the tools distrobox). The RunTests pool's standing protection is the unit-level race run, not CI. A CI job with a -race-built yagtest over the YAML suite costs about +30-60s. Post-merge full CI=true go test -race ./... was green 11-11 (deck-job u45race, exit 0: loader 14.3s, runtime 29.3s). Found 11-11 (pool review).
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Needs Lila: is +30-60s of CI worth a standing -race gate over the YAML suite? Recommendation: yes, as a non-blocking parallel job.

Decision (Lila, 2026-11-19 21:43 EST, AskUserQuestion): yes - add the race job as a non-blocking parallel CI job at about +30-60s. Implementation task now.
<!-- SECTION:NOTES:END -->
