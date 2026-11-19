---
id: TASK-4
title: 'channel_activity browse polish: first non-empty bucket + tracking-since'
status: Deferred
assignee: []
created_date: '2026-11-19T21:55:00Z'
labels:
  - ux
dependencies: []
ordinal: 4000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred (disposition checked 11-19: not shipped).** Two polish picks found live 2026-09-27: open the browse view on the first non-empty bucket (not an empty Stale page), and show "tracking since <date>" so "never seen" reads as "not yet". Check: "tracking since" appears in neither commands/channels/channel_activity.gohtml nor channel_activity_pager.gohtml (grep 11-19). The old blocker (pager 9,913/10,000 chars) is gone — the minifier shipped 2026-10-02. Trigger: the next channel_activity change, or a live complaint.
<!-- SECTION:DESCRIPTION:END -->
