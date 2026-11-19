---
id: TASK-34
title: 'Emulator: %T prints emulator type names, values not pointers'
status: Deferred
assignee: []
created_date: '2026-11-19T21:57:00Z'
labels:
  - emulator
dependencies: []
ordinal: 34000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** .User, .Member and .Message are values here where YAGPDB's are pointers: pointer-receiver methods on them are unreachable, and printf "%T" prints the emulator's type names (types.CtxMessage, types.Timestamp) instead of discordgo's. .Channel and .Guild are pointers already. No command calls such a method or compares %T names today (the %T comparisons in db, db_get_text and db_get_embed are against *templates.SDict and string only). Trigger: a command calls a pointer-receiver method on one of them or compares %T names.
<!-- SECTION:DESCRIPTION:END -->
