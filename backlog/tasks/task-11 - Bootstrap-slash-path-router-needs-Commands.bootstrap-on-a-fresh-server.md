---
id: TASK-11
title: 'Bootstrap slash path: router needs Commands.bootstrap on a fresh server'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - commands
dependencies:
  - TASK-2
ordinal: 11000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred; resolve BEFORE the twin batch flips bootstrap to trigger None (TASK-2).** /staff bootstrap goes through the router, which needs Commands.bootstrap — only config_sync fills that dict (hourly, for panel.json servers). On a brand-new server the slash path cannot bootstrap; only the text `bootstrap` can. Fix shape: keep bootstrap's text trigger, or give the router a fallback. Found 2026-10-04 (unit 5a review).
<!-- SECTION:DESCRIPTION:END -->
