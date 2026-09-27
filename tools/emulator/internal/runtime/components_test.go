package runtime

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// sentRows is the action rows of the last sent message.
func sentRows(t *testing.T, ctx *ExecutionContext) []*types.ActionsRow {
	t.Helper()
	if len(ctx.SentMessages) == 0 {
		t.Fatal("nothing sent")
	}
	var rows []*types.ActionsRow
	for _, c := range ctx.SentMessages[len(ctx.SentMessages)-1].Components {
		row, ok := c.(*types.ActionsRow)
		if !ok {
			t.Fatalf("component %T is not an action row", c)
		}
		rows = append(rows, row)
	}
	return rows
}

// rowShape prints the rows as "button:label" / "menu:custom_id" lists.
func rowShape(rows []*types.ActionsRow) string {
	var out [][]string
	for _, row := range rows {
		var comps []string
		for _, c := range row.Components {
			switch t := c.(type) {
			case *types.Button:
				comps = append(comps, "button:"+t.Label)
			case *types.SelectMenu:
				comps = append(comps, "menu:"+t.CustomID)
			}
		}
		out = append(out, comps)
	}
	return fmt.Sprint(out)
}

func mustFail(t *testing.T, src, want string) {
	t.Helper()
	_, err := run(t, newCtx(true, true), src)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("%s\n  want error %q\n  got %v", src, want, err)
	}
}

// menuOptions is a cslice of n string-menu options with distinct values.
func menuOptions(n int) string {
	var b strings.Builder
	b.WriteString("(cslice")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, ` (sdict "label" "l" "value" "v%d")`, i)
	}
	b.WriteString(")")
	return b.String()
}

// Every error text of CreateButton (components.go:179-254)
func TestCreateButtonErrors(t *testing.T) {
	for name, c := range map[string]struct{ src, want string }{
		"style number out of range": {`{{cbutton "label" "x" "style" 6}}`, "invalid button style"},
		"style number 0":            {`{{cbutton "label" "x" "style" 0}}`, "invalid button style"},
		"style name unknown":        {`{{cbutton "label" "x" "style" "pink"}}`, "invalid button style"},
		"link button without url": {`{{cbutton "label" "x" "style" "link"}}`,
			"a url field is required for a link button"},
		"no label or emoji": {`{{cbutton "style" "primary"}}`, "button must have a label or emoji"},
		// an emoji id is a string, as discordgo's `json:"id,string"` reads it
		"emoji id not a string": {`{{cbutton "emoji" (sdict "name" "e" "id" 123)}}`,
			"json: cannot unmarshal number into Go struct field Button.emoji.id of type int64"},
		"wrong value type":  {`{{cbutton "label" 5}}`, "cannot unmarshal number into Go struct field"},
		"one non-map value": {`{{cbutton "label"}}`, "cannot convert data of type: string"},
		"odd arguments":     {`{{cbutton "label" "x" "style"}}`, "invalid dict call"},
		"non-string key":    {`{{cbutton 1 2}}`, "Only string keys supported in sdict"},
	} {
		t.Run(name, func(t *testing.T) { mustFail(t, c.src, c.want) })
	}
}

// Every error text of CreateSelectMenu (components.go:256-329)
func TestCreateSelectMenuErrors(t *testing.T) {
	one := menuOptions(1)
	for name, c := range map[string]struct{ src, want string }{
		"type not a string": {`{{cmenu "type" 3 "options" ` + one + `}}`, "invalid select menu type"},
		"type unknown":      {`{{cmenu "type" "emoji" "options" ` + one + `}}`, "invalid select menu type"},
		"26 options": {`{{cmenu "options" ` + menuOptions(26) + `}}`,
			"invalid number of menu options in select menu, must have between 1 and 25"},
		"min values 26": {`{{cmenu "options" ` + one + ` "min_values" 26}}`,
			"invalid min values in select menu, must be between 0 and 25"},
		"min values negative": {`{{cmenu "options" ` + one + ` "min_values" -1}}`,
			"invalid min values in select menu, must be between 0 and 25"},
		"max values 26": {`{{cmenu "options" ` + one + ` "max_values" 26}}`,
			"invalid max values in select menu, max 25"},
		"duplicate option values": {
			`{{cmenu "options" (cslice (sdict "label" "a" "value" "v") (sdict "label" "b" "value" "v"))}}`,
			"select menu options must have unique values"},
		"wrong value type": {`{{cmenu "placeholder" 5}}`, "cannot unmarshal number into Go struct field"},
	} {
		t.Run(name, func(t *testing.T) { mustFail(t, c.src, c.want) })
	}
	// 25 options are fine
	if _, err := run(t, newCtx(true, true), `{{cmenu "options" `+menuOptions(25)+`}}`); err != nil {
		t.Errorf("25 options: %v", err)
	}
}

