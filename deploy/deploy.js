// Pasted into Claude-in-Chrome's javascript_tool while a YAGPDB control-panel tab is open
// and logged in. Defines window.yagDeploy. Plain browser JS: no modules, no build step.
//
// yagDeploy.run(manifest, {dryRun}) walks manifest.commands (from
// `make deploy-manifest SERVER=<s>`), diffs each command's live code in the panel against
// the manifest's sha256, and -- only when dryRun is false -- POSTs the *exact* bytes fetched
// from raw.githubusercontent.com at the pinned commit, reusing the live edit page's own form
// so none of the panel's extra fields (role_trigger_mode, slash_command_description, ...)
// get dropped. It never navigates, clicks, or creates/deletes commands.
(function () {
  "use strict";

  // Trailing-whitespace class: exactly Python's str.isspace() membership (this must match
  // scripts/deploy-manifest.py's `text.rstrip()`, since Python is the manifest side of the
  // live-panel-vs-manifest comparison), NOT JS's `\s` -- the two disagree (JS's \s includes
  // U+FEFF, which isspace() doesn't; isspace() includes U+001C-001F and U+0085, which \s
  // doesn't). Derived by running, in Python: `[hex(c) for c in range(0x110000) if
  // chr(c).isspace()]`.
  // eslint-disable-next-line no-control-regex -- U+001C-001F/U+0085 are load-bearing here.
  var TRAILING_WHITESPACE = new RegExp(
    "[" +
      "\\u0009\\u000A\\u000B\\u000C\\u000D" +
      "\\u001C\\u001D\\u001E\\u001F" +
      "\\u0020\\u0085\\u00A0\\u1680" +
      "\\u2000-\\u200A\\u2028\\u2029\\u202F\\u205F\\u3000" +
      "]+$"
  );

  function normalize(s) {
    return s.replace(/\r\n/g, "\n").replace(TRAILING_WHITESPACE, "");
  }

  async function sha256hex(s) {
    const bytes = new TextEncoder().encode(s);
    const digest = await crypto.subtle.digest("SHA-256", bytes);
    return Array.from(new Uint8Array(digest))
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
  }

  function parseHeader(code) {
    const typeMatch = code.match(/Trigger type:\s*`([^`]*)`/);
    const triggerMatch = code.match(/\n\s*Trigger:\s*`([^`]*)`/);
    const groupMatch = code.match(/\n\s*Group:\s*`([^`]*)`/);
    return {
      type: typeMatch ? typeMatch[1] : null,
      trigger: triggerMatch ? triggerMatch[1] : null,
      group: groupMatch ? groupMatch[1] : null,
    };
  }

  function panelTypeName(label) {
    const trimmed = (label || "").trim();
    if (trimmed.indexOf("Command") === 0) return "Command";
    return trimmed;
  }

  function panelGroupName(label) {
    return (label || "").trim();
  }

  // Pure/unit-testable: the drift object run() reports, or null when nothing differs. A
  // field is compared only when both the header and the panel have it.
  function headerDrift(headerInfo, { panelType, panelTrigger, panelGroup }) {
    const drift = {};
    if (headerInfo.type !== null && panelType !== null && headerInfo.type !== panelType) {
      drift.type = { header: headerInfo.type, panel: panelType };
    }
    if (
      headerInfo.trigger !== null &&
      panelTrigger !== null &&
      headerInfo.trigger !== panelTrigger
    ) {
      drift.trigger = { header: headerInfo.trigger, panel: panelTrigger };
    }
    if (headerInfo.group !== null && panelGroup !== null && headerInfo.group !== panelGroup) {
      drift.group = { header: headerInfo.group, panel: panelGroup };
    }
    return Object.keys(drift).length ? drift : null;
  }

  function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  function findUpdateForm(doc, id) {
    const forms = doc.querySelectorAll("form");
    for (const form of forms) {
      const action = form.getAttribute("action") || "";
      if (action.endsWith("commands/" + id + "/update")) return form;
    }
    return null;
  }

  // The panel shows site-wide notice banners, so an `.alert-danger`/`.alert.alert-error` on
  // a page is not by itself evidence that a save failed -- it's attached as `note`, and
  // success is always decided by the read-back hash (classifyReadBack), never by this alone.
  function findErrorAlert(doc) {
    const el = doc.querySelector(".alert-danger, .alert.alert-error");
    if (!el) return null;
    return (el.textContent || "").trim().slice(0, 200);
  }

  async function fetchEditPage(guild, id) {
    const url = "/manage/" + guild + "/customcommands/commands/" + id + "/";
    const res = await fetch(url, { credentials: "same-origin" });
    if (res.status === 404) return { status: 404, doc: null };
    const html = await res.text();
    const doc = new DOMParser().parseFromString(html, "text/html");
    return { status: res.status, doc };
  }

  // Pure decision logic, factored out of run() so it's unit-testable without a DOM/fetch
  // stub. Given the raw fetch's sha (already checked against manifestSha by the caller for
  // response.ok; rawSha itself may just be a mismatched digest) and the live sha, decides
  // what run() should do next for this command.
  //   "failed: raw mismatch" -- the raw fetch didn't reproduce the manifest's bytes; stop
  //     here even in a dry run, since a dry run is supposed to prove a real update would work.
  //   "same"                 -- live already matches the manifest; nothing to do.
  //   "would-update"         -- dry run only; live differs, but nothing is written.
  //   "update"               -- proceed to POST the raw code.
  function decide({ liveSha, manifestSha, rawSha, dryRun }) {
    if (rawSha !== manifestSha) return "failed: raw mismatch";
    if (liveSha === manifestSha) return "same";
    if (dryRun) return "would-update";
    return "update";
  }

  // Also pure/unit-testable: the panel's alert banners are site-wide notices, not reliable
  // failure signals, so success is decided ONLY by the read-back hash. An alert (if any) is
  // still surfaced as `note` for a human to see, whether the read-back matched or not.
  function classifyReadBack({ readBackSha, manifestSha, alertText }) {
    if (readBackSha === manifestSha) {
      return { status: "updated", note: alertText || null };
    }
    return { status: "failed: read-back mismatch", note: alertText || null };
  }

  async function run(manifest, opts) {
    const dryRun = !opts || opts.dryRun !== false;
    const guild = manifest.guild;
    const results = [];

    for (const cmd of manifest.commands) {
      const { path, id, sha: manifestSha, raw } = cmd;
      let result = { path, id, status: null, drift: null };

      try {
        const { status, doc } = await fetchEditPage(guild, id);
        if (status === 404 || !doc) {
          result.status = "missing";
          results.push(result);
          await sleep(800);
          continue;
        }

        const form = findUpdateForm(doc, id);
        if (!form) {
          result.status = "missing";
          results.push(result);
          await sleep(800);
          continue;
        }

        const textareas = form.querySelectorAll("textarea[name=responses]");
        if (textareas.length > 1) {
          result.status = "skipped-multi";
          results.push(result);
          await sleep(800);
          continue;
        }

        const liveCode = textareas.length === 1 ? textareas[0].value : "";
        const liveSha = await sha256hex(normalize(liveCode));

        // Always fetch raw, before the same/would-update/update decision -- even in a dry
        // run, so a dry run actually proves the raw bytes are fetchable and correct, and so
        // drift (below) reads the REPO header, never the live code's.
        let rawCode = null;
        let rawSha = null;
        const rawRes = await fetch(raw);
        if (rawRes.ok) {
          rawCode = await rawRes.text();
          rawSha = await sha256hex(normalize(rawCode));
        }

        const decision = decide({ liveSha, manifestSha, rawSha, dryRun });

        // Drift is report-only: compare the REPO header (from the raw fetch, not the live
        // code) against the panel's live type/trigger/group fields. Never changes them.
        const headerInfo =
          rawCode !== null ? parseHeader(rawCode) : { type: null, trigger: null, group: null };
        const typeSelect = form.querySelector("select[name=trigger_type], select[name=type]");
        const triggerInput = form.querySelector("input[name=trigger], input[name=text_trigger]");
        const groupSelect = form.querySelector("select[name=GroupID]");
        const panelType = typeSelect
          ? panelTypeName(typeSelect.options[typeSelect.selectedIndex].text)
          : null;
        const panelTrigger = triggerInput ? triggerInput.value : null;
        // The selected option's text is the live group name (its value is the group id).
        const panelGroup = groupSelect
          ? panelGroupName(groupSelect.options[groupSelect.selectedIndex].text)
          : null;
        result.drift = headerDrift(headerInfo, { panelType, panelTrigger, panelGroup });

        if (decision !== "update") {
          result.status = decision;
          results.push(result);
          await sleep(800);
          continue;
        }

        const fd = new FormData(form);
        fd.set("responses", rawCode);
        const postRes = await fetch(form.getAttribute("action"), {
          method: "POST",
          credentials: "same-origin",
          headers: { "Content-Type": "application/x-www-form-urlencoded" },
          body: new URLSearchParams(fd),
        });
        const postHtml = await postRes.text();
        const postDoc = new DOMParser().parseFromString(postHtml, "text/html");
        const alertText = findErrorAlert(postDoc);

        // ALWAYS re-fetch and hash, regardless of whether an alert matched: the panel shows
        // site-wide notice banners too, so an alert on the page is not evidence of failure
        // by itself. Success is decided by the read-back hash alone (classifyReadBack).
        const readBack = await fetchEditPage(guild, id);
        const readBackForm = readBack.doc ? findUpdateForm(readBack.doc, id) : null;
        const readBackTextarea = readBackForm
          ? readBackForm.querySelector("textarea[name=responses]")
          : null;
        const readBackSha = readBackTextarea
          ? await sha256hex(normalize(readBackTextarea.value))
          : null;

        const classified = classifyReadBack({ readBackSha, manifestSha, alertText });
        result.status = classified.status;
        result.note = classified.note;
        results.push(result);
      } catch (err) {
        result.status = "failed: " + (err && err.message ? err.message : String(err));
        results.push(result);
      }

      await sleep(800);
    }

    return results;
  }

  window.yagDeploy = {
    normalize,
    sha256hex,
    parseHeader,
    panelTypeName,
    panelGroupName,
    headerDrift,
    decide,
    classifyReadBack,
    run,
  };
})();
