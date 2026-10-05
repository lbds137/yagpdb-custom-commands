package runtime

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/funcs"
	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
	yagstd "github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// maxThreadNameRunes is Discord's limit on a thread's name. YAGPDB doesn't check it:
// Discord rejects the whole post with 50035 Invalid Form Body (the first message goes
// in the same call, so nothing is created).
const maxThreadNameRunes = 100

// createForumPost is YAGPDB's tmplCreateForumPost (vendor common/templates/
// context_funcs.go:1479-1531): a public thread in a forum channel, its first message
// sent with it. The thread is registered in Threads/ThreadOrder (resolvable by
// ChannelArg and getChannelOrThread, absent from .Guild.Channels and getChannel, as
// YAGPDB's state tracker keeps it) and its message recorded as a send that pings no
// one (see the send below). The create_thread counter is taken by withLimits
// (limitedFuncs), as YAGPDB's IncreaseCheckCallCounterPremium call does there.
func (e *Engine) createForumPost(channel, name, content interface{}, optional ...interface{}) (*types.CtxChannel, error) {

	if content == nil {
		return nil, errors.New("post content must not be nil")
	}

	cID := e.channelArg(channel)
	if cID == 0 {
		return nil, nil //dont send an error, a nil output would indicate invalid/unknown channel
	}

	// GS.GetChannel knows channels only: a resolved thread is "not in state" (vendor
	// context_funcs.go tmplCreateForumPost's cstate lookup)
	if _, isThread := e.ctx.Threads[cID]; isThread {
		return nil, errors.New("channel not in state")
	}

	cstate := e.ctx.channelState(cID)
	if cstate.Type != channelTypeForum {
		return nil, errors.New("must specify a forum channel")
	}

	partialThreaad, err := processThreadArgs(true, cstate, optional...)
	if err != nil {
		return nil, err
	}

	// vendor's ThreadStart: Type public thread, Invitable forced false, and
	// auto_archive_duration ignored (see processThreadArgs)
	var msgData *types.MessageSend
	switch v := content.(type) {
	case string:
		if len(v) == 0 {
			return nil, errors.New("post content must be non-zero length")
		}
		msgData, _ = e.complexMessage("content", v)
	case types.Embed, *types.MessageEmbed:
		msgData, _ = e.complexMessage("embed", v)
	case *types.MessageSend:
		msgData = v
	default:
		return nil, errors.New("post content must be string, embed, or complex message")
	}

	// ForumThreadStartComplex is one Discord call: it creates the thread AND its first
	// message, so a refusal (name, message limits, components) creates nothing
	title := funcs.ToString(name)
	if n := len([]rune(title)); n == 0 || n > maxThreadNameRunes {
		detail := fmt.Sprintf("the thread name is %d characters (max %d)", n, maxThreadNameRunes)
		if n == 0 {
			detail = "the thread name is empty"
		}
		if err := e.ctx.discordRefuses("createForumPost", errInvalidFormBody, detail); err != nil {
			return nil, err
		}
		return nil, nil
	}

	// The first message goes through the path sendMessage takes: Discord's message
	// limits (checkSend). What it does not take is the pings. Production smoke
	// 2026-10-02: the thread-start endpoint's first message carried allowed_mentions
	// (lib/discordgo/restapi.go:2634-2637 nests the MessageSend, message.go:340
	// marshals the field) yet never notified, while the same bot's regular messages
	// did. Discord doesn't notify on it (the content still renders), so it records as
	// pinging no one; a caller who wants the ping sends it as a second message (the
	// /prompt form's writer does).
	if e.ctx.ChannelsCannotSend[cID] {
		return nil, e.ctx.discordRefuses("createForumPost", errMissingPerms,
			fmt.Sprintf("the bot lacks permission to send messages in channel %d", cID))
	}
	contentText, embeds, components := msgData.Content, msgData.Embeds, msgData.Components
	notEmpty := msgData.HasFile || msgData.HasOther || len(components) > 0
	if ok, err := e.ctx.checkSend("createForumPost", contentText, embeds, notEmpty, false); !ok {
		return nil, err
	}
	if refused, err := e.ctx.checkComponents("createForumPost", components); refused {
		return nil, err
	}
	if msgData.HasFile {
		e.ctx.RecordFileUpload(cID, msgData.Filename, msgData.File)
	}

	threadID := e.ctx.nextThreadID()
	if e.ctx.ChannelDetails == nil {
		e.ctx.ChannelDetails = make(map[int64]types.CtxChannel)
	}
	e.ctx.Threads[threadID] = title
	e.ctx.ThreadOrder = append(e.ctx.ThreadOrder, threadID)
	if e.ctx.forumPosts == nil {
		e.ctx.forumPosts = map[int64]bool{}
	}
	e.ctx.forumPosts[threadID] = true
	e.ctx.ChannelDetails[threadID] = types.CtxChannel{ID: threadID, Name: title,
		Type:        channelTypeGuildPublicThread,
		ParentID:    cID,
		AppliedTags: *partialThreaad.AppliedTags}

	e.ctx.RecordSentMessage(threadID, contentText, embeds, components, Pings{})

	thread := e.ctx.channelState(threadID)
	return &thread, nil
}