// Button styles by name, colour and number, and the "link" alias of "url"
func TestCreateButtonStyles(t *testing.T) {
	for src, want := range map[string]types.ButtonStyle{
		`{{(cbutton "label" "x").Style}}`:                                  0, // MarshalJSON makes it primary
		`{{(cbutton "label" "x" "style" "blurple").Style}}`:                types.PrimaryButton,
		`{{(cbutton "label" "x" "style" "grey").Style}}`:                   types.SecondaryButton,
		`{{(cbutton "label" "x" "style" "green").Style}}`:                  types.SuccessButton,
		`{{(cbutton "label" "x" "style" "RED").Style}}`:                    types.DangerButton,
		`{{(cbutton "label" "x" "style" 4).Style}}`:                        types.DangerButton,
		`{{(cbutton "label" "x" "style" "5" "url" "https://x").Style}}`:    types.LinkButton,
		`{{(cbutton "label" "x" "style" "url" "link" "https://x").Style}}`: types.LinkButton,
	} {
		out, err := run(t, newCtx(true, true), src)
		if err != nil || out != strconv.Itoa(int(want)) {
			t.Errorf("%s: out=%q err=%v (want %d)", src, out, err, want)
		}
	}
	out, err := run(t, newCtx(true, true), `{{(cbutton "label" "x" "style" "link" "link" "https://x").URL}}`)
	if err != nil || out != "https://x" {
		t.Errorf(`"link" should set the url: %q %v`, out, err)
	}
	// a built button passes through unchanged (components.go:187-188)
	out, err = run(t, newCtx(true, true), `{{$b := cbutton "label" "x" "custom_id" "c"}}{{(cbutton $b).CustomID}}`)
	if err != nil || out != "c" {
		t.Errorf("cbutton of a button: %q %v", out, err)
	}
}

// A menu's type and Discord's JSON shape
func TestCreateSelectMenuTypes(t *testing.T) {
	for src, want := range map[string]string{
		`{{json (cmenu "options" ` + menuOptions(1) + `)}}`:                    `"type":3`,
		`{{json (cmenu "type" "user")}}`:                                       `"type":5`,
		`{{json (cmenu "type" "Role")}}`:                                       `"type":6`,
		`{{json (cmenu "type" "mentionable")}}`:                                `"type":7`,
		`{{json (cmenu "type" "channel" "channel_types" (cslice 0 2))}}`:       `"channel_types":[0,2]`,
		`{{$m := cmenu "type" "user" "custom_id" "c"}}{{(cmenu $m).CustomID}}`: `c`,
	} {
		out, err := run(t, newCtx(true, true), src)
		if err != nil || !strings.Contains(out, want) {
			t.Errorf("%s: out=%q err=%v (want %s)", src, out, err, want)
		}
	}
}

