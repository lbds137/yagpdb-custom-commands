---
id: TASK-14
title: 'Multi-word /hebrew pyramid: rate-limited delayed runs drop promised embeds'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - commands
dependencies: []
ordinal: 14000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** A multi-word /hebrew pyramid answers "N words follows" then schedules N delayed runs, but YAGPDB rate-limits delayed runs per channel (customcommands/bot.go:48, burst 10, 0.1/s) and DROPS a refused run without retry (handle_timed.go:66-69): a second 10-word pyramid in one channel within ~100s promises embeds that never come. The text pyramid shares the limit but promised nothing. Trigger: Lila sees it live, or the next pyramid change (say "up to" in the reply, or build all pyramids in one gematria call). Found 2026-10-04.
<!-- SECTION:DESCRIPTION:END -->
