# YAGPDB Custom Commands - Project Configuration

@~/.claude/CLAUDE.md

> **📚 Skills**: See `.claude/skills/` for specialized guidance on templates and emulator usage.

## Project Overview

This repository contains a comprehensive suite of custom commands for the YAGPDB Discord bot, designed to provide advanced server management capabilities through YAGPDB's `.gohtml` templating system.

### Key Features

- **User Onboarding**: Automated guest admission and agreement system
- **Staff Management**: Administrative tools for user screening, role management, and moderation
- **Server Utilities**: General-purpose commands including Hebrew/Gematria calculators, color analysis, bump management, and more
- **Database-Driven Configuration**: Flexible settings management through YAGPDB's built-in database
- **Modular Architecture**: Service-oriented design with reusable components

### Command Categories

1. **Guest Commands** (`guests/`) - User onboarding and agreement acceptance
2. **Staff Utility Commands** (`staff_utility/`) - Administrative and moderation tools
3. **Utility Commands** (`utility/`) - General-purpose commands for all users

### Development Resources

- **YAGPDB Source Code** (`vendor/yagpdb/`): Contains the complete YAGPDB codebase for reference. When researching YAGPDB template functions, available methods, or implementation details, **always check this directory first** instead of doing web searches. Key files:
  - `common/templates/general.go` - Template function implementations (sendMessage, complexMessage, cembed, etc.)
  - `common/templates/context.go` - Template execution context and data structures
  - `lib/discordgo/` - Discord API types and structures

- **Template Emulator** (`tools/emulator/`): Local testing tool that emulates YAGPDB template execution. Run templates locally before deploying to production.

## Architecture

### Core Components

1. **Database Structure**
   - Uses YAGPDB's hierarchical dictionary system:
     - `Global` - Server-wide configuration settings
     - `Commands` - Command ID mappings for cross-command references
     - `Roles` - Role ID storage and management
     - `Channels` - Channel ID configuration
     - `Admin` - Administrative settings and permissions
     - Specialized dictionaries (e.g., `Gematria`, `Knowledge`, `Rules`)

2. **Service Commands**
   - `embed_exec` - Universal embed creation service
   - `db` - Database operations interface
   - `message_link` - Message reference and linking service
   - `simple_db_edit` / `simple_db_lookup` - Database management utilities

3. **Command Structure**
   - Input validation using regex patterns
   - Permission checking based on roles
   - Consistent error handling with try-catch blocks
   - Embed-based responses for rich formatting
   - Auto-cleanup with trigger message deletion

### Data Flow

1. **Command Execution**:
   ```
   User Input → Permission Check → Input Validation → Business Logic → Database Operations → Response Generation → Message Cleanup
   ```

2. **Cross-Command Communication**:
   - Commands store their IDs in the database
   - Other commands can invoke them using `execCC`
   - Shared state through database dictionaries

3. **Service Pattern**:
   - Service commands act as libraries
   - Called via `execCC` with structured arguments
   - Return structured data or perform side effects

## Code Style

### YAGPDB Template Guidelines

- **Indentation**: Use tabs for `.gohtml` templates (YAGPDB convention)
- **Variable Naming**: 
  - Use `$camelCase` for template variables
  - Prefix with `$` for all variables (YAGPDB requirement)
  - Use descriptive names (e.g., `$targetUser`, `$embedColor`)
- **Line Length**: Keep under 100 characters where possible
- **Template Structure**:
  ```gohtml
  {{/* Command description and usage */}}
  {{/* Input validation */}}
  {{/* Permission checks */}}
  {{/* Main logic */}}
  {{/* Response generation */}}
  {{/* Cleanup */}}
  ```

### Best Practices

- Always include command documentation at the top
- Use `try-catch` blocks for error handling
- Delete trigger messages when appropriate
- Use embeds for rich responses
- Validate all user inputs with regex when needed
- Store reusable data in database dictionaries

## Error Handling Guidelines

### YAGPDB-Specific Error Handling

1. **Try-Catch Pattern**:
   ```gohtml
   {{try}}
       {{/* Risky operation */}}
   {{catch}}
       {{/* Error handling */}}
       {{sendMessage nil (cembed 
           "color" 0xff0000 
           "title" "Error" 
           "description" (print "An error occurred: " .Error)
       )}}
   {{end}}
   ```

2. **Input Validation**:
   - Always validate user inputs before processing
   - Use regex for pattern matching
   - Provide clear error messages for invalid inputs

3. **Permission Errors**:
   - Check permissions early in command execution
   - Return user-friendly messages for permission denials
   - Log administrative actions when appropriate

4. **Database Errors**:
   - Handle missing keys gracefully
   - Provide defaults for missing configuration
   - Validate data types when retrieving from database

5. **Common Error Scenarios**:
   - Invalid user/role/channel mentions
   - Missing database entries
   - Exceeded character limits (20k for premium, 10k for free)
   - Rate limiting issues
   - Invalid command arguments

## Known Issues and Patterns

### YAGPDB Limitations

