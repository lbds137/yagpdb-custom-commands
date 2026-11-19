---
id: TASK-15
title: 'avatar_viewer embed color: target role color never passed (her call)'
status: To Do
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - commands
  - needs-lila
dependencies: []
ordinal: 15000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Owner call pending (user-visible color).** avatar_viewer.gohtml's role-color loop (~lines 170-178) computes $color from the TARGET member's top colored role but never passes it to embed_exec, so the embed always takes the INVOKER's color (embed_exec derives it from AuthorID). Options: pass "Color" $color (avatar embeds adopt the viewed person's color), or delete the loop (invoker-colored, today's live behavior; the context-menu twin avatar_menu shipped without the loop). Lila's call — found 2026-10-01 grounding the context-menu unit.
<!-- SECTION:DESCRIPTION:END -->
