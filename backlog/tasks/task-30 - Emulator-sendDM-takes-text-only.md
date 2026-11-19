---
id: TASK-30
title: 'Emulator: sendDM takes text only'
status: Deferred
assignee: []
created_date: '2026-11-19T21:57:00Z'
labels:
  - emulator
dependencies: []
ordinal: 30000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** sendDM turns any argument into text; YAGPDB's takes an embed, a list of embeds or a complexMessage (parseMessageInput, context_funcs.go:74-86) and sends nothing for a message with no content, embed, file or components. No command calls sendDM (git grep 2026-09-25; grep 11-19 still none). Trigger: a command sendDMs an embed or a complexMessage.
<!-- SECTION:DESCRIPTION:END -->
