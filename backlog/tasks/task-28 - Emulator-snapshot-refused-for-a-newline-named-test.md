---
id: TASK-28
title: 'Emulator: snapshot refused for a newline-named test'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - emulator
dependencies: []
ordinal: 28000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** A test whose name starts with a newline cannot be snapshotted (yaml.v3 cannot write such a map key); the write is refused with an error, so nothing corrupts. Left as is 2026-09-25. Trigger: a test needs a leading-newline name.
<!-- SECTION:DESCRIPTION:END -->
