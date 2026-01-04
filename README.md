# YAGPDB Custom Commands

A comprehensive suite of custom commands for the [YAGPDB Discord bot](https://github.com/botlabs-gg/yagpdb), designed for advanced Discord server management and utility functions.

**Author:** Vladlena Costescu (@lbds137)

## Overview

This repository contains `.gohtml` template files that implement custom commands using YAGPDB's templating system. The commands are organized into three main categories, each serving different operational needs for Discord server administration.

## Architecture

### Database Structure

The system uses YAGPDB's database functionality with a centralized configuration approach:

- **Global Dictionary (`dbGet 0 "Global"`)**: Server-wide settings and configuration
- **Commands Dictionary (`dbGet 0 "Commands"`)**: Custom command ID mappings
- **Roles Dictionary (`dbGet 0 "Roles"`)**: Role ID mappings for permissions
- **Channels Dictionary (`dbGet 0 "Channels"`)**: Channel ID mappings for logging and operations
- **Admin Dictionary (`dbGet 0 "Admin"`)**: Administrative settings and messages
- Additional specialized dictionaries for specific features (Gematria, Knowledge, etc.)

### Core Dependencies

Most commands depend on two foundational utilities:

1. **`embed_exec`** - Centralized embed creation and message handling
2. **`db`** - Database operations interface

## Command Categories

### 🎭 Guests (`guests/`)

Commands for managing guest users and onboarding processes:

- **`agree.gohtml`** - User agreement to server rules, assigns roles automatically
- **`agree_clean.gohtml`** - Cleaned version of the agreement command

**Key Features:**
- Automatic role assignment (Agreement, Announcement Notify, Discussion Notify, Event Notify)
- Logging to designated channels
- Prevention of duplicate agreements

### 🛠️ Staff Utility (`staff_utility/`)

Administrative and moderation tools for server staff:

- **`admit_user.gohtml`** - Admit guests to full membership with role management
- **`archive.gohtml`** - Archive system functionality
- **`batch_delrep.gohtml`** - Batch delete and reputation management
- **`bootstrap.gohtml`** - Initial system setup and configuration
- **`bump_reset.gohtml`** - Server bump reset functionality
- **`directory.gohtml`** - User directory management
- **`gematria_bootstrap.gohtml`** - Initialize gematria calculation system
- **`guest.gohtml`** - Guest user management
- **`hiatus.gohtml`** - User hiatus management
- **`inactivity.gohtml`** - Inactivity tracking and management
- **`reject_user.gohtml`** - Reject guest applications
- **`role_ping.gohtml`** - Role-based ping management
- **`rule_edit.gohtml`** - Server rule editing interface
- **`rules.gohtml`** - Display server rules
- **`screen_user.gohtml`** - Screen potential users
- **`simple_db_edit.gohtml`** - Simplified database editing interface
- **`simple_db_lookup.gohtml`** - Database lookup utility
- **`staff_roles.gohtml`** - Staff role management
- **`ticket_adduser_exec.gohtml`** - Add users to support tickets

**Key Features:**
- Advanced permission checking
- Comprehensive logging and audit trails
- Integration with message linking and archiving
- Error handling for blocked bots

### 🔧 Utility (`utility/`)

General-purpose utility commands for all users:

#### Text and Language Tools
- **`alefbet.gohtml`** - Convert Phoenician/Arabic text to Hebrew with gematria calculation
- **`atbash.gohtml`** - Atbash cipher implementation for Hebrew text
- **`gematria.gohtml`** - Advanced gematria calculator with tarot associations
- **`rand_hebrew.gohtml`** - Generate random Hebrew text

#### Discord Utilities
- **`avatar_viewer.gohtml`** - View user avatars
- **`channel_link.gohtml`** - Generate channel links
- **`message_link.gohtml`** - Generate message links
- **`message_pointer.gohtml`** - Message reference utility
- **`hugemoji.gohtml`** - Display large emoji
- **`timestamp.gohtml`** - Parse Discord snowflake timestamps

#### Database and Configuration
- **`db.gohtml`** - Advanced database operations interface
- **`db_get_embed.gohtml`** - Retrieve database values as embeds
- **`db_get_text.gohtml`** - Retrieve database values as text
- **`embed_exec.gohtml`** - Universal embed creation and execution
- **`simple_db_edit.gohtml`** - Simplified database editing

#### Interactive and Fun
- **`dice_roll.gohtml`** - Dice rolling with customizable dice types
- **`pyramid.gohtml`** - Create text pyramids
- **`rand_color.gohtml`** - Generate random colors
- **`contrast.gohtml`** - Color contrast analysis
- **`contrasts.gohtml`** - Multiple color contrast comparison

#### Server Management
- **`bump_check.gohtml`** - Check server bump status
- **`bump_remind.gohtml`** - Server bump reminders
- **`rule.gohtml`** - Display specific rules
- **`kb.gohtml`** - Knowledge base access
- **`ticket_clean.gohtml`** - Ticket cleanup utility
- **`unhiatus.gohtml`** - Remove user hiatus status

#### Conversion and Calculation
- **`hex_to_int.gohtml`** - Hexadecimal to integer conversion

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
[prefix]bootstrap [embed_exec_id] [db_id] [staff_role_id]
```

This sets up:
- Global configuration defaults
- Command ID mappings
- Role and channel assignments
- Database structure initialization

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

## Security Considerations

- **Permission Validation**: Staff commands verify role permissions before execution
- **Input Sanitization**: User inputs are validated using regex patterns
- **Error Containment**: Try-catch blocks prevent command failures from breaking functionality
- **Audit Logging**: Important actions are logged to designated channels
- **Rate Limiting**: Built-in trigger deletion delays prevent spam

## Usage Examples

### Basic User Commands
```
/dice_roll 2d6          # Roll two six-sided dice
/timestamp 12345678901  # Parse Discord snowflake timestamp
/gematria hello world   # Calculate gematria value
```

### Staff Commands
```
/admit_user [message_link] adult    # Admit user as adult member
/simple_db_edit Admin "Welcome Message" "Welcome to our server!"
/screen_user [message_link]         # Screen a potential user
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

This repository includes a Go-based YAGPDB template emulator for testing commands without a live Discord server:

```bash
# Build the emulator
cd tools/emulator
go build -o bin/yagtest ./cmd/yagtest

# Run a single template
./bin/yagtest run ../../utility/timestamp.gohtml

# Run the full test suite
./bin/yagtest test testdata/command_tests.yaml
```

The emulator supports:
- All standard YAGPDB template functions
- Mock database with persistent state during test runs
- YAML-based test case definitions with custom context
- `execCC` chaining between templates

### Project Structure

```
├── guests/           # Guest onboarding commands
├── staff_utility/    # Staff/admin commands
├── utility/          # General utility commands
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