---
id: TASK-39
title: 'CI: -race job over the YAML suite (+30-60s)'
status: To Do
assignee: []
created_date: '2026-11-19T21:57:00Z'
updated_date: '2026-11-20T02:23:00Z'
labels:
  - ci
  - needs-lila
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
<!-- SECTION:NOTES:END -->