// A message's buttons pack 5 to a row, up to 5 rows (components.go:1028-1100)
func TestButtonsPackFiveByFive(t *testing.T) {
	buttons := func(n int) string {
		return `{{$b := cslice}}{{range seq 0 ` + strconv.Itoa(n) + `}}` +
			`{{$b = $b.Append (sdict "label" (print "b" .))}}{{end}}` +
			`{{sendMessage nil (complexMessage "buttons" $b)}}`
	}
	ctx := newCtx(true, true)
	if _, err := run(t, ctx, buttons(25)); err != nil {
		t.Fatal(err)
	}
	rows := sentRows(t, ctx)
	if len(rows) != 5 {
		t.Fatalf("want 5 rows, got %d: %s", len(rows), rowShape(rows))
	}
	for i, row := range rows {
		if len(row.Components) != 5 {
			t.Errorf("row %d has %d components", i, len(row.Components))
		}
	}
	if got := rows[4].Components[4].(*types.Button).Label; got != "b24" {
		t.Errorf("last packed button = %s", got)
	}
	// 7 buttons: a full row and a row of 2
	ctx = newCtx(true, true)
	if _, err := run(t, ctx, buttons(7)); err != nil {
		t.Fatal(err)
	}
	if got := rowShape(sentRows(t, ctx)); got != "[[button:b0 button:b1 button:b2 button:b3 button:b4] [button:b5 button:b6]]" {
		t.Errorf("rows = %s", got)
	}
	// As in YAGPDB (components.go:1068, 1090): past 25 the loop stops before the last
	// index, so the 5th row is never appended and 26 buttons give 4 rows
	ctx = newCtx(true, true)
	if _, err := run(t, ctx, buttons(26)); err != nil {
		t.Fatal(err)
	}
	if rows := sentRows(t, ctx); len(rows) != 4 {
		t.Errorf("26 buttons: want YAGPDB's 4 rows, got %d", len(rows))
	}
}

// Only the first 40 buttons are built (general.go:379-408): an invalid 41st isn't an
// error, an invalid 40th is
func TestButtonsCapAt40(t *testing.T) {
	src := func(badAt int) string {
		return `{{$b := cslice}}{{range seq 0 41}}{{if eq . ` + strconv.Itoa(badAt) + `}}` +
			`{{$b = $b.Append (sdict "custom_id" "nolabel")}}{{else}}` +
			`{{$b = $b.Append (sdict "label" (print "b" .))}}{{end}}{{end}}` +
			`{{sendMessage nil (complexMessage "buttons" $b)}}ok`
	}
	if out, err := run(t, newCtx(true, true), src(40)); err != nil || out != "ok" {
		t.Errorf("41st button should be ignored: %q %v", out, err)
	}
	mustFail(t, src(39), "button must have a label or emoji")
}

// Only the first 5 menus are built (general.go:409-435); each takes a row of its own
func TestMenusCapAt5(t *testing.T) {
	src := func(badAt int) string {
		return `{{$m := cslice}}{{range seq 0 6}}{{if eq . ` + strconv.Itoa(badAt) + `}}` +
			`{{$m = $m.Append (sdict "type" "emoji")}}{{else}}` +
			`{{$m = $m.Append (sdict "type" "user" "custom_id" (print "m" .))}}{{end}}{{end}}` +
			`{{sendMessage nil (complexMessage "menus" $m)}}ok`
	}
	ctx := newCtx(true, true)
	if out, err := run(t, ctx, src(5)); err != nil || out != "ok" {
		t.Fatalf("6th menu should be ignored: %q %v", out, err)
	}
	want := "[[menu:templates-m0] [menu:templates-m1] [menu:templates-m2] [menu:templates-m3] [menu:templates-m4]]"
	if got := rowShape(sentRows(t, ctx)); got != want {
		t.Errorf("rows = %s", got)
	}
	mustFail(t, src(4), "invalid select menu type")
}

// A menu in a flat "components" slice takes a row of its own; buttons continue after it
func TestMenuTakesItsOwnRow(t *testing.T) {
	ctx := newCtx(true, true)
	src := `{{$c := cslice (cbutton "label" "a") (cmenu "type" "user") (cbutton "label" "b")}}` +
		`{{sendMessage nil (complexMessage "components" $c)}}`
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	if got := rowShape(sentRows(t, ctx)); got != "[[button:a] [menu:templates-1] [button:b]]" {
		t.Errorf("rows = %s", got)
	}
}

