---
id: TASK-25
title: 'Emulator: Discord error body fidelity (50035 fields, error ordering)'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - emulator
dependencies: []
ordinal: 25000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** Error bodies are written as {"message": "...", "code": N} — the spacing is Discord's usual, not captured from a live response, and a real 50035 Invalid Form Body also lists the fields at fault. A catch comparing the whole text could differ; one matching the code or message will not. Also unprobed: which error Discord gives first when a call has two problems (the emulator checks a reaction's emoji before its message, an edit's target before its form body). Trigger: capture a real response, or a command's catch telling those errors apart.
<!-- SECTION:DESCRIPTION:END -->
