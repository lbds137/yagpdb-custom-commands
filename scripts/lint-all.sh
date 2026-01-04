#!/bin/bash
# Lint all YAGPDB templates using the Python linter
# Usage: ./scripts/lint-all.sh [--fix] [--verbose]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
LINTER="$PROJECT_ROOT/tools/linter/yagpdb_lint.py"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
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

echo -e "${BLUE}=== Linting All Templates ===${NC}"
echo ""

# Counters
PASS=0
FAIL=0
TOTAL=0

lint_template() {
    local template="$1"
    ((TOTAL++)) || true

    local output
    local exit_code=0
    output=$(python3 "$LINTER" "$template" 2>&1) || exit_code=$?

    if [[ $exit_code -eq 0 ]] && [[ -z "$output" || "$output" == *"OK"* || "$output" == *"pass"* ]]; then
        echo -e "${GREEN}✓${NC} $template"
        ((PASS++)) || true
        if [[ "$VERBOSE" == "true" && -n "$output" ]]; then
            echo "$output" | sed 's/^/  /'
        fi
    else
        echo -e "${RED}✗${NC} $template"
        ((FAIL++)) || true
        if [[ -n "$output" ]]; then
            echo "$output" | sed 's/^/  /'
        fi
    fi
}

# Lint utility templates
echo -e "${BLUE}--- Utility Commands ---${NC}"
for template in "$PROJECT_ROOT"/utility/*.gohtml; do
    if [[ -f "$template" ]]; then
        lint_template "$template"
    fi
done

echo ""

# Lint staff_utility templates
echo -e "${BLUE}--- Staff Utility Commands ---${NC}"
for template in "$PROJECT_ROOT"/staff_utility/*.gohtml; do
    if [[ -f "$template" ]]; then
        lint_template "$template"
    fi
done

echo ""

# Summary
echo -e "${BLUE}=== Summary ===${NC}"
echo -e "Total: $TOTAL | ${GREEN}Passed: $PASS${NC} | ${RED}Failed: $FAIL${NC}"

if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
