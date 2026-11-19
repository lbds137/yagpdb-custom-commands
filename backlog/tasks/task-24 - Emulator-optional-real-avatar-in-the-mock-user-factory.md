---
id: TASK-24
title: 'Emulator: optional real avatar in the mock user factory'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - emulator
dependencies: []
ordinal: 24000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** The mock user factory hardcodes a no-avatar user, so avatar URL assertions only exercise the computed default-avatar path; a real cdn/avatars/<id>/<hash>.png URL is unreachable in tests (size passthrough pinned by avatar_menu_tests). Promote when a unit needs to assert a real avatar URL: give the factory an optional avatar field.
<!-- SECTION:DESCRIPTION:END -->
