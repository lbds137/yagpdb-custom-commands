# YAGPDB Custom Commands - API Reference

## Overview

This document provides detailed reference information for developers working with the YAGPDB custom command system implemented in this repository.

## YAGPDB System Limits

### Custom Command Size Limits

**Character Limits:**
- **Premium Servers**: 20,000 characters per custom command
- **Free Servers**: 10,000 characters per custom command

**Important Notes:**
- This command suite was developed assuming premium server limits (20k characters)
- Many commands in this repository exceed the 10k free tier limit and will not work on non-premium servers
- The official YAGPDB documentation may be outdated regarding these limits
- Source: YAGPDB official support Discord server

### Other Key Limits

**Message/Embed Limits:**
- Embed description: 2,048 characters maximum
- Embed field value: 1,024 characters maximum
- Embed field name: 256 characters maximum
- Total embed size: 6,000 characters maximum
- Number of embeds per message: 10 maximum

**Database Limits:**
- Database entries per server: Varies by premium tier
- Key length: 256 characters maximum
- Value size: 100KB maximum per entry

**Command Execution Limits:**
- ExecCC concurrent calls: Configurable per server (typically 10-20)
- Command execution timeout: 10 seconds
- Template recursion depth: Limited to prevent infinite loops

## Core Services API

### embed_exec Service

**Purpose**: Universal embed creation and message delivery service

**Usage**:
```gohtml
{{ execCC $embed_exec $channelID 0 (sdict
    "ChannelID" .Channel.ID
    "Title" "Embed Title"
    "Description" "Embed description"
    "Fields" $fieldsArray
    "AuthorID" .User.ID
    "Color" $colorValue
    "ThumbnailURL" $thumbnailURL
    "ImageURL" $imageURL
    "DeleteResponse" true
    "DeleteDelay" $deleteDelay
) }}
```

**Parameters**:
- `ChannelID` (int): Target channel for message
- `Title` (string): Embed title
- `Description` (string): Main embed content (max 2000 chars)
- `Fields` (slice): Array of field objects
- `AuthorID` (int): User ID for author attribution
- `Color` (int): Embed color (defaults to role color or server default)
- `ThumbnailURL` (string): URL for thumbnail image
- `ImageURL` (string): URL for main image
- `DeleteResponse` (bool): Auto-delete the response
- `DeleteDelay` (int): Seconds before deletion

**Field Object Structure**:
```gohtml
(sdict 
    "name" "Field Name"
    "value" "Field Value"
    "inline" true
)
```

### db Service

**Purpose**: Advanced database operations with nested dictionary support

**Basic Usage**:
```gohtml
{{ execCC $db $channelID 0 (sdict
    "UserID" $userID
    "Operation" "get"
    "Key" "Category"
    "Title" "Custom Title"
) }}
```

**Operations**:

#### Get Operation
```gohtml
{{ execCC $db $channelID 0 (sdict
    "UserID" 0
    "Operation" "get"
    "Key" "Global:Embed Color"
) }}
```

#### Set Operation
```gohtml
{{ execCC $db $channelID 0 (sdict
    "UserID" 0
    "Operation" "set"
    "Key" "Global:New Setting"
    "Value" "New Value"
) }}
```

#### Keys Operation
```gohtml
{{ execCC $db $channelID 0 (sdict
    "UserID" 0
    "Operation" "keys"
    "Key" "Global"
) }}
```

#### Add Operation (Merge Dictionaries)
```gohtml
{{ execCC $db $channelID 0 (sdict
    "UserID" 0
    "Operation" "add"
    "Key" "Roles"
    "Value" (sdict "NewRole" "123456789")
) }}
```

#### Remove Operation
```gohtml
{{ execCC $db $channelID 0 (sdict
    "UserID" 0
    "Operation" "remove"
    "Key" "Roles"
    "Value" (sdict "OldRole" "")
) }}
```

#### Delete Operation
```gohtml
{{ execCC $db $channelID 0 (sdict
    "UserID" 0
    "Operation" "delete"
    "Key" "Temporary:Data"
) }}
```

## Configuration API

### Standard Configuration Loading

```gohtml
{{ $globalDict := (dbGet 0 "Global").Value }}
{{ $deleteTriggerDelay := toInt ($globalDict.Get "Delete Trigger Delay") }}
{{ $deleteResponseDelay := toInt ($globalDict.Get "Delete Response Delay") }}
{{ $defaultAvatar := $globalDict.Get "Default Avatar" }}
{{ $embedColor := toInt ($globalDict.Get "Embed Color") }}

{{ $commandsDict := (dbGet 0 "Commands").Value }}
{{ $embed_exec := toInt ($commandsDict.Get "embed_exec") }}
{{ $db := toInt ($commandsDict.Get "db") }}

{{ $rolesDict := (dbGet 0 "Roles").Value }}
{{ $staffRoleID := toInt ($rolesDict.Get "Staff") }}
{{ $memberRoleID := toInt ($rolesDict.Get "Member") }}

{{ $channelsDict := (dbGet 0 "Channels").Value }}
{{ $yagpdbChannelID := toInt ($channelsDict.Get "YAGPDB") }}
{{ $logChannelID := toInt ($channelsDict.Get "Log") }}
```

