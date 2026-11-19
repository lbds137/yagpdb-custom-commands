---
id: TASK-37
title: Pin the bare redirect-line outputs (22 commands)
status: Deferred
assignee: []
created_date: '2026-11-19T21:57:00Z'
labels:
  - tests
  - tools
dependencies: []
ordinal: 37000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** The bare redirect lines ("Use /x as a slash command ...", "Reached by ...") are unpinned: sweep 2026-10-20 found 43 of 54 non-Regex commands exit 0 with non-empty bare output; 22 are redirects, 1 is "### Missing Argument" (db_get_text), 19 warnings/embed dicts — only gematria's Usage line is pinned (scripts/test-all-templates.sh + scripts/bare-usage/). Shape: same file-per-command check keyed on the whole output, no Usage: prefix needed. Also unchecked: a pin whose command is deleted/renamed/Regex (the script walks commands/ and skips Regex before pin checks). Trigger: her go, or the next unit editing one of those messages.
<!-- SECTION:DESCRIPTION:END -->
