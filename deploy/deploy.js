// Pasted into Claude-in-Chrome's javascript_tool (or fetched and `(0, eval)`'d) while a
// YAGPDB control-panel tab is open and logged in. Defines window.yagDeploy. Plain browser
// JS: no modules, no build step.
//
// yagDeploy.run(manifest, {dryRun, enable, disable}) walks manifest.commands (from
// `make deploy-manifest SERVER=<s>`), diffs each command's live code AND panel structure
// against the manifest's sha256 and the command's committed header, and -- only when
// dryRun is false -- POSTs the *exact* bytes fetched from raw.githubusercontent.com at the
// pinned commit together with the header's structure (trigger type, trigger, group, defer
// mode, slash description and rows), reusing the live edit page's own form so every field
// the header doesn't manage (channels, roles, role_trigger_mode, ...) survives untouched.
// `enable` / `disable` are lists of command ids whose is_enabled is set; nothing else ever
// touches it. yagDeploy.create(guild, items, {dryRun}) creates new commands (DISABLED).
// It never deletes anything.
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

  // ---------------------------------------------------------------------------------------
  // Header parsing. The defer mode, slash description and slash rows are a port of the
  // emulator's reader (tools/emulator/internal/runtime/trigger.go ReadDeferMode,
  // ValidateHeader; slash.go ReadSlashCommand), error messages included. Go's
  // TestHeaderGolden writes deploy/testdata/headers.golden.json from the Go side, and
  // deploy.test.js asserts this port reads every committed header and every rejected header
  // the same.
  // ---------------------------------------------------------------------------------------

  // Go's strings.Fields / unicode.IsSpace set (NOT JS's \s: U+FEFF differs, U+0085 differs).
  var WS = "\\t\\n\\u000B\\u000C\\r \\u0085\\u00A0\\u1680\\u2000-\\u200A\\u2028\\u2029\\u202F\\u205F\\u3000";
  var WS_SPLIT = new RegExp("[" + WS + "]+");
  var WS_TRIM = new RegExp("^[" + WS + "]+|[" + WS + "]+$", "g");

  function fields(s) {
    return s.split(WS_SPLIT).filter(function (f) {
      return f !== "";
    });
  }

  function goTrim(s) {
    return s.replace(WS_TRIM, "");
  }

  function runeCount(s) {
    return Array.from(s).length;
  }

  // Go's %q for the strings headers hold: the quoting of the common cases (printable text,
  // quotes, backslashes, control characters). Non-printable Unicode that Go would \u-escape
  // is left as is.
  function goQuote(s) {
    var out = '"';
    for (const ch of s) {
      const c = ch.codePointAt(0);
      if (ch === '"') out += '\\"';
      else if (ch === "\\") out += "\\\\";
      else if (ch === "\n") out += "\\n";
      else if (ch === "\t") out += "\\t";
      else if (ch === "\r") out += "\\r";
      else if (c < 0x20 || c === 0x7f) out += "\\x" + c.toString(16).padStart(2, "0");
      else out += ch;
    }
    return out + '"';
  }

  // The panel's names (customcommands.go:179, :184) and limits (:179-190).
  var SLASH_NAME_RE = /^[-_\p{L}\p{N}]{1,32}$/u;
  var CONTEXT_MENU_NAME_RE = /^[-_\p{L}\p{N} ]{1,32}$/u;
  var MAX_SLASH_OPTIONS = 25;
  var MAX_SLASH_DESCRIPTION = 100;

  // The panel's option type keys (slashFormType, customcommands.go:278-302).
  var SLASH_FORM_TYPES = [
    "string", "string_menu", "integer", "integer_menu", "number", "number_menu",
    "boolean", "user", "channel", "role", "mentionable",
  ];
  var SLASH_FORM_TYPE_KEYS = SLASH_FORM_TYPES.join(", ");

  // The panel's interaction defer modes, index = the radio's value
  // (customcommands-editcmd.html:319-347; customcommands.go:135-140).
  var DEFER_LABELS = [
    "None", "Message Response", "Ephemeral Message Response", "Update Message Response",
  ];

  function eqFold(a, b) {
    return a.toLowerCase() === b.toLowerCase();
  }

  // The header's leading {{/* ... */}} comment, or "" (trigger.go headerComment).
  function headerComment(source) {
    var s = source.replace(/^[ \t\r\n]+/, "");
    if (s.slice(0, 2) !== "{{") return "";
    s = s.slice(2).replace(/^[- ]+/, "");
    if (s.slice(0, 2) !== "/*") return "";
    var end = s.indexOf("*/");
    return end >= 0 ? s.slice(0, end) : "";
  }

  function headerLine(key, header) {
    var m = new RegExp(key + ": `([^`]*)`", "i").exec(header);
    return m ? m[1] : null;
  }

  function headerLines(key, header) {
    var re = new RegExp(key + ": `([^`]*)`", "gi");
    var out = [];
    var m;
    while ((m = re.exec(header)) !== null) out.push(m[1]);
    return out;
  }

  function parseDeferMode(label) {
    var want = goTrim(label).toLowerCase();
    for (var i = 0; i < DEFER_LABELS.length; i++) {
      if (DEFER_LABELS[i].toLowerCase() === want) return i;
    }
    return -1;
  }

  function parseSlashSubcommandRow(row) {
    var f = fields(row);
    if (f.length < 2) throw new Error("Slash subcommand `" + row + "`: write `name description`");
    return { name: f[0], description: f.slice(1).join(" "), options: [] };
  }

  // "[sub.]name type[!] description" -> {sub, opt}
  function parseSlashOptionRow(row) {
    var f = fields(row);
    if (f.length < 3) {
      throw new Error(
        "Slash option `" + row + "`: write `[sub.]name type[!] description` (type: " +
          SLASH_FORM_TYPE_KEYS + "; ! = required)"
      );
    }
    var name = f[0];
    var sub = "";
    var dot = name.indexOf(".");
    if (dot >= 0) {
      sub = name.slice(0, dot);
      name = name.slice(dot + 1);
    }
    var formType = f[1];
    var required = false;
    if (formType.slice(-1) === "!") {
      formType = formType.slice(0, -1);
      required = true;
    }
    if (SLASH_FORM_TYPES.indexOf(formType) < 0) {
      throw new Error(
        "Invalid type for option " + goQuote(name) + ": " + goQuote(f[1]) + " isn't one of " +
          SLASH_FORM_TYPE_KEYS
      );
    }
    return {
      sub: sub,
      opt: { name: name, type: formType, description: f.slice(2).join(" "), required: required },
    };
  }

  // validateSlashOptionList, customcommands.go:715-741.
  function validateSlashOptionList(options) {
    if (options.length > MAX_SLASH_OPTIONS) {
      throw new Error("can have at most " + MAX_SLASH_OPTIONS + " options");
    }
    var seen = {};
    for (const opt of options) {
      var oname = goTrim(opt.name);
      if (!SLASH_NAME_RE.test(oname)) {
        throw new Error(
          "Option name " + goQuote(opt.name) +
            " must be 1-32 characters (letters, numbers, dashes, underscores)"
        );
      }
      if (oname !== oname.toLowerCase()) {
        throw new Error("Option name " + goQuote(opt.name) + " must be lowercase");
      }
      if (Object.prototype.hasOwnProperty.call(seen, oname)) {
        throw new Error("Duplicate option name " + goQuote(oname));
      }
      seen[oname] = true;
      var l = runeCount(opt.description);
      if (l < 1 || l > MAX_SLASH_DESCRIPTION) {
        throw new Error(
          "Description for option " + goQuote(oname) + " must be between 1 and " +
            MAX_SLASH_DESCRIPTION + " characters"
        );
      }
    }
  }

  // validateSlashCommandData's subcommand and option checks, customcommands.go:657-686.
  function validateSlashDef(def) {
    if (def.subcommands.length > 0) {
      var seenSubs = {};
      for (const sub of def.subcommands) {
        var sname = goTrim(sub.name);
        if (!SLASH_NAME_RE.test(sname)) {
          throw new Error(
            "Subcommand name " + goQuote(sub.name) +
              " must be 1-32 characters (letters, numbers, dashes, underscores)"
          );
        }
        if (sname !== sname.toLowerCase()) {
          throw new Error("Subcommand name " + goQuote(sub.name) + " must be lowercase");
        }
        if (Object.prototype.hasOwnProperty.call(seenSubs, sname)) {
          throw new Error("Duplicate subcommand name " + goQuote(sname));
        }
        seenSubs[sname] = true;
        var l = runeCount(sub.description);
        if (l < 1 || l > MAX_SLASH_DESCRIPTION) {
          throw new Error(
            "Description for subcommand " + goQuote(sname) + " must be between 1 and " +
              MAX_SLASH_DESCRIPTION + " characters"
          );
        }
        try {
          validateSlashOptionList(sub.options);
        } catch (err) {
          throw new Error("Subcommand " + goQuote(sname) + ": " + err.message);
        }
      }
      return;
    }
    validateSlashOptionList(def.options);
  }

  // slash.go ReadSlashCommand for a header already known to be a Slash Command: the
  // definition, or a thrown Error with the emulator's message.
  function readSlashCommand(source, triggerText, deferMode, description) {
    var name = goTrim(triggerText);
    if (triggerText !== triggerText.toLowerCase()) {
      throw new Error("Slash command name must be lowercase");
    }
    if (!SLASH_NAME_RE.test(name.toLowerCase())) {
      throw new Error(
        "Slash command name must be 1-32 characters and contain only letters, numbers, " +
          "dashes and underscores"
      );
    }
    if (deferMode === 3) {
      throw new Error('"Update message" defer mode is not valid for slash commands');
    }
    if (description !== null) {
      var dl = runeCount(description);
      if (dl < 1 || dl > MAX_SLASH_DESCRIPTION) {
        throw new Error(
          "Slash command description must be between 1 and " + MAX_SLASH_DESCRIPTION +
            " characters"
        );
      }
    }

    var header = headerComment(source);
    var def = { subcommands: [], options: [] };
    for (const row of headerLines("Slash subcommand", header)) {
      def.subcommands.push(parseSlashSubcommandRow(row));
    }
    for (const row of headerLines("Slash option", header)) {
      var parsed = parseSlashOptionRow(row);
      if (parsed.sub === "") {
        if (def.subcommands.length > 0) {
          throw new Error(
            "Slash option `" + row + "`: a command with subcommands has no top-level " +
              "options; write `<subcommand>." + parsed.opt.name + " ...`"
          );
        }
        def.options.push(parsed.opt);
        continue;
      }
      var placed = false;
      for (const sub of def.subcommands) {
        if (eqFold(sub.name, parsed.sub)) {
          sub.options.push(parsed.opt);
          placed = true;
          break;
        }
      }
      if (!placed) {
        throw new Error(
          "Slash option `" + row + "`: no Slash subcommand line names " + goQuote(parsed.sub) +
            " (subcommand lines come first)"
        );
      }
    }
    validateSlashDef(def);
    return def;
  }

  // slash.go validateDeployableSlashDef: two shapes the panel stores differently (required
  // options first, customcommands.go:396-398; *_menu shown back as the base type,
  // :308-324), which a read-back could never match.
  function validateDeployableSlashDef(def) {
    function check(options) {
      var seenOptional = false;
      for (const opt of options) {
        if (/_menu$/.test(opt.type)) {
          var base = opt.type.replace(/_menu$/, "");
          throw new Error(
            "Option " + goQuote(opt.name) + ": type " + goQuote(opt.type) +
              " is a choices menu, which a header can't express (the panel shows it back as " +
              goQuote(base) + "): use " + goQuote(base)
          );
        }
        if (opt.required && seenOptional) {
          throw new Error(
            "Option " + goQuote(opt.name) + " is required but follows an optional one: write the " +
              "required options first (the panel stores them that way)"
          );
        }
        if (!opt.required) seenOptional = true;
      }
    }
    for (const sub of def.subcommands) {
      try {
        check(sub.options);
      } catch (err) {
        throw new Error("Subcommand " + goQuote(sub.name) + ": " + err.message);
      }
    }
    check(def.options);
  }

  // trigger.go ValidateHeader: the message of the first thing it rejects, or null.
  function validateHeaderError(source, deferMode, trigger) {
    var header = headerComment(source);
    var switches = ["Case sensitive", "Show errors"];
    for (const sw of switches) {
      var v = headerLine(sw, header);
      if (v !== null && !eqFold(v, "true") && !eqFold(v, "false")) {
        return "header " + sw + ": `" + v + "` isn't true or false";
      }
    }
    var redirect = headerLine("Redirect errors", header);
    if (redirect !== null) {
      var okId = /^[+-]?\d+$/.test(redirect) && BigInt(redirect) > 0n && BigInt(redirect) < 2n ** 63n;
      if (!okId) return "header Redirect errors: `" + redirect + "` isn't a channel ID";
    }
    var defer = headerLine("Defer mode", header);
    if (defer !== null && parseDeferMode(defer) < 0) {
      return "header Defer mode: `" + defer + "` isn't one of the panel's: " + DEFER_LABELS.join(", ");
    }
    var isSlash = trigger.type !== null && eqFold(trigger.type, "Slash Command");
    var isContext =
      trigger.type !== null &&
      (eqFold(trigger.type, "User Context Menu") || eqFold(trigger.type, "Message Context Menu"));
    if (isSlash) {
      try {
        var def = readSlashCommand(source, trigger.text, deferMode, headerLine("Slash description", header));
        // Only a header naming a panel group is a deployable command (trigger.go ValidateHeader).
        if (headerLine("Group", header) !== null) validateDeployableSlashDef(def);
      } catch (err) {
        return "header: " + err.message;
      }
      return null;
    }
    if (isContext) {
      // validateContextMenuHeader, customcommands.go:538-540, :553-557
      if (!CONTEXT_MENU_NAME_RE.test(goTrim(trigger.text))) {
        return (
          "header: Context menu command name must be 1-32 characters and contain only " +
          "letters, numbers, spaces, dashes and underscores"
        );
      }
      if (deferMode === 3) {
        return 'header: "Update message" defer mode is not valid for context menu commands';
      }
    }
    if (
      headerLine("Slash option", header) !== null ||
      headerLine("Slash subcommand", header) !== null
    ) {
      return "header: Slash option and Slash subcommand lines need a Slash Command trigger";
    }
    if (headerLine("Slash description", header) !== null) {
      return "header: Slash description needs a Slash Command trigger";
    }
    return null;
  }

  // The header's panel-facing fields. `type`, `trigger` and `group` are read from the whole
  // source (null when absent); the rest follows the emulator. `deferMode` is the radio's
  // value (0-3, None when the line is absent), `slashDescription` null without the line,
  // `slash` {subcommands, options} for a Slash Command header and null otherwise, and
  // `error` the emulator's ValidateHeader message when it rejects the header.
  function parseHeader(code) {
    const typeMatch = code.match(/Trigger type:\s*`([^`]*)`/);
    const triggerMatch = code.match(/\n\s*Trigger:\s*`([^`]*)`/);
    const groupMatch = code.match(/\n\s*Group:\s*`([^`]*)`/);
    var header = headerComment(code);
    var deferLine = headerLine("Defer mode", header);
    var deferMode = 0;
    if (deferLine !== null && parseDeferMode(deferLine) >= 0) deferMode = parseDeferMode(deferLine);
    var typeLine = headerLine("Trigger type", header);
    var trigger = {
      type: typeLine,
      text: typeLine === null ? "" : headerLine("Trigger", header) || "",
    };
    var slashDescription = headerLine("Slash description", header);
    var slash = null;
    var error = validateHeaderError(code, deferMode, trigger);
    if (error === null && trigger.type !== null && eqFold(trigger.type, "Slash Command")) {
      slash = readSlashCommand(code, trigger.text, deferMode, slashDescription);
    }
    return {
      type: typeMatch ? typeMatch[1] : null,
      trigger: triggerMatch ? triggerMatch[1] : null,
      group: groupMatch ? groupMatch[1] : null,
      deferMode: deferMode,
      slashDescription: slashDescription,
      slash: slash,
      error: error,
    };
  }

  function panelTypeName(label) {
    const trimmed = (label || "").trim();
    if (trimmed.indexOf("Command") === 0) return "Command";
    return trimmed;
  }

  // The panel's cosmetic command name (form field `name`): the file's basename without
  // `.gohtml`, derived from the manifest path, not from a header line.
  function commandName(path) {
    return String(path || "").split("/").pop().replace(/\.gohtml$/, "");
  }

  function panelGroupName(label) {
    return (label || "").trim();
  }

  function collapseWs(s) {
    return (s || "").replace(/\s+/g, " ").trim();
  }

  // ---------------------------------------------------------------------------------------
  // Structure: the fields the header manages, as plain [name, value] entry lists (what
  // `new FormData(form).entries()` yields) and plain label maps -- no DOM in here.
  // ---------------------------------------------------------------------------------------

  var SLASH_COMMAND = "Slash Command";
  // Option fields a header row can't express; posted empty, and a live non-empty value
  // counts as a difference.
  var EXTRA_OPTION_FIELDS = ["choices", "min_value", "max_value", "min_length", "max_length", "channel_types"];

  function entryValue(entries, name) {
    for (const e of entries) if (e[0] === name) return e[1];
    return null;
  }

  function hasEntry(entries, name) {
    return entryValue(entries, name) !== null;
  }

  function labelFor(map, value) {
    if (value === null || !map) return null;
    for (const key of Object.keys(map)) if (map[key] === value) return key;
    return null;
  }

  function sortedKeys(obj) {
    return Object.keys(obj).sort(function (a, b) {
      return Number(a) - Number(b);
    });
  }

  function normalizeRow(fieldMap) {
    var extra = false;
    for (const f of EXTRA_OPTION_FIELDS) {
      if (fieldMap[f] !== undefined && fieldMap[f] !== "") extra = true;
    }
    return {
      name: fieldMap.name || "",
      type: fieldMap.type || "",
      description: fieldMap.description || "",
      required: fieldMap.required !== undefined,
      extra: extra,
    };
  }

  function emptyRow(row) {
    return row.name === "" && row.description === "";
  }

  // The live form's slash rows: {subcommands, options}; the subcommand rows only when "use
  // subcommands" is on, the flat rows only when it is off (the server ignores the other).
  function liveSlashRows(entries) {
    var flat = {};
    var subs = {};
    function sub(i) {
      return (subs[i] = subs[i] || { name: "", description: "", options: {} });
    }
    function put(target, field, value) {
      target[field] = target[field] === undefined ? value : target[field] + "," + value;
    }
    for (const e of entries) {
      var name = e[0];
      var m = /^slash_options\.(\d+)\.([a-z_]+)$/.exec(name);
      if (m) {
        flat[m[1]] = flat[m[1]] || {};
        put(flat[m[1]], m[2], e[1]);
        continue;
      }
      m = /^slash_subcommands\.(\d+)\.(name|description)$/.exec(name);
      if (m) {
        sub(m[1])[m[2]] = e[1];
        continue;
      }
      m = /^slash_subcommands\.(\d+)\.options\.(\d+)\.([a-z_]+)$/.exec(name);
      if (m) {
        var opts = sub(m[1]).options;
        opts[m[2]] = opts[m[2]] || {};
        put(opts[m[2]], m[3], e[1]);
      }
    }
    var useSubs = hasEntry(entries, "slash_use_subcommands");
    if (!useSubs) {
      return {
        subcommands: [],
        options: sortedKeys(flat)
          .map(function (i) { return normalizeRow(flat[i]); })
          .filter(function (r) { return !emptyRow(r); }),
      };
    }
    return {
      subcommands: sortedKeys(subs)
        .map(function (i) {
          var s = subs[i];
          return {
            name: s.name,
            description: s.description,
            options: sortedKeys(s.options)
              .map(function (j) { return normalizeRow(s.options[j]); })
              .filter(function (r) { return !emptyRow(r); }),
          };
        })
        .filter(function (s) { return !(s.name === "" && s.description === "" && s.options.length === 0); }),
      options: [],
    };
  }

  // The header's slash rows in the same comparable shape.
  function headerSlashRows(slash) {
    function row(o) {
      return { name: o.name, type: o.type, description: o.description, required: o.required, extra: false };
    }
    return {
      subcommands: slash.subcommands.map(function (s) {
        return { name: s.name, description: s.description, options: s.options.map(row) };
      }),
      options: slash.options.map(row),
    };
  }

  // The live form's managed fields, in the header's vocabulary. `labels` is
  // {type: {typeName: optionValue}, group: {groupName: optionValue}}.
  function liveStructure(entries, labels) {
    var deferValue = entryValue(entries, "interaction_defer_mode");
    return {
      type: labelFor(labels && labels.type, entryValue(entries, "type")),
      trigger: entryValue(entries, "trigger"),
      group: labelFor(labels && labels.group, entryValue(entries, "GroupID")),
      deferMode: deferValue === null ? null : DEFER_LABELS[Number(deferValue)] || null,
      name: entryValue(entries, "name"),
      slashDescription: entryValue(entries, "slash_command_description"),
      slash: liveSlashRows(entries),
    };
  }

  // {field: {live, header}} for every managed field where the header and the live form
  // disagree, or null. A header field that is absent (trigger, group, slash description) is
  // not managed; the defer mode always is (an absent line means None, as in the emulator);
  // the slash rows only for a Slash Command header.
  function structureDiff(header, live) {
    var diff = {};
    function differ(field, liveValue, headerValue) {
      diff[field] = { live: liveValue, header: headerValue };
    }
    if (typeof header.name === "string" && header.name !== live.name) {
      differ("name", live.name, header.name);
    }
    if (header.type !== null && header.type !== live.type) differ("type", live.type, header.type);
    if (header.trigger !== null && header.trigger !== live.trigger) {
      differ("trigger", live.trigger, header.trigger);
    }
    if (header.group !== null && header.group !== live.group) differ("group", live.group, header.group);
    var wantDefer = DEFER_LABELS[header.deferMode];
    if (wantDefer !== live.deferMode) differ("deferMode", live.deferMode, wantDefer);
    if (header.type === SLASH_COMMAND) {
      if (header.slashDescription !== null && header.slashDescription !== live.slashDescription) {
        differ("slashDescription", live.slashDescription, header.slashDescription);
      }
      var wantRows = headerSlashRows(header.slash || { subcommands: [], options: [] });
      if (JSON.stringify(wantRows) !== JSON.stringify(live.slash)) {
        differ("slash", live.slash, wantRows);
      }
    }
    return Object.keys(diff).length ? diff : null;
  }

  function optionEntries(prefix, o) {
    var out = [
      [prefix + ".name", o.name],
      [prefix + ".type", o.type],
      [prefix + ".description", o.description],
    ];
    if (o.required) out.push([prefix + ".required", "on"]);
    out.push(
      [prefix + ".choices", ""],
      [prefix + ".min_value", ""],
      [prefix + ".max_value", ""],
      [prefix + ".min_length", ""],
      [prefix + ".max_length", ""]
    );
    return out;
  }

  function valueOfLabel(map, label, what) {
    if (map && Object.prototype.hasOwnProperty.call(map, label)) return map[label];
    throw new Error(
      "header " + what + " " + JSON.stringify(label) + " isn't one of the panel's: " +
        (map ? Object.keys(map).join(", ") : "(no options)")
    );
  }

  // The entries to POST: `entries` with the header's managed fields replaced and everything
  // else (including fields no header knows) kept as is. `enable` true/false sets is_enabled,
  // anything else leaves it. Slash rows are replaced wholesale for a Slash Command header
  // and kept for every other type. Throws when the header names a type or group the panel's
  // selects don't have.
  function applyStructure(entries, header, labels, opts) {
    var enable = opts ? opts.enable : null;
    var isSlash = header.type === SLASH_COMMAND;
    var scalars = {};
    if (header.type !== null) scalars.type = valueOfLabel(labels && labels.type, header.type, "type");
    if (header.trigger !== null) scalars.trigger = header.trigger;
    if (header.group !== null) scalars.GroupID = valueOfLabel(labels && labels.group, header.group, "group");
    if (typeof header.name === "string") scalars.name = header.name;
    scalars.interaction_defer_mode = String(header.deferMode);
    if (isSlash && header.slashDescription !== null) {
      scalars.slash_command_description = header.slashDescription;
    }

    var out = [];
    var done = {};
    for (const e of entries) {
      var name = e[0];
      if (Object.prototype.hasOwnProperty.call(scalars, name)) {
        if (!done[name]) out.push([name, scalars[name]]);
        done[name] = true;
        continue;
      }
      if (isSlash && (name === "slash_use_subcommands" || /^slash_(options|subcommands)\./.test(name))) {
        continue;
      }
      if (name === "is_enabled" && (enable === true || enable === false)) continue;
      out.push([name, e[1]]);
    }
    for (const name of Object.keys(scalars)) if (!done[name]) out.push([name, scalars[name]]);

    if (isSlash && header.slash) {
      if (header.slash.subcommands.length > 0) out.push(["slash_use_subcommands", "on"]);
      header.slash.subcommands.forEach(function (s, i) {
        out.push(["slash_subcommands." + i + ".name", s.name]);
        out.push(["slash_subcommands." + i + ".description", s.description]);
        s.options.forEach(function (o, j) {
          Array.prototype.push.apply(out, optionEntries("slash_subcommands." + i + ".options." + j, o));
        });
      });
      header.slash.options.forEach(function (o, j) {
        Array.prototype.push.apply(out, optionEntries("slash_options." + j, o));
      });
    }
    if (enable === true) out.push(["is_enabled", "on"]);
    return out;
  }

  // ---------------------------------------------------------------------------------------
  // Panel access
  // ---------------------------------------------------------------------------------------

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
  // success is always decided by the read-back (classifyReadBack), never by this alone.
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

  function formEntries(form) {
    return Array.from(new FormData(form).entries());
  }

  // {optionText: optionValue} of a select, the text normalized to the header's vocabulary.
  function selectLabels(form, name, normalizeText) {
    const select = form.querySelector("select[name='" + name + "']");
    const map = {};
    if (!select) return map;
    for (const option of select.options) map[normalizeText(collapseWs(option.text))] = option.value;
    return map;
  }

  function formLabels(form) {
    return {
      type: selectLabels(form, "type", panelTypeName),
      group: selectLabels(form, "GroupID", panelGroupName),
    };
  }

  async function postForm(url, entries) {
    const res = await fetch(url, {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams(entries),
    });
    const html = await res.text();
    const doc = new DOMParser().parseFromString(html, "text/html");
    return { res, doc };
  }

  // Pure decision logic, factored out of run() so it's unit-testable without a DOM/fetch
  // stub. Given the raw fetch's sha (already checked against manifestSha by the caller for
  // response.ok; rawSha itself may just be a mismatched digest), the live sha, and whether
  // the structure or the enabled state would change, decides what run() does next.
  //   "failed: raw mismatch" -- the raw fetch didn't reproduce the manifest's bytes; stop
  //     here even in a dry run, since a dry run is supposed to prove a real update would work.
  //   "same"                 -- code, structure and enabled state already match.
  //   "would-update"         -- dry run only; something differs, but nothing is written.
  //   "update"               -- proceed to POST.
  function decide({ liveSha, manifestSha, rawSha, dryRun, structureDiffers, enableChange }) {
    if (rawSha !== manifestSha) return "failed: raw mismatch";
    if (liveSha === manifestSha && !structureDiffers && !enableChange) return "same";
    if (dryRun) return "would-update";
    return "update";
  }

  // Also pure/unit-testable: success is decided ONLY by what the read-back shows (the panel's
  // alert banners are site-wide notices, not failure signals): the code hash matches, the
  // header's structure matches, and is_enabled is what was asked. An alert (if any) is
  // surfaced as `note` for a human to see either way.
  function classifyReadBack({ readBackSha, manifestSha, alertText, structure, enabled, wantEnabled }) {
    const note = alertText || null;
    if (readBackSha !== manifestSha) return { status: "failed: read-back mismatch", note };
    if (structure) {
      return { status: "failed: read-back structure " + Object.keys(structure).join(","), note };
    }
    if (wantEnabled === true || wantEnabled === false) {
      if (enabled !== wantEnabled) {
        return { status: "failed: read-back enabled is " + enabled + ", wanted " + wantEnabled, note };
      }
    }
    return { status: "updated", note };
  }

  // The refusal list for run()'s options: one {id, reason} per id that is in both lists or
  // not in the manifest. Non-empty means nothing may be fetched or posted.
  function checkOptions(manifest, opts) {
    const enable = (opts && opts.enable) || [];
    const disable = (opts && opts.disable) || [];
    const known = {};
    for (const cmd of manifest.commands) known[String(cmd.id)] = true;
    const problems = [];
    const inDisable = {};
    for (const id of disable) inDisable[String(id)] = true;
    for (const id of enable) {
      if (inDisable[String(id)]) problems.push({ id, reason: "in both enable and disable" });
    }
    for (const id of enable.concat(disable)) {
      if (!known[String(id)]) problems.push({ id, reason: "not in the manifest" });
    }
    return problems;
  }

  async function run(manifest, opts) {
    const dryRun = !opts || opts.dryRun !== false;
    const enableIds = ((opts && opts.enable) || []).map(String);
    const disableIds = ((opts && opts.disable) || []).map(String);
    const guild = manifest.guild;

    // Refuse before anything is fetched, so a typo can't half-apply.
    const problems = checkOptions(manifest, opts);
    if (problems.length) {
      return problems.map((p) => ({
        path: null,
        id: p.id,
        status: "failed: options: " + p.reason,
      }));
    }

    const results = [];

    for (const cmd of manifest.commands) {
      const { path, id, sha: manifestSha, raw } = cmd;
      let result = {
        path, id, status: null, enabled: null, enabledWanted: null, structure: null,
      };

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
        // the structure below reads the REPO header, never the live code's.
        let rawCode = null;
        let rawSha = null;
        const rawRes = await fetch(raw);
        if (rawRes.ok) {
          rawCode = await rawRes.text();
          rawSha = await sha256hex(normalize(rawCode));
        }

        const entries = formEntries(form);
        const labels = formLabels(form);
        const liveEnabled = hasEntry(entries, "is_enabled");
        result.enabled = liveEnabled;

        let header = null;
        let diff = null;
        if (rawCode !== null) {
          header = parseHeader(rawCode);
          header.name = commandName(path);
          if (header.error === null) {
            diff = structureDiff(header, liveStructure(entries, labels));
          }
        }
        result.structure = diff;

        const wantEnabled = enableIds.indexOf(String(id)) >= 0 ? true
          : disableIds.indexOf(String(id)) >= 0 ? false : null;
        const enableChange = wantEnabled !== null && wantEnabled !== liveEnabled;
        result.enabledWanted = wantEnabled;

        let decision = decide({
          liveSha,
          manifestSha,
          rawSha,
          dryRun,
          structureDiffers: diff !== null,
          enableChange,
        });
        if (decision !== "failed: raw mismatch" && header && header.error !== null) {
          decision = "failed: " + header.error;
        }

        // Build the post body in BOTH modes, so a dry run proves the real run can: a type or
        // group the panel's selects lack throws here, and nothing is posted.
        let newEntries = null;
        if ((decision === "update" || decision === "would-update") && header) {
          try {
            newEntries = applyStructure(entries, header, labels, { enable: wantEnabled })
              .map((e) => (e[0] === "responses" ? ["responses", rawCode] : e));
          } catch (err) {
            decision = "failed: " + (err && err.message ? err.message : String(err));
          }
        }

        if (decision !== "update") {
          result.status = decision;
          results.push(result);
          await sleep(800);
          continue;
        }

        const { doc: postDoc } = await postForm(form.getAttribute("action"), newEntries);
        const alertText = findErrorAlert(postDoc);

        // ALWAYS re-fetch and check, regardless of whether an alert matched: the panel shows
        // site-wide notice banners too, so an alert on the page is not evidence of failure
        // by itself. Success is decided by the read-back (classifyReadBack).
        const readBack = await fetchEditPage(guild, id);
        const readBackForm = readBack.doc ? findUpdateForm(readBack.doc, id) : null;
        const readBackTextarea = readBackForm
          ? readBackForm.querySelector("textarea[name=responses]")
          : null;
        const readBackSha = readBackTextarea
          ? await sha256hex(normalize(readBackTextarea.value))
          : null;
        const readEntries = readBackForm ? formEntries(readBackForm) : [];
        const readLabels = readBackForm ? formLabels(readBackForm) : labels;

        const classified = classifyReadBack({
          readBackSha,
          manifestSha,
          alertText,
          structure: readBackForm
            ? structureDiff(header, liveStructure(readEntries, readLabels))
            : { form: null },
          enabled: hasEntry(readEntries, "is_enabled"),
          wantEnabled,
        });
        result.status = classified.status;
        result.note = classified.note;
        result.enabled = hasEntry(readEntries, "is_enabled");
        results.push(result);
      } catch (err) {
        result.status = "failed: " + (err && err.message ? err.message : String(err));
        results.push(result);
      }

      await sleep(800);
    }

    return results;
  }

  // ---------------------------------------------------------------------------------------
  // Creating commands
  // ---------------------------------------------------------------------------------------

  // {groupName: groupId} from the list page's group tabs: [[href, text], ...] of every
  // `a.nav-link` (customcommands.html:50-55 / editcmd.html:128-133, href
  // /manage/<guild>/customcommands/groups/<id>).
  function groupIdsFromLinks(links) {
    const map = {};
    for (const [href, text] of links) {
      const m = /\/customcommands\/groups\/(\d+)\/?$/.exec(href || "");
      if (m) map[panelGroupName(collapseWs(text))] = m[1];
    }
    return map;
  }

  // The new command's id from the redirect target handleNewCommand answers with
  // (web.go:538: /manage/<guild>/customcommands/commands/<id>/), or null.
  function newCommandId(url) {
    const m = /\/customcommands\/commands\/(\d+)\/?(?:[?#].*)?$/.exec(url || "");
    return m ? m[1] : null;
  }

  // {name: id} of the commands a list page shows: [[href, text], ...] of every
  // `h2.card-title a` (customcommands.html:169-186: href `#collapse_cmd<id>`, text
  // "#<id> - <type>: <trigger> ... Name: <name>", the name only when the command has one).
  function namedCommandsFromCards(cards) {
    const map = {};
    for (const [href, text] of cards) {
      const idm = /#collapse_cmd(\d+)$/.exec(href || "");
      const namem = /Name:\s*(.+)$/.exec(collapseWs(text));
      if (idm && namem) map[namem[1]] = idm[1];
    }
    return map;
  }

  // Status texts of create(): every failure after the command may exist says so.
  function createdButStatus(id, what) {
    return (
      "failed: created #" + id + " but " + what +
      " — still ENABLED with the placeholder: disable it in the panel"
    );
  }

  function noIdStatus(url) {
    return (
      "failed: no new command id in " + url + " — a command MAY have been created " +
      "(enabled, with the placeholder): check the panel"
    );
  }

  // create(guild, [{path, group}], {dryRun}) -> [{path, id, status}]. Each command is made
  // in its group (POST commands/new, web.go:428-540: ENABLED, placeholder response, default
  // trigger type Command) and IMMEDIATELY disabled by posting its own update form with
  // is_enabled cleared, then read back. The caller records the ids in deploy/panel.json,
  // commits, pushes, and runs run() (with `enable`) on them.
  async function create(guild, items, opts) {
    const dryRun = !opts || opts.dryRun !== false;
    const results = [];
    const listUrl = "/manage/" + guild + "/customcommands/";
    const listRes = await fetch(listUrl, { credentials: "same-origin" });
    const listDoc = new DOMParser().parseFromString(await listRes.text(), "text/html");
    const groups = groupIdsFromLinks(
      Array.from(listDoc.querySelectorAll("a.nav-link")).map((a) => [a.getAttribute("href"), a.textContent])
    );
    const newForm = Array.from(listDoc.querySelectorAll("form")).find((f) =>
      (f.getAttribute("action") || "").endsWith("customcommands/commands/new")
    );
    // Idempotency guard: the commands that already carry a name (the ungrouped page and each
    // group's page list their own commands), so a repeated create can't make a duplicate.
    const existing = namedCommandsFromCards(
      Array.from(listDoc.querySelectorAll("h2.card-title a")).map((a) => [a.getAttribute("href"), a.textContent])
    );
    for (const gid of Object.keys(groups).map((g) => groups[g])) {
      const res = await fetch(listUrl + "groups/" + gid, { credentials: "same-origin" });
      const doc = new DOMParser().parseFromString(await res.text(), "text/html");
      Object.assign(
        existing,
        namedCommandsFromCards(
          Array.from(doc.querySelectorAll("h2.card-title a")).map((a) => [a.getAttribute("href"), a.textContent])
        )
      );
    }
    for (const item of items) {
      const result = { path: item.path, id: null, status: null };
      results.push(result);
      try {
        if (!newForm) {
          result.status = "failed: no new-command form on the list page";
          continue;
        }
        if (!Object.prototype.hasOwnProperty.call(groups, item.group)) {
          result.status = "failed: unknown group " + JSON.stringify(item.group);
          continue;
        }
        if (Object.prototype.hasOwnProperty.call(existing, commandName(item.path))) {
          result.status = "failed: exists as #" + existing[commandName(item.path)];
          continue;
        }
        if (dryRun) {
          result.status = "would-create";
          continue;
        }
        const newEntries = formEntries(newForm).filter((e) => e[0] !== "GroupID");
        newEntries.push(["GroupID", groups[item.group]]);
        const { res } = await postForm(newForm.getAttribute("action"), newEntries);
        const id = newCommandId(res.url);
        if (!id) {
          result.status = noIdStatus(res.url);
          continue;
        }
        result.id = id;
        existing[commandName(item.path)] = id;

        // The new command is live (enabled, placeholder response, empty Command trigger)
        // until this second POST lands.
        const page = await fetchEditPage(guild, id);
        const form = page.doc ? findUpdateForm(page.doc, id) : null;
        if (!form) {
          result.status = createdButStatus(id, "its edit form is missing");
          continue;
        }
        await postForm(
          form.getAttribute("action"),
          formEntries(form)
            .filter((e) => e[0] !== "is_enabled" && e[0] !== "name")
            .concat([["name", commandName(item.path)]])
        );
        const back = await fetchEditPage(guild, id);
        const backForm = back.doc ? findUpdateForm(back.doc, id) : null;
        const backEntries = backForm ? formEntries(backForm) : [];
        result.status =
          backForm && !hasEntry(backEntries, "is_enabled")
            ? "created-disabled"
            : createdButStatus(id, "not verified disabled");
      } catch (err) {
        const message = err && err.message ? err.message : String(err);
        result.status = result.id
          ? createdButStatus(result.id, "an error stopped the second step (" + message + ")")
          : "failed: " + message + " — a command MAY have been created if the first POST " +
            "went out: check the panel";
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
    commandName,
    liveStructure,
    structureDiff,
    applyStructure,
    decide,
    classifyReadBack,
    checkOptions,
    groupIdsFromLinks,
    newCommandId,
    namedCommandsFromCards,
    createdButStatus,
    noIdStatus,
    run,
    create,
  };
})();
