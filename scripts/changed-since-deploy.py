#!/usr/bin/env python3
"""Build the manual paste list for `make changed-since-deploy`.

Runs `git diff --name-status -M deployed -- <dirs>` (cwd-relative, so it works against
whatever git repo the caller is standing in) through the unchanged
scripts/changed-since-deploy.awk (its A/M/R/D logic is not reimplemented here), then
excludes every path deploy/panel.json's "unmanaged" list names -- pasting one of those
would overwrite the live copy's real values with the placeholder committed here. Each
excluded path still gets a line explaining why it was skipped, so the owner isn't left
wondering why a changed file didn't show up.

Usage: scripts/changed-since-deploy.py <dir> [<dir> ...] [--panel <path>]
"""
import argparse
import json
import os
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
AWK_SCRIPT = os.path.join(SCRIPT_DIR, "changed-since-deploy.awk")
DELETED_PREFIX = "deleted (remove from YAGPDB): "


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("dirs", nargs="+", help="pathspec dirs to diff (DEPLOY_DIRS)")
    parser.add_argument("--panel", default="deploy/panel.json")
    args = parser.parse_args()

    diff = subprocess.run(
        ["git", "diff", "--name-status", "-M", "deployed", "--", *args.dirs],
        capture_output=True, text=True, check=True,
    ).stdout

    awk_out = subprocess.run(
        ["awk", "-f", AWK_SCRIPT],
        input=diff, capture_output=True, text=True, check=True,
    ).stdout

    with open(args.panel, encoding="utf-8") as f:
        panel = json.load(f)
    unmanaged = set(panel.get("unmanaged", []))

    paste_lines = []
    skipped_lines = []
    for line in awk_out.splitlines():
        if not line:
            continue
        if line.startswith(DELETED_PREFIX):
            path = line[len(DELETED_PREFIX):]
            if path in unmanaged:
                skipped_lines.append(f"skipped (unmanaged, never touch in YAGPDB): {path}")
            else:
                paste_lines.append(line)
        else:
            path = line
            if path in unmanaged:
                skipped_lines.append(f"skipped (unmanaged, never paste): {path}")
            else:
                paste_lines.append(line)

    if paste_lines:
        for line in paste_lines:
            print(line)
    else:
        rev = subprocess.run(
            ["git", "log", "-1", "--format=%h %as", "deployed"],
            capture_output=True, text=True, check=True,
        ).stdout.strip()
        print(f"Nothing to paste: no command changed since {rev}")

    for line in skipped_lines:
        print(line)

    return 0


if __name__ == "__main__":
    sys.exit(main())
