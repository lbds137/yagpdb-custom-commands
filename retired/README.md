# Retired Commands

The owner stopped using these commands once Discord's server join applications replaced
guest screening, and deleted them from YAGPDB; they moved here on 2026-09-25. They're kept here, unmaintained, for anyone who
wants the flow — and they're still covered by the emulator's admission tests
(`tools/emulator/testdata/admission_tests.yaml`).

- **`admit_user.gohtml`** - Admit guests to full membership with role management
- **`archive.gohtml`** - Archive a guest's introduction message into the introduction-archive channel
- **`guest.gohtml`** - Guest user management
- **`reject_user.gohtml`** - Reject guest applications
- **`screen_user.gohtml`** - Screen potential users
- **`ticket_adduser_exec.gohtml`** - Add users to support tickets

They aren't smoke-tested by `scripts/test-all-templates.sh`/`make test-templates`, aren't
scanned by the standalone `scripts/lint-all.sh` helper, and aren't listed by
`make changed-since-deploy` (all three scan only `staff_utility/` and `utility/`). `make lint`
still walks them, since `tools/linter/yagpdb_lint.py --dir .` recurses the whole repo except
`tools/`, `vendor/` and `.git/` — its warnings are non-blocking either way.
