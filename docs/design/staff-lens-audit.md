# Staff-lens audit: can a regular moderator use the staff tools?

2026-10-05. Read-only audit of the `Staff Utility` surface (plus `/db` and the staff-only
branches of Utility commands), against the hypothesis in `docs/FUTURE_IMPROVEMENTS.md`
("Staff-facing tools are still techy"): storage shape leaks into tasks. Sources: command
headers and templates under `commands/`, `docs/design/*.md`, `docs/LIVE_CHECKS.md`. No
usage data exists in the repo, so "staff-routine" below is a judgement from the job each
command does, not from how often it runs.

## Verdict

The hypothesis holds, but it is narrow. Three tools carry nearly all of it: `/edit entry`
(Category / Key / Value), `/db` (dictionary vocabulary, colon nesting, `User ID 0`), and
the `/staff inactivity action:` free-text word. The routine tools (`/hiatus`, `/prompt`,
`/staff activity`, `/staff rules`, `/role_ping`, the pickers, `/edit rule` and its delete
confirm) already speak in jobs and are mostly fine. The rest of the problem is error
text: bare "isn't set up" and parameter-speak refusals that give no next step.

## 1. The surface

| Command | Group | Class | Evidence |
|---|---|---|---|
| `/hiatus` | Staff Utility | staff-routine | job: step away; header, self-service |
| `/unhiatus` | Utility (on purpose) | staff-routine | counterpart of `/hiatus`; gate is "recorded hiatus" |
| `/role_ping` (`role_ping_slash`) | Staff Utility | staff-routine | role picker, in-template staff gate |
| `/prompt` + `prompt_post` | Staff Utility | staff-routine | posting prompts; staff gate |
| `/staff activity` (`channel_activity` + `_pager`) | Staff Utility | staff-routine | audit quiet channels |
| `/staff rules` (`rules`) | Staff Utility | staff-routine | post rules into a channel |
| `/staff inactivity` (`inactivity` + `inactivity_prune_pick`) | Staff Utility | staff-routine | announcements, date, kick review; the busiest staff tool |
| `/staff delrep` (`batch_delrep`) | Staff Utility | staff-routine | clear reputation |
| `/staff bump_reset` (`bump_reset`) | Staff Utility | staff-routine (rare) | fixes a missed bump |
| `/edit rule`, `/edit delete rule` (`edit_slash`, `edit_modal`, `edit_confirm`) | Staff Utility | staff-routine | editing rules text |
| `/edit entry`, `/edit delete entry` | Staff Utility | admin in practice | edits raw stored dictionaries; any category incl. Roles |
| `/db` (`db_slash`) | **Utility** | admin | staff gate only picks the row (staff = global row 0, others = own row); anyone can run it |
| `/staff roles` (`staff_roles` + `_pick`) | Staff Utility | admin | decides who counts as staff; picker already exists |
| `/staff bootstrap` (`bootstrap`) | Staff Utility | admin / setup-only | writes defaults, sets the bot channel to the current channel; **no in-template gate** |
| `/setup` (`setup_slash`) | Staff Utility | admin / setup-only | pickers; `staff` is Administrator-only in-template |
| `gematria_bootstrap` | Staff Utility | admin / setup-only | one-time dictionary load, text command |
| `simple_db_lookup` | Staff Utility | admin | text-only `Category Key`; covered by `/db view` |
| `config_sync` | Staff Utility | plumbing | hourly interval, generated |
| `directory` | Staff Utility | plumbing | 168-hour interval, posts the channel directory |
| `staff_slash` (`/staff` router) | Staff Utility | plumbing | dispatches only; its descriptions are staff-facing |
| `staff_roles_pick`, `inactivity_prune_pick`, `channel_activity_pager`, `edit_modal`, `edit_confirm`, `prompt_post` | Staff Utility | plumbing | component and modal handlers; their text is staff-facing |
| Staff-only branches in Utility commands (`db_get_embed` refuses other rows to non-staff, `channel_link` staff wording, `hugemoji` NSFW bypass, `message_link`, `dismiss`) | Utility | plumbing | no staff UI of their own |

Sizes below: **copy** = text only, **small** = small logic change, **design** = new UI or
a decision. Discord limits are checked for every proposed string (all within them).

