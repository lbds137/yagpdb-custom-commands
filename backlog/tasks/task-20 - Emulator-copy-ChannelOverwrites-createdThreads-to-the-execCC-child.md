---
id: TASK-20
title: 'Emulator: copy ChannelOverwrites + createdThreads to the execCC child'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - emulator
dependencies: []
ordinal: 20000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** The execCC child context copies Channels, Threads and ThreadOrder (shipped 2026-10-20, TestExecCCChildThreads), but NOT ChannelOverwrites (not pinned by any test) and not createdThreads — a child creating a forum post after the caller created threads reuses IDs and overwrites the caller's Threads entry. Trigger: the next unit where a child creates a thread or forum post. Found by code reading 2026-10-04.
<!-- SECTION:DESCRIPTION:END -->
