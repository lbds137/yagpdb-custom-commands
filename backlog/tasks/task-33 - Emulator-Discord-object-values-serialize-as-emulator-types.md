---
id: TASK-33
title: 'Emulator: Discord-object values serialize as emulator types'
status: Deferred
assignee: []
created_date: '2026-11-19T21:57:00Z'
labels:
  - emulator
dependencies: []
ordinal: 33000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** Values holding Discord objects (a member, a message, a cembed, a whole dbGet entry) serialize as the emulator's types, so stored sizes differ from YAGPDB's; a value whose overflow past 100000 bytes is only whitespace is stored whole where YAGPDB stores it cut (reading it back would fail). None of the 26 dbSet calls in everyone/ and staff/ stores a Discord object; the largest stored dict comes from a 7.4 KB source. Trigger: a command stores a Discord object or a value nears 100000 bytes; fix = store the msgpack bytes and decode with a copy of YAGPDB's newDecoder.
<!-- SECTION:DESCRIPTION:END -->
