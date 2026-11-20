---
id: TASK-38
title: >-
  Forum first-message notification dead zone: documented, no upstream issue
  (ruled out 11-19)
status: Deferred
assignee: []
created_date: '2026-11-19T21:57:00Z'
updated_date: '2026-11-20T02:43:00Z'
labels:
  - watch
dependencies: []
priority: medium
ordinal: 60
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Owner call (upstream stance memory applies).** Production finding, smoke 2026-10-01/02: a forum post's FIRST message never notifies its mentions even with allowed_mentions transmitted — the payload provably rode along (restapi.go:2634-2637 nests MessageSend; message.go:340 marshals the field), the same bot/role/allowlist pings deliver via sendMessage and in-thread. Thread-start first message = notification dead zone; /prompt works around it (post bare, then mention as a second message). Candidate upstream issue for botlabs (runtime behavior, not a YAGPDB code bug); likely unfixable client-side. Before any submission: check the project's AI stance (researched 10-01: MIT, PRs to dev, nothing AI-hostile recorded); security-adjacent findings ask Lila first.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Needs Lila: file the forum first-message dead zone upstream with botlabs, or leave it documented? Recommendation: leave it (likely unfixable client-side; /prompt works around it).

Decision (Lila, 2026-11-19 21:43 EST, AskUserQuestion): leave documented - RULED OUT, no upstream issue to botlabs. The dead-zone evidence and the /prompt workaround stay recorded in this task; revisit only if the workaround breaks.
<!-- SECTION:NOTES:END -->
