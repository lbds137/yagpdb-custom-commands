#!/usr/bin/env python3
"""Sanity-check deploy/panel.json against the command tree, run in `make ci`.

- Every mapped path exists (tracked in the working tree) and isn't under retired/.
- Every non-retired command file (everyone/**, staff/**) has a "main" id.
- Ids are unique per server.

The expected set of command files is derived from the tree every run (never a hardcoded
count), via `git ls-files` against the working tree -- not `git ls-tree HEAD`, so an
uncommitted `git mv` (e.g. retiring a command) is picked up immediately rather than only
after a commit.
"""
import json
import subprocess
import sys

PANEL_JSON = "deploy/panel.json"
COMMAND_DIRS = ("everyone", "staff")


def list_command_files():
    out = subprocess.run(
        ["git", "ls-files"] + list(COMMAND_DIRS),
        capture_output=True, text=True, check=True,
    ).stdout
    return {line for line in out.splitlines() if line.endswith(".gohtml")}


def main() -> int:
    errors = []

    with open(PANEL_JSON, encoding="utf-8") as f:
        panel = json.load(f)

    mapped = panel["commands"]
    all_files = list_command_files()

    for path in mapped:
        if path.startswith("retired/"):
            errors.append(f"{path}: mapped but under retired/")
            continue
        if path not in all_files:
            errors.append(f"{path}: mapped but does not exist at HEAD")

    for path in sorted(all_files):
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

    print(f"panel.json check OK: {len(mapped)} mapped commands, {len(all_files)} command files")
    return 0


if __name__ == "__main__":
    sys.exit(main())
