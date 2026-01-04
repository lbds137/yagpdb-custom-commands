#!/bin/bash
# Test all YAGPDB templates against the emulator
# Usage: ./scripts/test-all-templates.sh [--verbose] [--stop-on-fail]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
YAGTEST="$PROJECT_ROOT/bin/yagtest"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
PASS=0
FAIL=0
SKIP=0

# Options
VERBOSE=false
STOP_ON_FAIL=false

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --verbose       Show detailed output for each template"
    echo "  --stop-on-fail  Stop on first failure"
    echo "  --help          Show this help message"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --stop-on-fail|-s)
            STOP_ON_FAIL=true
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

# Check if yagtest exists
if [[ ! -x "$YAGTEST" ]]; then
    echo -e "${YELLOW}Building yagtest...${NC}"
    make -C "$PROJECT_ROOT" build-emulator
fi

echo -e "${BLUE}=== Testing All Templates ===${NC}"
echo ""

test_template() {
    local template="$1"
    local basename
    basename=$(basename "$template")

    # Run the template
    local output
    local exit_code=0
    output=$(timeout 5 "$YAGTEST" run "$template" 2>&1) || exit_code=$?

    # Check for errors
    if [[ $exit_code -ne 0 ]] || echo "$output" | grep -qi "error"; then
        echo -e "${RED}FAIL${NC}: $template"
        ((FAIL++)) || true

        if [[ "$VERBOSE" == "true" ]]; then
            echo "$output" | head -10 | sed 's/^/  /'
            echo ""
        fi

        if [[ "$STOP_ON_FAIL" == "true" ]]; then
            echo -e "\n${RED}Stopping on first failure${NC}"
            exit 1
        fi
    else
        echo -e "${GREEN}PASS${NC}: $template"
        ((PASS++)) || true

        if [[ "$VERBOSE" == "true" ]]; then
            echo "$output" | head -5 | sed 's/^/  /'
            echo ""
        fi
    fi
}

# Test utility templates
echo -e "${BLUE}--- Utility Commands ---${NC}"
for template in "$PROJECT_ROOT"/utility/*.gohtml; do
    if [[ -f "$template" ]]; then
        test_template "$template"
    fi
done

echo ""

# Test staff_utility templates
echo -e "${BLUE}--- Staff Utility Commands ---${NC}"
for template in "$PROJECT_ROOT"/staff_utility/*.gohtml; do
    if [[ -f "$template" ]]; then
        test_template "$template"
    fi
done

echo ""

# Summary
TOTAL=$((PASS + FAIL + SKIP))
echo -e "${BLUE}=== Summary ===${NC}"
echo -e "Total: $TOTAL | ${GREEN}Passed: $PASS${NC} | ${RED}Failed: $FAIL${NC} | Skipped: $SKIP"

if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
