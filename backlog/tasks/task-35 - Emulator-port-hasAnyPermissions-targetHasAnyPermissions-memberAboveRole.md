---
id: TASK-35
title: 'Emulator: port hasAnyPermissions / targetHasAnyPermissions / memberAbove(Role)'
status: Deferred
assignee: []
created_date: '2026-11-19T21:57:00Z'
labels:
  - emulator
dependencies: []
ordinal: 35000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** Port the c579722-only template functions hasAnyPermissions, targetHasAnyPermissions, memberAbove, memberAboveRole (context_funcs.go 771-847, 1051-1087). They need a permission model beyond getTargetPermissionsIn (runtime/permissions.go already copies dstate's CalculatePermissions and GetMemberPermissions; the emulator never implemented hasPermissions either). Trigger: a command calls any of them — grep 11-19: none in commands/.
<!-- SECTION:DESCRIPTION:END -->