// Custom rows: a menu can't share one (components.go:1050-1053), and a row's 6th
// component and a 6th row are dropped
func TestCustomRows(t *testing.T) {
	menu := `(cmenu "options" ` + menuOptions(1) + `)`
	mustFail(t, `{{sendMessage nil (complexMessage "components" (cslice (cslice (cbutton "label" "a") `+menu+`)))}}`,
		"a select menu cannot share an action row with other components")
	// As in YAGPDB (components.go:1050, 1055: Type() is compared with the string menu's
	// type), a user/role/mentionable/channel menu may share a custom row
	ctx := newCtx(true, true)
	src := `{{sendMessage nil (complexMessage "components" (cslice (cslice (cbutton "label" "a") (cmenu "type" "user"))))}}`
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	if got := rowShape(sentRows(t, ctx)); got != "[[button:a menu:templates-1]]" {
		t.Errorf("rows = %s", got)
	}
	mustFail(t, `{{sendMessage nil (complexMessage "components" (cslice (cslice "x")))}}`,
		"invalid component passed to send message builder")
	mustFail(t, `{{sendMessage nil (complexMessage "components" (cslice "x"))}}`,
		"invalid component passed to send message builder")
	mustFail(t, `{{sendMessage nil (complexMessage "components" "x")}}`,
		"invalid component passed to send message builder")

	ctx = newCtx(true, true)
	src = `{{$row := cslice}}{{range seq 0 6}}{{$row = $row.Append (cbutton "label" (print "b" .))}}{{end}}` +
		`{{$rows := cslice}}{{range seq 0 6}}{{$rows = $rows.Append $row}}{{end}}` +
		`{{sendMessage nil (complexMessage "components" $rows)}}`
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	rows := sentRows(t, ctx)
	if len(rows) != 5 || len(rows[4].Components) != 5 {
		t.Errorf("rows = %s", rowShape(rows))
	}
	// a string menu given first ends its row (components.go:1055-1057)
	ctx = newCtx(true, true)
	src = `{{sendMessage nil (complexMessage "components" (cslice (cslice ` + menu + ` (cbutton "label" "x"))))}}`
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	if got := rowShape(sentRows(t, ctx)); got != "[[menu:templates-0]]" {
		t.Errorf("rows = %s", got)
	}
}

// Custom IDs (components.go:1102-1117, 1119-1166): numbered when empty, prefixed once,
// capped at 100 with the prefix, and a link button loses its ID
func TestCustomIDs(t *testing.T) {
	ctx := newCtx(true, true)
	src := `{{sendMessage nil (complexMessage "buttons" (cslice ` +
		`(sdict "label" "a") ` +
		`(sdict "label" "b" "custom_id" "mine") ` +
		`(sdict "label" "c" "custom_id" "templates-already") ` +
		`(sdict "label" "d" "style" "link" "url" "https://x" "custom_id" "dropped") ` +
		`(sdict "label" "e")))}}`
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range sentRows(t, ctx)[0].Components {
		ids = append(ids, c.(*types.Button).CustomID)
	}
	want := `[templates-0 templates-mine templates-already  templates-3]`
	if got := fmt.Sprint(ids); got != want {
		t.Errorf("custom ids = %s, want %s", got, want)
	}

	ninety := strings.Repeat("x", 90)
	if _, err := run(t, newCtx(true, true), `{{sendMessage nil (complexMessage "buttons" (sdict "label" "a" "custom_id" "`+ninety+`"))}}`); err != nil {
		t.Errorf("a 90-char id fits: %v", err)
	}
	mustFail(t, `{{sendMessage nil (complexMessage "buttons" (sdict "label" "a" "custom_id" "`+ninety+`x"))}}`,
		"custom id too long (max 90 chars)")
	// the prefix counts: 91 chars after it is over 100
	mustFail(t, `{{sendMessage nil (complexMessage "menus" (sdict "type" "user" "custom_id" "templates-`+ninety+`x"))}}`,
		"custom id too long (max 90 chars)")
}

