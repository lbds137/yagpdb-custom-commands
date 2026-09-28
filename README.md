# YAGPDB Custom Commands

A comprehensive suite of custom commands for the [YAGPDB Discord bot](https://github.com/botlabs-gg/yagpdb), designed for advanced Discord server management and utility functions.

**Author:** Vladlena Costescu (@lbds137)

## Overview

This repository contains `.gohtml` template files that implement custom commands using YAGPDB's templating system. The commands are organized into two main categories, each serving different operational needs for Discord server administration, plus a `retired/` folder for commands no longer deployed.

## Architecture

### Database Structure

The system uses YAGPDB's database functionality with a centralized configuration approach:

- **Global Dictionary (`dbGet 0 "Global"`)**: Server-wide settings and configuration
- **Commands Dictionary (`dbGet 0 "Commands"`)**: Custom command ID mappings, written
  hourly by `config_sync` from `deploy/panel.json`
- **Roles Dictionary (`dbGet 0 "Roles"`)**: Role ID mappings for permissions
- **Channels Dictionary (`dbGet 0 "Channels"`)**: Channel ID mappings for logging and operations
- **Admin Dictionary (`dbGet 0 "Admin"`)**: Administrative settings and messages
- Additional specialized dictionaries for specific features (Gematria, Knowledge, etc.)

### Core Dependencies

Most commands depend on two foundational utilities:

1. **`embed_exec`** - Centralized embed creation and message handling
2. **`db`** - Database operations interface

## Command Categories

Every deployed command lives under `commands/<topic>/`, one folder per topic. The YAGPDB
control-panel group (the permission scope) is not a folder any more: each file's header
names it on a `Group:` line, `Utility` (everyone) or `Staff Utility` (staff), and a topic
folder mixes both. Staff-group commands are marked below.

#### Bump (`commands/bump/`)
- **`bump_check.gohtml`** - Check server bump status
- **`bump_remind.gohtml`** - Server bump reminders
- **`bump_reset.gohtml`** (staff) - Server bump reset functionality

#### Rules (`commands/rules/`)
- **`rule.gohtml`** - Display specific rules
- **`rules.gohtml`** (staff) - Display all server rules
- **`rule_edit.gohtml`** (staff) - Server rule editing interface

#### Gematria (`commands/gematria/`)
- **`gematria.gohtml`** - Advanced gematria calculator with tarot associations
- **`gematria_bootstrap.gohtml`** (staff) - Initialize gematria calculation system

#### Hebrew (`commands/hebrew/`)
- **`alefbet.gohtml`** - Convert Phoenician/Arabic text to Hebrew with gematria calculation
- **`atbash.gohtml`** - Atbash cipher implementation for Hebrew text
- **`pyramid.gohtml`** - Create text pyramids
- **`rand_hebrew.gohtml`** - Generate random Hebrew text

#### Color (`commands/color/`)
- **`contrast.gohtml`** - Color contrast analysis
- **`contrasts.gohtml`** - Multiple color contrast comparison
- **`hex_to_int.gohtml`** - Hexadecimal to integer conversion
- **`rand_color.gohtml`** - Generate random colors

#### Database (`commands/db/`)
- **`db.gohtml`** - Advanced database operations interface
- **`db_get_embed.gohtml`** - Retrieve database values as embeds
- **`db_get_text.gohtml`** - Retrieve database values as text
- **`simple_db_edit.gohtml`** (staff) - Simple database editing
- **`simple_db_lookup.gohtml`** (staff) - Simple database lookup

#### Members (`commands/members/`)
- **`hiatus.gohtml`** (staff) - User hiatus management
- **`unhiatus.gohtml`** - Remove user hiatus status (in the `Utility` group on purpose:
  a staff member on hiatus has lost the staff roles the `Staff Utility` group requires)
- **`inactivity.gohtml`** (staff) - Inactivity tracking and management
- **`staff_roles.gohtml`** (staff) - Staff role management
- **`role_ping.gohtml`** (staff) - Role-based ping management
- **`batch_delrep.gohtml`** (staff) - Batch delete and reputation management

#### Channels (`commands/channels/`)
- **`channel_activity.gohtml`** (staff) - Dead-channel audit from what `channel_tracker`
  records: cleans up deleted channels' records, then opens the browse view
  (`channel_activity_pager`)
- **`channel_activity_pager.gohtml`** (staff) - The audit's browse view (Message Component
  trigger `^ca:`): a summary, one bucket (active / quiet / stale / never seen) per page of
  15, bucket and paging buttons that edit the page in place, and a CSV download (ephemeral)
- **`channel_tracker.gohtml`** - Records each channel's last-active time on every message
  (Regex `.*`, no output), for `channel_activity`
- **`directory.gohtml`** (staff) - User directory management
- **`channel_link.gohtml`** - Generate channel links
- **`message_pointer.gohtml`** - Message reference utility

