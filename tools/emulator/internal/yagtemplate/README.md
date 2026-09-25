# yagtemplate

YAGPDB's fork of Go's `text/template` (`lib/template` in
[botlabs-gg/yagpdb](https://github.com/botlabs-gg/yagpdb)), copied so the emulator
executes templates exactly as YAGPDB does: `try`/`catch`, `while`, `return`,
`execTemplate`, YAGPDB's `eq`/`index`/`len`/`and`/`or`, and the operation limit.

- Source: yagpdb commit `0cf2ec5` (2025-12-18), `lib/template` and `lib/template/parse`.
- Licenses: Go's BSD license (`LICENSE-GO`, the original text/template) and YAGPDB's MIT
  license (`LICENSE-YAGPDB`, its changes).
- Local changes, all marked `EMULATOR PATCH`:
  - `Template.OnMaxOps`: report exceeding the operation limit through a callback
    instead of stopping, so yagtest can warn outside `-strict` (it still stops at 10x
    the limit, so runaway loops end).
  - Import paths rewritten; doc comments reformatted by current gofmt.

To update: copy `vendor/yagpdb/lib/template` over this directory (after
`vendor/update-yagpdb.sh`), rewrite the import paths, reapply the patches, and run
`go test ./internal/yagtemplate/...`.

YAGPDB's original README (its changes from the stdlib):

> Slightly modified version of the stdlib text/template package:
>
> Changes:
>
>  - allows newlines inside actions
