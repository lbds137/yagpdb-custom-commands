---
id: TASK-12
title: 'message_pointer: private-thread membership residual'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - commands
dependencies: []
ordinal: 12000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred residual.** /pointer refuses without View Channel on the target channel (a thread: its parent, as YAGPDB computes it), but a PRIVATE thread needs membership, not the parent's View Channel: a member who sees the parent can still point into a private thread they are not in, and its name shows. Trigger: if private threads carry anything sensitive on a server (check with Lila), add a thread-membership check — vendor has no template function for it today; verify first. Found 2026-10-04.
<!-- SECTION:DESCRIPTION:END -->
