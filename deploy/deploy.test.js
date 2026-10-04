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

function loadDeployJs(extra) {
  const code = fs.readFileSync(path.join(__dirname, "deploy.js"), "utf8");
  const sandbox = {
    crypto: webcrypto,
    TextEncoder,
    Uint8Array,
    console,
    ...(extra || {}),
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
  "commands/knowledge/define_slash.gohtml",
  "commands/rules/rules.gohtml",
  // Has Hebrew text -- exercises UTF-8 encoding parity, not just ASCII.
  "commands/hebrew/alefbet.gohtml",
  "commands/plumbing/embed_exec.gohtml",
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
  {
    label: "trailing U+0085 (NEL)",
    text: "payload" + String.fromCodePoint(0x0085),
  },
  {
    label: "trailing U+001C (file separator)",
    text: "payload" + String.fromCodePoint(0x001c),
  },
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

test("deploy-manifest.py --hash with no path argument prints usage and exits 2", () => {
  const result = spawnSync("python3", ["scripts/deploy-manifest.py", "--hash"], {
    cwd: REPO_ROOT,
    encoding: "utf8",
  });
  assert.equal(result.status, 2);
  assert.match(result.stderr, /usage: deploy-manifest\.py --hash <file\|->/);
});

test("parseHeader reads a Command header", () => {
  const yagDeploy = loadDeployJs();
  const code = [
    "{{- /*",
    "  Author: Vladlena Costescu (@lbds137)",
    "  Trigger type: `Command`",
    "  Trigger: `rules`",
    "  Group: `Staff Utility`",
    "  Dependencies: `embed_exec`",
    "*/ -}}",
  ].join("\n");
  // parseHeader's return value is an object from the vm sandbox's Realm, so compare fields
  // rather than assert.deepEqual (which also checks prototype identity across Realms).
  const header = yagDeploy.parseHeader(code);
  assert.equal(header.type, "Command");
  assert.equal(header.trigger, "rules");
  assert.equal(header.group, "Staff Utility");
});

test("parseHeader reads the Group line when it follows Trigger type (no Trigger line)", () => {
  const yagDeploy = loadDeployJs();
  const code = [
    "{{- /*",
    "  Author: Vladlena Costescu (@lbds137)",
    "  Trigger type: `Hourly interval`",
    "  Group: `Utility`",
    "  Interval: `168`",
    "*/ -}}",
  ].join("\n");
  const header = yagDeploy.parseHeader(code);
  assert.equal(header.type, "Hourly interval");
  assert.equal(header.trigger, null);
  assert.equal(header.group, "Utility");
});

test("parseHeader reads every committed command's Group line as a known panel group", () => {
  const yagDeploy = loadDeployJs();
  const files = fs
    .readdirSync(path.join(REPO_ROOT, "commands"), { withFileTypes: true, recursive: true })
    .filter((entry) => entry.isFile() && entry.name.endsWith(".gohtml"))
    .map((entry) => path.join(entry.parentPath, entry.name));
  assert.ok(files.length > 0, "no command files found under commands/");
  for (const file of files) {
    const header = yagDeploy.parseHeader(fs.readFileSync(file, "utf8"));
    assert.ok(
      header.group === "Utility" || header.group === "Staff Utility",
      `${path.relative(REPO_ROOT, file)}: Group is ${JSON.stringify(header.group)}`
    );
  }
});

test("panelGroupName trims the panel's option text", () => {
  const yagDeploy = loadDeployJs();
  assert.equal(yagDeploy.panelGroupName("Staff Utility"), "Staff Utility");
  assert.equal(yagDeploy.panelGroupName("\n        Utility\n      "), "Utility");
  assert.equal(yagDeploy.panelGroupName(null), "");
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

// ---------------------------------------------------------------------------------------
// Structure: header parsing parity with the emulator, and the pure form helpers.
// deploy.js's return values come from the vm sandbox's Realm, so they go through J() before
// a deep comparison.
// ---------------------------------------------------------------------------------------

const J = (value) => JSON.parse(JSON.stringify(value));
const GOLDEN = JSON.parse(
  fs.readFileSync(path.join(__dirname, "testdata", "headers.golden.json"), "utf8")
);

function commandFiles() {
  return fs
    .readdirSync(path.join(REPO_ROOT, "commands"), { withFileTypes: true, recursive: true })
    .filter((entry) => entry.isFile() && entry.name.endsWith(".gohtml"))
    .map((entry) =>
      path.relative(REPO_ROOT, path.join(entry.parentPath, entry.name)).split(path.sep).join("/")
    )
    .sort();
}

test("header golden covers exactly the committed command files", () => {
  assert.deepEqual(Object.keys(GOLDEN.commands).sort(), commandFiles());
});

test("parseHeader reads the defer mode, slash description and slash rows as the emulator does", () => {
  const yagDeploy = loadDeployJs();
  for (const file of commandFiles()) {
    const header = yagDeploy.parseHeader(fs.readFileSync(path.join(REPO_ROOT, file), "utf8"));
    assert.equal(header.error, null, `${file}: ${header.error}`);
    assert.deepEqual(
      J({
        deferMode: header.deferMode,
        slashDescription: header.slashDescription,
        slash: header.slash,
      }),
      GOLDEN.commands[file],
      file
    );
  }
});

test("parseHeader reads the golden's valid samples (other defer modes, descriptions) as the emulator does", () => {
  const yagDeploy = loadDeployJs();
  assert.ok(GOLDEN.samples.some((s) => s.header.deferMode === 2), "a non-None defer mode");
  for (const s of GOLDEN.samples) {
    const header = yagDeploy.parseHeader(s.source);
    assert.equal(header.error, null, `${s.name}: ${header.error}`);
    assert.deepEqual(
      J({
        deferMode: header.deferMode,
        slashDescription: header.slashDescription,
        slash: header.slash,
      }),
      s.header,
      s.name
    );
  }
});

test("parseHeader rejects the same headers with the same message as the emulator", () => {
  const yagDeploy = loadDeployJs();
  assert.ok(GOLDEN.errors.length > 15, "the golden carries the Go error cases");
  for (const c of GOLDEN.errors) {
    assert.equal(yagDeploy.parseHeader(c.source).error, c.error, c.name);
  }
});

const LABELS = {
  type: {
    Command: "cmd",
    "Slash Command": "slash_command",
    "Hourly interval": "interval_hours",
    "Message Component": "component",
  },
  group: { None: "0", Utility: "11", "Staff Utility": "12" },
};

const SLASH_SUBS = [
  "{{- /*",
  "  Author: Vladlena Costescu (@lbds137)",
  "  Trigger type: `Slash Command`",
  "  Trigger: `kv`",
  "  Group: `Utility`",
  "  Slash description: `Key value store`",
  "  Defer mode: `Ephemeral Message Response`",
  "  Slash subcommand: `get read a key`",
  "  Slash option: `get.key string! the key`",
  "  Slash subcommand: `set write a key`",
  "  Slash option: `set.key string! the key`",
  "  Slash option: `set.ttl integer seconds`",
  "*/ -}}",
].join("\n");

const SLASH_FLAT = [
  "{{- /*",
  "  Trigger type: `Slash Command`",
  "  Trigger: `probe`",
  "  Group: `Staff Utility`",
  "  Slash option: `who user! a member`",
  "  Slash option: `note string a note`",
  "*/ -}}",
].join("\n");

const PLAIN = [
  "{{- /*",
  "  Trigger type: `Command`",
  "  Trigger: `rules`",
  "  Group: `Staff Utility`",
  "*/ -}}",
].join("\n");

// A live form: an old slash command with stale subcommand rows and a flat option the
// header no longer has, plus fields no header manages.
const LIVE_ENTRIES = [
  ["id", "7"],
  ["type", "cmd"],
  ["trigger", "old"],
  ["case_sensitive", "on"],
  ["interaction_defer_mode", "0"],
  ["role_context_channel", "123"],
  ["slash_command_description", "Old description"],
  ["slash_use_subcommands", "on"],
  ["slash_subcommands.0.name", "stale"],
  ["slash_subcommands.0.description", "gone"],
  ["slash_subcommands.0.options.0.name", "x"],
  ["slash_subcommands.0.options.0.type", "string"],
  ["slash_subcommands.0.options.0.description", "x"],
  ["slash_subcommands.0.options.0.required", "on"],
  ["slash_subcommands.0.options.0.choices", "a\nb"],
  ["slash_subcommands.1.name", "stale2"],
  ["slash_subcommands.1.description", "gone2"],
  ["slash_options.0.name", "flatold"],
  ["slash_options.0.type", "string"],
  ["slash_options.0.description", "d"],
  ["responses", "old code"],
  ["channels", "5"],
  ["channels", "6"],
  ["is_enabled", "on"],
  ["GroupID", "0"],
  ["future_field", "kept"],
];

function names(entries) {
  return J(entries).map((e) => e[0]);
}

test("applyStructure replaces slash rows wholesale, in header order, and keeps everything else", () => {
  const yagDeploy = loadDeployJs();
  const header = yagDeploy.parseHeader(SLASH_SUBS);
  const out = J(yagDeploy.applyStructure(LIVE_ENTRIES, header, LABELS, {}));
  const get = (name) => out.filter((e) => e[0] === name).map((e) => e[1]);

  assert.deepEqual(get("type"), ["slash_command"]);
  assert.deepEqual(get("trigger"), ["kv"]);
  assert.deepEqual(get("GroupID"), ["11"]);
  assert.deepEqual(get("interaction_defer_mode"), ["2"]);
  assert.deepEqual(get("slash_command_description"), ["Key value store"]);
  assert.deepEqual(get("slash_use_subcommands"), ["on"]);

  // No old row survives: no stale subcommands, no flat option, no leftover choices.
  assert.deepEqual(
    out.filter((e) => /^slash_subcommands\.\d+\.name$/.test(e[0])).map((e) => e[1]),
    ["get", "set"]
  );
  assert.equal(get("slash_options.0.name").length, 0);
  assert.ok(!JSON.stringify(out).includes("stale"));
  assert.deepEqual(get("slash_subcommands.0.options.0.choices"), [""]);
  assert.deepEqual(get("slash_subcommands.1.options.1.name"), ["ttl"]);
  assert.deepEqual(get("slash_subcommands.1.options.1.type"), ["integer"]);

  // An unchecked checkbox is absent from FormData, so a non-required row has no `required`
  // entry and a required one has "on".
  assert.deepEqual(get("slash_subcommands.0.options.0.required"), ["on"]);
  assert.deepEqual(get("slash_subcommands.1.options.0.required"), ["on"]);
  assert.deepEqual(get("slash_subcommands.1.options.1.required"), []);

  // Row order follows the header: get before set.
  const order = names(out);
  assert.ok(order.indexOf("slash_subcommands.0.name") < order.indexOf("slash_subcommands.1.name"));

  // Unmanaged entries, including a duplicated multi-select and a field nobody knows.
  assert.deepEqual(get("id"), ["7"]);
  assert.deepEqual(get("case_sensitive"), ["on"]);
  assert.deepEqual(get("role_context_channel"), ["123"]);
  assert.deepEqual(get("responses"), ["old code"]);
  assert.deepEqual(get("channels"), ["5", "6"]);
  assert.deepEqual(get("future_field"), ["kept"]);
});

test("applyStructure writes top-level rows when the header has no subcommands", () => {
  const yagDeploy = loadDeployJs();
  const header = yagDeploy.parseHeader(SLASH_FLAT);
  const out = J(yagDeploy.applyStructure(LIVE_ENTRIES, header, LABELS, {}));
  const get = (name) => out.filter((e) => e[0] === name).map((e) => e[1]);
  assert.deepEqual(get("slash_use_subcommands"), []);
  assert.equal(names(out).filter((n) => n.startsWith("slash_subcommands.")).length, 0);
  assert.deepEqual(get("slash_options.0.name"), ["who"]);
  assert.deepEqual(get("slash_options.0.type"), ["user"]);
  assert.deepEqual(get("slash_options.0.required"), ["on"]);
  assert.deepEqual(get("slash_options.1.name"), ["note"]);
  assert.deepEqual(get("slash_options.1.required"), []);
  // No Slash description line: the live description is left untouched.
  assert.deepEqual(get("slash_command_description"), ["Old description"]);
});

test("applyStructure keeps a non-slash command's slash_* entries", () => {
  const yagDeploy = loadDeployJs();
  const header = yagDeploy.parseHeader(PLAIN);
  const out = J(yagDeploy.applyStructure(LIVE_ENTRIES, header, LABELS, {}));
  const slashOf = (entries) => entries.filter((e) => e[0].startsWith("slash_"));
  assert.deepEqual(slashOf(out), slashOf(J(LIVE_ENTRIES)));
  const get = (name) => out.filter((e) => e[0] === name).map((e) => e[1]);
  assert.deepEqual(get("type"), ["cmd"]);
  assert.deepEqual(get("trigger"), ["rules"]);
  assert.deepEqual(get("GroupID"), ["12"]);
  assert.deepEqual(get("interaction_defer_mode"), ["0"]);
});

test("applyStructure sets is_enabled only when asked", () => {
  const yagDeploy = loadDeployJs();
  const header = yagDeploy.parseHeader(PLAIN);
  const count = (entries) => J(entries).filter((e) => e[0] === "is_enabled").length;
  const off = LIVE_ENTRIES.filter((e) => e[0] !== "is_enabled");
  assert.equal(count(yagDeploy.applyStructure(LIVE_ENTRIES, header, LABELS, { enable: false })), 0);
  assert.equal(count(yagDeploy.applyStructure(off, header, LABELS, { enable: true })), 1);
  assert.equal(count(yagDeploy.applyStructure(LIVE_ENTRIES, header, LABELS, { enable: true })), 1);
  assert.equal(count(yagDeploy.applyStructure(LIVE_ENTRIES, header, LABELS, { enable: null })), 1);
  assert.equal(count(yagDeploy.applyStructure(off, header, LABELS, { enable: null })), 0);
  assert.equal(count(yagDeploy.applyStructure(off, header, LABELS)), 0);
});

test("applyStructure throws on a type or group the panel's selects don't have", () => {
  const yagDeploy = loadDeployJs();
  const header = yagDeploy.parseHeader(PLAIN.replace("Staff Utility", "Nowhere"));
  assert.throws(() => yagDeploy.applyStructure(LIVE_ENTRIES, header, LABELS, {}), /group "Nowhere"/);
  const odd = yagDeploy.parseHeader(PLAIN.replace("`Command`", "`Teleport`"));
  assert.throws(() => yagDeploy.applyStructure(LIVE_ENTRIES, odd, LABELS, {}), /type "Teleport"/);
});

test("liveStructure reads applyStructure's output back with no diff (round trip)", () => {
  const yagDeploy = loadDeployJs();
  for (const source of [SLASH_SUBS, SLASH_FLAT, PLAIN]) {
    const header = yagDeploy.parseHeader(source);
    const out = yagDeploy.applyStructure(LIVE_ENTRIES, header, LABELS, {});
    assert.equal(
      yagDeploy.structureDiff(header, yagDeploy.liveStructure(out, LABELS)),
      null,
      source.split("\n")[2]
    );
  }
});

test("structureDiff names what differs, and live extras the header can't express count", () => {
  const yagDeploy = loadDeployJs();
  const header = yagDeploy.parseHeader(SLASH_SUBS);
  const diff = J(yagDeploy.structureDiff(header, yagDeploy.liveStructure(LIVE_ENTRIES, LABELS)));
  assert.deepEqual(Object.keys(diff).sort(), [
    "deferMode", "group", "slash", "slashDescription", "trigger", "type",
  ]);
  assert.deepEqual(diff.type, { live: "Command", header: "Slash Command" });
  assert.deepEqual(diff.deferMode, { live: "None", header: "Ephemeral Message Response" });
  assert.deepEqual(diff.group, { live: "None", header: "Utility" });

  // choices on an option row: the header can't say them, the apply would blank them
  const applied = J(yagDeploy.applyStructure(LIVE_ENTRIES, header, LABELS, {}));
  const withChoices = applied.map((e) =>
    e[0] === "slash_subcommands.0.options.0.choices" ? [e[0], "a\nb"] : e
  );
  const extra = J(yagDeploy.structureDiff(header, yagDeploy.liveStructure(withChoices, LABELS)));
  assert.deepEqual(Object.keys(extra), ["slash"]);

  // a header without Group / Trigger lines manages neither
  const none = yagDeploy.parseHeader("{{- /*\n  Trigger type: `None`\n*/ -}}");
  const live = yagDeploy.liveStructure(
    [["type", "none"], ["interaction_defer_mode", "0"]],
    { type: { None: "none" }, group: {} }
  );
  assert.equal(yagDeploy.structureDiff(none, live), null);
});

test("decide and classifyReadBack account for structure and enabled state", () => {
  const yagDeploy = loadDeployJs();
  const base = { liveSha: "m", manifestSha: "m", rawSha: "m", dryRun: true };
  assert.equal(yagDeploy.decide(base), "same");
  assert.equal(yagDeploy.decide({ ...base, structureDiffers: true }), "would-update");
  assert.equal(yagDeploy.decide({ ...base, enableChange: true }), "would-update");
  assert.equal(yagDeploy.decide({ ...base, dryRun: false, structureDiffers: true }), "update");

  const rb = { readBackSha: "m", manifestSha: "m", alertText: null };
  assert.equal(yagDeploy.classifyReadBack({ ...rb, structure: null, wantEnabled: null }).status, "updated");
  assert.equal(
    yagDeploy.classifyReadBack({ ...rb, structure: { type: {}, group: {} } }).status,
    "failed: read-back structure type,group"
  );
  assert.equal(
    yagDeploy.classifyReadBack({ ...rb, structure: null, enabled: false, wantEnabled: true }).status,
    "failed: read-back enabled is false, wanted true"
  );
  assert.equal(
    yagDeploy.classifyReadBack({ ...rb, structure: null, enabled: true, wantEnabled: true }).status,
    "updated"
  );
  // not asked about is_enabled: whatever it is, it isn't checked
  assert.equal(
    yagDeploy.classifyReadBack({ ...rb, structure: null, enabled: false, wantEnabled: null }).status,
    "updated"
  );
});

test("run refuses a bad enable/disable list before any fetch or POST", async () => {
  const calls = [];
  const yagDeploy = loadDeployJs({
    fetch: async (url, init) => {
      calls.push([url, init && init.method]);
      throw new Error("must not be called");
    },
  });
  const manifest = {
    guild: "1",
    commands: [
      { path: "commands/a.gohtml", id: 5, sha: "x", raw: "https://raw.example/a" },
      { path: "commands/b.gohtml", id: 6, sha: "y", raw: "https://raw.example/b" },
    ],
  };

  const both = J(await yagDeploy.run(manifest, { dryRun: false, enable: [5], disable: [5] }));
  assert.equal(both.length, 1);
  assert.equal(both[0].id, 5);
  assert.match(both[0].status, /^failed: options: in both/);

  const unknown = J(await yagDeploy.run(manifest, { dryRun: false, enable: [6], disable: [99] }));
  assert.equal(unknown.length, 1);
  assert.equal(unknown[0].id, 99);
  assert.match(unknown[0].status, /^failed: options: not in the manifest/);

  assert.equal(calls.length, 0, "no GET and no POST on a refused run");
});

test("groupIdsFromLinks and newCommandId read the panel's group tabs and redirect", () => {
  const yagDeploy = loadDeployJs();
  const groups = J(
    yagDeploy.groupIdsFromLinks([
      ["/manage/1/customcommands/", "Ungrouped"],
      ["/manage/1/customcommands/groups/11", "Utility"],
      ["/manage/1/customcommands/groups/12/", "\n   Staff Utility\n  "],
      ["/manage/1/other", "Utility"],
    ])
  );
  assert.deepEqual(groups, { Utility: "11", "Staff Utility": "12" });
  assert.equal(yagDeploy.newCommandId("https://yagpdb.xyz/manage/1/customcommands/commands/103/"), "103");
  assert.equal(yagDeploy.newCommandId("https://yagpdb.xyz/manage/1/customcommands/commands/103"), "103");
  assert.equal(yagDeploy.newCommandId("https://yagpdb.xyz/manage/1/customcommands/"), null);
  assert.equal(yagDeploy.newCommandId(undefined), null);
});

test("name is managed: derived from the path, applied, diffed and round-tripped", () => {
  const yagDeploy = loadDeployJs();
  assert.equal(yagDeploy.commandName("commands/color/color_slash.gohtml"), "color_slash");
  assert.equal(yagDeploy.commandName("rule.gohtml"), "rule");

  const header = yagDeploy.parseHeader(PLAIN);
  header.name = "rules";
  const live = [...LIVE_ENTRIES, ["name", "rule browse"]];
  const diff = J(yagDeploy.structureDiff(header, yagDeploy.liveStructure(live, LABELS)));
  assert.deepEqual(diff.name, { live: "rule browse", header: "rules" });

  const out = J(yagDeploy.applyStructure(live, header, LABELS, {}));
  assert.deepEqual(out.filter((e) => e[0] === "name").map((e) => e[1]), ["rules"]);
  assert.equal(yagDeploy.structureDiff(header, yagDeploy.liveStructure(out, LABELS)), null);

  // an empty live name (a command nobody named) is a diff too, and is filled in
  const empty = J(yagDeploy.applyStructure([...LIVE_ENTRIES, ["name", ""]], header, LABELS, {}));
  assert.deepEqual(empty.filter((e) => e[0] === "name").map((e) => e[1]), ["rules"]);

  // a header object without a name (name not derived) leaves the field alone
  const bare = yagDeploy.parseHeader(PLAIN);
  const kept = J(yagDeploy.applyStructure(live, bare, LABELS, {}));
  assert.deepEqual(kept.filter((e) => e[0] === "name").map((e) => e[1]), ["rule browse"]);
});

// A stub panel for run(): one edit form at /commands/5/, a raw fetch for the pinned bytes,
// and just enough DOM for deploy.js (querySelector/All on the doc and the form).
function stubPanel(rawCode, liveEntries, posts) {
  const opts = (map) => Object.keys(map).map((text) => ({ text, value: map[text] }));
  const form = {
    __entries: liveEntries,
    getAttribute: (n) => (n === "action" ? "/manage/1/customcommands/commands/5/update" : null),
    querySelectorAll: (sel) => (sel === "textarea[name=responses]" ? [{ value: "old live code" }] : []),
    querySelector: (sel) =>
      sel.includes("'type'") ? { options: opts(LABELS.type) }
        : sel.includes("GroupID") ? { options: opts(LABELS.group) }
          : null,
  };
  const doc = { querySelectorAll: (sel) => (sel === "form" ? [form] : []), querySelector: () => null };
  return {
    setTimeout: (f) => f(),
    DOMParser: class { parseFromString() { return doc; } },
    FormData: class {
      constructor(f) { this.list = f.__entries; }
      entries() { return this.list[Symbol.iterator](); }
    },
    URLSearchParams,
    fetch: async (url, init) => {
      if (init && init.method === "POST") posts.push(url);
      if (String(url).startsWith("https://raw.example")) return { ok: true, text: async () => rawCode };
      return { ok: true, status: 200, url, text: async () => "<html>" };
    },
  };
}

test("a dry run reports failed, not would-update, when the real run couldn't build its post", async () => {
  for (const [group, want] of [["Nowhere", /^failed: header group "Nowhere"/], ["Utility", /^would-update$/]]) {
    const posts = [];
    const raw = PLAIN.replace("Staff Utility", group);
    const probe = loadDeployJs();
    const sha = await probe.sha256hex(probe.normalize(raw));
    const yagDeploy = loadDeployJs(stubPanel(raw, LIVE_ENTRIES.filter((e) => e[0] !== "slash_use_subcommands"), posts));
    const manifest = {
      guild: "1",
      commands: [{ path: "commands/x/rules.gohtml", id: 5, sha, raw: "https://raw.example/rules" }],
    };
    for (const dryRun of [true, false]) {
      const res = J(await yagDeploy.run(manifest, { dryRun }));
      if (dryRun) {
        assert.match(res[0].status, want, group);
        assert.equal(res[0].enabledWanted, null);
      } else if (group === "Nowhere") {
        assert.match(res[0].status, want, group);
        assert.equal(posts.length, 0, "nothing posted when the post can't be built");
      }
    }
    if (group === "Utility") assert.equal(posts.length, 1, "positive control: the real run posts");
  }
});

test("run states the planned enable/disable per command (enabledWanted)", async () => {
  const raw = PLAIN;
  const probe = loadDeployJs();
  const sha = await probe.sha256hex(probe.normalize(raw));
  const yagDeploy = loadDeployJs(stubPanel(raw, LIVE_ENTRIES, []));
  const manifest = {
    guild: "1",
    commands: [{ path: "commands/x/rules.gohtml", id: 5, sha, raw: "https://raw.example/rules" }],
  };
  const dis = J(await yagDeploy.run(manifest, { dryRun: true, disable: [5] }));
  assert.equal(dis[0].enabled, true);
  assert.equal(dis[0].enabledWanted, false);
  const none = J(await yagDeploy.run(manifest, { dryRun: true }));
  assert.equal(none[0].enabledWanted, null);
});

test("create's status texts say the command may exist, and the guard reads the list page's names", () => {
  const yagDeploy = loadDeployJs();
  const failed = yagDeploy.createdButStatus(104, "not verified disabled");
  assert.match(failed, /^failed: created #104 but not verified disabled/);
  assert.match(failed, /still ENABLED with the placeholder: disable it in the panel$/);
  const noId = yagDeploy.noIdStatus("https://yagpdb.xyz/manage/1/customcommands/");
  assert.match(noId, /^failed: no new command id in https:\/\/yagpdb\.xyz\/manage\/1\/customcommands\//);
  assert.match(noId, /a command MAY have been created/);

  const names = J(
    yagDeploy.namedCommandsFromCards([
      ["#collapse_cmd103", "\n  #103 -\n  Slash Command\n  : color\n  Name: color_slash\n"],
      ["#collapse_cmd7", "#7 - Command : rules"],
      ["#collapse_cmd89", "#89 - Command : rule Name: rule browse"],
      ["/elsewhere", "Name: nope"],
    ])
  );
  assert.deepEqual(names, { color_slash: "103", "rule browse": "89" });
});

test("parseHeader reads Defer mode and Slash description lines", () => {
  const yagDeploy = loadDeployJs();
  const header = yagDeploy.parseHeader(SLASH_SUBS);
  assert.equal(header.deferMode, 2);
  assert.equal(header.slashDescription, "Key value store");
  assert.equal(header.type, "Slash Command");
  assert.equal(header.error, null);
  assert.equal(yagDeploy.parseHeader(PLAIN).deferMode, 0);
  assert.equal(yagDeploy.parseHeader(PLAIN).slashDescription, null);
  assert.equal(yagDeploy.parseHeader(PLAIN).slash, null);
});