### Configuration Categories

#### Global Dictionary
```gohtml
Keys:
- "Embed Color" (string): Default embed color as integer string
- "Delete Trigger Delay" (string): Seconds before trigger deletion
- "Delete Response Delay" (string): Seconds before response deletion
- "Default Avatar" (string): URL for default avatar image
- "Command Prefix" (string): Server command prefix
- "Guild Premium Tier" (string): Server nitro boost level
- "ExecCC Limit" (string): Maximum concurrent execCC calls
- "Server URL" (string): Server website URL
```

#### Commands Dictionary
```gohtml
Keys:
- "embed_exec" (string): Custom command ID for embed service
- "db" (string): Custom command ID for database service
- "message_link" (string): Custom command ID for message linking
- [command_name] (string): Custom command ID for specific commands
```

#### Roles Dictionary
```gohtml
Keys:
- "Staff" (string): Staff role ID
- "Member" (string): Member role ID
- "Guest" (string): Guest role ID
- "Agreement" (string): Agreement role ID
- "Minor" (string): Minor role ID
- "Adult" (string): Adult role ID
- [role_name] (string): Custom role IDs
```

#### Channels Dictionary
```gohtml
Keys:
- "YAGPDB" (string): YAGPDB operations channel ID
- "Introduction" (string): Introduction channel ID
- "Welcome" (string): Welcome channel ID
- "Log" (string): General log channel ID
- [channel_name] (string): Custom channel IDs
```

## Common Patterns API

### Argument Parsing

```gohtml
{{ $args := parseArgs 2 "Usage: [arg1] [arg2]"
    (carg "string" "first argument")
    (carg "int" "second argument")
}}

{{ $arg1 := $args.Get 0 }}
{{ $arg2 := $args.Get 1 }}
{{ $hasArg2 := $args.IsSet 1 }}
```

### Permission Checking

```gohtml
{{ $staffRoleID := toInt ($rolesDict.Get "Staff") }}
{{ $hasPermission := hasRoleID $staffRoleID }}
{{ if not $hasPermission }}
    {{ execCC $embed_exec $yagpdbChannelID 0 (sdict
        "ChannelID" .Channel.ID
        "Title" "Permission Denied"
        "Description" "⚠️ You don't have permission to use this command."
        "DeleteResponse" true
        "DeleteDelay" $deleteResponseDelay
    ) }}
    {{ return }}
{{ end }}
```

### Message Link Parsing

```gohtml
{{ $baseURLRegex := "https://(ptb.|canary.)?discord(?:app)?.com/channels/" }}
{{ $fullLinkRegex := joinStr "" $baseURLRegex "\\d{16,}/\\d{16,}/\\d{16,}" }}
{{ $messageLink := reFind $fullLinkRegex $input }}

{{ if $messageLink }}
    {{ $gcmString := reReplace $baseURLRegex $messageLink "" }}
    {{ $gcmSlice := split $gcmString "/" }}
    {{ $guildID := index $gcmSlice 0 }}
    {{ $channelID := index $gcmSlice 1 }}
    {{ $messageID := index $gcmSlice 2 }}
{{ end }}
```

### Error Handling

```gohtml
{{ try }}
    {{ /* Risky operation */ }}
    {{ addMessageReactions $channelID $messageID "✅" }}
{{ catch }}
    {{ execCC $embed_exec $yagpdbChannelID 0 (sdict
        "ChannelID" $channelID
        "Title" "Bot Blocked"
        "Description" "⚠️ The user has the bot blocked or insufficient permissions."
    ) }}
{{ end }}
```

### Input Validation

```gohtml
{{ $snowflakeRegex := "\\A\\d{16,}\\z" }}
{{ $validID := reFind $snowflakeRegex $input }}

{{ $trimmedInput := reReplace "\\A[[:space:]]+" $input "" }}
{{ $trimmedInput = reReplace "[[:space:]]+\\z" $trimmedInput "" }}

{{ $diceRegex := "\\A\\d+d\\d+\\z" }}
{{ $validDice := reFind $diceRegex $trimmedInput }}
```

### User Information

```gohtml
{{ $userID := .User.ID }}
{{ $member := getMember $userID }}
{{ $userAvatar := .User.AvatarURL "128" }}
{{ $userString := .User.String }}
{{ $userMention := .User.Mention }}

{{ if $member.Nick }}
    {{ $displayName := $member.Nick }}
{{ else }}
    {{ $displayName := $member.User.Username }}
{{ end }}
```

### Role Management

