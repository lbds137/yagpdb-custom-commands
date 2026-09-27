# Emulator: Discord interactions (design, 2026-09-27)

Why: Lila's UX picks (docs/FUTURE_IMPROVEMENTS.md "Interactive UX") need slash commands,
buttons/menus, modals and context menus. The emulator must model them from YAGPDB's code
before any command uses them. Vendor: YAGPDB c579722; paths below are under
vendor/yagpdb unless noted. "INF" marks an inference, not read in code. Drafted by a
planning agent, spot-checked by the driver (components.go:1102-1117 custom-ID cap 90 after
the `templates-` prefix; context_interactions.go:13 the one-response error text).

## (a) Template-visible surface

Trigger types: customcommands/customcommands.go:81-87 (Component 7, Modal 8, Slash 12,
UserContextMenu 13, MessageContextMenu 14); triggerStrings :109-125; panel labels
customcommands/assets/customcommands-editcmd.html:199-207 ("Message Component", "Modal
Submission", "Slash Command", "User Context Menu", "Message Context Menu"). Defer modes
:135-138, labels editcmd.html:322-347 (None / Message Response / Ephemeral Message
Response / Update Message Response).

Data keys per handler:
- Component (customcommands/handle_component.go:273-319): `Interaction`,
  `InteractionData`, `CustomID` (prefix `templates-` stripped, :280; const
  common/templates/context_interactions.go:15), `Cmd`, `CmdArgs`, `StrippedID`,
  `StrippedMsg`; `IsButton` (:293) or `IsMenu` + `MenuType`
  ("string"/"user"/"role"/"mentionable"/"channel") + `Values` []string (:295-308).
  `.Message` = the component's message re-fetched (:76-80) with Author/Member set to the
  clicker (:311-316). `.User/.Member` = clicker.
- Modal (:337-437): same base keys; `IsModal`, `Values` (ordered) and `ModalValues`
  sdict keyed by field custom_id with {type,value,custom_id} (:355-424); `.Message` =
  the source message or a blank one (:426-434).
- Slash (customcommands/handle_slashcommand.go:109-176): `Interaction`,
  `InteractionData`, `IsSlashCommand`, `CommandName`, `Cmd`, `SubCommand` ("" without
  one, :130-140), `Options` sdict keyed by option name (:147-160), `Args` (name first) /
  `CmdArgs` in definition order, absent optionals skipped (:151-155). Values: string,
  int64, float64, bool, *User, *Channel, *Role, mentionable = role else user (:181-238).
  `.Message` = blank {GuildID, ChannelID, Member, Author}, ID 0 (:166-173).
- Context menu (customcommands/handle_contextmenu.go:121-159): `Interaction`,
  `InteractionData`, `IsContextMenuCommand`, `CommandName`, `Cmd`, `Author`,
  `CommandType` "user"/"message", `TargetUser`, `TargetMember`; the message type also
  sets `.Message` = the clicked message (:145-155). No `.User/.Member` (nil member :123;
  common/templates/context.go:366-369) — reuse the emulator's NoMember.
- `.Interaction`: lib/discordgo/interactions.go:185-220 (ID, Type, GuildID, ChannelID,
  Message, Member, Token) plus RespondedTo/Deferred (context.go:286-290).
- execCC passes the same Interaction pointer to the child
  (customcommands/tmplextensions.go:240-243); the child runs in a goroutine (:249).

Functions (context_interactions.go:17-30): sendResponse[NoEscape][RetID] :307-372,
updateMessage[NoEscape] :374-413, sendModal :258-305, ephemeralResponse :226-231 (no-op
without an interaction), editResponse :172-224, getResponse :233-256,
deleteInteractionResponse :148-170; tokenArg :418-452 (nil token = current interaction;
a "response" only while !RespondedTo, else a followup). Builders: cbutton/cmenu/cmodal
(context.go:101-104), CreateButton components.go:179-254, CreateSelectMenu :256-329,
complexMessage keys `ephemeral` general.go:374-378, `buttons` (max 40) :379-408, `menus`
(max 5) :409-435, `components` :352-373; row packing components.go:1028-1100; custom IDs
prefixed/auto-numbered/capped general.go:502-503 → components.go:1102-1117.

Response routing (context.go:598-693): with an interaction, the template's output is the
interaction response (:663-676, sets RespondedTo), a followup if already responded
(:677-683), or an edit of the deferred response (:684-693); `ephemeral` sets the flag
(:656-658) and skips response reactions/publish (:709-719). Deferral:
handle_component.go:161-190 (RespondedTo+Deferred set before the run). Error posting is
unchanged with an interaction (customcommands/bot.go:773-780).

## (b) YAML schema (follow ContextDef/ReactionDef, tools/emulator/internal/loader/testcase.go)

`context.interaction:` — one flat, type-discriminated def like `reaction:`; rejected
together with args/message_content/reaction/exec_data at load:
```yaml
interaction: { type: component, custom_id: "pg:ca:stale:2", message_id: 7, values: ["3"], component: string_menu }  # button (default) | string_menu | user_menu | role_menu | mentionable_menu | channel_menu
interaction: { type: modal, custom_id: "edit:rule:3", fields: { rule_text: "new text" }, message_id: 7 }
interaction: { type: slash, subcommand: get, options: { key: "Global", who: 5, where: 9, rank: 111 } }
interaction: { type: user_menu, target: 5 }
interaction: { type: message_menu, message_id: 7 }
```
Slash option values are typed by the header: user → as getMember builds it, channel → a
guild.channels entry, role → a guild.roles entry, integer → int64, number → float64; the
loader errors on a type mismatch or a missing required option (Discord guarantees those
shapes, INF). `message_id` must be in `messages:`; a component run's `.Message` gets the
test user as author.

Recorded (ExecutionContext.InteractionResponses, in order), asserted like `reactions:`
(absent = unchecked, `[]` = none):
```yaml
interaction_responses:
  - { kind: message, ephemeral: true, content_contains: "…", embed_title: "…", components_contains: "pg:ca:stale:3" }
  - { kind: update, embed_contains: "Page 2" }
  - { kind: modal, title: "Edit rule", custom_id: "edit:rule:3", fields: ["rule_text"] }
  - { kind: followup, … }   # and deferred_edit
```
message/followup/deferred_edit also land in `sent_messages` (MessageCheck gains
`ephemeral` and `components_contains`); `update` lands in `edited_messages` and edits
the component's message in place. Snapshots gain `interaction_responses:` and
`ephemeral`/`components` on messages.

## (c) Matching and header syntax

- Component/Modal: `Trigger type: \`Message Component\`` / `\`Modal Submission\``
  (also accept "Component"/"Modal"); `Trigger:` a regex against the prefix-stripped
  custom ID, `(?m)` + `(?i)` unless `Case sensitive: \`true\``, via matchRegexSplitArgs
  (handle_component.go:321-335, 439-453). `.CmdArgs` = the ID after the match, split on
  spaces. A non-matching custom_id → no_trigger semantics.
- Slash: `Trigger type: \`Slash Command\``, `Trigger: \`db\`` (lowercase, EqualFold :78;
  validation :610-612). New header lines, one per panel row: `Slash option:
  \`[sub.]name type[!] description\`` with type from slashFormType keys
  (customcommands.go:278-302: string, string_menu, integer, integer_menu, number,
  number_menu, boolean, user, channel, role, mentionable), `!` = required, `sub.` = its
  subcommand; `Slash subcommand: \`get description\``. `Defer mode: \`Ephemeral Message
  Response\`` (panel labels; default None; "Update Message Response" rejected for Slash
  and context menus: :614, :539).
- Context menus: `Trigger type: \`User Context Menu\``, `Trigger: \`View avatar\``
  (trimmed EqualFold, handle_contextmenu.go:82).
- execCC/scheduleUniqueCC into these types is allowed (tmplextensions.go:202-206).

## (d) Limits and errors to copy verbatim

- `interaction_response` counter 1: sendResponse as the response
  (context_interactions.go:335), updateMessage (:384), sendModal (:271) → `cannot respond
  to an interaction > 1 time; consider using a followup` (:13). `modal` counter 1 →
  `cannot send multiple modals to the same interaction` (:267-269). All count `api_call`.
- Without an interaction: updateMessage `no interaction data in context; consider
  editMessage or editResponse` (:377), sendModal `no interaction data in context` (:260),
  sendResponse `invalid interaction token` (tokenArg :422-426), ephemeralResponse "" (:227).
- sendModal `invalid modal passed to sendModal` (:289, :296); CreateModal `cannot have
  both 'components' and 'fields' in a cmodal` (:82), max 5 fields (:105), default
  custom_id `templates--0` (:77), unknown key `invalid key "x" passed to send message
  builder` (:140).
- Builders: `invalid button style`, `a url field is required for a link button`,
  `button must have a label or emoji` (components.go:213/230/247/250); menu errors
  :283-323; `a select menu cannot share an action row with other components`, `invalid
  component passed to send message builder` (:1050-1053); `custom id too long (max 90
  chars)` (:1114-1116); 5 rows x 5, link buttons lose their ID (:1148-1151); SendResponse
  caps components at 5 rows (context.go:641-646).
- Defer: after a deferred run the output is an edit of the deferred response
  (context.go:684-693); an ephemeral defer stays ephemeral. Warnings (INF, Discord's side):
  a run that ends with no response ("The application did not respond"); a deferred run
  with no output (stuck "thinking").
- Divergence to document (INF): YAGPDB's execCC child is concurrent
  (tmplextensions.go:249), so a parent that both execCCs and prints races for the single
  response; the emulator runs the child inline (the child's sendResponse wins, the
  parent's output becomes a followup). Commands should never mix the two.
- Panel-level, record only: 10/50 slash CCs, 5/15 context-menu CCs per type
  (customcommands.go:958-963); CCActionExecLimit for several CCs on one custom ID
  (handle_component.go:261-268).

## (e) Left out (promote trigger)

Foreign tokens for editResponse/getResponse/deleteInteractionResponse (a command stores a
token); Components V2 (components.go:863-977; a command uses is_components_v2); modal
Label/checkbox/radio/select fields (handle_component.go:373-420; a modal needs more than
text inputs); resolving menu `Values` beyond strings; the "restricted" reply (:142-159)
and CmdRunsInChannel/ForUser (panel restrictions get modelled); option min/max/choice
validation (Discord's job); autocomplete; DM interactions (:26-29); sendDM disabled for
context menus (handle_contextmenu.go:118-120; copy the error if avatar_viewer DMs).

## (f) Units, in order

1. Component builders (~600 lines): CreateButton/CreateSelectMenu/
   distributeComponentsIntoActionsRows/validateCustomID and minimal discordgo
   Button/SelectMenu/ActionsRow types into yagstd/types; complexMessage(+Edit)
   `buttons`/`menus`/`components` fill MessageSend.Components; SentMessage/CtxMessage/
   snapshot carry components; `components_contains`. Go tests on every vendor error text.
2. Interaction core + Component trigger (~900 lines): Interaction on ExecutionContext,
   `interaction:` (component only), header trigger types, data keys, SendResponse's three
   modes + ephemeral, sendResponse*/updateMessage/ephemeralResponse, counters, execCC
   sharing the pointer, `interaction_responses`, snapshot, not-responded warnings, `Defer
   mode:` header. Unblocks channel_activity's browse view.
3. Slash + context menus (~700 lines): header option/subcommand parsing, typed options,
   .Options/.SubCommand/.CmdArgs, .TargetUser/.TargetMember/.Author, NoMember for context
   menus. Unblocks /db, the context-menu entries, the picker commands.
4. Modals (~500 lines): cmodal/CreateModal/CreateTextInput, sendModal, Modal trigger with
   .Values/.ModalValues, `interaction: {type: modal}`. Unblocks /edit.
Units 3 and 4 are independent after 2; each updates scripts/find-missing-functions.sh
and .claude/skills/yagpdb-emulator.md.
