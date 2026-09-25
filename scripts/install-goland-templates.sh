#!/bin/bash
# Install the YAGPDB live templates into every GoLand config on this machine.
# Restart GoLand afterwards; the templates appear under Settings → Editor → Live Templates → YAGPDB.
# Usage: ./scripts/install-goland-templates.sh

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="$PROJECT_ROOT/tools/ide/goland/YAGPDB.xml"

case "$(uname -s)" in
    Darwin) CONFIG_ROOT="$HOME/Library/Application Support/JetBrains" ;;
    *) CONFIG_ROOT="${XDG_CONFIG_HOME:-$HOME/.config}/JetBrains" ;;
esac

shopt -s nullglob
configs=("$CONFIG_ROOT"/GoLand*)
if [[ ${#configs[@]} -eq 0 ]]; then
    echo "No GoLand config found under $CONFIG_ROOT. Start GoLand once, then rerun this." >&2
    exit 1
fi

for config in "${configs[@]}"; do
    mkdir -p "$config/templates"
    cp "$SOURCE" "$config/templates/YAGPDB.xml"
    echo "Installed to $config/templates/YAGPDB.xml"
done
echo "Restart GoLand to load them."