## 2. Findings by command

### /edit (storage vocabulary, the biggest leak)

**F1. The entry form is a database form** (copy). `edit_slash.gohtml:76-80,144`, header
lines 24-26.
Current: modal "Edit entry" with labels `Category`, `Key`, `Value`; placeholders `e.g. Admin`,
`the entry's name`, `the new text`; options `category string category to prefill`,
`key string key to prefill`, `create boolean make the category if it doesn't exist yet`;
refusal "There is no `X` category. Check the spelling, or set `create` to True to make it."
(`edit_slash:103`, `edit_modal:114`; `edit_slash:222` has a shorter variant.)
Job: change a message the bot posts or a setting, e.g. the inactivity opening announcement.
Why it confuses: a moderator doesn't know that "Inactivity Prune" is a category or that
"Opening Announcement" is a key; the example `Admin` points at an admin section; nothing
says capitals don't matter (the code title-cases both, `edit_slash:94,121`).
Proposed (rename the user-visible words and the option names `category`/`key` to
`section`/`name` in this one file, plus its tests):
- Slash description: `Edit a rule, or a stored setting or message` (43)
- `entry`: `edit a stored setting or message (advanced)` (43)
- `entry.section`: `section to edit, e.g. Inactivity Prune (capitals don't matter)` (62)
- `entry.name`: `name of the entry, e.g. Opening Announcement` (44)
- `entry.create`: `start a brand-new section (only if it doesn't exist yet)` (56)
- Modal labels `Section`, `Name`, `New text`; placeholders `e.g. Inactivity Prune`,
  `e.g. Opening Announcement`, `the new text`; with `create`, the section placeholder
  `a new or existing section`.
- Refusal: `⚠️ There is no section called `X`. Check the spelling, or set `create` to True to start a new one.`
Overlaps: stopgap until F7.

**F2. Editing a non-text entry silently offers to replace it with text** (small).
`edit_slash.gohtml:131-136` (the `$valueField` prefill branch handles only strings and
over-4000 strings; the rule branch at :50-55 has a non-text `else`, the entry branch has none).
Current: for a list or settings group (e.g. `Staff:Roles`) the form opens empty with
placeholder "the new text"; submitting stores text over the list.
Why: a moderator who opens it to look at a value can destroy a list; nothing warns them.
Proposed: when the old value is not a string, answer ephemerally instead of opening the
form: `⚠️ `Section › Name` holds a list or a group of settings, not text, so it can't be edited here. An admin can change it with /db.` (about 125, a message not a field).

**F3. Delete's "exactly one form" refusal** (copy). `edit_slash.gohtml:162`, `:169`;
header `delete.rule integer rule number`, `delete.category string category`, `delete.key string key`.
Current: "⚠️ Give exactly one form: `rule`, or both `category` and `key`." and "⚠️ You must
provide an integer greater than zero!".
Why: "form" is jargon, and the three options give no hint they are alternatives.
Proposed (before F4 lands): `⚠️ To delete a rule, give only `rule`. To delete an entry, give both `section` and `name`.`
and `⚠️ Rule numbers start at 1.` Options: `rule number (to delete a rule)` (30),
`section of the entry (to delete an entry, with name)` (51), `name of the entry (with section)` (32).
Also `edit_slash:246`: "There is no `K` entry in `C` to remove." becomes `⚠️ There is no entry called `K` in `S`.`

**F4. Split `delete` into `delete_rule` and `delete_entry`** (small). `edit_slash` header and
the `delete` branch (:158-293).
Current: one subcommand with three optional options and a validity rule (F3).
Why: Discord's subcommand list is the best picker we have; with two subcommands no
refusal exists. Verbs stay "delete"; the confirm (Delete / Dismiss buttons) is unchanged.
Proposed: `delete_rule` "delete a rule (asks you to confirm)" (35), option `rule` required;
`delete_entry` "delete a stored entry (asks you to confirm)" (43), options `section`, `name`
required. `/edit` then has four subcommands, within the 10-free cap. Panel rows replaced
wholesale by the deploy; the router isn't involved.

