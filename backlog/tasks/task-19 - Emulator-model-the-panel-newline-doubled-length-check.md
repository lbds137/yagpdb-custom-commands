---
id: TASK-19
title: 'Emulator: model the panel newline-doubled length check'
status: Done
assignee: []
created_date: '2026-11-19T21:56:00Z'
updated_date: '2026-11-22T17:35:00Z'
labels:
  - emulator
dependencies: []
priority: medium
ordinal: 30
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**To Do (emulator fidelity).** The panel counts every newline TWICE against the response limit (form arrives CRLF server-side; runes + newline-count vs 20,000 premium — customcommands-editcmd.html updateCCLength, web/validation.go ValidateTemplateField), so a template can pass every emulator test and still be REFUSED at save (db_slash was 19,881 runes + 449 lines). The emulator's premium char check counts runes only. Guard meanwhile: the linter's response-length rule errors over 20,000. Note: deploy.js posts LF and the server-side deploy check is runes alone; the tooling keeps the stricter limit so a later panel save also passes. Found 2026-09-30.
<!-- SECTION:DESCRIPTION:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Shipped 2026-11-22 on branch drain-decided-1122: checkSourceLength counts runes + newlines (\r stripped first), message keeps 'refuses to save'; red-first unit tests incl. boundaries and CRLF; full YAML suite 897/897.
<!-- SECTION:NOTES:END -->
