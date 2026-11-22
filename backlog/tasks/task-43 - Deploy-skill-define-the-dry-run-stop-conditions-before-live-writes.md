---
id: TASK-43
title: 'Deploy skill: define the dry-run stop conditions before live writes'
status: Done
assignee: []
created_date: '2026-11-22T18:19:00Z'
updated_date: '2026-11-22T18:40:00Z'
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
Shipped in the TASK-43 PR: SKILL.md step 7 lists the four stop conditions against the expected set (make changed-since-deploy + this round's creations, both servers); step 6 is read-back only and the Lila report moved to the end of step 7 as the round's final report; step 10 rose checks the same set; CLAUDE.md ruling dated 2026-11-22, duplicate fallback sentence removed.
<!-- SECTION:NOTES:END -->
