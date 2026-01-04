#!/bin/bash
# Quick search for YAGPDB template functions
# Usage: ./search-functions.sh <pattern>
#   e.g., ./search-functions.sh "db"
#   e.g., ./search-functions.sh "sendMessage"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
YAGPDB_DIR="$SCRIPT_DIR/yagpdb"

if [ -z "$1" ]; then
    echo "Usage: $0 <search_pattern>"
    echo ""
    echo "Examples:"
    echo "  $0 db           # Find database functions"
    echo "  $0 sendMessage  # Find message sending functions"
    echo "  $0 'func.*Get'  # Regex: functions starting with Get"
    exit 1
fi

echo "=== Searching YAGPDB source for: $1 ==="
echo ""

# Search in template definitions
echo "--- Template Function Definitions ---"
grep -rn "$1" "$YAGPDB_DIR/common/templates/" --include="*.go" 2>/dev/null | head -30

echo ""
echo "--- Core Template Library ---"
grep -rn "$1" "$YAGPDB_DIR/lib/template/" --include="*.go" 2>/dev/null | head -20
