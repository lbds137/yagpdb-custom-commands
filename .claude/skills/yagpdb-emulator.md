# YAGPDB Template Emulator

## Quick Commands

Run from the repo root. Needs Go (mise pins 1.27 on the dev machine).

```bash
make test              # template tests in tools/emulator/testdata/, with db_schema.yaml
make test-go           # go vet + Go unit tests
make ci                # everything CI runs (.github/workflows/test.yml): test-go, test, lint, gofmt
make watch             # rerun template tests on changes
make update-snapshots  # accept an intended change in snapshot output

./bin/yagtest run -args "get,Global" -verbose utility/db.gohtml
./bin/yagtest run -no-premium -strict utility/db.gohtml   # free-server limits, fail on breach
./bin/yagtest check utility/*.gohtml                      # parse + static warnings
./scripts/test-all-templates.sh   # smoke-run every command with no args (parseArgs failures are expected)
./scripts/find-missing-functions.sh
```

Flags go before the file. `run` and `test` take `-strict` and `-schema <file>`.

## Test Case Format

```yaml
- name: "Test description"            # unique within the file (snapshots are keyed by name)
  template: "../../../utility/example.gohtml"   # or template_source: |
  strict: true                        # optional: fail on YAGPDB limits
  snapshot: true                      # optional: compare with __snapshots__/<file>.snap.yaml
  context:
    args: ["arg1", "arg2"]
    premium: false                    # optional, default true
    user: { id: 1, roles: [111] }
    reaction: { emoji: "🎮", message_id: 5, added: true }   # reaction-triggered run
  setup_db:
    - { user_id: 0, key: "Global", value: { Delete Trigger Delay: 5 } }
  command_map: { 123: "templates/mock_embed_exec.gohtml" }  # execCC targets
  expected:
    output_contains: "..."            # also output_equals, output_matches, error_contains
    warning_contains: "..."
  assertions:
    db_checks: [{ user_id: 0, key: "K", value_equals: 1 }]   # or value_contains, not_exists
    sent_messages: [{ channel_id: 9, embed_title: "Title" }]
    role_changes: [{ user_id: 1, role_id: 111, action: "add" }]
```

## Project Structure

```
tools/emulator/
├── cmd/yagtest/          # CLI: main.go (run/test/check), watch.go
├── internal/
│   ├── runtime/          # engine.go (FuncMap, Execute), context.go, limits.go (YAGPDB call
│   │                     # counters), loopcheck.go, hints.go, preprocess.go (try/catch)
│   ├── funcs/            # function implementations (standard, database, args, discord)
│   ├── loader/           # YAML tests, runner, snapshots
│   ├── schema/           # db_schema.yaml checks
│   ├── state/            # mock database
│   └── types/            # SDict, Slice, TemplateValue, context structs
└── testdata/             # *.yaml suites, templates/ for execCC, __snapshots__/
```

## Adding Missing Functions

1. Check YAGPDB's implementation in `vendor/yagpdb/common/templates/` (fetch with
   `vendor/update-yagpdb.sh`)
2. Implement it in `internal/funcs/` or on the Engine in `internal/runtime/engine.go`
3. Register it in `BuildFuncMap` in `engine.go`
4. If YAGPDB limits it (`IncreaseCheckCallCounter` in its source), add it to
   `limitedFuncs` in `limits.go`
5. Update the IMPLEMENTED array in `scripts/find-missing-functions.sh`
6. After updating vendor/yagpdb, rerun `scripts/gen-yagpdb-funcs.sh` (function list for hints)

## Fidelity Notes

- YAGPDB's `and`/`or` evaluate every argument (no short-circuit); the emulator matches.
- `eq`/`ne`/`index`/`len` are ports of YAGPDB's: `eq 1 1.0`, `eq nil 1` and an out-of-range
  `index` are errors, as in production.
- Stored numbers come back as float64 (YAGPDB returns value_num), so `eq (dbGet 0 "n").Value 5`
  is an error; compare with `5.0` or convert with `toInt`. Strings come back as-is; dicts come
  back wrapped for `.Get`/`.Set`.
- `execTemplate` returns the value a `{{define}}`d template passes to `return`.
- The template operation limit (1M / 2.5M ops) is not enforced: stdlib text/template can't count ops.
- Output is not whitespace-trimmed like YAGPDB's response; assertions trim it.

## When to Use This Skill

- Running local template tests
- Debugging emulator issues
- Adding new function implementations
- Creating test cases for templates
