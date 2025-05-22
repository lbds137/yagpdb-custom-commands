# YAGPDB Custom Commands - Architecture Documentation

## System Overview

This custom command suite implements a sophisticated Discord server management system using YAGPDB's templating engine. The architecture follows a modular, database-driven approach with centralized configuration management.

## Core Architecture Principles

### 1. Centralized Configuration
All configuration is stored in YAGPDB's database using a hierarchical dictionary structure:

```
Database Structure:
├── Global (User ID: 0)
│   ├── Embed Color
│   ├── Delete Trigger Delay
│   ├── Delete Response Delay
│   ├── Default Avatar
│   ├── Command Prefix
│   └── Guild Premium Tier
├── Commands (User ID: 0)
│   ├── embed_exec → Command ID
│   ├── db → Command ID
│   └── [other_command] → Command ID
├── Roles (User ID: 0)
│   ├── Staff → Role ID
│   ├── Guest → Role ID
│   ├── Member → Role ID
│   └── [role_name] → Role ID
└── Channels (User ID: 0)
    ├── YAGPDB → Channel ID
    ├── Introduction → Channel ID
    └── [channel_name] → Channel ID
```

### 2. Dependency Injection Pattern
Commands declare their dependencies in headers and resolve them at runtime:

```gohtml
{{- /*
  Dependencies: `embed_exec`, `db`
*/ -}}

{{ $commandsDict := (dbGet 0 "Commands").Value }}
{{ $embed_exec := toInt ($commandsDict.Get "embed_exec") }}
{{ $db := toInt ($commandsDict.Get "db") }}
```

### 3. Service-Oriented Design
Core services are implemented as reusable custom commands:

- **`embed_exec`**: Universal embed creation service
- **`db`**: Database operations service
- **`message_link`**: Message linking service

## Data Flow Architecture

### Command Execution Flow
```
User Input → YAGPDB → Custom Command → Configuration Loading → Business Logic → Output Service → Discord
```

### Detailed Flow:
1. **Input Validation**: Parse and validate user arguments
2. **Configuration Loading**: Load required configuration dictionaries
3. **Permission Checking**: Verify user permissions for protected commands
4. **Business Logic**: Execute command-specific functionality
5. **Output Generation**: Create embedded responses via `embed_exec`
6. **Cleanup**: Delete triggers and manage message lifecycle

### Error Handling Flow
```
Error Detection → Try-Catch Block → Error Categorization → User Feedback → Logging → Graceful Degradation
```

## Component Architecture

### 1. Foundation Layer

#### Database Abstraction (`db.gohtml`)
Provides CRUD operations with nested dictionary support:

```gohtml
{{ define "operations" }}
  - keys: List dictionary keys
  - get: Retrieve values
  - set: Store values
  - add: Merge dictionaries
  - remove: Delete entries
  - delete: Remove keys
{{ end }}
```

**Key Features:**
- Nested key access (`Parent:Child:Grandchild`)
- Type-safe operations
- JSON serialization support
- Permission-based access control

#### Embed Service (`embed_exec.gohtml`)
Centralized message formatting and delivery:

```gohtml
{{ define "embed_structure" }}
  - Author Information (User/Guild)
  - Color Management (Role-based/Custom)
  - Content Truncation (2000 char limit)
  - Auto-deletion Support
  - Thumbnail/Image Support
{{ end }}
```

### 2. Business Logic Layer

#### User Management
- **Guest Processing**: `agree.gohtml`, `admit_user.gohtml`
- **Role Management**: Automatic role assignment and removal
- **Status Tracking**: Hiatus, inactivity, screening states

#### Content Management
- **Rule System**: Dynamic rule storage and retrieval
- **Knowledge Base**: Searchable information repository
- **Archival System**: Message preservation and organization

#### Utility Services
- **Text Processing**: Hebrew/Phoenician/Arabic conversion
- **Mathematical Operations**: Gematria, color contrast, dice rolling
- **Discord Integration**: Timestamp parsing, avatar viewing, message linking

### 3. Presentation Layer

#### Response Formatting
All responses use structured embeds with consistent formatting:

```gohtml
{{ define "response_template" }}
  Title: Command-specific title
  Description: Main content
  Fields: Structured data display
  Author: User attribution
  Color: Role-based or custom
  Thumbnail: Context-appropriate image
{{ end }}
```

#### Error Presentation
Standardized error messages with actionable feedback:

```gohtml
{{ define "error_template" }}
  Emoji: ⚠️ (Warning) or ❌ (Error)
  Title: Clear error category
  Description: Specific problem and solution
  Auto-deletion: Temporary display for non-critical errors
{{ end }}
```

## Security Architecture

### 1. Permission Model