```gohtml
{{ $roleID := toInt ($rolesDict.Get "Member") }}
{{ $hasRole := hasRoleID $roleID }}

{{ if not $hasRole }}
    {{ giveRoleID $userID $roleID }}
{{ end }}

{{ takeRoleID $userID $oldRoleID }}
```

## Utility Functions API

### String Operations

```gohtml
{{ $result := joinStr " " $slice.StringSlice }}
{{ $split := split $string "/" }}
{{ $replaced := reReplace "pattern" $string "replacement" }}
{{ $found := reFind "pattern" $string }}
{{ $lower := lower $string }}
{{ $upper := upper $string }}
{{ $title := title $string }}
```

### Type Conversions

```gohtml
{{ $intValue := toInt $stringValue }}
{{ $stringValue := toString $intValue }}
{{ $duration := toDuration $microseconds }}
{{ $json := json $dictValue }}
{{ $dict := jsonToSdict $jsonString }}
```

### Collections

```gohtml
{{ $slice := cslice "item1" "item2" "item3" }}
{{ $slice = $slice.Append "item4" }}
{{ $slice = $slice.AppendSlice $otherSlice }}
{{ $length := len $slice }}
{{ $item := index $slice 0 }}

{{ $dict := sdict "key1" "value1" "key2" "value2" }}
{{ $dict.Set "key3" "value3" }}
{{ $value := $dict.Get "key1" }}
{{ $hasKey := $dict.HasKey "key1" }}
{{ $dict.Del "key2" }}
```

### Mathematical Operations

```gohtml
{{ $sum := add $a $b }}
{{ $difference := sub $a $b }}
{{ $product := mult $a $b }}
{{ $quotient := div $a $b }}
{{ $remainder := mod $a $b }}
{{ $maximum := max $a $b }}
{{ $minimum := min $a $b }}
{{ $random := randInt $min $max }}
```

### Date and Time

```gohtml
{{ $now := currentTime }}
{{ $timestamp := $now.Unix }}
{{ $formatted := $now.Format "2006-01-02 15:04:05" }}
{{ $duration := $now.Sub $earlierTime }}

{{ $snowflakeTime := div $snowflakeID 4194304 | add 1420070400000 | mult 1000000 | toDuration | (newDate 1970 1 1 0 0 0).Add }}
```

## Template Recursion API

### Recursive Template Definition

```gohtml
{{ define "templateName" }}
    {{ $data := . }}
    {{ if $data.Get "baseCondition" }}
        {{ return $data }}
    {{ else }}
        {{ /* Process data */ }}
        {{ $data.Set "property" $newValue }}
        {{ return execTemplate "templateName" $data }}
    {{ end }}
{{ end }}
```

### Template Execution

```gohtml
{{ $parameters := sdict "property1" $value1 "property2" $value2 }}
{{ $result := execTemplate "templateName" $parameters }}
```

## Advanced Patterns

### Bulk Operations

```gohtml
{{ $bulkData := sdict }}
{{- range $item := $collection -}}
    {{ $bulkData.Set $item.Key $item.Value }}
{{- end -}}
{{ dbSet $userID "BulkCategory" $bulkData }}
```

### Conditional Execution

```gohtml
{{ $condition := and $var1 (or $var2 $var3) (not $var4) }}
{{ if $condition }}
    {{ /* Execute block */ }}
{{ else if $alternativeCondition }}
    {{ /* Alternative block */ }}
{{ else }}
    {{ /* Default block */ }}
{{ end }}
```

### Dynamic Field Generation

```gohtml
{{ $fields := cslice }}
{{ if $showField1 }}
    {{ $fields = $fields.Append (sdict "name" "Field 1" "value" $value1) }}
{{ end }}
{{ if $showField2 }}
    {{ $fields = $fields.Append (sdict "name" "Field 2" "value" $value2) }}
{{ end }}

{{ execCC $embed_exec $channelID 0 (sdict
    "Title" "Dynamic Fields"
    "Fields" $fields
) }}
```

## Best Practices

### Performance Optimization

1. **Cache Configuration**: Load configuration dictionaries once per command
2. **Efficient String Operations**: Use `joinStr` for concatenation
3. **Minimize Database Calls**: Batch operations when possible
4. **Reuse Variables**: Don't reload the same data multiple times

### Error Prevention

1. **Always Use Try-Catch**: For Discord API operations
2. **Validate Inputs**: Use regex patterns for validation
3. **Provide Defaults**: Use `or` operator for fallback values
4. **Check Permissions**: Verify access before operations

### Code Organization

1. **Standard Header**: Include author, trigger type, trigger, dependencies
2. **Configuration Section**: Load all required dictionaries first
3. **Validation Section**: Validate inputs and permissions
4. **Business Logic**: Implement command functionality
5. **Output Section**: Generate responses using embed_exec
6. **Cleanup Section**: Delete triggers and temporary data

This API reference provides the foundation for developing new commands and extending the existing system functionality.