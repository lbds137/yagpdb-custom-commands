---
id: TASK-31
title: 'Emulator: getThread / editChannelTopic / thread metadata gaps'
status: Deferred
assignee: []
created_date: '2026-11-19T21:57:00Z'
labels:
  - emulator
dependencies: []
ordinal: 31000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** Thread gaps in the emulator: getThread is not implemented at all (no emulator function calls it; its live-Discord-API fallback for an undeclared thread is unmodelled); editChannelTopic + ChannelArgNoDMNoThread missing (only vendor editChannelTopic uses that arg path); a declared thread entry has the same shape as any channel and lacks ThreadMetadata/message-count/parent-forum-tag fields, and .Guild.Channels holds no thread state (reading a thread's own metadata errors where YAGPDB gives the value). Threads resolve by ID/name correctly since 2026-09-27 (channels_test.go). Trigger: a command calls getThread/editChannelTopic, or reads a thread's own metadata.
<!-- SECTION:DESCRIPTION:END -->
