#!/bin/bash
# Analyze templates to find functions not yet implemented in the emulator
# Usage: ./scripts/find-missing-functions.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Analyzing Template Functions ===${NC}"
echo ""

# Known implemented functions (from engine.go FuncMap)
IMPLEMENTED=(
    # Type conversion
    "str" "toString" "toInt" "toInt64" "toFloat" "toDuration" "toRune" "toByte"
    # String manipulation
    "lower" "upper" "title" "hasPrefix" "hasSuffix" "trimSpace" "split" "joinStr"
    "slice" "urlescape" "urlunescape" "print" "println" "printf"
    # Math
    "add" "sub" "mult" "div" "fdiv" "mod" "abs" "sqrt" "cbrt" "pow" "log"
    "round" "roundCeil" "roundFloor" "roundEven" "min" "max"
    # Collections
    "dict" "sdict" "cslice" "json" "jsonToSdict" "sort"
    # Time
    "currentTime" "formatTime" "parseTime" "newDate"
    # Regex
    "reFind" "reFindAll" "reReplace" "reSplit" "reQuoteMeta"
    # Utilities
    "in" "inFold" "kindOf" "seq" "randInt"
    # Database
    "dbGet" "dbSet" "dbSetExpire" "dbDel" "dbDelById" "dbDelByID" "dbIncr"
    "dbGetPattern" "dbGetPatternReverse" "dbCount" "dbTopEntries" "dbBottomEntries" "dbRank"
    # Discord - Messages
    "sendMessage" "sendMessageRetID" "sendDM" "editMessage" "getMessage" "deleteMessage"
    "deleteTrigger" "deleteResponse" "addReactions" "addMessageReactions"
    "deleteAllMessageReactions"
    # Mentions
    "mentionRoleID" "mentionRole" "mentionEveryone" "mentionHere"
    # Discord - Roles
    "hasRole" "hasRoleID" "targetHasRole" "targetHasRoleID" "addRole" "giveRole" "removeRole" "takeRole"
    "setRoles" "giveRoleID" "takeRoleID" "addRoleID" "removeRoleID" "getRole"
    # Discord - Members/Users
    "getMember" "userArg" "getTargetPermissionsIn"
    # Discord - Channels
    "getChannel" "getChannelOrThread"
    # Discord - Tickets
    "createTicket"
    # Embeds
    "cembed" "complexMessage" "complexMessageEdit" "sendTemplate"
    # Control flow
    "execCC" "exec" "execAdmin" "execTemplate" "scheduleUniqueCC" "cancelScheduledUniqueCC"
    "sleep" "catch"
    # Args
    "parseArgs" "carg"
    # Logic (built-in and custom)
    "or" "and" "not" "eq" "ne" "lt" "le" "gt" "ge" "len" "index" "return" "try"
    # Go template built-ins
    "if" "else" "end" "range" "with" "define" "template" "block" "nil" "true" "false"
)

# Extract function-like patterns from templates
echo -e "${YELLOW}Extracting functions from templates...${NC}"

TEMP_FILE=$(mktemp)
trap 'rm -f "$TEMP_FILE"' EXIT

# Find all function calls in templates
grep -rohE '\{\{-?\s*[a-zA-Z_][a-zA-Z0-9_]*' "$PROJECT_ROOT"/utility/*.gohtml "$PROJECT_ROOT"/staff_utility/*.gohtml 2>/dev/null | \
    sed 's/{{-\?[[:space:]]*//' | \
    grep -v '^\$' | \
    sort | uniq -c | sort -rn > "$TEMP_FILE"

echo ""
echo -e "${BLUE}=== Functions Used in Templates ===${NC}"
echo ""

# Check each function
MISSING=()
while read -r count func; do
    # Skip empty lines and comments
    [[ -z "$func" ]] && continue
    [[ "$func" == "/*" ]] && continue
    [[ "$func" == "*/" ]] && continue

    # Check if implemented
    is_implemented=false
    for impl in "${IMPLEMENTED[@]}"; do
        if [[ "$func" == "$impl" ]]; then
            is_implemented=true
            break
        fi
    done

    if [[ "$is_implemented" == "true" ]]; then
        printf "${GREEN}✓${NC} %-25s (used %3d times)\n" "$func" "$count"
    else
        printf "${RED}✗${NC} %-25s (used %3d times) ${YELLOW}NOT IMPLEMENTED${NC}\n" "$func" "$count"
        MISSING+=("$func:$count")
    fi
done < "$TEMP_FILE"

echo ""
echo -e "${BLUE}=== Summary ===${NC}"

if [[ ${#MISSING[@]} -eq 0 ]]; then
    echo -e "${GREEN}All functions are implemented!${NC}"
else
    echo -e "${YELLOW}Missing functions:${NC}"
    for item in "${MISSING[@]}"; do
        func="${item%:*}"
        count="${item#*:}"
        echo -e "  ${RED}$func${NC} (used $count times)"
    done
    echo ""
    echo -e "Total missing: ${RED}${#MISSING[@]}${NC}"
fi

# Show which files use missing functions
if [[ ${#MISSING[@]} -gt 0 ]]; then
    echo ""
    echo -e "${BLUE}=== Files Using Missing Functions ===${NC}"
    for item in "${MISSING[@]}"; do
        func="${item%:*}"
        echo -e "\n${YELLOW}$func${NC}:"
        grep -l "\\b$func\\b" "$PROJECT_ROOT"/utility/*.gohtml "$PROJECT_ROOT"/staff_utility/*.gohtml 2>/dev/null | \
            sed "s|$PROJECT_ROOT/||" | sed 's/^/  /'
    done
fi
