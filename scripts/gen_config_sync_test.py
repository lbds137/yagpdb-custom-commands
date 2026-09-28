#!/usr/bin/env python3
"""Test scripts/gen-config-sync.py, run in `make ci`.

Runs the real script (as a subprocess, via its --panel and --out options) against small
fixture panel.json files in a directory under the system temp dir (left in place -- never
removed, per this repo's landmines), never against the repo's own panel.json, and checks:

- the output: one id map per server, keyed by guild, sorted keys, ids as strings
- a retired/ entry is skipped
- a server a command has no id for doesn't get that key
- two entries with the same basename are an error (exit 2), nothing written
- --check exits 1 (with the `make config-sync` hint) when the file is stale or missing,
  and 0 when it is fresh
"""
import json
import os
import subprocess
import sys
import tempfile

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
GEN = os.path.join(SCRIPT_DIR, "gen-config-sync.py")

PANEL = {
    "servers": {
        "one": {"name": "Server One", "guild": "111111111111111111"},
        "two": {"name": "Server Two", "guild": "222222222222222222"},
    },
    "commands": {
        "commands/zeta/zulu.gohtml": {"one": 9, "two": 4},
        "commands/alpha/alpha.gohtml": {"one": 5},
        "retired/old.gohtml": {"one": 7, "two": 8},
        "commands/mid/mike.gohtml": {"one": 12, "two": 3},
    },
}


def run(*args):
    return subprocess.run(
        ["python3", GEN, *args], capture_output=True, text=True,
    )


def write_panel(directory, name, panel):
    path = os.path.join(directory, name)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(panel, f)
    return path


def main() -> int:
    scratch = tempfile.mkdtemp(prefix="yagpdb-gen-config-sync-test-")
    errors = []

    panel_path = write_panel(scratch, "panel.json", PANEL)
    out = os.path.join(scratch, "config_sync.gohtml")

    # --check before the file exists: stale (missing)
    proc = run("--check", "--panel", panel_path, "--out", out)
    if proc.returncode != 1 or "run: make config-sync" not in proc.stderr:
        errors.append(f"--check on a missing file: want exit 1 with the hint, got "
                      f"{proc.returncode} {proc.stderr!r}")

    proc = run("--panel", panel_path, "--out", out)
    if proc.returncode != 0:
        errors.append(f"write: exit {proc.returncode}: {proc.stderr}")
        content = ""
    else:
        with open(out, encoding="utf-8") as f:
            content = f.read()

    expected_one = (
        "{{- if eq .Guild.ID 111111111111111111 -}}\n"
        "    {{- /* one */ -}}\n"
        "    {{- $ids = sdict\n"
        '        "alpha" "5"\n'
        '        "mike" "12"\n'
        '        "zulu" "9" -}}\n'
    )
    expected_two = (
        "{{- else if eq .Guild.ID 222222222222222222 -}}\n"
        "    {{- /* two */ -}}\n"
        "    {{- $ids = sdict\n"
        '        "mike" "3"\n'
        '        "zulu" "4" -}}\n'
        "{{- end -}}\n"
    )
    for label, block in (("server one's map", expected_one), ("server two's map", expected_two)):
        if block not in content:
            errors.append(f"{label} not in the output as expected:\n{block}")
    if content.find(expected_one) > content.find(expected_two):
        errors.append("server maps out of panel.json order")
    if '"old"' in content:
        errors.append("the retired/ entry was not skipped")
    for needle in ('Trigger type: `Hourly interval`', 'Interval: `1`',
                   'Group: `Staff Utility`', 'Dependencies: none', 'make config-sync',
                   '{{- $commandsDict := or (dbGet 0 "Commands").Value sdict -}}',
                   '{{- dbSet 0 "Commands" $commandsDict -}}'):
        if needle not in content:
            errors.append(f"output lacks {needle!r}")
    # calls, not the header's mention of them
    if content.count("dbGet 0") != 1 or content.count("dbSet 0") != 1:
        errors.append("output should have exactly one dbGet and one dbSet")

    # --check on the fresh file
    proc = run("--check", "--panel", panel_path, "--out", out)
    if proc.returncode != 0:
        errors.append(f"--check on a fresh file: want exit 0, got {proc.returncode} "
                      f"{proc.stderr!r}")

    # --check after a one-byte edit: stale
    with open(out, "a", encoding="utf-8") as f:
        f.write(" ")
    proc = run("--check", "--panel", panel_path, "--out", out)
    if proc.returncode != 1 or "run: make config-sync" not in proc.stderr:
        errors.append(f"--check on an edited file: want exit 1 with the hint, got "
                      f"{proc.returncode} {proc.stderr!r}")

    # --check after a panel.json change: stale
    changed = json.loads(json.dumps(PANEL))
    changed["commands"]["commands/alpha/alpha.gohtml"]["one"] = 6
    changed_path = write_panel(scratch, "panel-changed.json", changed)
    run("--panel", panel_path, "--out", out)
    proc = run("--check", "--panel", changed_path, "--out", out)
    if proc.returncode != 1:
        errors.append(f"--check after a panel.json change: want exit 1, got {proc.returncode}")

    # duplicate basename: error, nothing written
    dup = json.loads(json.dumps(PANEL))
    dup["commands"]["commands/other/alpha.gohtml"] = {"two": 1}
    dup_path = write_panel(scratch, "panel-dup.json", dup)
    dup_out = os.path.join(scratch, "dup.gohtml")
    proc = run("--panel", dup_path, "--out", dup_out)
    if proc.returncode != 2 or "duplicate basename 'alpha'" not in proc.stderr:
        errors.append(f"duplicate basename: want exit 2 naming 'alpha', got "
                      f"{proc.returncode} {proc.stderr!r}")
    if os.path.exists(dup_out):
        errors.append("duplicate basename: the output file was written anyway")

    if errors:
        print(f"gen-config-sync test failed ({len(errors)} problem(s)), scratch {scratch}:",
              file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        return 1

    print("gen-config-sync test OK: output, retired skip, per-server ids, duplicate "
          "basename, --check stale/fresh")
    return 0


if __name__ == "__main__":
    sys.exit(main())
