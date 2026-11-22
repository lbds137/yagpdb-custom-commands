---
id: TASK-42
title: 'Race CI job: consider making it blocking'
status: Deferred
assignee: []
created_date: '2026-11-22T17:59:00Z'
labels:
  - ci
dependencies: []
priority: low
ordinal: 40000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
PR #32's re-review suggests making the race job blocking once it has run green a while, since a warning-only job can be ignored. Lila decided non-blocking on 11-19 (automerge refuses red checks), so this is her call. Trigger: raise it when the job has a record (e.g. a few weeks of green runs) or the first time a real race warning is missed.
<!-- SECTION:DESCRIPTION:END -->
