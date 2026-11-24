---
id: TASK-47
title: Rename /staff announce/prune/prune_date back under an inactivity name
status: To Do
assignee: []
created_date: '2026-11-24T12:54:00Z'
updated_date: '2026-11-24T14:30:00Z'
priority: medium
labels:
  - ux
dependencies: []
ordinal: 45000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Lila 11-23: "this is not a generic announcement... before the command renames the namespace was /inactivity. staff makes sense as a category though". YAGPDB slash commands have ONE subcommand level (SlashCommandSubcommand has Options only, customcommands.go:166-174), so /staff inactivity announce cannot nest. Name pick is hers (AUQ 11-24). Panel rows change; the router maps the new names to the inactivity handler (staff_slash.gohtml $handlers).
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Lila 2026-11-24 AUQ "inactivity_ prefix": /staff inactivity_announce (which: start/end/remind menu, TASK-46), /staff inactivity_prune [user], /staff inactivity_date date.
Priority medium (11-24): a user-visible fix Lila asked for, unblocked by TASK-46, but the current names still work, so nothing is broken meanwhile.
<!-- SECTION:NOTES:END -->
