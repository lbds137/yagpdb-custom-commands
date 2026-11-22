---
id: TASK-40
title: 'Free tier: /db and /cipher over the 10k cap'
status: To Do
assignee: []
created_date: '2026-11-22T17:51:00Z'
labels:
  - free-tier
dependencies: []
priority: low
ordinal: 38000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
On a free-tier server the deploy refuses commands whose minified dist/free copy is over 10,000 panel characters. Measured 2026-11-22: dist/free/db/db_slash.gohtml 10,405 (it went over with the PR #32 /db set guardrails, TASK-16; it was 9,996) and dist/free/cipher/cipher_slash.gohtml 11,828. Same class as TASK-8 (/hebrew). All three servers are premium today, so nothing live is affected. Promote when a free-tier server is added: split, move data to a DB dict, or trim.
<!-- SECTION:DESCRIPTION:END -->
