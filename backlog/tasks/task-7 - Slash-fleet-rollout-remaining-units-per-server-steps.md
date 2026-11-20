---
id: TASK-7
title: 'Slash-fleet rollout: remaining units + per-server steps'
status: In Progress
assignee: []
created_date: '2026-11-19T21:55:00Z'
updated_date: '2026-11-20T02:23:00Z'
labels:
  - slash-fleet
dependencies: []
priority: medium
ordinal: 20
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**In progress since 2026-10-04; unit 1 (/color) live on main, contrasts retired.** Design: docs/design/slash-fleet.md. Remaining: group related commands into roots (one level of subcommands, handle_slashcommand.go:125-140; a root holds the logic or execCC-dispatches), free-tier top-10 picks (10 free / 50 premium slash CCs), per-server steps: give lotv /color (color_slash 8.7k runes, under the free cap; deploy embed_exec's Respond/Embeds version first), then contrast/rand_color retire there fully (lotv 18, 16 stay only for lotv). No deployed server is free today (main, The Rose, lotv all premium per the panel, checked 2026-10-04).
<!-- SECTION:DESCRIPTION:END -->