#### Role-Based Access Control
```gohtml
{{ $staffRoleID := toInt ($rolesDict.Get "Staff") }}
{{ $permissionCheck := hasRoleID $staffRoleID }}
{{ if not $permissionCheck }}
    {{ /* Deny access */ }}
{{ end }}
```

#### Scope-Based Permissions
- **Global Scope**: Server-wide settings (Staff only)
- **User Scope**: Personal data access
- **Channel Scope**: Channel-specific operations

### 2. Input Validation

#### Regex-Based Validation
```gohtml
{{ $messageLink := reFind "https://(ptb.|canary.)?discord(?:app)?.com/channels/\\d{16,}/\\d{16,}/\\d{16,}" $input }}
{{ $snowflakeID := reFind "\\A\\d{16,}\\z" $input }}
```

#### Type Safety
```gohtml
{{ $safeInt := toInt (or $input 0) }}
{{ $safeString := toString $input }}
```

### 3. Error Containment

#### Try-Catch Blocks
```gohtml
{{ try }}
    {{ /* Risky operation */ }}
{{ catch }}
    {{ /* Fallback handling */ }}
{{ end }}
```

#### Graceful Degradation
Commands continue functioning even when dependent services fail.

## Performance Architecture

### 1. Optimization Strategies

#### Lazy Loading
Configuration dictionaries are loaded only when needed:

```gohtml
{{ $configDict := or .ExecData.Config (dbGet 0 "Config").Value }}
```

#### Efficient String Operations
Use `joinStr` instead of concatenation for better performance:

```gohtml
{{ $result := joinStr "" "Hello " "world" }}  {{/* Preferred */}}
{{ $result := add "Hello " "world" }}         {{/* Avoid */}}
```

#### Bulk Operations
Process multiple items in single database operations:

```gohtml
{{ range $key, $value := $bulkData }}
    {{ $dict.Set $key $value }}
{{ end }}
{{ dbSet $userID "BulkCategory" $dict }}
```

### 2. Resource Management

#### Memory Efficiency
- Reuse dictionary objects
- Clear large variables after use
- Use slices for collections

#### Rate Limiting
- Built-in trigger deletion delays
- Response message auto-deletion
- Execution count limits

## Integration Architecture

### 1. Discord API Integration

#### Message Operations
```gohtml
{{ $messageID := sendMessageRetID $channelID $embed }}
{{ deleteMessage $channelID $messageID $delay }}
{{ addMessageReactions $channelID $messageID $emoji }}
```

#### Member Management
```gohtml
{{ $member := getMember $userID }}
{{ giveRoleID $userID $roleID }}
{{ takeRoleID $userID $roleID }}
```

### 2. External Service Integration

#### URL Validation and Processing
```gohtml
{{ $validURL := reFind "https://[\\w.-]+\\.[a-z]{2,}" $input }}
```

#### Data Format Conversion
```gohtml
{{ $jsonData := json $dictData }}
{{ $dictData := jsonToSdict $jsonString }}
```

## Extensibility Architecture

### 1. Plugin System
New commands integrate seamlessly by following the established patterns:

1. **Header Declaration**: Author, trigger, dependencies
2. **Configuration Loading**: Standard dictionary loading
3. **Business Logic**: Command-specific functionality
4. **Output Generation**: Use `embed_exec` service
5. **Cleanup**: Standard trigger deletion

### 2. Configuration Extension
New configuration categories can be added without modifying existing commands:

```gohtml
{{ $myCustomDict := or (dbGet 0 "MyCustom").Value sdict }}
{{ $myCustomDict.Set "NewSetting" "NewValue" }}
{{ dbSet 0 "MyCustom" $myCustomDict }}
```

### 3. Service Extension
New services follow the dependency injection pattern:

```gohtml
{{- /*
  Dependencies: `embed_exec`, `my_new_service`
*/ -}}

{{ $myNewService := toInt ($commandsDict.Get "my_new_service") }}
{{ execCC $myNewService .Channel.ID 0 $serviceData }}
```

## Deployment Architecture

### 1. Bootstrap Process
The `bootstrap.gohtml` command initializes the entire system:

1. Create base configuration dictionaries
2. Set default values for all settings
3. Register command ID mappings
4. Configure role and channel associations
5. Initialize specialized systems (Gematria, etc.)

### 2. Incremental Updates
Individual commands can be updated without system-wide disruption:

- Configuration changes propagate automatically
- New dependencies are resolved at runtime
- Backward compatibility is maintained through default values

### 3. Monitoring and Maintenance
Built-in systems for operational oversight:

- Comprehensive logging to designated channels
- Error tracking and reporting
- Performance monitoring through execution delays
- Automated cleanup of temporary data

This architecture ensures scalability, maintainability, and reliability while providing a rich feature set for Discord server management.