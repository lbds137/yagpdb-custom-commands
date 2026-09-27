// Run with `node --test deploy/deploy.test.js` (Node 24 on the dev machine and in CI).
//
// Loads deploy.js into a vm context with a minimal `window` + `crypto` (node:crypto's
// webcrypto, same SubtleCrypto API browsers expose) so the exact same code that gets pasted
// into the Claude-in-Chrome javascript_tool runs here. The point of the hash tests is parity
// with scripts/deploy-manifest.py's Python normalization (CRLF->LF, then rstrip, UTF-8): they
// hash real command files with both and compare.
"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { webcrypto } = require("node:crypto");
const { spawnSync } = require("node:child_process");

const REPO_ROOT = path.join(__dirname, "..");

// Shells out to scripts/deploy-manifest.py's own --hash mode (its own normalize() +
// sha256_hex(), see that file's docstring), so this test is checking deploy.js's
// independent JS implementation against the Python source of truth, not against a second
// copy of the same expectation.
function pythonHash(input) {
  const result = spawnSync(
    "python3",
    ["scripts/deploy-manifest.py", "--hash", input.fromStdin ? "-" : input.path],
    {
      cwd: REPO_ROOT,
      input: input.fromStdin ? input.text : undefined,
      encoding: "utf8",
    }
  );
  assert.equal(
    result.status,
    0,
    `deploy-manifest.py --hash failed: ${result.stderr || result.error}`
  );
  return result.stdout.trim();
}

function loadDeployJs() {
  const code = fs.readFileSync(path.join(__dirname, "deploy.js"), "utf8");
  const sandbox = {
    crypto: webcrypto,
    TextEncoder,
    Uint8Array,
    console,
  };
  sandbox.window = {};
  vm.createContext(sandbox);
  vm.runInContext(code, sandbox, { filename: "deploy.js" });
  return sandbox.window.yagDeploy;
}

// Real command files the parity test hashes both ways (deploy.js's JS implementation vs.
// scripts/deploy-manifest.py's --hash mode) and compares -- no pinned constants, so an edit
// to any of these files never breaks this test on its own (only a real normalize/hash
// disagreement would).
const PARITY_FILES = [
  "everyone/knowledge/define.gohtml",
  "staff/rules.gohtml",
  // Has Hebrew text -- exercises UTF-8 encoding parity, not just ASCII.
  "everyone/hebrew/alefbet.gohtml",
  "everyone/services/embed_exec.gohtml",
];

test("normalize/sha256hex match the Python manifest normalization for real command files", async () => {
  const yagDeploy = loadDeployJs();
  for (const relPath of PARITY_FILES) {
    const raw = fs.readFileSync(path.join(REPO_ROOT, relPath), "utf8");
    const jsHash = await yagDeploy.sha256hex(yagDeploy.normalize(raw));
    const pyHash = pythonHash({ path: relPath });
    assert.equal(jsHash, pyHash, `sha256hex(normalize()) mismatch for ${relPath}`);
  }
});

// Synthetic edge cases for the trailing-whitespace strip: Python's str.rstrip() (with no
// argument) strips characters where str.isspace() is true, and JS's `\s` character class
// doesn't have the same membership -- notably JS's \s includes U+FEFF (BOM/zero-width
// no-break space) while Python's isspace() does not, and Python's isspace() includes the
// C0 separators U+001C-001F and U+0085 (NEL) while JS's \s does not. Both include U+00A0 and
// U+2028. deploy.js's normalize() must match Python's set exactly, since deploy-manifest.py
// (Python) is the manifest side of the live-panel-vs-manifest comparison.
const EDGE_CASES = [
  { label: "CRLF line endings", text: "line one\r\nline two\r\n" },
  { label: "trailing spaces+tabs+newlines", text: "payload  \t \n\n  \t\n" },
  { label: "Hebrew text", text: "שלום עולם\n" },
  { label: "trailing U+00A0 (no-break space)", text: "payload " },
  { label: "trailing U+FEFF (BOM/zero-width no-break space)", text: "payload﻿" },
  { label: "trailing U+2028 (line separator)", text: "payload " },
];

test("normalize matches Python's rstrip()/isspace() exactly on synthetic edge cases", async () => {
  const yagDeploy = loadDeployJs();
  for (const { label, text } of EDGE_CASES) {
    const jsHash = await yagDeploy.sha256hex(yagDeploy.normalize(text));
    const pyHash = pythonHash({ fromStdin: true, text });
    assert.equal(jsHash, pyHash, `normalize() disagreement for: ${label}`);
  }
});

test("parseHeader reads a Command header", () => {
  const yagDeploy = loadDeployJs();
  const code = [
    "{{- /*",
    "  Author: Vladlena Costescu (@lbds137)",
    "  Trigger type: `Command`",
    "  Trigger: `rules`",
    "  Dependencies: `embed_exec`",
    "*/ -}}",
  ].join("\n");
  // parseHeader's return value is an object from the vm sandbox's Realm, so compare fields
  // rather than assert.deepEqual (which also checks prototype identity across Realms).
  const header = yagDeploy.parseHeader(code);
  assert.equal(header.type, "Command");
  assert.equal(header.trigger, "rules");
});

