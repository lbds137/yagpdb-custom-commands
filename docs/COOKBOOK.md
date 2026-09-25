# Cookbook

Small, complete custom commands for common server features. Each recipe is a file in
[`cookbook/`](cookbook/) that you can paste into YAGPDB as-is after changing the IDs at
the top, and each one is tested against the emulator in
[`cookbook_tests.yaml`](../tools/emulator/testdata/cookbook_tests.yaml), so `make test`
fails if a recipe breaks.

| Recipe | Trigger | Shows how to |
|--------|---------|--------------|
| [Economy](cookbook/economy.gohtml) | Command `coins` | keep a per-user balance, run a daily cooldown |
| [Moderation log](cookbook/moderation_log.gohtml) | Command `warn` | check a staff role, count warnings, post a log embed |
| [Welcome](cookbook/welcome.gohtml) | Join message | give new members a role and greet them |
| [Reaction roles](cookbook/reaction_roles.gohtml) | Reaction (added + removed) | map emoji to roles on one message |
| [Leveling](cookbook/leveling.gohtml) | Regex `\A` | award XP with a cooldown and announce level-ups |
| [Leaderboard](cookbook/leaderboard.gohtml) | Command `leaderboard` | rank every member with a single database call |

## Trying a recipe locally

```bash
make build-emulator
./bin/yagtest run docs/cookbook/economy.gohtml                 # -coins
./bin/yagtest run -args daily -verbose docs/cookbook/economy.gohtml   # -coins daily
./bin/yagtest test tools/emulator/testdata/cookbook_tests.yaml  # all recipe tests
```

`-verbose` also prints the database afterwards. Add `-no-premium -strict` to run with a
free server's limits.

## Techniques

**Per-user data.** `dbSet`/`dbGet` take a user ID first. Using the member's ID
(`.User.ID`, or the target of a command) gives every member their own entry under the
same key, which is how the economy, warnings and XP recipes store data.

**Counters.** `dbIncr` adds to a number and returns the new total in one call, and it
starts from 0 when the entry doesn't exist. Prefer it to a `dbGet` + `dbSet` pair.

**Cooldowns.** `dbSetExpire` writes an entry that disappears after a number of seconds.
If the entry exists, the cooldown is still running: see the daily claim and the XP
cooldown.

**Leaderboards.** `dbTopEntries "xp" 10 0` returns the top entries of *every* member for
that key, sorted by value, in one database call. Looping over members and calling
`dbGet` for each would use up the database limit (below) almost immediately.

**Reaction triggers.** A reaction-triggered command sees `.Reaction` (with `.MessageID`
and `.Emoji`) and `.ReactionAdded` (false when the reaction was removed). Unicode emoji
are matched by the character; custom emoji by `.Emoji.APIName`, which is `name:id`.

## YAGPDB gotchas these recipes avoid

- **`and`/`or` evaluate every argument.** Unlike current Go templates, YAGPDB's do not
  stop early, so `and ($args.IsSet 0) (lower ($args.Get 0))` fails when the argument is
  missing. Check first with a separate `if` (see the economy recipe). The emulator
  reproduces this.
- **`dbGet` returns an entry, not the value.** Read the stored data with `.Value`. When
  the key doesn't exist, `dbGet` returns nil and `(dbGet 0 "Key").Value` is an error, so
  guard it with `with` or `if`.
- **Database calls are limited per run**: 10 without premium, 50 with it, and calls that
  read many entries (`dbGetPattern`, `dbTopEntries`, `dbCount`) also count against a
  second limit of 2 / 10. `yagtest -strict` fails a run that would hit these limits, and
  the emulator warns about database calls inside `range` loops.
- **Only one `sendDM` per run**, and a second one is dropped without any error.
- **Responses over 2,000 characters** are replaced with an error notice. Build long output
  into an embed description (4,096 characters) instead.
