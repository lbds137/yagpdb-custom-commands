---
id: TASK-13
title: '/staff handlers: stale nonzero execCC id errors with no answer'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - commands
dependencies: []
ordinal: 13000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred residual.** A /staff handler whose execCC target id is NONZERO but stale/disabled still errors in the handler (a goroutine) with no answer to the invoker; wrapping its ~15 execCC sites was not worth it while config_sync rewrites the ids hourly. Trigger: a stale id seen live, or config_sync stops running on a server. Found 2026-10-04.
<!-- SECTION:DESCRIPTION:END -->
