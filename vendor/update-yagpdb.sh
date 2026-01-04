#!/bin/bash
# Update YAGPDB source reference
# This script clones/updates the YAGPDB bot source code for reference

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
YAGPDB_DIR="$SCRIPT_DIR/yagpdb"

echo "=== YAGPDB Source Reference Updater ==="

if [ -d "$YAGPDB_DIR" ]; then
    echo "Updating existing YAGPDB clone..."
    cd "$YAGPDB_DIR"
    git fetch origin
    git reset --hard origin/master
    echo "Updated to: $(git rev-parse --short HEAD)"
else
    echo "Cloning YAGPDB repository..."
    git clone --depth 1 https://github.com/botlabs-gg/yagpdb.git "$YAGPDB_DIR"
    echo "Cloned at: $(cd "$YAGPDB_DIR" && git rev-parse --short HEAD)"
fi

echo ""
echo "Key directories for template development:"
echo "  - $YAGPDB_DIR/lib/template/"
echo "  - $YAGPDB_DIR/common/templates/"
echo ""
echo "Done! Use 'grep' to search for available template functions."
