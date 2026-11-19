---
id: TASK-29
title: 'Emulator: cembed returns a map, not a discordgo MessageEmbed'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - emulator
dependencies: []
ordinal: 29000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** cembed returns the emulator's map (JSON keys: "title", "author") where YAGPDB's returns a *discordgo.MessageEmbed, so (cembed "title" "x").Title reads nothing here. A message read back holds discordgo-shaped embeds already. No command reads a cembed's fields directly (git grep; a cembed kept in a variable and read later was not traced). Trigger: a command reads one.
<!-- SECTION:DESCRIPTION:END -->
