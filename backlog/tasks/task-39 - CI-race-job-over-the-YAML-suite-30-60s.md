---
id: TASK-39
title: 'CI: -race job over the YAML suite, non-blocking (approved 11-19)'
status: Done
assignee: []
created_date: '2026-11-19T21:57:00Z'
updated_date: '2026-11-22T17:34:00Z'
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

Shipped 2026-11-22 on branch drain-decided-1122: make test-race + a 'race' job in test.yml. Non-blocking = the step never fails the job (automerge refuses on any non-success check run); a race or failure shows as a ::warning annotation and in the job summary. actionlint clean; the -race run itself is first exercised by this PR's CI.
<!-- SECTION:NOTES:END -->
