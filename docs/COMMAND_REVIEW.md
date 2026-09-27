# Command review (2026-09-27)

Every live command in `utility/` (26) and `staff_utility/` (13), with a recommendation.
Nothing here is changed yet: each retire or fix waits on Lila's call. Retiring = move to
`retired/` and remove it from YAGPDB. Line numbers are as of 9079236.

**Lila's decisions (2026-09-27):** retire `dice_roll` (done, moved to `retired/`); keep
`db_get_text`, `ticket_clean`, the bump trio and `inactivity` (prune cycles still run).
The bugs below are fixed. Later the same day she approved all four improvement groups:
clearer errors (atbash, channel_link, hiatus/unhiatus Mod Log), first tests for the
zero-coverage commands, tests for the untested branches, and clamping ExecCC Limit to 10.
All four shipped the same day (4dfa4cb, 63baa64, b84030f, b1c03d9 and the branch tests);
the new tests found three staff_roles bugs, fixed in b1c03d9.

**Open, her call:** hugemoji silently refuses a non-staff user's link to an NSFW channel
from a non-NSFW one (no message; only the trigger is deleted). The branch tests snapshot
that as it is; a short explanation would be the clearer-errors treatment.

## Bugs found (fixed 2026-09-27, each pinned by a test that failed on the old command)

1. **Deleting a middle rule hides the last one.** `rule_edit N (nil)` removes `Rule #N`, but
   `rule` (line 20) and `rules` (line 26) bound rule numbers by `len $rulesDict`. With rules
   1-3, deleting #2 leaves len 2, so #3 can no longer be shown. Fix: `rule_edit` refuses
   gaps (delete only the last rule, or renumber), or `rule`/`rules` use the highest key.
2. **`simple_db_lookup` can't read `Commands`.** It title-cases the key (line 29), so
   `commands embed_exec` looks up `Embed_exec`; `simple_db_edit` (lines 30-33) already
   exempts `Commands`. Fix: copy that exemption.
3. **`db dump` skips two dictionaries.** `$knownKeys` (db.gohtml:170) lacks `Staff` and
   `Inactivity Prune`, both live (hiatus/unhiatus, inactivity).
4. **`contrasts` drops extra colors silently** past the ExecCC limit (lines 29-40);
   `hugemoji` and `rules` both say what they skipped.
5. Known, already filed: ExecCC Limit isn't clamped to YAGPDB's 10 (FUTURE_IMPROVEMENTS).

## Retire candidates

| Command | Why | Recommendation |
|---|---|---|
| `dice_roll` | YAGPDB's built-in `-roll 2d6` does the same and more (`vendor/yagpdb/stdcommands/roll/roll.go:15-28`, RPG dice syntax) | Retire |
| `db_get_text` | `db_get_embed` with plain-text output; no command calls it; carries a dead `$embed_exec` (line 12) | Retire unless you use it by hand |
| `ticket_clean` | Regex `.*` that deletes any non-`tickets open` message in its channel; only useful if the ticket channel is still live (its sibling `ticket_adduser_exec` is retired) | Retire if tickets are gone |
| bump trio (`bump_check`, `bump_remind`, `bump_reset`) | Only useful while the server bumps on Disboard; `bump_check` fires on any 👍 in its channel | Retire all three if bumping stopped; otherwise keep |
| `inactivity` | Only useful if you still run prune cycles; needs Member/Active/Inactive roles set by hand | Your call on usage |

## Keep, with improvements

| Command | Improvement |
|---|---|
| `db` | Bug 3. The largest file (17 KB) and best tested (33 cases) |
| `db_get_embed` | None; service for `kb` and `simple_db_lookup` |
| `simple_db_lookup` | Bug 2 |
| `rule`, `rules`, `rule_edit` | Bug 1; `rules` has 0 tests |
| `contrasts` | Bug 4 |
| `atbash` | Empty or all-unknown input prints an empty cipher with no message (every sibling has an error path); 1 test for 7.7 KB |
| `channel_link` | A failed send to the target channel (line 30) skips everything with no feedback; the same-channel "Channel Details" branch has no test |
| `hugemoji` | No tests for emoji from a message link, the NSFW guard, or the limit warning |
| `pyramid`, `rand_hebrew` | Multi-word fan-out / `NxM` mode untested |
| `hiatus`, `unhiatus` | 0 tests between them; an unset `Channels > Mod Log` sends the log to channel 0 with no error. (`unhiatus` lives in `utility/` on purpose: the user has no staff role while on hiatus) |
| `embed_exec` | Used by nearly every command, no direct test of its 4096/6000 truncation |
| `hex_to_int`, `rand_color`, `timestamp`, `batch_delrep`, `staff_roles`, `bump_*` | 0 tests each |

## Keep as is

`alefbet`, `gematria`, `gematria_bootstrap`, `avatar_viewer` (no native avatar command;
it also does sizes, the server icon and message-link subjects), `kb`, `message_link`,
`message_pointer`, `role_ping`, `directory`, `bootstrap`, `simple_db_edit`.

## Noted, not recommending

- Duplication: the nested-key walk (db, db_get_embed, db_get_text), the message-link regex
  (4 copies), the hex tables (contrast, hex_to_int, rand_color), and the bump cooldown math.
  YAGPDB has no shared includes across commands, so consolidating means execCC services;
  not worth the ExecCC budget for these.
- `batch_delrep`'s cap of 5 matches YAGPDB's `exec` limit (`commands/tmplexec.go:19`).
- `inactivity prune` uses `exec "kick"`, which runs with the invoking staff member's
  permissions (tmplexec.go:20-21); fine if staff hold Kick Members.
- No staff command checks the invoker's role in the template; they rely on the role
  restriction set in YAGPDB's control panel.
- `tools/emulator/testdata/admission_tests.yaml` still tests the six retired commands in CI.
  Keep it until an emulator change breaks one of them; then drop the file rather than fix
  retired code (deleting it waits on Lila's yes).
