---
id: TASK-32
title: 'Emulator: human-speech ping gate in bot-created threads'
status: Deferred
assignee: []
created_date: '2026-11-19T21:57:00Z'
labels:
  - emulator
dependencies: []
ordinal: 32000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** Discord's human-speech gate: a bot mention in a bot-created thread does not notify until a human has spoken there (probed 2026-10-02: /role_ping silent in a fresh thread; after the owner typed one message, the same ping reached her alt). No command sends mentions into fresh threads any more (prompt_post pings the run channel). Promote when a command must mention inside a bot-created thread: model the gate on the mocks (a human_spoke flag on created threads, thread-message pings suppressed until it is set) and a fixture knob to set it.
<!-- SECTION:DESCRIPTION:END -->
