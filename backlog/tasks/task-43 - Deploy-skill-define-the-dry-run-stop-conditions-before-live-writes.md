---
id: TASK-43
title: 'Deploy skill: define the dry-run stop conditions before live writes'
status: Done
assignee: []
created_date: '2026-11-22T18:19:00Z'
updated_date: '2026-11-22T18:51:00Z'
labels:
  - deploy
dependencies: []
priority: high
ordinal: 41000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
From PR #33's passing review (2026-11-22). Deploy rounds are now Claude's (Lila's ruling), so the dry run is the only gate before a live write. Do this BEFORE the first Claude-run round: (1) step 7's 'stop when the dry run shows anything unexpected' gets explicit conditions: a would-update on a file unchanged since the deployed tag; a structure diff on a command the round didn't touch; any enabledWanted change other than enabling commands this round created; any failed/missing/skipped-multi result. (2) Step 6's 'report statuses to Lila / show her the would-update list' becomes the round's final report, not a gate. (3) CLAUDE.md's '(owner ruling)' gets its date, 2026-11-22, and the duplicated fallback sentence goes.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Shipped in PR #34: SKILL.md step 7 lists five stop conditions (the four in the description plus a slash diff with live extra: true, i.e. panel-only choices/min/max the post would clear) against the expected set: make changed-since-deploy run after the round's last commit (so a create's config_sync regen is in it) + this round's creations, both servers, both dry-run before any write. Step 6 is read-back only; the report to Lila is the round's final report. mark-deployed moved to step 10, after main AND rose are clean (Gemini review). create gates would-create / created-disabled explicit. CLAUDE.md ruling dated 2026-11-22, duplicate fallback sentence removed.
<!-- SECTION:NOTES:END -->
