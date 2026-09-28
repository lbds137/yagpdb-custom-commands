package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	yagstd "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// YAGPDB's message component builders, copied from vendor common/templates/components.go
// (c579722) with their error texts: cbutton (CreateButton) and cmenu (CreateSelectMenu),
// the packing of components into action rows, and custom-ID validation. Components V2
// isn't modelled; the modal builders are in modals.go.

// TemplateCustomIDPrefix is what YAGPDB puts before every custom ID a template sets
// (common/templates/context_interactions.go:15), so it knows the component is its own.
const TemplateCustomIDPrefix = "templates-"

// indirect is YAGPDB's (general.go:593-603): the value behind pointers and interfaces.
func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() {
			return v, true
		}
		if v.Kind() == reflect.Interface && v.NumMethod() > 0 {
			break
		}
	}
	return v, false
}

// createComponent is CreateComponent (components.go:18-135) for the types the emulator
// builds: the values (one map, or sdict key-value pairs) are marshalled to JSON and
// decoded into the component, so unknown keys are dropped and wrong types are errors.
func createComponent(expectedType types.ComponentType, values ...any) (types.MessageComponent, error) {
	if len(values) < 1 && expectedType != types.ActionsRowComponent {
		return types.ActionsRow{}, errors.New("no values passed to component builder")
	}

	var m map[string]any
	switch t := values[0].(type) {
	case types.SDict:
		m = t
	case *types.SDict:
		m = *t
	case map[string]any:
		m = t
	default:
		dict, err := yagstd.StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		m = dict
	}

	encoded, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	var component types.MessageComponent
	switch expectedType {
	case types.ActionsRowComponent:
		component = types.ActionsRow{}
	case types.ButtonComponent:
		var comp types.Button
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case types.SelectMenuComponent:
		var comp types.SelectMenu
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case types.TextInputComponent:
		comp := types.NewShortTextInput()
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case types.UserSelectMenuComponent:
		comp := types.SelectMenu{MenuType: types.UserSelectMenu}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case types.RoleSelectMenuComponent:
		comp := types.SelectMenu{MenuType: types.RoleSelectMenu}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case types.MentionableSelectMenuComponent:
		comp := types.SelectMenu{MenuType: types.MentionableSelectMenu}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case types.ChannelSelectMenuComponent:
		comp := types.SelectMenu{MenuType: types.ChannelSelectMenu}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	case types.LabelComponent:
		comp := types.Label{}
		err = json.Unmarshal(encoded, &comp)
		component = comp
	default:
		return nil, fmt.Errorf("component type %d is not modelled", expectedType)
	}

	if err != nil {
		return nil, err
	}
	return component, nil
}

// CreateButton is YAGPDB's cbutton (components.go:179-254): a button from one map or
// sdict key-value pairs. "style" takes a name, a colour or a number 1-5; "link" is an
// alias of "url". A link button needs a url, and every button a label or an emoji.
func CreateButton(values ...any) (*types.Button, error) {
	var messageSdict map[string]any
	switch t := values[0].(type) {
	case types.SDict:
		messageSdict = t
	case *types.SDict:
		messageSdict = *t
	case map[string]any:
		messageSdict = t
	case *types.Button:
		return t, nil
	default:
		dict, err := yagstd.StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		messageSdict = dict
	}

	convertedButton := make(map[string]any)
	for k, v := range messageSdict {
		switch strings.ToLower(k) {
		case "style":
			var val string
			switch typed := v.(type) {
			case string:
				val = typed
			case types.ButtonStyle:
				val = strconv.Itoa(int(typed))
			case *types.ButtonStyle:
				val = strconv.Itoa(int(*typed))
			default:
				num := int(yagstd.ToInt64(typed)) // tmplToInt
				if num < 1 || num > 5 {
					return nil, errors.New("invalid button style")
				}
				val = strconv.Itoa(num)
			}

			switch strings.ToLower(val) {
			case "primary", "blue", "purple", "blurple", "1":
				convertedButton["style"] = types.PrimaryButton
			case "secondary", "grey", "2":
				convertedButton["style"] = types.SecondaryButton
			case "success", "green", "3":
				convertedButton["style"] = types.SuccessButton
			case "danger", "destructive", "red", "4":
				convertedButton["style"] = types.DangerButton
			case "link", "url", "5":
				convertedButton["style"] = types.LinkButton
			default:
				return nil, errors.New("invalid button style")
			}
		case "link":
			// discord made a button style named "link" but it needs a "url"
			// not a "link" field. this makes it a bit more user friendly
			convertedButton["url"] = v
		default:
			convertedButton[k] = v
		}
	}

	var button types.Button
	b, err := createComponent(types.ButtonComponent, convertedButton)
	if err == nil {
		button = b.(types.Button)
		// validation
		if button.Style == types.LinkButton && button.URL == "" {
			return nil, errors.New("a url field is required for a link button")
		}
		if button.Label == "" && button.Emoji == nil {
			return nil, errors.New("button must have a label or emoji")
		}
	}
	return &button, err
}

