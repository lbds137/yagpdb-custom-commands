---
id: TASK-22
title: 'Emulator: UTF-16 code-unit counting for embed limits'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - emulator
dependencies: []
ordinal: 22000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred, unverified.** embed_exec cuts titles/descriptions by code point and the emulator's limit checks count code points too, but Discord may count characters outside the BMP (many emoji) as two UTF-16 units: an emoji-heavy title/description near the limit would still be rejected live and invisible here. Trigger: a live rejection of an emoji-heavy embed, or a Discord doc/source that settles how it counts. (PR 7 review suggestion.)
<!-- SECTION:DESCRIPTION:END -->