test("parseHeader reads a Regex header with backslashes", () => {
  const yagDeploy = loadDeployJs();
  const code = [
    "{{- /*",
    "  Author: Vladlena Costescu (@lbds137)",
    "  Trigger type: `Regex`",
    "  Trigger: `\\A<#\\d{16,}>\\z`",
    "  Dependencies: `embed_exec`",
    "*/ -}}",
  ].join("\n");
  const header = yagDeploy.parseHeader(code);
  assert.equal(header.type, "Regex");
  assert.equal(header.trigger, "\\A<#\\d{16,}>\\z");
});

test("parseHeader reads a None header (no Trigger line)", () => {
  const yagDeploy = loadDeployJs();
  const code = [
    "{{- /*",
    "  Author: Vladlena Costescu (@lbds137)",
    "  Trigger type: `None`",
    "*/ -}}",
  ].join("\n");
  const header = yagDeploy.parseHeader(code);
  assert.equal(header.type, "None");
  assert.equal(header.trigger, null);
});

test("panelTypeName maps panel select labels to header vocabulary", () => {
  const yagDeploy = loadDeployJs();
  assert.equal(yagDeploy.panelTypeName("Command"), "Command");
  assert.equal(yagDeploy.panelTypeName("Command (prefix required)"), "Command");
  assert.equal(yagDeploy.panelTypeName("Regex"), "Regex");
  assert.equal(yagDeploy.panelTypeName("None"), "None");
  assert.equal(yagDeploy.panelTypeName("Hourly interval"), "Hourly interval");
  assert.equal(yagDeploy.panelTypeName("Minute interval"), "Minute interval");
  assert.equal(yagDeploy.panelTypeName("Something else"), "Something else");
});

test("panelTypeName handles the panel's actual multi-line Command label (observed 2026-09-27)", () => {
  const yagDeploy = loadDeployJs();
  const label = "Command\n        (mention/cmd\n        prefix)";
  assert.equal(yagDeploy.panelTypeName(label), "Command");
});

test("decide: raw fetch mismatch fails even in a dry run", () => {
  const yagDeploy = loadDeployJs();
  assert.equal(
    yagDeploy.decide({ liveSha: "a", manifestSha: "m", rawSha: "not-m", dryRun: true }),
    "failed: raw mismatch"
  );
  assert.equal(
    yagDeploy.decide({ liveSha: "a", manifestSha: "m", rawSha: "not-m", dryRun: false }),
    "failed: raw mismatch"
  );
  // rawSha === null happens when the raw fetch's response wasn't ok -- also a mismatch.
  assert.equal(
    yagDeploy.decide({ liveSha: "a", manifestSha: "m", rawSha: null, dryRun: true }),
    "failed: raw mismatch"
  );
});

test("decide: live already matches the manifest -> same (raw still had to match first)", () => {
  const yagDeploy = loadDeployJs();
  assert.equal(
    yagDeploy.decide({ liveSha: "m", manifestSha: "m", rawSha: "m", dryRun: true }),
    "same"
  );
  assert.equal(
    yagDeploy.decide({ liveSha: "m", manifestSha: "m", rawSha: "m", dryRun: false }),
    "same"
  );
});

test("decide: live differs from manifest -> would-update in a dry run, update otherwise", () => {
  const yagDeploy = loadDeployJs();
  assert.equal(
    yagDeploy.decide({ liveSha: "old", manifestSha: "m", rawSha: "m", dryRun: true }),
    "would-update"
  );
  assert.equal(
    yagDeploy.decide({ liveSha: "old", manifestSha: "m", rawSha: "m", dryRun: false }),
    "update"
  );
});

test("classifyReadBack: read-back sha decides success, an alert is only a note", () => {
  const yagDeploy = loadDeployJs();
  // classifyReadBack's return value is an object from the vm sandbox's Realm (see the
  // parseHeader tests above for why assert.deepEqual/deepStrictEqual can't be used directly
  // across Realms), so compare fields.
  function assertClassified(result, expectedStatus, expectedNote) {
    assert.equal(result.status, expectedStatus);
    assert.equal(result.note, expectedNote);
  }

  assertClassified(
    yagDeploy.classifyReadBack({ readBackSha: "m", manifestSha: "m", alertText: null }),
    "updated",
    null
  );
  // A site-wide notice banner can appear on a page that saved successfully -- it must not
  // turn a matching read-back into a failure, only ride along as `note`.
  assertClassified(
    yagDeploy.classifyReadBack({
      readBackSha: "m",
      manifestSha: "m",
      alertText: "Unrelated site notice",
    }),
    "updated",
    "Unrelated site notice"
  );
  assertClassified(
    yagDeploy.classifyReadBack({ readBackSha: "stale", manifestSha: "m", alertText: null }),
    "failed: read-back mismatch",
    null
  );
  assertClassified(
    yagDeploy.classifyReadBack({
      readBackSha: "stale",
      manifestSha: "m",
      alertText: "Save failed",
    }),
    "failed: read-back mismatch",
    "Save failed"
  );
});