#### Knowledge (`commands/knowledge/`)
- **`define.gohtml`** - Glossary term lookup (links to thenighthouse.org)

#### General (`commands/general/`)
- **`avatar_viewer.gohtml`** - View user avatars
- **`hugemoji.gohtml`** - Display large emoji
- **`timestamp.gohtml`** - Parse Discord snowflake timestamps

#### Plumbing (`commands/plumbing/`)
- **`embed_exec.gohtml`** - Universal embed creation and execution
- **`message_link.gohtml`** - Generate message links (called by log_user, unmanaged: never
  deployed, the live copy holds the real watched ID)
- **`ticket_clean.gohtml`** - Ticket cleanup utility
- **`bootstrap.gohtml`** (staff) - Initial system setup and configuration
- **`config_sync.gohtml`** (staff, hourly) - Writes every command's panel ID into the
  `Commands` dict; generated from `deploy/panel.json` by `make config-sync`, never edited by
  hand

**Key Features:**
- Advanced permission checking
- Comprehensive logging and audit trails
- Integration with message linking and archiving
- Error handling for blocked bots

### 📦 Retired (`retired/`)

Commands no longer deployed to the live server, kept for reference. See
[`retired/README.md`](retired/README.md) for the full list and why they were retired.

## Technical Implementation

### Common Patterns

#### Configuration Loading
```go
{{ $globalDict := (dbGet 0 "Global").Value }}
{{ $deleteTriggerDelay := toInt ($globalDict.Get "Delete Trigger Delay") }}
{{ $deleteResponseDelay := toInt ($globalDict.Get "Delete Response Delay") }}

{{ $commandsDict := (dbGet 0 "Commands").Value }}
{{ $embed_exec := toInt ($commandsDict.Get "embed_exec") }}
```

#### Error Handling and User Feedback
```go
{{ if not $requiredValue }}
    {{ execCC $embed_exec $yagpdbChannelID 0 (sdict
        "ChannelID" .Channel.ID
        "Title" "Error Title"
        "Description" "⚠️ Error message here"
        "DeleteResponse" true
        "DeleteDelay" $deleteResponseDelay
    ) }}
{{ end }}
```

#### Permission Checking
```go
{{ $permissionCheck := hasRoleID $staffRoleID }}
{{ if not $permissionCheck }}
    {{ /* Handle unauthorized access */ }}
{{ end }}
```

#### Message Link Parsing
```go
{{ $baseURLRegex := "https://(ptb.|canary.)?discord(?:app)?.com/channels/" }}
{{ $fullLinkRegex := joinStr "" $baseURLRegex "\\d{16,}/\\d{16,}/\\d{16,}" }}
{{ $messageLink := reFind $fullLinkRegex $messageLinkArg }}
```

### Advanced Features

#### Template Recursion (Gematria)
The gematria system uses recursive templates for numerical reduction:
```go
{{ define "reduce" }}
  {{ $dData := . }}
  {{ if lt ($dData.Get "redStep") 10 }}
    {{ return $dData }}
  {{ else }}
    {{ /* Recursive reduction logic */ }}
    {{ return (execTemplate "reduce" $dData) }}
  {{ end }}
{{ end }}
```

#### Bot Blocking Detection
```go
{{ try }}
    {{ deleteAllMessageReactions $channelID $messageID }}
    {{ addMessageReactions $channelID $messageID $emoji }}
{{ catch }}
    {{ execCC $embed_exec $yagpdbChannelID 0 (sdict
        "ChannelID" $channelID
        "Title" "Bot Blocked"
        "Description" (joinStr "" "⚠️ The user has the bot blocked!")
    ) }}
{{ end }}
```

## Setup and Configuration

### 1. Bootstrap Process

Use the `bootstrap.gohtml` command to initialize the system:

```
[prefix]bootstrap [staff_role_id]
```

The staff role ID is optional: without it, a rerun keeps the Staff role already set.

This sets up:
- Global configuration defaults
- The Staff role and the YAGPDB channel (the one it runs in)
- Database structure initialization

