#!/usr/bin/env python3
"""Build the one-line JSON deploy manifest for a YAGPDB server.

Usage: scripts/deploy-manifest.py <server> [--panel <panel.json>]
       (or SERVER=<server> via `make deploy-manifest`; --panel is for tests)

Prints one line of JSON to stdout:
    {"server":"main","guild":"...","commit":"<full sha>",
     "commands":[{"path":...,"source":...,"id":...,"sha":...,"raw":...}, ...]}

`path` is the commands/ path (the command's id and name). `source` is the file whose bytes
get deployed, and `sha` and `raw` are its: equal to `path`, except on a server whose panel.json
"tier" is "free", where a command with a dist/free copy (`dist/free/<path without the leading
"commands/">`) deploys that copy instead; a stale dist/free (yagmin check, as
`make minify-check`) refuses the manifest.

Every entry's panel count (runes plus the newline count: a browser save of the panel form
sends CRLF, so each newline counts twice) is checked against the server's tier cap
(premium 20,000, free 10,000); the manifest refuses (exit 1) and lists every entry over it.
deploy.js's own POST sends LF and is checked on runes alone, so this is deliberately the
stricter count: a later save of the same code from the panel must pass too.

Refuses (exit 1, message on stderr) if HEAD is not contained in any origin/*
branch (raw.githubusercontent.com URLs for it would 404), unless
DEPLOY_MANIFEST_SKIP_ORIGIN_CHECK=1 is set in the environment (test-only
escape hatch, documented here and in the test suite that uses it) -- or if
any mapped file differs between the working tree and HEAD, unless
DEPLOY_MANIFEST_SKIP_DIRTY_CHECK=1 (also test-only: it then hashes the working tree
instead of HEAD, so `make ci` can run it mid-edit, including for a command not yet
committed).

Usage: scripts/deploy-manifest.py --hash <file>   (or `--hash -` to read stdin as UTF-8)

Test/tooling mode: prints sha256_hex(normalize(content)) for the given file (or stdin)
using this script's own normalize() and sha256_hex(), with no other behavior change.
This is what deploy/deploy.test.js shells out to, to check deploy.js's independent JS
normalize/sha256hex implementation for parity with this script's, on both real command
files and synthetic edge-case strings.
"""
import argparse
import hashlib
import json
import os
import subprocess
import sys

REPO = "lbds137/yagpdb-custom-commands"
PANEL_JSON = "deploy/panel.json"
# The panel's per-command size cap by server tier: MaxCCResponsesLength* in
# vendor/yagpdb/customcommands/customcommands.go:955-958 (premium 20,000, free 10,000).
CAPS = {"premium": 20000, "free": 10000}


def run(*args):
    return subprocess.run(
        args, capture_output=True, check=True
    ).stdout


def normalize(text: str) -> str:
    return text.replace("\r\n", "\n").rstrip()


