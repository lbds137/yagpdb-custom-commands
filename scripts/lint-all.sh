#!/bin/bash
# Lint all YAGPDB templates in commands/ with a single Python linter invocation
# Usage: ./scripts/lint-all.sh [--fix] [--verbose]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
LINTER="$PROJECT_ROOT/tools/linter/yagpdb_lint.py"

# Colors
RED='\033[0;31m'
NC='\033[0m'

# Options
VERBOSE=false
FIX=false

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --verbose, -v   Show detailed lint output"
    echo "  --fix           Attempt to fix issues (if linter supports it)"
    echo "  --help, -h      Show this help message"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --fix)
            FIX=true
            shift
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Check if linter exists
if [[ ! -f "$LINTER" ]]; then
    echo -e "${RED}Error: Linter not found at $LINTER${NC}"
    exit 1
fi

ARGS=()
if [[ "$VERBOSE" == "true" ]]; then
    ARGS+=(-v)
fi
if [[ "$FIX" == "true" ]]; then
    ARGS+=(--fix)
fi

exit_code=0
python3 "$LINTER" --dir "$PROJECT_ROOT/commands" "${ARGS[@]}" || exit_code=$?
exit $exit_code