**F5. `/edit rule N` for a missing rule says "Edit"** (copy). `edit_slash.gohtml:59` (modal
title `Edit rule N`), header `rule.rule integer! the rule's number`, placeholder :61.
Why: a typo (rule 30 of 12) opens "Edit rule 30" with an empty box and saving creates rule
30 and a numbering gap; the staff member believed they were editing.
Proposed: option `rule number (a number that doesn't exist yet adds a new rule)` (61); modal
title `New rule N (doesn't exist yet)` (at most 33) when no such rule; keep `Edit rule N`
otherwise. The save ack already says "Rule #N added", which is good.

**F6. "dictionary" and "Stored as"** (copy). `edit_slash:233` / `edit_modal:128` ("The `X`
category doesn't hold a dictionary of entries."), `edit_modal:171,177,203` (embed field
"Stored as").
Why: "dictionary" is developer speech, and "Stored as" appears when the key was
capitalised for the user without saying why.
Proposed: `⚠️ `X` can't be edited by name: it holds a single value or a list. An admin can change it with /db.`
and the field `Saved as` with value `` `Greeting` (first letters are capitalised for you)`` (field name 8, value under 1024).

**F7. A guided `/edit entry` picker** (design). Whole `entry` flow.
Job: "change the greeting".
Why: any fix above still asks for two typed names the moderator has to know.
Proposed: bare `/edit entry` posts an ephemeral string select of a curated list of the
messages and settings staff actually edit (label in plain words, value maps to
section+name in code, e.g. "Inactivity: opening announcement"), then opens the form with
only the text field, prefilled. A second select of all sections (up to 25 via
`dbGetPattern`) is an optional "advanced" path. Needs one Message Component handler and a
panel slot. Overlaps `pickers.md` (same mechanics as the staff-roles and prune panels,
including the `<spawner snowflake>` custom-id idiom) and the `/setup` story ("unsetting a
key is /edit's job"). This is the one real fix for the hypothesis; the curated list is the
owner's product call.

### /db

**F8. Slash descriptions** (copy). Header `db_slash.gohtml:11-34`.
Current: `Database dictionary tools`; `set set a value`; `view show one value`;
`remove remove from a dict or array`; `key string! database key, nest with :`; `value ... JSON for dicts`.
Why: "dictionary", "dict", "array", "JSON" and "nest" assume a programmer; the command is
visible to every member (Utility group).
Proposed: description `Advanced: read or change stored server settings` (47); `view` "show one stored value"; `browse` "list the names inside a group"; `set` "replace a stored value"; `add` "add an item to a list or group"; `remove` "take an item out of a list or group"; `delete` "delete a stored entry"; `export` "download entries as a JSON file"; key option `name of the entry; use : to go inside a group, e.g. Roles:Staff` (63); value `the new value (JSON for lists and groups)`; user `another member's own data (staff)`. All well under 100.

**F9. Result embed leaks the row and the engine's errors** (small). `db_slash.gohtml:289-300`
(embed fields `User ID`, `Key`, `Result`; the `$displayUserID` is `0` for staff) and the
error strings at :115-:287 (`No value found!`, `No dictionary found for the given key!`,
`No array or dictionary found for the given key!`, `Invalid value provided! Please double check your input and try again.`).
Why: "User ID 0" means nothing to a human; the errors say what the engine lacks, not what to do.
Proposed: the field `User ID` becomes `Whose data` with `Server-wide` when the row is 0 and a
member mention otherwise (small logic); errors become
`⚠️ Nothing is stored under that name.`, `⚠️ That name isn't a group of entries, so there is nothing to list.`,
`⚠️ That name isn't a list or a group, so you can't add to or remove from it.`,
`⚠️ I couldn't read that value. Lists and groups must be valid JSON, e.g. {"a": 1}.`

### /staff router and its handlers

**F11. Router descriptions** (copy). `staff_slash.gohtml:11-34` (the header the user sees).
| Row | Current | Proposed (length) |
|---|---|---|
| `roles` | `set the staff roles (or open the role picker)` | `choose which roles count as staff` (33) |
| `roles.roles` | `staff role IDs or mentions (omit for the role picker)` | `leave empty to pick the roles from a menu (recommended)` (55) |
| `inactivity` | `inactivity prune announcements, date and review` | `inactivity prune: announcements, next date, review a member` (58) |
| `inactivity.action` | `start, end, remind, date or prune` | `start/end/remind post that announcement; date sets the next prune date; prune reviews a member` (94) |
| `inactivity.date` | `the next prune date (for date)` | `the next prune date as announcements should show it, e.g. March 1` (65) |
| `inactivity.user` | `the member to review (for prune; omit for the picker)` | `member to review (prune only; leave empty to pick from a menu)` (62) |
| `activity` | `channel activity, oldest first` | `see which channels have gone quiet` (34) |
| `rules` | `post the server rules` | `post the server rules in this channel` (37) |
| `rules.from` / `rules.to` | `first rule to post` / `last rule to post` | `first rule number to post (default: all)` / `last rule number to post (default: all)` |
| `bump_reset` | `clear the last bump time` | `reset the bump timer` (21) |
| `delrep` | `delete members' reputation` | `clear members' reputation` (26) |
| `bootstrap` | `initialize the server's configuration` | see F16 |
Check the defaults for rules.from/to before shipping ("default: all" is read from
`rules.gohtml`'s optional-range text path, not run).

**F12. `/staff delrep users:` wants snowflake IDs** (small). `staff_slash` header
`delrep.users string! user IDs or mentions, up to 5`; handler `batch_delrep.gohtml:30-32,69-70`.
Why: moderators then need Developer Mode to copy IDs. A user picker exists in Discord's UI.
Proposed: options `delrep.user1 user! member whose reputation to clear` (44) and
`user2`..`user5` `user another member to clear (optional)` (38); the router joins the IDs
into the existing `ExecData.Users` string, so the handler is untouched. The "Missing User
IDs" refusal then only fires on the text path.

**F13. `inactivity action:` is a typed word** (small). `staff_slash.gohtml:44-47`, header :20-24.
Why: five jobs hide behind one free-text option, and typos land on "Invalid Action
Specified". Static choices would fix it, but the deploy cannot express them (F25).
Proposed: split into three subcommands, `announce` (option `which`: start, end or remind;
still typed, but three words), `prune` (option `user`, picker when empty) and `prune_date`
(option `date` required). `/staff` goes from 7 to 9 subcommands: fine at premium (25), under the 10 free
cap. Check each server's tier first; adding separate announce_* would exceed 10 on a free
server.

**F14. Inactivity refusals speak in parameters** (copy). `inactivity.gohtml:156-157,176-177,206-207`.
Current: titles `Missing Date Parameter`, `Invalid User Parameter`, `Invalid Action Specified` with
"⚠️ You must enter a non-empty date parameter!", "...a valid user parameter!", "...a valid action parameter!".
Proposed (neutral for both text and slash paths): `Date needed` / `⚠️ Give the new prune date, for example `March 1`.`;
`Member not found` / `⚠️ I couldn't find that member. Pick one with `user`, or leave it empty to choose from a menu.`;
`Unknown action` / `⚠️ Choose one of: `start`, `end`, `remind`, `date` or `prune`.`

**F15. The "date" reply is a database receipt, with a doubled title** (small).
`inactivity.gohtml:129-141`.
Current: embed title `Inactivity Prune Inactivity Prune Date Editing` (`$pruneCategory` joined to a heading that
already starts with it), description a JSON block, fields `User ID 0`, `Key Inactivity Prune`,
`Result ✅ Value successfully added!`.
Why: the job was "set the next prune date"; the reply shows storage. Also the title is a plain
bug (the repeated words), which this audit files as a defect to fix with this change.
Proposed: title `Next prune date updated` (23), description
`**Previous:** <old>\n**Next:** <new>`, no fields (staff path only, `Respond` stays).

**F16. `/staff bootstrap` is open to every staff member** (small). `bootstrap.gohtml`
(no `hasRoleID`/Administrator check; it runs `$channelsDict.Set "YAGPDB" <current channel>`
and rewrites defaults), `staff_slash` header `bootstrap initialize the server's configuration`.
Why: it sits in the same list as the routine tools; a moderator who tries it reassigns the
bot channel to wherever they typed. This is not storage vocabulary, but it is the same
"admin tool shown to moderators" problem, and `/setup channels bot:` is the safe route.
Proposed: add the Administrator check `/setup` already uses (`setup_slash.gohtml:65-77`)
with `⚠️ Only an Administrator can run bootstrap.`; description
`(admin) first-time setup of stored defaults; run it in the bot channel` (70); success embed
(`bootstrap.gohtml:98-99`) `Bootstrap Execution Complete` / `The bootstrapping process completed successfully!`
becomes `Server defaults saved` / `First-time setup is done. Next, run /setup to pick the roles and channels.`
Alternative: drop it from `/staff` entirely (7 to 6 subcommands). Owner's call.

**F17. "This command isn't set up on this server yet." has no next step** (copy).
8 staff-surface sites in 7 files: `staff_slash:77,86`, `staff_roles:42`, `inactivity:54`, `batch_delrep:37`,
`channel_activity:32`, `hiatus_slash:41`, `unhiatus_slash:51`; the same sentence is in 15 command
files total (the Utility slash commands too, from commit de2afc3).
Why: a moderator cannot act on it, and "set up" suggests they should do something.
Proposed (82): `⚠️ Part of this command isn't installed on this server yet. Please tell an admin.`

**F18. `/staff activity` never says what Active / Quiet / Stale mean** (copy).
`channel_activity_pager.gohtml:188-197`; the 7 / 30 day thresholds live only in the header comment.
Also `:200` "_No activity has been recorded yet. Make sure channel_tracker is running._".
Proposed: a legend line under the summary: `Active: a message in the last 7 days · Quiet: 7-29 days · Stale: 30+ days · Never seen: none recorded` (about 103, in the embed description, not a field).
Empty state: `_No channel activity has been recorded yet. Ask an admin to check that activity tracking is on._`

**F19. `/staff bump_reset` replies with a storage key** (copy). `bump_reset.gohtml:26,28`.
Current: "`Last Bump` reset!" (the global dict key, in code font).
Proposed: `✅ Bump timer reset.` Wording assumes `Last Bump` is what `bump_remind` and `bump_check`
read to time the reminder (both do: `bump_remind.gohtml:10`, `bump_check.gohtml:15`); the owner should
confirm that is how staff think of it.

### /hiatus, /unhiatus, /role_ping, /prompt

**F20. Hiatus errors** (copy). `hiatus_slash.gohtml:43,45`, `unhiatus_slash.gohtml:53,55,57`.
Current: "Staff role IDs must be defined before using this command!"; "You don't hold any of the staff roles! (...)";
"None of your recorded roles are staff roles anymore, so nothing was restored."
Proposed: `⚠️ No staff roles are set up yet. Ask an admin to run /staff roles.` (67); `⚠️ You have no staff roles to give up. (Already on hiatus? /unhiatus brings them back.)`;
`⚠️ None of the roles you gave up are staff roles any more, so nothing was restored. Ask an admin.`
The main replies ("You're on hiatus. Removed: ... Use /unhiatus to return.") are fine.

**F21. Prune panel wording** (copy). `inactivity_prune_pick.gohtml:94-95,110,121`; `inactivity.gohtml:189-190`.
Current: "Previous prune: _not set_", "🥾 <@x> will be kicked!", "No action was taken — <@x> is not inactive."
Why: "not set" has no next step, and "not inactive" means "lacks the Inactive role" (code :79-80), which a
moderator won't connect to the roles.
Proposed: `_not set yet (use /staff inactivity, action date)_`; `🥾 Kick <@x> from the server? Press Kick to confirm, or Cancel.`;
`❌ No action taken: <@x> doesn't have the Inactive role, so they can't be pruned.` Buttons Kick/Cancel and the picker placeholder (change `Pick a member to prune…` to `Pick a member to review…` to match the description line) are fine.

**F22. /setup embed doesn't say what each role is for** (copy). `setup_slash.gohtml:127-138`.
Current: lines `**Member** @x`, `**Active** @x`, `**Bump** not set`.
Proposed: append the job, from the option descriptions already in the header: `**Member** @x, pinged by the inactivity opening and closing announcements`; and `not set, pick one with /setup roles` for the unset state. Small, low priority. `/setup` is otherwise the model of the good version: pickers, plain labels, refusals that name the option.

**F23. /prompt title box doesn't say the date is added** (copy). `prompt.gohtml:46-47`
(`"label" "Title"` field has no placeholder; `prompt_post` prepends `YYYY-MM-DD: `).
Proposed: placeholder `The date is added for you, e.g. 2026-10-05: Your title` (54).

### Adjacent, not storage vocabulary

**F24. Any staff member can edit the Staff role through /edit or /db** (design, owner's call).
`edit_modal.gohtml` accepts any category; `db_slash.gohtml` gates only the row. `/setup`
deliberately refuses `staff` from non-Administrators (`setup_slash.gohtml:16,77`), but
`/edit entry section:Roles name:Staff` and `/db set key:Roles:Staff` reach the same key. A product and
permission decision (Administrator check for the Roles, Staff, Channels and Commands
sections, or move `/db` to an admin group), not a copy fix. ALREADY DECIDED (Lila
2026-10-04, FUTURE_IMPROVEMENTS.md "/setup roles staff: is Administrator only"): /edit and
/db keep writing the key, staff-gated, unchanged. Listed only because F16 is the same
bypass through a third path (`/staff bootstrap staff_role:`), which that ruling didn't name.

**F25. Static slash choices can't be shipped today** (design, blocked).
`yagpdb-deploy` skill: "an option's choices ... are posted empty" and the header grammar has no choices
syntax, so a deploy replaces hand-set choices with none. Don't rely on choices until the
header grammar, `deploy.js`, the Go emulator and its golden are extended together; prefer the
subcommand splits (F4, F13).

## 3. Fine as they are

- `/hiatus` and `/unhiatus` main replies, descriptions, and the mod-log record.
- `/role_ping`: `Ping any role with an optional message.`, role picker, plain gate text.
- `/prompt` options and form (apart from F23); `/staff rules` ack ("Posted rules 1-2.");
  `/edit rule` flow and the delete confirm for rules ("Delete rule #3?", Delete / Dismiss).
- Staff-roles picker panel (`Pick the staff roles…`) and the prune picker structure.
- `simple_db_lookup`: text-only admin tool with a usage line that explains category and key;
  `/db view` (colon nesting) is no friendlier, so no change; revisit after F7.
- `gematria_bootstrap` (one-time admin): "The gematria and tarot dictionaries have been set." is fine.
- `config_sync`, `directory`: no human-facing text beyond the directory embed.

## 4. Recommended batch

Highest comprehension gain per effort, copy-only first. The first nine are one editing
pass over strings; the rest need a small code change or a decision.

1. **F17** Replace "isn't set up on this server yet" on the staff surface with the "tell an admin" wording (7 files).
2. **F14** Inactivity refusals: `Date needed`, `Member not found`, `Unknown action`.
3. **F11** Router descriptions (`/staff` rows), including the `action` and `date` options.
4. **F1** `/edit entry` labels, placeholders, option descriptions and refusal (Section / Name / New text, examples).
5. **F3** `/edit delete` refusal and option descriptions.
6. **F20** Hiatus and unhiatus errors with a next step.
7. **F18** Channel-activity legend and empty state.
8. **F19, F21, F23, F8** `bump_reset` reply, prune panel wording, `/prompt` title placeholder, `/db` descriptions.
9. **F5** New-vs-edit rule title and option description.
10. **F15** Fix the date reply (a visible bug: the doubled title).
11. **F2** Refuse editing non-text entries (data-loss guard).
12. **F16** Administrator gate on `/staff bootstrap`, plus the setup next-step text.
13. **F12** `/staff delrep` user pickers.
14. **F4 and F13** Split `/edit delete`; split `/staff inactivity`.
15. **F9, F6, F22** `/db` embed row and engine errors, `/edit` jargon, `/setup` hints.
16. **F7** Guided `/edit entry` picker (design; owner picks which entries staff may edit).
17. **F24, F25** Owner's call / blocked.

Counts: 24 findings (numbered F1-F25; F10 is unused, the simple_db_lookup note is under "Fine as
they are") = 14 copy (F1, F3, F5, F6, F8, F11, F14, F17-F23), 7 small logic (F2, F4, F9, F12,
F13, F15, F16), 3 design (F7, F24, F25).