// createThread is YAGPDB's tmplCreateThread (vendor common/templates/context_funcs.go:
// 1239-1314): an empty thread of a text, announcement or forum channel — no first message,
// which is the observable difference from createForumPost. Its optionals are positional,
// not a dict: private, auto_archive_duration, invitable. The thread is registered in
// Threads/ThreadOrder (resolvable by ChannelArg and getChannelOrThread, absent from
// .Guild.Channels and getChannel, as YAGPDB's state tracker keeps it) and the create_thread
// counter is taken by withLimits (limitedFuncs), as YAGPDB's IncreaseCheckCallCounterPremium
// call does there. Not modeled: the thread's metadata (the auto_archive_duration and
// invitable it was created with — ChannelDetails carries neither, and no template surface
// reads them back).
func (e *Engine) createThread(channel, msgID, name interface{}, optionals ...interface{}) (*types.CtxChannel, error) {

	cID := e.channelArg(channel)
	if cID == 0 {
		return nil, nil //dont send an error, a nil output would indicate invalid/unknown channel
	}

	// GS.GetChannel knows channels only: a resolved thread is "not in state" (vendor
	// context_funcs.go tmplCreateThread's cstate lookup)
	if _, isThread := e.ctx.Threads[cID]; isThread {
		return nil, errors.New("channel not in state")
	}

	cstate := e.ctx.channelState(cID)

	// vendor's ThreadStart: type public thread; the optionals are positional
	threadType := channelTypeGuildPublicThread
	mID := funcs.ToInt64(msgID)
	for index, opt := range optionals {
		switch index {
		case 0:
			switch opt := opt.(type) {
			case bool:
				if opt {
					threadType = channelTypeGuildPrivateThread
				}
			default:
				return nil, errors.New("createThread 'private' must be a boolean")
			}
		case 1:
			// discordgo.AutoArchiveDuration OneHour, OneDay, ThreeDays, OneWeek; parsed and
			// validated, then not modeled (see the function comment)
			switch yagstd.ToIntTmpl(opt) {
			case 60, 1440, 4320, 10080:
			default:
				return nil, errors.New("createThread 'auto_archive_duration' must be 60, 1440, 4320, or 10080")
			}
		case 2:
			// invitable is parsed and validated, then not modeled (see the function comment)
			switch opt.(type) {
			case bool:
			default:
				return nil, errors.New("createThread 'invitable' must be a boolean")
			}
		default:
			return nil, errors.New("createThread: Too many arguments")
		}
	}

	if cstate.Type == channelTypeNews {
		threadType = channelTypeGuildNewsThread
	}

	// This is where the ThreadStartComplex call goes: Discord's refusals are modelled
	// here, and YAGPDB checks for neither. A message to attach to that isn't known to
	// exist is 10008 (the reactions precedent; the route resolves the message before it
	// validates the body), and an empty or over-100-character name is 50035 Invalid Form
	// Body.
	if mID > 0 && !e.ctx.messageExists(cID, mID) {
		if err := e.ctx.discordRefuses("createThread", errUnknownMessage,
			fmt.Sprintf("no message %d in channel %d (a test's messages, a sent one or the run's message)", mID, cID)); err != nil {
			return nil, err
		}
		return nil, nil
	}
	title := funcs.ToString(name)
	if n := len([]rune(title)); n == 0 || n > maxThreadNameRunes {
		detail := fmt.Sprintf("the thread name is %d characters (max %d)", n, maxThreadNameRunes)
		if n == 0 {
			detail = "the thread name is empty"
		}
		if err := e.ctx.discordRefuses("createThread", errInvalidFormBody, detail); err != nil {
			return nil, err
		}
		return nil, nil
	}

	threadID := e.ctx.nextThreadID()
	if e.ctx.ChannelDetails == nil {
		e.ctx.ChannelDetails = make(map[int64]types.CtxChannel)
	}
	e.ctx.Threads[threadID] = title
	e.ctx.ThreadOrder = append(e.ctx.ThreadOrder, threadID)
	e.ctx.ChannelDetails[threadID] = types.CtxChannel{ID: threadID, Name: title,
		Type:     threadType,
		ParentID: cID}

	// no first message goes out with it: the thread is created empty
	thread := e.ctx.channelState(threadID)
	return &thread, nil
}

