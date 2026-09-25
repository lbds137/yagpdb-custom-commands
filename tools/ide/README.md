# Editor support

## GoLand

GoLand already treats `.gohtml` as a **Go Template** file: it highlights the template
syntax and flags unknown or duplicate variables. It doesn't know YAGPDB's functions.

[`goland/YAGPDB.xml`](goland/YAGPDB.xml) adds live templates for this repo's patterns.
Install them with:

```bash
./scripts/install-goland-templates.sh   # copies into every GoLand config, then restart GoLand
```

Type the abbreviation in a `.gohtml` file and press Tab:

| Abbreviation | Inserts |
|--------------|---------|
| `yhead` | the command header (Author, Trigger type, Trigger, Dependencies) |
| `ycfg` | loading Global/Commands/Channels: delete delays, `$embed_exec`, `$yagpdbChannelID` |
| `yargs` | `parseArgs` with one `carg` (pick the type from a list) |
| `ydbget` | a `dbGet` read with a default, safe when the key is missing |
| `yembed` | an embed sent through the `embed_exec` service (after `ycfg`) |
| `ycembed` | `sendMessage` with a `cembed` |
| `ytry` | try/catch around the selection, reporting the error in an embed |
| `ystaff` | the staff-role check from the Roles config |
| `yexec` | an `execCC` call with an `sdict` |
| `ydel` | `deleteTrigger` with the configured delay (after `ycfg`) |

They're under Settings → Editor → Live Templates → YAGPDB, where you can edit them. To
keep an edit, copy it back into `goland/YAGPDB.xml`: the install script overwrites the
installed copy.

Not covered: highlighting YAGPDB's function names and showing their docs on hover. Those
need a JetBrains plugin (a separate Kotlin project). Until then, `yagtest` error hints
link to YAGPDB's function docs.
