---
id: TASK-15
title: 'avatar_viewer embed color: pass the target role color (decided 11-19)'
status: To Do
assignee: []
created_date: '2026-11-19T21:56:00Z'
updated_date: '2026-11-20T02:43:00Z'
labels:
  - commands
dependencies: []
priority: medium
ordinal: 40
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Owner call pending (user-visible color).** avatar_viewer.gohtml's role-color loop (~lines 170-178) computes $color from the TARGET member's top colored role but never passes it to embed_exec, so the embed always takes the INVOKER's color (embed_exec derives it from AuthorID). Options: pass "Color" $color (avatar embeds adopt the viewed person's color), or delete the loop (invoker-colored, today's live behavior; the context-menu twin avatar_menu shipped without the loop). Lila's call — found 2026-10-01 grounding the context-menu unit.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Needs Lila: should the avatar embed take the viewed person's role color (pass Color $color) or stay invoker-colored (delete the dead loop)? Recommendation: pass the target's color.

Decision (Lila, 2026-11-19 21:43 EST, AskUserQuestion): pass the target color - hand the computed target role color to embed_exec as its Color argument so the avatar embed adopts the color of the viewed member. Implementation task now.
<!-- SECTION:NOTES:END -->
