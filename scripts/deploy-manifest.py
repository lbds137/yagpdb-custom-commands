#!/usr/bin/env python3
"""Build the one-line JSON deploy manifest for a YAGPDB server.

Usage: scripts/deploy-manifest.py <server>   (or SERVER=<server> via `make deploy-manifest`)

Prints one line of JSON to stdout:
    {"server":"main","guild":"...","commit":"<full sha>",
     "commands":[{"path":...,"id":...,"sha":...,"raw":...}, ...]}

Refuses (exit 1, message on stderr) if HEAD is not contained in any origin/*
branch (raw.githubusercontent.com URLs for it would 404), unless
DEPLOY_MANIFEST_SKIP_ORIGIN_CHECK=1 is set in the environment (test-only
escape hatch, documented here and in the test suite that uses it) -- or if
any mapped file differs between the working tree and HEAD.
"""
import hashlib
import json
import os
import subprocess
import sys

REPO = "lbds137/yagpdb-custom-commands"
PANEL_JSON = "deploy/panel.json"


def run(*args):
    return subprocess.run(
        args, capture_output=True, check=True
    ).stdout


def normalize(text: str) -> str:
    return text.replace("\r\n", "\n").rstrip()


def sha256_hex(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def head_on_origin() -> bool:
    out = subprocess.run(
        ["git", "branch", "-r", "--contains", "HEAD"],
        capture_output=True, text=True, check=True,
    ).stdout
    return any(line.strip().startswith("origin/") for line in out.splitlines())


def working_tree_matches_head(path: str) -> bool:
    diff = subprocess.run(
        ["git", "diff", "--quiet", "HEAD", "--", path],
    )
    return diff.returncode == 0


def main() -> int:
    if len(sys.argv) > 1:
        server = sys.argv[1]
    else:
        server = os.environ.get("SERVER")
    if not server:
        print("usage: deploy-manifest.py <server> (or SERVER=<server>)", file=sys.stderr)
        return 1

    with open(PANEL_JSON, encoding="utf-8") as f:
        panel = json.load(f)

    if server not in panel["servers"]:
        print(f"unknown server: {server}", file=sys.stderr)
        return 1

    skip_origin_check = os.environ.get("DEPLOY_MANIFEST_SKIP_ORIGIN_CHECK") == "1"
    if not skip_origin_check and not head_on_origin():
        print(
            "refusing: HEAD is not contained in any origin/* branch "
            "(raw.githubusercontent.com URLs would 404) -- push first",
            file=sys.stderr,
        )
        return 1

    head_sha = subprocess.run(
        ["git", "rev-parse", "HEAD"], capture_output=True, text=True, check=True
    ).stdout.strip()

    commands = []
    for path, ids in panel["commands"].items():
        if path.startswith("retired/"):
            print(f"warning: {path} is retired but mapped in panel.json", file=sys.stderr)
            continue
        if server not in ids:
            print(f"warning: {path} has no id for server {server}", file=sys.stderr)
            continue
        if not working_tree_matches_head(path):
            print(
                f"refusing: {path} differs between the working tree and HEAD",
                file=sys.stderr,
            )
            return 1
        content = run("git", "show", f"HEAD:{path}").decode("utf-8")
        normalized = normalize(content)
        digest = sha256_hex(normalized)
        raw = f"https://raw.githubusercontent.com/{REPO}/{head_sha}/{path}"
        commands.append({
            "path": path,
            "id": ids[server],
            "sha": digest,
            "raw": raw,
        })

    manifest = {
        "server": server,
        "guild": panel["servers"][server]["guild"],
        "commit": head_sha,
        "commands": commands,
    }
    print(json.dumps(manifest))
    return 0


if __name__ == "__main__":
    sys.exit(main())
