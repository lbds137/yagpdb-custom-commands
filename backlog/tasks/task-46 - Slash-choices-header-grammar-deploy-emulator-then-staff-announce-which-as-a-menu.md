---
id: TASK-46
title: >-
  Slash choices: header grammar + deploy + emulator, then /staff announce which
  as a menu
status: Done
assignee: []
created_date: '2026-11-24T12:54:00Z'
updated_date: '2026-11-24T14:30:00Z'
labels:
  - ux
  - deploy
dependencies: []
ordinal: 44000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Lila 11-23 (screenshot of /staff announce which:start): constrained options need a choices list, not a typed word. Unblocks staff-lens F25 (docs/design/staff-lens-audit.md), which said: extend the header grammar, deploy.js, the Go emulator and its golden together. Vendor: SlashCommandOption.Choices []string (customcommands.go:157), max 25 choices of 100 chars (:189-190); a *_menu key is the base type with choices (:263-324). Sweep 11-24 of every Slash option line (git grep): announce.which (start/end/remind) is the only constrained free-text option; the rest are free text, numbers, users, roles or channels. Rows change on deploy.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Done 2026-11-24: PR #40 merged (8d027da). Header line `Slash choices: `[sub.]name a | b | c`` for *_menu options in the emulator, deploy.js and the shared golden; the dry run still stops on panel-only choices. The /staff announce which menu itself ships with the rename in TASK-47.
<!-- SECTION:NOTES:END -->