def sha256_hex(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def panel_count(content: str) -> int:
    """The count a panel-form save checks: runes plus the newline count (sent as CRLF).

    `content` is the file as deploy.js posts it (the raw bytes, trailing whitespace kept,
    unlike normalize()), with CRLF folded to LF so a CRLF file isn't counted twice over;
    see FUTURE_IMPROVEMENTS.md, "Emulator Enhancements", first bullet.
    """
    lf = content.replace("\r\n", "\n")
    return len(lf) + lf.count("\n")


def free_copy_path(path: str) -> str:
    """The dist/free copy's path for a commands/ path (yagmin batch mirrors the layout)."""
    return "dist/free/" + path.removeprefix("commands/")


def exists_in_tree(path: str, working_tree: bool) -> bool:
    """Whether `path` exists: in the working tree (test mode), else at HEAD."""
    if working_tree:
        return os.path.isfile(path)
    return subprocess.run(
        ["git", "cat-file", "-e", f"HEAD:{path}"], capture_output=True
    ).returncode == 0


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
    if len(sys.argv) > 1 and sys.argv[1] == "--hash":
        if len(sys.argv) < 3:
            print("usage: deploy-manifest.py --hash <file|->", file=sys.stderr)
            return 2
        target = sys.argv[2]
        if target == "-":
            content = sys.stdin.buffer.read().decode("utf-8")
        else:
            with open(target, encoding="utf-8") as f:
                content = f.read()
        print(sha256_hex(normalize(content)))
        return 0

    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("server", nargs="?")
    parser.add_argument("--panel", default=PANEL_JSON)
    args = parser.parse_args()
    server = args.server or os.environ.get("SERVER")
    if not server:
        print(
            "usage: deploy-manifest.py <server> [--panel <panel.json>] (or SERVER=<server>)",
            file=sys.stderr,
        )
        return 1

    with open(args.panel, encoding="utf-8") as f:
        panel = json.load(f)

    if server not in panel["servers"]:
        print(f"unknown server: {server}", file=sys.stderr)
        return 1
    tier = panel["servers"][server].get("tier")
    if tier not in CAPS:
        print(f"server {server}: tier must be one of {sorted(CAPS)}, got {tier!r}",
              file=sys.stderr)
        return 1
    cap = CAPS[tier]

    skip_origin_check = os.environ.get("DEPLOY_MANIFEST_SKIP_ORIGIN_CHECK") == "1"
    skip_dirty_check = os.environ.get("DEPLOY_MANIFEST_SKIP_DIRTY_CHECK") == "1"
    if not skip_origin_check and not head_on_origin():
        print(
            "refusing: HEAD is not contained in any origin/* branch "
            "(raw.githubusercontent.com URLs would 404) -- push first",
            file=sys.stderr,
        )
        return 1

    # A free server deploys dist/free copies: refuse a stale one (a command edited without
    # `make minify`), the same check as `make minify-check`.
    if tier == "free":
        stale = subprocess.run(
            ["go", "run", "./cmd/yagmin", "check", "-src", "../../commands",
             "-dst", "../../dist/free", "-over", str(CAPS["free"])],
            cwd="tools/emulator", capture_output=True, text=True,
        )
        if stale.returncode != 0:
            print("refusing: dist/free is stale (run `make minify`):\n"
                  + stale.stdout + stale.stderr, file=sys.stderr)
            return 1

    head_sha = subprocess.run(
        ["git", "rev-parse", "HEAD"], capture_output=True, text=True, check=True
    ).stdout.strip()

    commands = []
    over_cap = []
    for path, ids in panel["commands"].items():
        if path.startswith("retired/"):
            print(f"warning: {path} is retired but mapped in panel.json", file=sys.stderr)
            continue
        # Not on this server by design (panel.json is the map; deploy_panel_check.py
        # fails a file with no id on any server), so no warning: the skill treats one as
        # a stop.
        if server not in ids:
            continue
        source = path
        if tier == "free":
            minified = free_copy_path(path)
            if exists_in_tree(minified, skip_dirty_check):
                source = minified
        # the command and its deployed source both: a free server's dist copy is only as
        # fresh as the minify-check run below, which reads the working tree
        for checked in dict.fromkeys((path, source)):
            if not skip_dirty_check and not working_tree_matches_head(checked):
                print(
                    f"refusing: {checked} differs between the working tree and HEAD",
                    file=sys.stderr,
                )
                return 1
        if skip_dirty_check:
            # Test mode: hash the working tree, so a command added or edited mid-change
            # (not yet at HEAD) still gets a manifest entry.
            with open(source, encoding="utf-8") as f:
                content = f.read()
        else:
            content = run("git", "show", f"HEAD:{source}").decode("utf-8")
        normalized = normalize(content)
        digest = sha256_hex(normalized)
        count = panel_count(content)
        if count > cap:
            over_cap.append((path, source, count))
        raw = f"https://raw.githubusercontent.com/{REPO}/{head_sha}/{source}"
        commands.append({
            "path": path,
            "source": source,
            "id": ids[server],
            "sha": digest,
            "raw": raw,
        })

    if over_cap:
        print(f"refusing: {len(over_cap)} command(s) over the {tier} cap of {cap}:",
              file=sys.stderr)
        for path, source, count in over_cap:
            print(f"  {path} (source {source}): panel count {count} > {cap}",
                  file=sys.stderr)
        return 1

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
