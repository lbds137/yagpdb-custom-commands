#!/usr/bin/env python3
"""Test scripts/deploy_panel_check.py's error branches, run in `make ci`.

Builds a throwaway git repo under the system temp dir (left in place -- never removed, per
this repo's landmines) with a panel.json rigged to hit three error branches at once, and
runs the real check (via its `--panel` option) against it -- not a reimplementation:

- a file listed in "commands" that is also listed as "unmanaged" (mapped AND unmanaged)
- an "unmanaged" path that does not exist in the working tree
- a normal file with no id on any server in panel.json (checked here too so
  unmanaged-filtering doesn't accidentally swallow it); a file with only a lotv id passes
"""
import json
import os
import subprocess
import sys
import tempfile

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
DEPLOY_PANEL_CHECK_PY = os.path.join(SCRIPT_DIR, "deploy_panel_check.py")


def run(cmd, cwd, check=True):
    return subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, check=check)


def main() -> int:
    repo = tempfile.mkdtemp(prefix="yagpdb-deploy-panel-check-test-")

    run(["git", "init", "-q"], cwd=repo)
    run(["git", "config", "user.email", "test@example.invalid"], cwd=repo)
    run(["git", "config", "user.name", "Test"], cwd=repo)

    os.makedirs(os.path.join(repo, "commands", "general"))
    os.makedirs(os.path.join(repo, "deploy"))

    def write(rel, content):
        with open(os.path.join(repo, rel), "w", encoding="utf-8") as f:
            f.write(content)

    # No "main" id in panel.json -> triggers the no-main-id branch.
    write("commands/general/normal.gohtml", "normal content\n")
    # Mapped in "commands" AND listed as "unmanaged" -> triggers that branch.
    write("commands/general/dual.gohtml", "dual content\n")
    # An id on a server other than main only -> fine (a lotv-only command).
    write("commands/general/lotv_only.gohtml", "lotv content\n")

    panel = {
        "servers": ["main", "lotv"],
        "commands": {
            "commands/general/dual.gohtml": {"main": "1"},
            "commands/general/lotv_only.gohtml": {"lotv": "2"},
        },
        "unmanaged": [
            "commands/general/dual.gohtml",
            # Does not exist in the working tree -> triggers that branch.
            "commands/general/missing.gohtml",
        ],
    }
    write("deploy/panel.json", json.dumps(panel))

    run(["git", "add", "-A"], cwd=repo)
    run(["git", "commit", "-q", "-m", "fixture"], cwd=repo)
    # A new command not yet `git add`-ed still needs an id (a worker's tree is untracked).
    write("commands/general/untracked.gohtml", "untracked content\n")

    proc = run(
        ["python3", DEPLOY_PANEL_CHECK_PY, "--panel", "deploy/panel.json"],
        cwd=repo, check=False,
    )

    errors = []
    if proc.returncode == 0:
        errors.append("check exited 0; expected a nonzero exit for the rigged panel.json")

    expected_substrings = [
        "commands/general/dual.gohtml: mapped in commands but also listed as unmanaged",
        "commands/general/missing.gohtml: unmanaged but does not exist in the working tree",
        "commands/general/normal.gohtml: no server id in panel.json",
        "commands/general/untracked.gohtml: no server id in panel.json",
    ]
    for substr in expected_substrings:
        if substr not in proc.stderr:
            errors.append(f"missing expected error: {substr!r}")
    if "lotv_only.gohtml" in proc.stderr:
        errors.append("a command with only a lotv id was reported; any server's id is enough")

    if errors:
        print(f"deploy_panel_check test failed ({len(errors)} problem(s)):", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        print(f"throwaway repo left at: {repo}", file=sys.stderr)
        print("--- stderr ---", file=sys.stderr)
        print(proc.stderr, file=sys.stderr)
        return 1

    print(f"deploy_panel_check test OK (throwaway repo left at {repo})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