Command ID mappings (the `Commands` dict) come from `config_sync`, an hourly command
generated from `deploy/panel.json`: after adding a command's panel ID there, run
`make config-sync` and deploy `config_sync`; its next run writes the IDs (merged: keys it
doesn't know stay). Its panel entry needs a channel set and Enabled ticked: an interval
command without a channel (or whose channel was deleted) stops running, with no error.

### 2. Required Custom Commands

Before using this system, you need these custom commands configured in YAGPDB:
- `embed_exec` - For creating embedded messages
- `db` - For database operations
- Any command-specific dependencies listed in file headers

### 3. Database Categories

The system expects these database categories to be available:
- `Global` - Server-wide settings
- `Commands` - Command ID mappings
- `Roles` - Role ID mappings
- `Channels` - Channel ID mappings
- `Admin` - Administrative settings
- `Gematria` - Gematria calculation data (for Hebrew text commands)

### 4. Free (non-premium) servers

`bootstrap.gohtml` defaults Global `ExecCC Limit` to `10`, YAGPDB premium's per-run `execCC`
cap. A free (non-premium) server should set Global `ExecCC Limit` to `1`, free's own per-run
cap, or commands that fan out via `execCC` (like `pyramid`, `contrasts` and `hugemoji`) will
error out instead of warning and skipping the overflow. `db` (17,538 characters) and
`gematria` (11,514 characters) also exceed free's 10,000-character command size limit for
now; a minified build is planned.

## Security Considerations

- **Permission Validation**: Staff commands verify role permissions before execution
- **Input Sanitization**: User inputs are validated using regex patterns
- **Error Containment**: Try-catch blocks prevent command failures from breaking functionality
- **Audit Logging**: Important actions are logged to designated channels
- **Rate Limiting**: Built-in trigger deletion delays prevent spam

## Usage Examples

### Basic User Commands
```
/timestamp 12345678901  # Parse Discord snowflake timestamp
/gematria hello world   # Calculate gematria value
```

### Staff Commands
```
/simple_db_edit Admin "Welcome Message" "Welcome to our server!"
```

### Database Operations
```
/db get:0 Global                                    # Get global configuration
/db set:0 Admin:Welcome "Hi there!"                 # Set welcome message
/db keys:0 Roles                                    # List all role mappings
/db add:0 "Directory:Exclude Categories" "Archive"  # Append to array
/db remove:0 "Directory:Exclude Categories" "Old"   # Remove from array
/db dump                                            # Export config as JSON file
/db dump Global                                     # Export specific key only
```

## Error Handling

The system implements comprehensive error handling:

- **Invalid Arguments**: Commands validate input format and provide usage guidance
- **Missing Permissions**: Unauthorized access attempts are logged and blocked  
- **Bot Blocking**: Graceful handling when users have the bot blocked
- **Database Errors**: Safe fallbacks when database operations fail
- **Message Processing**: Robust parsing of Discord message links and IDs

## Development

### Local Testing with Emulator

This repository includes a Go-based YAGPDB template emulator for testing commands without a live Discord server. It needs Go (1.24+; the dev machine uses 1.27 via mise).

```bash
make test          # all template tests (tools/emulator/testdata/), checked against db_schema.yaml
make watch         # rerun them whenever a command or test file changes
make ci            # everything CI runs: Go vet + unit tests, template tests, linter, gofmt

./bin/yagtest run commands/general/timestamp.gohtml         # run one command
./bin/yagtest run -args "get,Global" -verbose commands/db/db.gohtml
./bin/yagtest run -no-premium -strict commands/db/db.gohtml  # fail where a free server would
./bin/yagtest check commands/*/*.gohtml                     # parse only, plus static warnings
```

The emulator runs templates on YAGPDB's own template engine and standard functions (copied
from its source, not reimplemented), so language features, built-ins and errors match
production. On top of that:
- Mocks for Discord and the database: messages, members and roles you declare in a test,
  `execCC` chaining, reaction triggers, and a database that copies values in and out the way
  YAGPDB serializes them
- **Execution limits** taken from YAGPDB's source: database calls (10, or 50 with premium),
  Discord API calls, DMs, `execCC`, template operations (1M, or 2.5M with premium), output
  size, response length and template length.
  By default a breached limit is a warning; `-strict` fails the run the way production does.
  Inside `{{try}}`, a function over its call limit or refused by Discord goes to
  `{{catch}}` either way, as in production (not the calls YAGPDB skips silently, like
  `sendDM`)
- **Warnings** for database calls inside `range` loops, and for values that don't match
  `db_schema.yaml` (`-schema`)
- **Error hints**: typo suggestions from YAGPDB's function list, links to its docs
- **YAML tests** with output, database, message and role assertions, `strict: true`,
  `context.premium: false`, `expected.warning_contains`, `setup_templates` (run a bootstrap
  command first, on the same database), and `snapshot: true`
  (saved under `__snapshots__/`; `make update-snapshots` accepts an intended change)

[`docs/COOKBOOK.md`](docs/COOKBOOK.md) has tested example commands, and
[`tools/ide/`](tools/ide/) has GoLand snippets.

### Project Structure

```
├── commands/         # Every deployed command, one folder per topic (panel group in the header)
├── retired/          # Commands no longer deployed
├── tools/emulator/   # Local testing emulator
├── docs/             # Documentation
└── scripts/          # Development scripts
```

## Contributing

When adding new commands:

1. Follow the established header format with author, trigger type, trigger, and dependencies
2. Use the common configuration loading patterns
3. Implement proper error handling and user feedback
4. Add appropriate logging for administrative actions
5. Test thoroughly with various input scenarios
6. Document any new database categories or dependencies

## License

This project is released under the same license as YAGPDB (MIT License).