// A single "buttons"/"menus"/"components" value is one row; a link button given alone
// loses its ID (general.go:400-405)
func TestSingleComponentValues(t *testing.T) {
	ctx := newCtx(true, true)
	src := `{{sendMessage nil (complexMessage ` +
		`"buttons" (sdict "label" "a" "style" "link" "url" "https://x" "custom_id" "gone") ` +
		`"menus" (sdict "type" "user" "custom_id" "m") ` +
		`"components" (cbutton "label" "b"))}}`
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	rows := sentRows(t, ctx)
	if got := rowShape(rows); got != "[[button:a] [menu:templates-m] [button:b]]" {
		t.Fatalf("rows = %s", got)
	}
	if id := rows[0].Components[0].(*types.Button).CustomID; id != "" {
		t.Errorf("link button kept custom id %q", id)
	}
	// numbered by the IDs in use so far; the link button's cleared one doesn't count
	if id := rows[2].Components[0].(*types.Button).CustomID; id != "templates-1" {
		t.Errorf("third component's id = %q", id)
	}
}

// A message with only components isn't empty; getMessage reads them back in Discord's
// shape; editMessage replaces them with a new "components" and keeps them without one
func TestComponentsAreStoredAndEdited(t *testing.T) {
	ctx := newCtx(true, true)
	src := `{{$id := sendMessageRetID nil (complexMessage "buttons" (sdict "label" "Next" "custom_id" "pg:2"))}}` +
		`{{$m := getMessage nil $id}}{{$row := index $m.Components 0}}{{$b := index $row.Components 0}}` +
		`{{$b.CustomID}} {{$b.Label}} {{$b.Style}}|` +
		`{{editMessage nil $id (complexMessageEdit "content" "edited")}}` +
		`{{len (getMessage nil $id).Components}}|` +
		`{{editMessage nil $id (complexMessageEdit "content" "again" "components" cslice)}}` +
		`{{len (getMessage nil $id).Components}}|` +
		`{{editMessage nil $id (complexMessageEdit "menus" (sdict "type" "role" "custom_id" "r"))}}` +
		`{{(index (index (getMessage nil $id).Components 0).Components 0).CustomID}}`
	out, err := run(t, ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	if want := "templates-pg:2 Next 0|1|0|templates-r"; strings.TrimSpace(out) != want {
		t.Errorf("out = %q, want %q", out, want)
	}
	if len(ctx.EditedMessages) != 3 || len(ctx.EditedMessages[0].Components) != 1 ||
		len(ctx.EditedMessages[1].Components) != 0 || len(ctx.EditedMessages[2].Components) != 1 {
		t.Errorf("edited components: %+v", ctx.EditedMessages)
	}
	if w := kinds(ctx, KindLimit); len(w) != 0 {
		t.Errorf("no warnings expected, got %q", w)
	}
}

// A fetched message's components are a copy: changing them doesn't change the stored ones
func TestFetchedComponentsAreACopy(t *testing.T) {
	ctx := newCtx(true, true)
	if _, err := run(t, ctx, `{{sendMessage nil (complexMessage "buttons" (sdict "label" "a"))}}`); err != nil {
		t.Fatal(err)
	}
	fetched := NewEngine(ctx).getMessage(nil, ctx.SentMessages[0].ID)
	fetched.Components[0].(*types.ActionsRow).Components[0].(*types.Button).Label = "changed"
	stored := ctx.SentMessages[0].Components[0].(*types.ActionsRow).Components[0].(*types.Button)
	if stored.Label != "a" {
		t.Errorf("stored label changed to %q", stored.Label)
	}
}

// Discord's JSON shape of a sent row, as snapshots show it
func TestComponentsJSONShape(t *testing.T) {
	ctx := newCtx(true, true)
	src := `{{sendMessage nil (complexMessage "buttons" (sdict "label" "Go" "style" "green" "emoji" (sdict "name" "🚀")))}}`
	if _, err := run(t, ctx, src); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(ctx.SentMessages[0].Components[0])
	if err != nil {
		t.Fatal(err)
	}
	want := `{"components":[{"label":"Go","style":3,"disabled":false,"emoji":{"name":"🚀"},"custom_id":"templates-0","type":2}],"type":1}`
	if string(data) != want {
		t.Errorf("json = %s\n  want %s", data, want)
	}
}
