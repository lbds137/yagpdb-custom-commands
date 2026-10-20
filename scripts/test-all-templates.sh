#!/bin/bash
# Smoke test: run every command once, with no arguments, on the database a fresh
# bootstrap plus config_sync leave (tools/emulator/testdata/initial_db.json). A command that answers with
# its parseArgs usage message passes, as does one that exits 0 and prints a Usage: line; either way its
# usage text must match the pin in scripts/bare-usage/ (same path, .txt), so a wording change
# fails. Regex-trigger commands are skipped, since an empty message can't match their trigger.
# What's left failing is a real error. The YAML suites (make test) test commands with arguments.
# Usage: ./scripts/test-all-templates.sh [--verbose] [--stop-on-fail]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
YAGTEST="$PROJECT_ROOT/bin/yagtest"
DB="$PROJECT_ROOT/tools/emulator/testdata/initial_db.json"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
PASS=0
USAGE=0
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
    local name="${template#"$PROJECT_ROOT"/}"
    local pin="$SCRIPT_DIR/bare-usage/${name#commands/}"
    pin="${pin%.gohtml}.txt"

    if grep -q 'Trigger type: `Regex`' "$template"; then
        echo -e "${YELLOW}SKIP${NC}: $name (regex trigger)"
        ((SKIP++)) || true
        return
    fi

    local output
    local exit_code=0
    output=$(timeout 5 "$YAGTEST" run -db "$DB" "$template" 2>&1) || exit_code=$?
    local plain
    plain=$(echo "$output" | sed 's/\x1b\[[0-9;]*m//g')

    # A run that asks for arguments prints its usage between the Execution Error line and
    # the Hint line. A run that exits 0 and prints a Usage: line (gematria) prints it all.
    # Match the line start only: a loose "usage" also hits the checkout path in yagtest warnings.
    local asks_for_args=false
    local prints_usage=false
    local usage=""
    if [[ $exit_code -ne 0 ]] && echo "$plain" | grep -q 'executing "yagtest" at <parseArgs'; then
        asks_for_args=true
        prints_usage=true
        usage=$(echo "$plain" | awk '/^Execution Error:/{on=1; next} /^Hint:/{exit} on')
    elif [[ $exit_code -eq 0 ]] && echo "$plain" | grep -q '^Usage:'; then
        prints_usage=true
        usage="$plain"
    fi

    local usage_problem=""
    if [[ -f "$pin" ]]; then
        if [[ "$prints_usage" != "true" ]]; then
            usage_problem="pinned, but prints no usage"
        elif [[ "$usage" != "$(cat "$pin")" ]]; then
            usage_problem="usage text changed"
        fi
    elif [[ "$prints_usage" == "true" ]]; then
        usage_problem="no pinned usage text"
    fi

    if [[ -n "$usage_problem" ]]; then
        echo -e "${RED}FAIL${NC}: $name ($usage_problem)"
        ((FAIL++)) || true
        if [[ -f "$pin" ]]; then
            echo "  expected (${pin#"$PROJECT_ROOT"/}):"
            sed 's/^/    /' "$pin"
        fi
        echo "  actual:"
        if [[ "$prints_usage" == "true" ]]; then
            printf '%s\n' "$usage" | sed 's/^/    /'
        else
            echo "$plain" | head -5 | sed 's/^/    /'
        fi
        echo ""
        if [[ "$STOP_ON_FAIL" == "true" ]]; then
            echo -e "\n${RED}Stopping on first failure${NC}"
            exit 1
        fi
        return
    fi

    if [[ $exit_code -eq 0 ]]; then
        echo -e "${GREEN}PASS${NC}: $name"
        ((PASS++)) || true
        if [[ "$VERBOSE" == "true" ]]; then
            echo "$output" | head -5 | sed 's/^/  /'
            echo ""
        fi
    elif [[ "$asks_for_args" == "true" ]]; then
        echo -e "${GREEN}PASS${NC}: $name (asks for arguments)"
        ((USAGE++)) || true
    else
        echo -e "${RED}FAIL${NC}: $name"
        ((FAIL++)) || true
        if [[ "$VERBOSE" == "true" ]]; then
            echo "$output" | head -10 | sed 's/^/  /'
            echo ""
        fi
        if [[ "$STOP_ON_FAIL" == "true" ]]; then
            echo -e "\n${RED}Stopping on first failure${NC}"
            exit 1
        fi
    fi
}

# Test every command (commands/<topic>/, both panel groups)
echo -e "${BLUE}--- Commands ---${NC}"
while IFS= read -r template; do
    test_template "$template"
done < <(find "$PROJECT_ROOT/commands" -name '*.gohtml' | sort)

echo ""

# Summary
TOTAL=$((PASS + USAGE + FAIL + SKIP))
echo -e "${BLUE}=== Summary ===${NC}"
echo -e "Total: $TOTAL | ${GREEN}Passed: $PASS${NC} | ${GREEN}Asked for arguments: $USAGE${NC} | ${RED}Failed: $FAIL${NC} | Skipped: $SKIP"

if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
