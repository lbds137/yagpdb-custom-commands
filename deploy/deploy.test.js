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

const REPO_ROOT = path.join(__dirname, "..");

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

// Expected shas: computed directly with scripts/deploy-manifest.py's own normalize()
// (`text.replace("\r\n","\n").rstrip()`) + `hashlib.sha256(...).hexdigest()` against the
// files at HEAD (2026-09-27, commit 680b48a) -- NOT re-derived from deploy.js. This is the
// parity check: deploy.js's independent JS implementation must land on the same digests.
const PARITY_CASES = [
  {
    // Recomputed 2026-09-27 (retire-kb): define.gohtml gained the kb-alias sdict.
    path: "everyone/knowledge/define.gohtml",
    sha256: "79d649eaead85457935e1cc1886555a64add4847453f5f6e3f7337cdf3a01a73",
  },
  {
    path: "staff/rules.gohtml",
    sha256: "5b5ae100902d97a0a181e9df99c511367b9f31c36f4d8c565577e3bc8e470ad7",
  },
  {
    // Has Hebrew text -- exercises UTF-8 encoding parity, not just ASCII.
    path: "everyone/hebrew/alefbet.gohtml",
    sha256: "daba3c53b5344ab11614e95f9854062a74a7213646345f5107888fc154025575",
  },
];

test("normalize/sha256hex match the Python manifest normalization for real command files", async () => {
  const yagDeploy = loadDeployJs();
  for (const { path: relPath, sha256 } of PARITY_CASES) {
    const raw = fs.readFileSync(path.join(REPO_ROOT, relPath), "utf8");
    const jsHash = await yagDeploy.sha256hex(yagDeploy.normalize(raw));
    assert.equal(jsHash, sha256, `sha256hex(normalize()) mismatch for ${relPath}`);
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
