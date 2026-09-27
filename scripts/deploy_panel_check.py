#!/usr/bin/env python3
"""Sanity-check deploy/panel.json against the command tree, run in `make ci`.

- Every mapped path exists (tracked in the working tree) and isn't under retired/.
- Every non-retired, non-unmanaged command file (everyone/**, staff/**) has a "main" id.
- Ids are unique per server.
- Every "unmanaged" path exists in the working tree and does NOT also appear in "commands"
  (an unmanaged file has no id and is never touched by the deploy tooling).

The expected set of command files is derived from the tree every run (never a hardcoded
count), via `git ls-files` (tracked plus untracked, non-ignored) against the working tree -- not
`git ls-tree HEAD`, so an
uncommitted `git mv` (e.g. retiring a command) is picked up immediately rather than only
after a commit.
"""
import argparse
import json
import subprocess
import sys

PANEL_JSON = "deploy/panel.json"
COMMAND_DIRS = ("everyone", "staff")


def list_command_files():
    out = subprocess.run(
        # --others: a new, not yet added command counts too (else a worker's tree passes)
        ["git", "ls-files", "--cached", "--others", "--exclude-standard"] + list(COMMAND_DIRS),
        capture_output=True, text=True, check=True,
    ).stdout
    return {line for line in out.splitlines() if line.endswith(".gohtml")}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--panel", default=PANEL_JSON, help="path to panel.json to check")
    args = parser.parse_args()

    errors = []

    with open(args.panel, encoding="utf-8") as f:
        panel = json.load(f)

    mapped = panel["commands"]
    unmanaged = set(panel.get("unmanaged", []))
    all_files = list_command_files()

    for path in mapped:
        if path.startswith("retired/"):
            errors.append(f"{path}: mapped but under retired/")
            continue
        if path not in all_files:
            errors.append(f"{path}: mapped but does not exist in the working tree")
        if path in unmanaged:
            errors.append(f"{path}: mapped in commands but also listed as unmanaged")

    for path in unmanaged:
        if path not in all_files:
            errors.append(f"{path}: unmanaged but does not exist in the working tree")

    for path in sorted(all_files - unmanaged):
        ids = mapped.get(path)
        if ids is None or "main" not in ids:
            errors.append(f"{path}: no 'main' id in panel.json")

    for server in panel["servers"]:
        seen = {}
        for path, ids in mapped.items():
            if server not in ids:
                continue
            server_id = ids[server]
            if server_id in seen:
                errors.append(
                    f"server {server}: id {server_id} used by both "
                    f"{seen[server_id]} and {path}"
                )
            else:
                seen[server_id] = path

    if errors:
        print(f"panel.json check failed ({len(errors)} problem(s)):", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        return 1

    print(
        f"panel.json check OK: {len(mapped)} mapped commands, {len(all_files)} command "
        f"files, {len(unmanaged)} unmanaged"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