1. **Character Limits**:
   - Premium: 20,000 characters per command
   - Free: 10,000 characters per command
   - Use minification for large commands

2. **Execution Limits**:
   - Commands timeout after 10 seconds
   - Database operations have rate limits
   - Nested `execCC` calls have depth limits

3. **Template Quirks**:
   - Variable scope is global within a template
   - No true functions, only templates
   - Limited string manipulation functions
   - Case-sensitive template function names

### Common Patterns

1. **Database Initialization**:
   ```gohtml
   {{$db := dbGet 0 "Global"}}
   {{$settings := or $db.Value Dict}}
   ```

2. **Permission Checking**:
   ```gohtml
   {{$perms := getTargetPermissionsIn .User.ID .Channel.ID}}
   {{if not (or (.Permissions.Administrator) (ge $perms 8))}}
       {{/* No permission */}}
   {{end}}
   ```

3. **Mention Parsing**:
   ```gohtml
   {{$args := parseArgs 1 "Usage: command <user>" 
       (carg "userid" "target user")}}
   {{$target := userArg ($args.Get 0)}}
   ```

4. **Embed Responses**:
   ```gohtml
   {{sendMessage nil (cembed 
       "title" "Success"
       "color" 0x00ff00
       "description" "Operation completed")}}
   ```

## Claude Code Tool Usage Guidelines

### Approved Tools
The following tools are generally safe to use without explicit permission:

1. **File Operations and Basic Commands**
    - `Read` - Read file contents (always approved)
    - `Write` - Create new files or update existing files (approved for most files except configs)
    - `Edit` - Edit portions of files (approved for most files except configs)
    - `MultiEdit` - Make multiple edits to a file (approved for most files except configs)
    - `LS` - List files in a directory (always approved)
    - `Bash` with common commands:
        - `ls`, `pwd`, `find`, `grep` - Listing and finding files/content
        - `cp`, `mv` - Copying and moving files
        - `mkdir`, `rmdir`, `rm` - Creating and removing directories/files
        - `cat`, `head`, `tail` - Viewing file contents
        - `diff` - Comparing files
    - Create and delete directories (excluding configuration directories)
    - Move and rename files and directories

2. **File Search and Analysis**
    - `Glob` - Find files using glob patterns (always approved)
    - `Grep` - Search file contents with regular expressions (always approved)
    - `Search` - General purpose search tool for local filesystem (always approved)
    - `Task` - Use agent for file search and analysis (always approved)
    - `WebSearch` - Search the web for information (always approved)
    - `WebFetch` - Fetch content from specific URLs (always approved)

### Tools Requiring Approval
The following operations should be discussed before executing:

1. **Git Operations**
    - Do not push to remote repositories (will trigger deployment)
    - Commits are allowed but discuss significant changes first
    - Branch operations should be explicitly requested

### Best Practices
1. Use the Task agent when analyzing unfamiliar areas of the codebase
2. Use Batch to run multiple tools in parallel when appropriate
3. Never abandon challenging tasks or take shortcuts to avoid difficult work
4. If you need more time or context to properly complete a task, communicate this honestly
5. Take pride in your work and maintain high standards even when faced with obstacles

### YAGPDB Development Workflow

1. **Testing Commands**:
   - Use the linter (`python lint.py`) to check for syntax errors
   - Test in a development server before production
   - Check character count stays within limits
   - Verify database operations don't conflict

2. **Command Development**:
   - Start with the command structure template
   - Implement input validation first
   - Add permission checks early
   - Build core logic incrementally
   - Test edge cases thoroughly

3. **Database Management**:
   - Document all database keys used
   - Maintain consistency in naming
   - Clean up unused entries
   - Use the bootstrap system for initialization

4. **Code Organization**:
   - Keep related commands in the same directory
   - Use service commands for shared functionality
   - Maintain clear command descriptions
   - Update documentation when adding features

### Task Management and To-Do Lists
1. **Maintain Comprehensive To-Do Lists**: Use the TodoWrite and TodoRead tools extensively to create and manage detailed task lists.
    - Create a to-do list at the start of any non-trivial task or multi-step process
    - Be thorough and specific in task descriptions, including file paths and implementation details when relevant
    - Break down complex tasks into smaller, clearly defined subtasks
    - Include success criteria for each task when possible

2. **Prioritize and Track Progress Meticulously**:
    - Mark tasks as `in_progress` when starting work on them
    - Update task status to `completed` immediately after completing each task
    - Add new tasks that emerge during the work process
    - Provide detailed context for each task to ensure work can be resumed if the conversation is interrupted or context is reset

3. **Context Resilience Strategy**:
    - Write to-do lists with the assumption that context might be lost or compacted
    - Include sufficient detail in task descriptions to enable work continuation even with minimal context
    - When implementing complex solutions, document the approach and rationale in the to-do list
    - Regularly update the to-do list with your current progress and next steps

4. **Organize To-Do Lists by Component or Feature**:
    - Group related tasks together
    - Maintain a hierarchical structure where appropriate
    - Include dependencies between tasks when they exist
    - For test-related tasks, include specifics about test expectations and mocking requirements