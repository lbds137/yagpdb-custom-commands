#!/usr/bin/env python3
"""Test scripts/deploy-manifest.py, run in `make ci`.

Runs it for SERVER=main with the origin-check bypassed (this is a test escape hatch, not
something the real deploy flow uses -- see deploy-manifest.py's own docstring and
DEPLOY_MANIFEST_SKIP_ORIGIN_CHECK), and checks: valid JSON, the command count matches the
number of panel.json entries with a "main" id (derived from panel.json each run), and 2 of the
shas match a direct hash computed here independently of deploy-manifest.py's own code.
"""
import hashlib
import json
import os
import subprocess
import sys

DIRECT_CHECK_FILES = [
    "commands/knowledge/define.gohtml",
    "commands/rules/rules.gohtml",
]


def normalize(text: str) -> str:
    return text.replace("\r\n", "\n").rstrip()


def main() -> int:
    env = dict(os.environ)
    env["DEPLOY_MANIFEST_SKIP_ORIGIN_CHECK"] = "1"
    env["DEPLOY_MANIFEST_SKIP_DIRTY_CHECK"] = "1"
    proc = subprocess.run(
        ["python3", "scripts/deploy-manifest.py", "main"],
        capture_output=True, text=True, env=env,
    )
    if proc.returncode != 0:
        print("deploy-manifest.py main exited nonzero:", file=sys.stderr)
        print(proc.stderr, file=sys.stderr)
        return 1

    try:
        manifest = json.loads(proc.stdout)
    except json.JSONDecodeError as e:
        print(f"manifest is not valid JSON: {e}", file=sys.stderr)
        return 1

    with open("deploy/panel.json", encoding="utf-8") as f:
        panel = json.load(f)
    expected_count = sum(1 for ids in panel["commands"].values() if "main" in ids)

    errors = []
    if manifest.get("server") != "main":
        errors.append(f"server: expected 'main', got {manifest.get('server')!r}")
    if len(manifest.get("commands", [])) != expected_count:
        errors.append(
            f"command count: expected {expected_count}, got {len(manifest.get('commands', []))}"
        )

    by_path = {c["path"]: c for c in manifest.get("commands", [])}
    for path in DIRECT_CHECK_FILES:
        if path not in by_path:
            errors.append(f"{path}: missing from manifest")
            continue
        with open(path, encoding="utf-8") as f:
            content = f.read()
        expected_sha = hashlib.sha256(normalize(content).encode("utf-8")).hexdigest()
        actual_sha = by_path[path]["sha"]
        if actual_sha != expected_sha:
            errors.append(f"{path}: sha mismatch (expected {expected_sha}, got {actual_sha})")

    if errors:
        print(f"deploy-manifest test failed ({len(errors)} problem(s)):", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        return 1

    print(f"deploy-manifest test OK: {expected_count} main commands, 2 shas verified directly")
    return 0


if __name__ == "__main__":
    sys.exit(main())