// CreateSelectMenu is YAGPDB's cmenu (components.go:256-329): a select menu from one map
// or sdict key-value pairs; "type" picks string (the default), user, role, mentionable or
// channel. As there, the option-count check only fires above 25 (the string-menu clause
// compares MenuType with StringSelectMenu, which a string menu leaves 0).
func CreateSelectMenu(values ...any) (*types.SelectMenu, error) {
	var messageSdict map[string]any
	switch t := values[0].(type) {
	case types.SDict:
		messageSdict = t
	case *types.SDict:
		messageSdict = *t
	case map[string]any:
		messageSdict = t
	case *types.SelectMenu:
		return t, nil
	default:
		dict, err := yagstd.StringKeyDictionary(values...)
		if err != nil {
			return nil, err
		}
		messageSdict = dict
	}

	menuType := types.SelectMenuComponent

	convertedMenu := make(map[string]any)
	for k, v := range messageSdict {
		switch strings.ToLower(k) {
		case "type":
			val, ok := v.(string)
			if !ok {
				return nil, errors.New("invalid select menu type")
			}
			switch strings.ToLower(val) {
			case "string", "text":
			case "user":
				menuType = types.UserSelectMenuComponent
			case "role":
				menuType = types.RoleSelectMenuComponent
			case "mentionable":
				menuType = types.MentionableSelectMenuComponent
			case "channel":
				menuType = types.ChannelSelectMenuComponent
			default:
				return nil, errors.New("invalid select menu type")
			}
		default:
			convertedMenu[k] = v
		}
	}

	var menu types.SelectMenu
	m, err := createComponent(menuType, convertedMenu)
	if err == nil {
		menu = m.(types.SelectMenu)

		// validation
		if menu.MenuType == types.StringSelectMenu && len(menu.Options) < 1 || len(menu.Options) > 25 {
			return nil, errors.New("invalid number of menu options in select menu, must have between 1 and 25")
		}
		if menu.MinValues != nil {
			if *menu.MinValues < 0 || *menu.MinValues > 25 {
				return nil, errors.New("invalid min values in select menu, must be between 0 and 25")
			}
		}
		if menu.MaxValues > 25 {
			return nil, errors.New("invalid max values in select menu, max 25")
		}
		checked := []string{}
		for _, o := range menu.Options {
			if slices.Contains(checked, o.Value) { // YAGPDB's in()
				return nil, errors.New("select menu options must have unique values")
			}
			checked = append(checked, o.Value)
		}
	}
	return &menu, err
}

// distributeComponentsIntoActionsRows is YAGPDB's (components.go:1028-1100): a slice of
// slices is the template's own rows (at most 5 rows of 5, a menu alone in its row);
// a flat slice is packed 5 to a row, a menu taking a row of its own, up to 25.
func distributeComponentsIntoActionsRows(components reflect.Value) (returnComponents []types.TopLevelComponent, err error) {
	if components.Len() < 1 {
		return make([]types.TopLevelComponent, 0), nil
	}

	const maxRows = 5       // Discord limitation
	const maxComponents = 5 // (per action row) Discord limitation
	v, _ := indirect(reflect.ValueOf(components.Index(0).Interface()))
	if v.Kind() == reflect.Slice {
		// slice within a slice. user is defining their own action row
		// layout; treat each slice as an action row
		for rowIdx := 0; rowIdx < components.Len() && rowIdx < maxRows; rowIdx++ {
			currentInputRow := reflect.ValueOf(components.Index(rowIdx).Interface())
			tempRow := types.ActionsRow{}
			for compIdx := 0; compIdx < currentInputRow.Len() && compIdx < maxComponents; compIdx++ {
				var component types.InteractiveComponent
				switch val := currentInputRow.Index(compIdx).Interface().(type) {
				case *types.Button:
					component = val
				case *types.SelectMenu:
					component = val
				default:
					return nil, errors.New("invalid component passed to send message builder")
				}
				if component.Type() == types.SelectMenuComponent && len(tempRow.Components) > 0 {
					return nil, errors.New("a select menu cannot share an action row with other components")
				}
				tempRow.Components = append(tempRow.Components, component)
				if component.Type() == types.SelectMenuComponent {
					break // move on to next row
				}
			}
			returnComponents = append(returnComponents, &tempRow)
		}
	} else {
		currentComponents := make([]types.InteractiveComponent, 0)
		for i := 0; i < components.Len() && i < maxRows*maxComponents; i++ {
			var component types.InteractiveComponent
			var isMenu bool

			switch val := components.Index(i).Interface().(type) {
			case *types.Button:
				component = val
			case *types.SelectMenu:
				isMenu = true
				component = val
			default:
				return nil, errors.New("invalid component passed to send message builder")
			}

			availableSpace := 5 - len(currentComponents)
			if !isMenu && availableSpace > 0 || isMenu && availableSpace == 5 {
				currentComponents = append(currentComponents, component)
			} else {
				returnComponents = append(returnComponents, &types.ActionsRow{Components: slices.Clone(currentComponents)})
				currentComponents = []types.InteractiveComponent{component}
			}

			// if it's a menu, the row is full now, append and start a new one
			if isMenu {
				returnComponents = append(returnComponents, &types.ActionsRow{Components: []types.InteractiveComponent{component}})
				currentComponents = []types.InteractiveComponent{}
			}

			if i == components.Len()-1 && len(currentComponents) > 0 { // if we're at the end, append the last row
				returnComponents = append(returnComponents, &types.ActionsRow{Components: slices.Clone(currentComponents)})
			}
		}
	}
	return
}

