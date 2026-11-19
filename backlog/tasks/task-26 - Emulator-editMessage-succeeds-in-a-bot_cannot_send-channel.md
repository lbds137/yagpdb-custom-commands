---
id: TASK-26
title: 'Emulator: editMessage succeeds in a bot_cannot_send channel'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - emulator
dependencies: []
ordinal: 26000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** A bot_cannot_send channel refuses sends only: editMessage there still succeeds, though YAGPDB's ChannelMessageEditComplex would error too (context_funcs.go tmplEditMessage). Deferred 2026-09-27. Trigger: a command edits a message in a channel it did not just post to (channel_link only edits messages it just posted).
<!-- SECTION:DESCRIPTION:END -->
