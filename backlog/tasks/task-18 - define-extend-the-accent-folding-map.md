---
id: TASK-18
title: '/define: extend the accent-folding map'
status: Deferred
assignee: []
created_date: '2026-11-19T21:56:00Z'
labels:
  - commands
dependencies: []
ordinal: 18000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Deferred.** /define (define_slash.gohtml) folds accents with a hand-written map (Latin-1 vowels, ñ, ç, ý/ÿ, macron vowels), while the site's slugify strips every combining mark after NFD: a term with any other mark (č, ş, ő, ą...) gets a hyphen where the site drops the mark, so its anchor misses. Today's 67 terms are all covered (only Ásatrú and Santería accented, checked 2026-09-27). Trigger: a glossary term with another accented letter added to the-night-house src/content/glossary.md — add its letter to the map.
<!-- SECTION:DESCRIPTION:END -->