// validateCustomID is YAGPDB's (components.go:1102-1117): an empty ID is numbered by
// how many the message has so far, the templates- prefix is added when missing, and the
// result must fit Discord's 100 characters (90 after the prefix).
func validateCustomID(id string, used map[string]bool) (string, error) {
	if id == "" {
		id = fmt.Sprint(len(used))
	}

	if !strings.HasPrefix(id, TemplateCustomIDPrefix) {
		id = fmt.Sprint(TemplateCustomIDPrefix, id)
	}

	const maxCIDLength = 100 // discord limitation
	if len(id) > maxCIDLength {
		return "", fmt.Errorf("custom id too long (max %d chars)", maxCIDLength-len(TemplateCustomIDPrefix))
	}

	return id, nil
}

// validateTopLevelComponentsCustomIDs is YAGPDB's (components.go:1119-1166) for action
// rows: every button and menu gets a valid custom ID, and a link button loses its ID.
// Other top-level components (V2, not modelled) are skipped.
func validateTopLevelComponentsCustomIDs(rows []types.TopLevelComponent, used map[string]bool) error {
	if used == nil {
		used = make(map[string]bool)
	}
	for rowIdx := 0; rowIdx < len(rows); rowIdx++ {
		var rowComps []types.InteractiveComponent
		switch r := rows[rowIdx].(type) {
		case *types.ActionsRow:
			rowComps = r.Components
		default:
			continue
		}
		for compIdx := 0; compIdx < len(rowComps); compIdx++ {
			var err error
			switch c := (rowComps)[compIdx].(type) {
			case *types.Button:
				if c.Style == types.LinkButton {
					c.CustomID = ""
					continue
				}
				c.CustomID, err = validateCustomID(c.CustomID, used)
				used[c.CustomID] = true
			case *types.SelectMenu:
				c.CustomID, err = validateCustomID(c.CustomID, used)
				used[c.CustomID] = true
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// duplicateCustomID is the first custom ID two components of a message share, or "".
// YAGPDB never checks this (validateTopLevelComponentsCustomIDs writes `used[id]` but
// only reads it to number empty IDs, components.go:1102-1117), so a command with two
// buttons on one ID builds fine and Discord refuses the message (COMPONENT_CUSTOM_ID_
// DUPLICATED under 50035 Invalid Form Body). Link buttons carry no ID.
func duplicateCustomID(rows []types.TopLevelComponent) string {
	seen := make(map[string]bool)
	for _, row := range rows {
		r, ok := row.(*types.ActionsRow)
		if !ok {
			continue
		}
		for _, comp := range r.Components {
			var id string
			switch c := comp.(type) {
			case *types.Button:
				if c.Style == types.LinkButton {
					continue
				}
				id = c.CustomID
			case *types.SelectMenu:
				id = c.CustomID
			default:
				continue
			}
			if id == "" {
				continue
			}
			if seen[id] {
				return id
			}
			seen[id] = true
		}
	}
	return ""
}

// modalOnlyComponent reports whether the rows hold a label or a text input.
func modalOnlyComponent(rows []types.TopLevelComponent) bool {
	for _, row := range rows {
		switch r := row.(type) {
		case types.Label, *types.Label:
			return true
		case types.ActionsRow:
			if rowHasTextInput(r.Components) {
				return true
			}
		case *types.ActionsRow:
			if rowHasTextInput(r.Components) {
				return true
			}
		}
	}
	return false
}

func rowHasTextInput(comps []types.InteractiveComponent) bool {
	for _, c := range comps {
		switch c.(type) {
		case types.TextInput, *types.TextInput:
			return true
		}
	}
	return false
}

// checkComponents is the Discord-side check every send, edit and interaction response
// runs on its components: a repeated custom ID is refused (see duplicateCustomID).
// refused reports that the call must not go through; err is the error to return, nil when
// the refusal is only a warning (discordRefuses).
func (ctx *ExecutionContext) checkComponents(fn string, rows []types.TopLevelComponent) (refused bool, err error) {
	if modalOnlyComponent(rows) {
		// A cmodal's .Data sent as a message (parseMessageInput) carries its text inputs
		// and labels, which Discord only takes in a modal (INF: not captured)
		return true, ctx.discordRefuses(fn, errInvalidFormBody,
			"the message holds a text input or a label, which only go in a modal (sendModal)")
	}
	id := duplicateCustomID(rows)
	if id == "" {
		return false, nil
	}
	return true, ctx.discordRefuses(fn, errInvalidFormBody,
		fmt.Sprintf("two components share the custom_id %q (COMPONENT_CUSTOM_ID_DUPLICATED); "+
			"YAGPDB doesn't check this, so give every button and menu a distinct custom_id", id))
}
