# YAGPDB Template Development

## Quick Reference

### File Structure
```
guests/          - User onboarding commands
staff_utility/   - Admin/moderation tools
utility/         - General-purpose commands
```

### Common Patterns

**Configuration Loading**:
```gohtml
{{ $globalDict := (dbGet 0 "Global").Value }}
{{ $commandsDict := (dbGet 0 "Commands").Value }}
{{ $embed_exec := toInt ($commandsDict.Get "embed_exec") }}
```

**Permission Check**:
```gohtml
{{ $rolesDict := (dbGet 0 "Roles").Value }}
{{ $staffRoleID := toInt ($rolesDict.Get "Staff") }}
{{ if not (hasRoleID $staffRoleID) }}
    {{/* Deny access */}}
{{ end }}
```

**Error Handling**:
```gohtml
{{ try }}
    {{ /* Risky operation */ }}
{{ catch }}
    {{ /* Handle error gracefully */ }}
{{ end }}
```

**Embed Response via embed_exec**:
```gohtml
{{ execCC $embed_exec $channelID 0 (sdict
    "ChannelID" .Channel.ID
    "Title" "Title Here"
    "Description" "Content here"
    "DeleteResponse" true
    "DeleteDelay" $deleteResponseDelay
) }}
```

### Database Dictionaries

| Key | Purpose |
|-----|---------|
| `Global` | Server-wide settings (delays, colors, limits) |
| `Commands` | Command ID mappings for execCC |
| `Roles` | Role ID storage |
| `Channels` | Channel ID configuration |
| `Admin` | Administrative settings |
| `Gematria` | Hebrew letter values |
| `Rules` | Server rules |
| `Knowledge` | KB articles |

### Service Commands

- **embed_exec** - Universal embed creation
- **db** - Database operations (get/set/add/remove/delete/dump)
- **message_link** - Message reference handling

### Limits

| Resource | Free | Premium |
|----------|------|---------|
| Command size | 10k chars | 20k chars |
| Operations per run (no time limit) | 1,000,000 | 2,500,000 |
| `sleep` per run | 60 sec | 60 sec |
| Output per run | 25,000 bytes | 25,000 bytes |
| `execCC` + `scheduleUniqueCC` calls per run | 1 | 10 |
| `sendDM` per run | 1 | 1 |
| Command response (YAGPDB replaces a longer one) | 2,000 chars | 2,000 chars |
| Nested immediate `execCC` | 2 deep | 2 deep |
| Database calls per run | 10 | 50 |
| Database entries per server | 50 × members | 500 × members |
| Discord API calls per run | 100 | 100 |
| Message content (Discord) | 2,000 chars | 2,000 chars |
| Embed description (Discord) | 4,096 chars | 4,096 chars |

Details and sources: `docs/API_REFERENCE.md` → YAGPDB System Limits.

### Research First

When investigating YAGPDB functions:
1. Check `vendor/yagpdb/common/templates/general.go` first
2. Check `vendor/yagpdb/common/templates/context.go` for context
3. Only web search if not found locally

## When to Use This Skill

- Writing new `.gohtml` template commands
- Debugging existing templates
- Understanding database structure
- Looking up YAGPDB function signatures
