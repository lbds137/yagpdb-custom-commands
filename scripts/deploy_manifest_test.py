#!/usr/bin/env python3
"""Test scripts/deploy-manifest.py, run in `make ci`.

Runs it for SERVER=main with the origin-check bypassed (this is a test escape hatch, not
something the real deploy flow uses -- see deploy-manifest.py's own docstring and
DEPLOY_MANIFEST_SKIP_ORIGIN_CHECK), and checks: valid JSON, the command count matches the
number of panel.json entries with a "main" id (derived from panel.json each run), and 2 of the
shas match a direct hash computed here independently of deploy-manifest.py's own code.

Then, with throwaway panel.json files (the --panel option), the per-tier cases: a free server
takes a command's dist/free copy (source, sha, raw) and the command itself when there is no
copy, a premium server never takes the copy, over-cap commands (hebrew_slash's minified copy,
multiple free-tier entries, and a premium 20,000-cap entry) are all refused by name with exit 1,
a stale dist/free is refused on a free server, and a server with no tier is refused.
"""
import hashlib
import importlib.util
import io
import json
import os
import subprocess
import sys
import tempfile

DIRECT_CHECK_FILES = [
    "commands/knowledge/define_slash.gohtml",
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

    for path, entry in by_path.items():
        if entry.get("source") != path:
            errors.append(f"{path}: premium entry source {entry.get('source')!r} != path")

    errors += check_free_tier(env)

    if errors:
        print(f"deploy-manifest test failed ({len(errors)} problem(s)):", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        return 1

    print(f"deploy-manifest test OK: {expected_count} main commands, 2 shas verified "
          "directly, free-tier source and cap cases checked")
    return 0


def run_with_panel(env, servers, commands, server):
    """Run deploy-manifest.py against a throwaway panel.json (via --panel)."""
    with tempfile.TemporaryDirectory(prefix="yagpdb-deploy-manifest-test-") as panel_dir:
        panel_path = os.path.join(panel_dir, "panel.json")
        with open(panel_path, "w", encoding="utf-8") as f:
            json.dump({"servers": servers, "commands": commands}, f)
        return subprocess.run(
            ["python3", "scripts/deploy-manifest.py", server, "--panel", panel_path],
            capture_output=True, text=True, env=env,
        )


def panel_count(text: str) -> int:
    # the script's count: CRLF folded, trailing whitespace kept (it counts what is posted)
    lf = text.replace("\r\n", "\n")
    return len(lf) + lf.count("\n")


def check_free_tier(env):
    errors = []
    free = {"name": "Free", "guild": "1", "tier": "free"}
    premium = {"name": "Prem", "guild": "2", "tier": "premium"}
    minified = "commands/db/db_slash.gohtml"
    minified_copy = "dist/free/db/db_slash.gohtml"
    plain = "commands/knowledge/define_slash.gohtml"

    # a free server: a command with a dist/free copy deploys the copy, one without deploys itself
    proc = run_with_panel(
        env, {"free": free}, {minified: {"free": 1}, plain: {"free": 2}}, "free",
    )
    if proc.returncode != 0:
        return [f"free server manifest exited {proc.returncode}: {proc.stderr}"]
    entries = {c["path"]: c for c in json.loads(proc.stdout)["commands"]}
    with open(minified_copy, encoding="utf-8") as f:
        copy_text = f.read()
    got = entries.get(minified, {})
    if got.get("source") != minified_copy:
        errors.append(f"{minified}: free source {got.get('source')!r}, want {minified_copy!r}")
    want_sha = hashlib.sha256(normalize(copy_text).encode("utf-8")).hexdigest()
    if got.get("sha") != want_sha:
        errors.append(f"{minified}: free sha is not the dist copy's")
    if not got.get("raw", "").endswith("/" + minified_copy):
        errors.append(f"{minified}: free raw {got.get('raw')!r} is not the dist copy's URL")
    got = entries.get(plain, {})
    if got.get("source") != plain:
        errors.append(f"{plain}: no dist copy, so source should be the path, got "
                      f"{got.get('source')!r}")

    # the same command on a premium server is the command itself, not the copy
    proc = run_with_panel(
        env, {"prem": premium}, {minified: {"prem": 1}}, "prem",
    )
    if proc.returncode != 0:
        errors.append(f"premium manifest exited {proc.returncode}: {proc.stderr}")
    else:
        got = json.loads(proc.stdout)["commands"][0]
        if got["source"] != minified:
            errors.append(f"premium {minified}: source {got['source']!r}, want the path")

    # over the cap: hebrew_slash's minified copy is over 10,000 by panel count; db_slash's
    # copy is not, so only hebrew_slash is named, and its source is the dist copy
    over = "commands/hebrew/hebrew_slash.gohtml"
    over_copy = "dist/free/hebrew/hebrew_slash.gohtml"
    with open(over_copy, encoding="utf-8") as f:
        over_count = panel_count(f.read())
    if over_count <= 10000:
        errors.append(f"fixture premise broken: {over_copy} panel count {over_count} <= 10000")
    proc = run_with_panel(
        env, {"free": free}, {minified: {"free": 1}, over: {"free": 2}}, "free",
    )
    if proc.returncode != 1:
        errors.append(f"over-cap free manifest exited {proc.returncode}, want 1")
    if proc.stdout.strip():
        errors.append("over-cap free manifest printed a manifest to stdout")
    if over not in proc.stderr or str(over_count) not in proc.stderr \
            or over_copy not in proc.stderr or "10000" not in proc.stderr:
        errors.append(f"over-cap stderr should name {over}, {over_copy}, {over_count} and "
                      f"the cap: {proc.stderr!r}")
    if minified in proc.stderr:
        errors.append(f"{minified} is under the cap but was named: {proc.stderr!r}")

    # multiple over-cap entries on a free server: every over-cap entry is listed, not just
    # the first, and the under-cap entry alongside them is not
    with tempfile.TemporaryDirectory(prefix="yagpdb-deploy-manifest-caps-") as cap_dir:
        over_a = os.path.join(cap_dir, "over_a.gohtml")
        over_b = os.path.join(cap_dir, "over_b.gohtml")
        for path in (over_a, over_b):
            with open(path, "w", encoding="utf-8") as f:
                f.write("x" * 10001)
        with open(over_a, encoding="utf-8") as f:
            count_a = panel_count(f.read())
        with open(over_b, encoding="utf-8") as f:
            count_b = panel_count(f.read())
        if count_a <= 10000 or count_b <= 10000:
            errors.append(
                f"fixture premise broken: {over_a} ({count_a}), {over_b} ({count_b}) <= 10000"
            )
        proc = run_with_panel(
            env,
            {"free": free},
            {over_a: {"free": 1}, plain: {"free": 2}, over_b: {"free": 3}},
            "free",
        )
        if proc.returncode != 1:
            errors.append(f"multi over-cap free manifest exited {proc.returncode}, want 1")
        if proc.stdout.strip():
            errors.append("multi over-cap free manifest printed a manifest to stdout")
        if (over_a not in proc.stderr or over_b not in proc.stderr
                or str(count_a) not in proc.stderr or str(count_b) not in proc.stderr
                or "10000" not in proc.stderr):
            errors.append(
                f"multi over-cap stderr should name {over_a}, {over_b}, {count_a}, "
                f"{count_b} and 10000: {proc.stderr!r}"
            )
        if plain in proc.stderr:
            errors.append(f"{plain} is under the cap but was named: {proc.stderr!r}")

        # premium 20,000 cap: a source over 20,000 on a premium server is refused with 20000
        over_prem = os.path.join(cap_dir, "over_prem.gohtml")
        with open(over_prem, "w", encoding="utf-8") as f:
            f.write("x" * 20001)
        with open(over_prem, encoding="utf-8") as f:
            prem_count = panel_count(f.read())
        if prem_count <= 20000:
            errors.append(
                f"fixture premise broken: {over_prem} panel count {prem_count} <= 20000"
            )
        proc = run_with_panel(
            env, {"prem": premium}, {over_prem: {"prem": 1}}, "prem",
        )
        if proc.returncode != 1:
            errors.append(f"over-cap premium manifest exited {proc.returncode}, want 1")
        if proc.stdout.strip():
            errors.append("over-cap premium manifest printed a manifest to stdout")
        if (over_prem not in proc.stderr or str(prem_count) not in proc.stderr
                or "20000" not in proc.stderr):
            errors.append(
                f"over-cap premium stderr should name {over_prem}, {prem_count} and "
                f"20000: {proc.stderr!r}"
            )

    # stale dist/free on a free server is refused before building the manifest
    with tempfile.TemporaryDirectory(prefix="yagpdb-deploy-manifest-stale-") as stale_dir:
        panel_path = os.path.join(stale_dir, "panel.json")
        with open(panel_path, "w", encoding="utf-8") as f:
            json.dump({"servers": {"free": free}, "commands": {plain: {"free": 1}}}, f)
        spec = importlib.util.spec_from_file_location(
            "deploy_manifest", "scripts/deploy-manifest.py"
        )
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        real_run = mod.subprocess.run
        orig_argv = sys.argv[:]
        orig_env = os.environ.copy()
        orig_stderr = sys.stderr
        captured_err = io.StringIO()

        def fake_run(args, *a, **kw):
            if list(args[:2]) == ["go", "run"]:
                return subprocess.CompletedProcess(
                    args, returncode=1, stdout="", stderr="yagmin: stale\n"
                )
            return real_run(args, *a, **kw)

        try:
            mod.subprocess.run = fake_run
            sys.argv = ["scripts/deploy-manifest.py", "free", "--panel", panel_path]
            os.environ["DEPLOY_MANIFEST_SKIP_ORIGIN_CHECK"] = "1"
            os.environ["DEPLOY_MANIFEST_SKIP_DIRTY_CHECK"] = "1"
            sys.stderr = captured_err
            rc = mod.main()
        finally:
            mod.subprocess.run = real_run
            sys.argv = orig_argv
            os.environ.clear()
            os.environ.update(orig_env)
            sys.stderr = orig_stderr

        err_text = captured_err.getvalue()
        if rc != 1 or "stale" not in err_text:
            errors.append(
                f"stale dist/free should exit 1 naming stale, got rc={rc}: {err_text!r}"
            )

    # a server with no (or a bad) tier is refused
    proc = run_with_panel(
        env, {"x": {"name": "X", "guild": "3"}}, {plain: {"x": 1}}, "x",
    )
    if proc.returncode != 1 or "tier" not in proc.stderr:
        errors.append(f"a server with no tier should be refused naming tier: {proc.stderr!r}")
    return errors


if __name__ == "__main__":
    sys.exit(main())
