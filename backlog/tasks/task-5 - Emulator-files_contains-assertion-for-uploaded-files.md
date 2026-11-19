---
id: TASK-5
title: 'Emulator: files_contains assertion for uploaded files'
status: Deferred
assignee: []
created_date: '2026-11-19T21:55:00Z'
labels:
  - emulator
  - tests
dependencies: []
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** The emulator's YAML tests cannot assert an uploaded file's content (complexMessage "file"/"filename"): only a snapshot pins it. Add a files_contains (or files: list with filename/content_contains) check to sent_messages / interaction_responses, mirroring components_contains. Trigger: the next command that uploads a file, or a CSV format change a snapshot diff would hide in noise. (Found 2026-09-27 building the CSV download.)
<!-- SECTION:DESCRIPTION:END -->