// tagIDFromName is vendor's (context_funcs.go:1544-1558): the available tag with that
// name or ID (as a string), or 0.
func tagIDFromName(c *types.CtxChannel, tagName string) int64 {

	if c.AvailableTags == nil {
		return 0
	}

	// walk available tags list and see if there's a match
	for _, tag := range c.AvailableTags {
		if tag.Name == tagName || strconv.FormatInt(tag.ID, 10) == tagName {
			return tag.ID
		}
	}

	return 0
}

type partialThread struct {
	RateLimitPerUser    *int
	AppliedTags         *[]int64
	AutoArchiveDuration *int
	Invitable           *bool
}

// processThreadArgs is vendor's (context_funcs.go:1568-1666) over the emulator's
// CtxChannel: a partial thread from the optional key-value pairs after a post's or
// thread's fixed arguments. Note the vendor asymmetry kept here: the forum post
// YAGPDB starts (discordgo.ThreadStart) reads only slowmode and tags —
// auto_archive_duration and invitable are parsed and validated, then ignored
// (Invitable is forced false).
func processThreadArgs(newThread bool, parent types.CtxChannel, values ...interface{}) (*partialThread, error) {

	c := &partialThread{}
	if newThread {
		c = &partialThread{
			RateLimitPerUser: &parent.DefaultThreadRateLimitPerUser,
			AppliedTags:      &[]int64{},
		}
	}

	if len(values) == 0 {
		return c, nil
	}

	threadSdict, err := yagstd.StringKeyDictionary(values...)
	if err != nil {
		return c, err
	}

	for key, val := range threadSdict {

		key = strings.ToLower(key)
		switch key {
		case "slowmode":
			ratelimit := yagstd.ToIntTmpl(val)
			c.RateLimitPerUser = &ratelimit
		case "tags":
			if parent.AvailableTags == nil {
				break
			}

			var tags []int64
			v, _ := indirect(reflect.ValueOf(val))
			const maxTags = 5 // discord limit
			if v.Kind() == reflect.String {
				tag := tagIDFromName(&parent, funcs.ToString(val))
				// ensure supplied id is valid
				if tag > 0 {
					tags = []int64{tag}
					c.AppliedTags = &tags
				}
			} else if v.Kind() == reflect.Slice {
				// used to get rid of any duplicate tags the user might have sent
				seen := make(map[string]struct{})
				size := v.Len()
				if size > maxTags {
					size = maxTags
				}

				tags = make([]int64, 0, size)
				for i := 0; i < v.Len() && len(seen) < size; i++ {
					name := funcs.ToString(v.Index(i).Interface())
					if len(name) == 0 {
						continue
					}

					_, ok := seen[name]
					if ok {
						continue
					}

					// try to convert and check if the id is valid
					tag := tagIDFromName(&parent, name)
					if tag == 0 {
						continue
					}

					seen[name] = struct{}{}
					tags = append(tags, tag)
				}
				c.AppliedTags = &tags

			} else {
				return c, errors.New("`tags` must be of type string or cslice")
			}
		case "auto_archive_duration":
			duration := yagstd.ToIntTmpl(val) // discordgo.AutoArchiveDuration values below
			switch duration {
			case 60, 1440, 4320, 10080: // OneHour, OneDay, ThreeDays, OneWeek
				c.AutoArchiveDuration = &duration
			default:
				return nil, errors.New("'auto_archive_duration' must be 60, 1440, 4320, or 10080")
			}
		case "invitable":
			val, ok := val.(bool)
			if ok {
				invitable := val
				c.Invitable = &invitable
				continue
			}
			return c, errors.New("'invitable' must be a boolean")
		default:
			return c, errors.New(`invalid key "` + key + `"`)
		}
	}

	return c, nil
}
