# Pending live checks

Lila tests in batches (2026-10-04): each shipped unit adds its checks here, and she runs
a larger pass with these instructions. A unit's follow-up (retiring its text twins) waits
for its section to pass. Remove a section once it passes, and record the result in the
unit's design-doc section.

## Unit 5: /staff and /pointer (live 2026-10-04, c7b6099)

Server: Gay Night House, as a staff member, unless a step says otherwise. Every reply
described as "private" shows "Only you can see this".

### /staff roles
1. `/staff roles` with no option: a public "Staff Roles Configuration" panel with a role picker,
   your current staff roles preselected. Change nothing; dismiss it or leave it.
2. `/staff roles roles:<paste the current staff role mentions or IDs>`: a public
   "Staff Roles Configuration" embed listing them. (Same roles in = nothing changes.)
3. `/staff roles roles:hello`: falls back to the picker panel (no IDs found).

### /staff bump_reset
1. `/staff bump_reset`: private "`Last Bump` reset!".

### /staff delrep
1. `/staff delrep users:<one member ID>`: a public "Batch Reputation Deletion" embed
   naming them; their reputation is cleared.
2. With 5 IDs (pick members whose rep you're fine clearing): the reply must arrive
   (Discord's limit is 3 seconds; "The application did not respond" means it was too
   slow — note it).
3. `/staff delrep users:abc`: a private "must provide at least one user ID" refusal.

### /staff inactivity
1. `/staff inactivity action:date date:<the real next prune date>`: a public embed
   showing Previous Date (the old next date) and Next Date (yours). Check with
   `/db view key:Inactivity Prune` if you like.
2. `/staff inactivity action:date` (no date): a private refusal.
3. `/staff inactivity action:prune`: a public picker panel; pick a member, then Cancel.
4. `/staff inactivity action:prune user:<a member>`: the public confirm (or "active,
   Kick disabled" verdict); press Cancel. Don't press Kick unless you mean it.
5. `/staff inactivity action:nonsense`: a private "must enter a valid action" refusal.
6. Only if you actually want to post one: `action:start`, `end` or `remind` posts the
   announcement and pings the role as normal channel messages, and you get a private
   "Posted the …" confirmation. Check the ping notified.

### /staff activity
1. `/staff activity`: a public "Channel activity" page. Press the next-page button and
   the CSV button; both must work. Note if the first reply was slow.

### /staff rules
1. `/staff rules from:1 to:2` in a test or staff channel: rules 1-2 post as normal
   messages, plus a private "Posted rules 1-2.". Delete the posted rules after.
2. `/staff rules from:2 to:2`: private "Posted rule 2.".

### /staff bootstrap
Skip (it rewrites the server config).

### /pointer
1. `/pointer link:<a message link from a public channel> comment:test`: a public
   "Message Pointer" embed with the comment and links.
2. `/pointer link:hello`: a private "does not match any valid message" refusal.
3. From a NON-staff account (if you have one): `/pointer` with a link into a
   staff-only channel: the same private refusal, and the channel name never shows.

### Report back
Which steps passed, and anything slow or odd. After a pass: the old text commands'
triggers get switched to None (not disabled: /staff calls them), and the text
`message_pointer` is disabled on main.

## config_sync part 2: /setup (main 113, rose 35)

Server: Gay Night House, as a staff member. Every reply is private ("Only you can see
this"). Nothing here changes a setting unless you pick the role or channel it already
has, so re-picking the current values is the safe way to test the save path.

1. `/setup view`: a "Server setup" embed with a Roles field (Staff, Member, Active,
   Inactive, Bump) and a Channels field (Bot, Mod log), each a mention or "not set".
   Note anything shown "not set" that you expected to be set.
2. `/setup roles` with no options: the Roles field only, no "(updated)" marks.
3. `/setup roles staff:<the current staff role>`: the Roles field, Staff marked
   "(updated)" and unchanged. Run `/setup view` again: still the same.
4. `/setup roles bump:@everyone`: a private refusal naming `bump`; nothing changes.
5. `/setup roles bump:<a bot's own role, e.g. YAGPDB's>`: a private "managed by a bot
   or integration" refusal.
6. `/setup channels mod_log:<a category>`: a private "pick a text or announcement
   channel" refusal.
7. `/setup channels mod_log:<the current mod log channel>`: Channels field, Mod log
   marked "(updated)".
8. From a staff account WITHOUT Administrator (if you have one): `/setup roles
   staff:<any role>`: a private "only an Administrator can change the staff role"
   refusal; `/setup roles bump:<the current bump role>` still works.
9. Only if you want to change one for real: pick the new role or channel, then check the
   command that uses it (the table in docs/design/slash-fleet.md says which).

### Report back
Which steps passed, and what step 1 showed as "not set".
