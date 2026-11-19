---
id: TASK-23
title: 'Emulator: record modal field values in interaction_responses'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - emulator
dependencies: []
ordinal: 23000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** A modal interaction response records its shape only — kind, title, custom_id, field ids — not each text input's value/placeholder/required/style. /edit's prefill (edit_tests.yaml) is pinned by modal shape alone; a prefill regression passes the suite. Promote before any unit needs to assert a modal field's value or placeholder: record the fields in the response and expose them to yaml assertions and snapshots (expect snapshot churn in modal tests).
<!-- SECTION:DESCRIPTION:END -->
