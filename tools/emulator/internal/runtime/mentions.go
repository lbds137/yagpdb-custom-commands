package runtime

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// Pings are the mentions in a message that notify someone, as Discord decides them from
// the message's allowed mentions. Users and Roles are sorted.
type Pings struct {
	Everyone bool // @everyone or @here
	Users    []int64
	Roles    []int64
}

// Empty reports whether the message pings no one.
func (p Pings) Empty() bool {
	return !p.Everyone && len(p.Users) == 0 && len(p.Roles) == 0
}

func (p Pings) String() string {
	var parts []string
	if p.Everyone {
		parts = append(parts, "everyone")
	}
	for _, id := range p.Users {
		parts = append(parts, fmt.Sprintf("<@%d>", id))
	}
	for _, id := range p.Roles {
		parts = append(parts, fmt.Sprintf("<@&%d>", id))
	}
	if len(parts) == 0 {
		return "nobody"
	}
	return strings.Join(parts, " ")
}

var (
	userMentionRe = regexp.MustCompile(`<@!?(\d+)>`)
	roleMentionRe = regexp.MustCompile(`<@&(\d+)>`)
)

// pingsOf is Discord's rule for which mentions in content ping: a mention type in Parse
// pings every mention of that type, and otherwise only the IDs listed in Users or Roles do.
// Mentions in embeds never ping. The emulator assumes the bot may ping every role.
func pingsOf(content string, allowed types.AllowedMentions) Pings {
	var p Pings
	if slices.Contains(allowed.Parse, "everyone") {
		p.Everyone = strings.Contains(content, "@everyone") || strings.Contains(content, "@here")
	}
	p.Users = mentionedIDs(userMentionRe, content, slices.Contains(allowed.Parse, "users"), allowed.Users)
	p.Roles = mentionedIDs(roleMentionRe, content, slices.Contains(allowed.Parse, "roles"), allowed.Roles)
	return p
}

func mentionedIDs(re *regexp.Regexp, content string, all bool, allowed []int64) []int64 {
	var ids []int64
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		id := yagstd.ToInt64(m[1])
		if id != 0 && (all || slices.Contains(allowed, id)) && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// usersOnly is the allowed mentions YAGPDB sends with by default: users ping, roles and
// @everyone don't.
func usersOnly() types.AllowedMentions {
	return types.AllowedMentions{Parse: []string{"users"}}
}

// noEscape is what the NoEscape functions send with: every mention pings.
func noEscape() types.AllowedMentions {
	return types.AllowedMentions{Parse: []string{"users", "roles", "everyone"}, RepliedUser: true}
}

// responseMentions is YAGPDB's Context.MessageSend: the response pings users, @everyone
// and @here once mentionEveryone or mentionHere ran, and the roles mentionRole returned.
func (ctx *ExecutionContext) responseMentions() types.AllowedMentions {
	allowed := usersOnly()
	if ctx.mentionEveryone {
		allowed.Parse = append(allowed.Parse, "everyone")
	}
	allowed.Roles = ctx.mentionRoles
	return allowed
}

// parseAllowedMentions is YAGPDB's parseAllowedMentions, for complexMessage's
// "allowed_mentions".
func parseAllowedMentions(Data interface{}) (*types.AllowedMentions, error) {

	if m, ok := Data.(types.AllowedMentions); ok {
		return &m, nil
	}

	converted, err := yagstd.StringKeyDictionary(Data)
	if err != nil {
		return nil, err
	}

	var parsingUsers bool
	var parsingRoles bool

	allowedMentions := &types.AllowedMentions{}
	for k, v := range converted {

		switch strings.ToLower(k) {
		case "parse":
			var parseMentions []string
			var parseSlice yagstd.Slice
			conv, err := parseSlice.AppendSlice(v)
			if err != nil {
				return nil, errors.New(`Allowed Mentions Parsing: invalid datatype passed to "Parse", accepts a slice only`)
			}
			for _, elem := range conv.(yagstd.Slice) {
				elem_conv, _ := elem.(string)
				if elem_conv != "users" && elem_conv != "roles" && elem_conv != "everyone" {
					return nil, errors.New(`Allowed Mentions Parsing: invalid slice element in "Parse", accepts "roles", "users", and "everyone"`)
				}
				parseMentions = append(parseMentions, elem_conv)
				if elem_conv == "users" {
					parsingUsers = true
				} else if elem_conv == "roles" {
					parsingRoles = true
				}
			}
			allowedMentions.Parse = parseMentions
		case "users", "roles":
			var newslice []int64
			var parseSlice yagstd.Slice
			conv, err := parseSlice.AppendSlice(v)
			if err != nil {
				return nil, fmt.Errorf(`allowed Mentions Parsing: invalid datatype passed to "%s", accepts a slice of snowflakes only`, k)
			}
			for _, elem := range conv.(yagstd.Slice) {
				if (yagstd.ToInt64(elem)) == 0 {
					return nil, fmt.Errorf(`allowed Mentions Parsing: "%s" IDSlice: invalid ID passed -`+fmt.Sprint(elem), k)
				}
				newslice = append(newslice, yagstd.ToInt64(elem))
			}
			if len(newslice) > 100 {
				newslice = newslice[:100]
			}
			if strings.ToLower(k) == "users" {
				allowedMentions.Users = newslice
			} else {
				allowedMentions.Roles = newslice
			}
		case "replied_user":
			isRepliedUserMention, ok := v.(bool)
			if !ok {
				return nil, errors.New(`Allowed Mentions Parsing: invalid datatype passed to "replied_user", accepts a bool only`)
			}
			allowedMentions.RepliedUser = isRepliedUserMention
		default:
			return nil, errors.New(`Allowed Mentions Parsing: invalid key "` + k + `" for Allowed Mentions`)
		}
	}

	if parsingUsers && allowedMentions.Users != nil {
		return nil, errors.New(`Allowed Mentions Parsing: conflicting values passed, you cannot parse all users if only allowing a set of users`)
	} else if parsingRoles && allowedMentions.Roles != nil {
		return nil, errors.New(`Allowed Mentions Parsing: conflicting values passed, you cannot parse all roles if only allowing a set of roles`)
	}

	return allowedMentions, nil
}
