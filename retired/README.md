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

`dice_roll.gohtml` (dice rolling) moved here on 2026-09-27: YAGPDB's built-in `-roll 2d6`
does the same with fuller dice syntax. Its tests stay in
`tools/emulator/testdata/dice_roll_tests.yaml`.

`agree.gohtml` (a guest accepts the rules) and `agree_clean.gohtml` (regex `.*` cleanup of
off-format messages in the agreement channel) were the guest-era rules-agreement pair,
deleted from the repo by accident in 75f9a9b on 2026-01-03. Restored here 2026-09-27,
byte-identical to the copies still live in the panel, which are being deleted from YAGPDB
alongside this restoration.

They aren't smoke-tested by `scripts/test-all-templates.sh`/`make test-templates`, aren't
scanned by the standalone `scripts/lint-all.sh` helper, and aren't listed by
`make changed-since-deploy` (all three scan only `everyone/` and `staff/`). `make lint`
still walks them, since `tools/linter/yagpdb_lint.py --dir .` recurses the whole repo except
`tools/`, `vendor/` and `.git/` — its warnings are non-blocking either way.
