#!/usr/bin/env python3
"""Test the unmanaged-exclusion behavior of `make changed-since-deploy`, run in `make ci`.

Builds a throwaway git repo under the system temp dir (left in place -- never removed, per
this repo's landmines) with a `deployed` tag and a deploy/panel.json listing two unmanaged
paths, then runs scripts/changed-since-deploy.py -- the exact script the Makefile target
calls -- against four change types, without reimplementing its filtering logic:

- an unmanaged add: must be skipped ("never paste"), never listed
- a normal add: must be listed
- a normal pure rename (same content): must produce no output at all
- an unmanaged delete: must be skipped ("never touch in YAGPDB"), not "remove from YAGPDB"
"""
import json
import os
import subprocess
import sys
import tempfile

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
CHANGED_SINCE_DEPLOY_PY = os.path.join(SCRIPT_DIR, "changed-since-deploy.py")


def run(cmd, cwd, check=True):
    return subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, check=check)


def main() -> int:
    repo = tempfile.mkdtemp(prefix="yagpdb-changed-since-deploy-test-")

    run(["git", "init", "-q"], cwd=repo)
    run(["git", "config", "user.email", "test@example.invalid"], cwd=repo)
    run(["git", "config", "user.name", "Test"], cwd=repo)

    os.makedirs(os.path.join(repo, "everyone", "general"))
    os.makedirs(os.path.join(repo, "deploy"))

    def write(rel, content):
        with open(os.path.join(repo, rel), "w", encoding="utf-8") as f:
            f.write(content)

    write("everyone/general/normal_old.gohtml", "old normal content\n")
    write("everyone/general/unmanaged_old.gohtml", "old unmanaged content\n")
    write(
        "deploy/panel.json",
        json.dumps({
            "unmanaged": [
                "everyone/general/unmanaged_new.gohtml",
                "everyone/general/unmanaged_old.gohtml",
            ]
        }),
    )

    run(["git", "add", "-A"], cwd=repo)
    run(["git", "commit", "-q", "-m", "deployed state"], cwd=repo)
    run(["git", "tag", "deployed"], cwd=repo)

    # Normal add -> must be listed.
    write("everyone/general/normal_new.gohtml", "new normal content\n")
    # Unmanaged add -> must be skipped, never listed.
    write("everyone/general/unmanaged_new.gohtml", "new unmanaged content\n")
    # Normal pure rename (identical content) -> must print nothing.
    run(["git", "mv", "everyone/general/normal_old.gohtml",
         "everyone/general/normal_renamed.gohtml"], cwd=repo)
    # Unmanaged delete -> must be skipped with the "never touch" wording.
    os.remove(os.path.join(repo, "everyone", "general", "unmanaged_old.gohtml"))

    # `git diff <deployed>` (no --cached) ignores untracked files entirely, so stage
    # everything -- this only affects the index, changed-since-deploy.py still diffs
    # against `deployed`, not HEAD, so nothing here is committed.
    run(["git", "add", "-A"], cwd=repo)

    proc = run(
        ["python3", CHANGED_SINCE_DEPLOY_PY, "everyone", "--panel", "deploy/panel.json"],
        cwd=repo, check=False,
    )
    errors = []
    if proc.returncode != 0:
        errors.append(f"changed-since-deploy.py exited {proc.returncode}: {proc.stderr}")

    out_lines = proc.stdout.splitlines()

    if "everyone/general/normal_new.gohtml" not in out_lines:
        errors.append("normal add not listed")
    if "everyone/general/unmanaged_new.gohtml" in out_lines:
        errors.append("unmanaged add was listed (should be skipped)")
    if ("skipped (unmanaged, never paste): everyone/general/unmanaged_new.gohtml"
            not in out_lines):
        errors.append("unmanaged add missing its skipped line")
    if any("normal_renamed" in line or "normal_old" in line for line in out_lines):
        errors.append("pure rename produced output (expected nothing)")
    if any("remove from YAGPDB" in line and "unmanaged_old" in line for line in out_lines):
        errors.append("unmanaged delete used the 'remove from YAGPDB' wording")
    if ("skipped (unmanaged, never touch in YAGPDB): everyone/general/unmanaged_old.gohtml"
            not in out_lines):
        errors.append("unmanaged delete missing its skipped line")

    if errors:
        print(f"changed-since-deploy test failed ({len(errors)} problem(s)):", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        print(f"throwaway repo left at: {repo}", file=sys.stderr)
        print("--- stdout ---", file=sys.stderr)
        print(proc.stdout, file=sys.stderr)
        return 1

    print(f"changed-since-deploy test OK (throwaway repo left at {repo})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
