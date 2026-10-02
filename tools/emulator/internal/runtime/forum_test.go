package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
)

// forumCtx is a guild with a forum channel (77, tags news/meta, default thread slowmode
// 5), a text channel (78), an announcement channel (80) and a declared thread (79).
func forumCtx(strict bool) *ExecutionContext {
	ctx := newCtx(strict, true)
	ctx.ChannelName = "general"
	ctx.Channels = map[int64]string{ctx.ChannelID: "general", 77: "forum", 78: "texty", 80: "newsy"}
	ctx.ChannelOrder = []int64{ctx.ChannelID, 77, 78, 80}
	ctx.ChannelDetails = map[int64]types.CtxChannel{
		77: {ID: 77, Name: "forum", Type: channelTypeForum, DefaultThreadRateLimitPerUser: 5,
			AvailableTags: []types.ForumTag{{ID: 101, Name: "news"}, {ID: 102, Name: "meta"}}},
		78: {ID: 78, Name: "texty", Type: channelTypeText},
		80: {ID: 80, Name: "newsy", Type: channelTypeNews},
	}
	ctx.Threads[79] = "old-thread"
	ctx.ThreadOrder = append(ctx.ThreadOrder, 79)
	return ctx
}

// sentOutside counts the messages the run sent to any channel but except (a failed
// run's error text is posted to its own channel, which doesn't count as a post).
func sentOutside(ctx *ExecutionContext, except int64) int {
	n := 0
	for _, m := range ctx.SentMessages {
		if m.ChannelID != except {
			n++
		}
	}
	return n
}

// A post creates a public thread of the forum channel, resolvable as a thread is
// (ChannelArg, getChannelOrThread; getChannel, channels-only, doesn't find it), with the
// first message recorded in it
func TestCreateForumPostCreatesResolvableThread(t *testing.T) {
	ctx := forumCtx(true)
	out, err := run(t, ctx, `{{$p := createForumPost 77 "my title" "hello"}}`+
		`{{$p.ID}} {{$p.Name}} {{$p.IsThread}} {{$p.ParentID}} {{$p.AppliedTags}}`)
	want := fmt.Sprintf("%d my title true 77 []", firstThreadID)
	if err != nil || out != want {
		t.Fatalf("got %q, %v; want %q", out, err, want)
	}
	if len(ctx.SentMessages) != 1 || ctx.SentMessages[0].ChannelID != firstThreadID ||
		ctx.SentMessages[0].Content != "hello" {
		t.Errorf("first message %+v, want content %q in channel %d", ctx.SentMessages, "hello", firstThreadID)
	}
	if _, ok := ctx.Threads[firstThreadID]; !ok {
		t.Errorf("thread %d not registered in Threads (%v)", firstThreadID, ctx.Threads)
	}
	if got, ok := ctx.Channels[firstThreadID]; ok {
		t.Errorf("a thread must not land in .Guild.Channels; Channels[%d] = %q", firstThreadID, got)
	}

	// resolvable by ID and by name afterwards, as a declared thread is
	ctx = forumCtx(true)
	out, err = run(t, ctx, `{{$p := createForumPost 77 "my title" "hello"}}`+
		`{{(getChannelOrThread "my title").ID}} {{(getChannelOrThread "My Title").ID}}`)
	if err != nil || out != fmt.Sprintf("%d %d", firstThreadID, firstThreadID) {
		t.Errorf("name lookup: got %q, %v", out, err)
	}
	// getChannel never finds a thread (GS.GetChannel is channels-only)
	ctx = forumCtx(true)
	_, err = run(t, ctx, `{{$p := createForumPost 77 "t" "x"}}{{(getChannel $p.ID).Name}}`)
	if err == nil || !strings.Contains(err.Error(), "channel not in state") {
		t.Errorf("getChannel of the thread: got %v, want \"channel not in state\"", err)
	}
}

// The post's message pings by its allowed mentions: a complexMessage parsing roles lets
// a typed role mention ping, while plain string content (YAGPDB's users-only default)
// pings only users
func TestCreateForumPostPings(t *testing.T) {
	ctx := forumCtx(true)
	if _, err := run(t, ctx, `{{createForumPost 77 "t" (complexMessage "content" "<@&111> hi"`+
		` "allowed_mentions" (sdict "parse" (cslice "roles")))}}`); err != nil {
		t.Fatal(err)
	}
	if p := ctx.SentMessages[0].Pings; len(p.Roles) != 1 || p.Roles[0] != 111 || len(p.Users) != 0 {
		t.Errorf("parse roles: got pings %s, want role 111 only", p)
	}

	ctx = forumCtx(true)
	if _, err := run(t, ctx, `{{createForumPost 77 "t" "<@&111> hi <@222>"}}`); err != nil {
		t.Fatal(err)
	}
	// the notify lesson: a typed role mention alone pings no one without allowed_mentions
	if p := ctx.SentMessages[0].Pings; len(p.Roles) != 0 || len(p.Users) != 1 || p.Users[0] != 222 {
		t.Errorf("plain content: got pings %s, want user 222 only", p)
	}
}

// One post per run, premium or not (create_thread 1/1), with ErrTooManyCalls
func TestCreateForumPostLimit(t *testing.T) {
	const two = `{{createForumPost 77 "a" "x"}}{{createForumPost 77 "b" "y"}}`
	for _, premium := range []bool{true, false} {
		forum := forumCtx(true)
		ctx := newCtx(true, premium)
		ctx.ChannelDetails, ctx.Channels, ctx.ChannelOrder = forum.ChannelDetails, forum.Channels, forum.ChannelOrder
		ctx.Threads, ctx.ThreadOrder = forum.Threads, forum.ThreadOrder
		_, err := run(t, ctx, two)
		if err == nil || !strings.Contains(err.Error(), ErrTooManyCalls.Error()) ||
			!strings.Contains(explained(ctx, err), "createForumPost: over the limit of 1 create_thread calls per run") {
			t.Errorf("premium %v: want ErrTooManyCalls naming the limit, got %v (%s)", premium, err, explained(ctx, err))
		}
		if len(ctx.SentMessages) != 1 {
			// the first post's message; the failed run's error text goes to the run's
			// channel as its own message (show_errors), not the thread's
			inThread := 0
			for _, m := range ctx.SentMessages {
				if m.ChannelID != ctx.ChannelID {
					inThread++
				}
			}
			if inThread != 1 || len(ctx.Threads) != 2 {
				t.Errorf("premium %v: only the first post happens, sent %+v, threads %v", premium, ctx.SentMessages, ctx.Threads)
			}
		}
	}
}

// Content and channel errors carry YAGPDB's texts; an unresolvable channel is a nil
// channel without one
func TestCreateForumPostErrors(t *testing.T) {
	cases := []struct{ src, err string }{
		{`{{createForumPost 77 "t" nil}}`, "post content must not be nil"},
		{`{{createForumPost 77 "t" ""}}`, "post content must be non-zero length"},
		{`{{createForumPost 77 "t" (sdict "a" 1)}}`, "post content must be string, embed, or complex message"},
		{`{{createForumPost 78 "t" "x"}}`, "must specify a forum channel"},
		{`{{createForumPost "texty" "t" "x"}}`, "must specify a forum channel"},
		{`{{createForumPost 79 "t" "x"}}`, "channel not in state"}, // a declared thread
	}
	for _, c := range cases {
		if _, err := run(t, forumCtx(true), c.src); err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: got %v, want it to contain %q", c.src, err, c.err)
		}
	}

	// an unknown channel is no error: nil channel, nothing sent (vendor returns nil, nil)
	ctx := forumCtx(true)
	out, err := run(t, ctx, `{{createForumPost 999 "t" "x"}}`)
	if err != nil || out != "<nil>" || len(ctx.SentMessages) != 0 {
		t.Errorf("unknown channel: got %q, %v, sent %+v; want <nil>, no error, nothing sent", out, err, ctx.SentMessages)
	}
}

// An empty or over-100-character thread name is Discord's 50035 Invalid Form Body: an
// error in strict mode, a warning (and no post) otherwise; nothing is created either way
func TestCreateForumPostNameRefused(t *testing.T) {
	long := strings.Repeat("a", 101)
	for _, src := range []string{
		`{{createForumPost 77 "" "x"}}`,
		fmt.Sprintf(`{{createForumPost 77 %q "x"}}`, long),
	} {
		ctx := forumCtx(true)
		_, err := run(t, ctx, src)
		if err == nil || !strings.Contains(err.Error(), `"code": 50035`) ||
			!strings.Contains(err.Error(), "Invalid Form Body") {
			t.Errorf("%s: strict: want Discord's 50035, got %v", src, err)
		}
		// the failed run's error text is posted to the run's channel (show_errors), so
		// "nothing created" means no message outside it and no new thread
		if len(ctx.Threads) != 1 || sentOutside(ctx, ctx.ChannelID) != 0 {
			t.Errorf("%s: strict: the post must not exist; sent %+v, threads %v", src, ctx.SentMessages, ctx.Threads)
		}

		ctx = forumCtx(false)
		out, err := run(t, ctx, src)
		if err != nil || out != "<nil>" {
			t.Errorf("%s: non-strict: got %q, %v; want <nil>, no error", src, out, err)
		}
		if len(ctx.SentMessages) != 0 || len(ctx.Threads) != 1 {
			t.Errorf("%s: non-strict: nothing sent, got %+v, threads %v", src, ctx.SentMessages, ctx.Threads)
		}
		want := "the thread name"
		if w := kinds(ctx, KindLimit); len(w) == 0 || !strings.Contains(w[0], want) {
			t.Errorf("%s: non-strict: want a warning about %q, got %q", src, want, w)
		}
	}
}

// processThreadArgs, directly (vendor context_funcs.go:1568-1666): defaults from the
// parent, tags by name and by ID, the vendor error texts, and the parsed-but-ignored
// auto_archive_duration and invitable
func TestProcessThreadArgs(t *testing.T) {
	parent := forumCtx(true).ChannelDetails[77]

	// a new thread defaults to the parent's DefaultThreadRateLimitPerUser and no tags
	pt, err := processThreadArgs(true, parent)
	if err != nil || *pt.RateLimitPerUser != 5 || pt.AppliedTags == nil || len(*pt.AppliedTags) != 0 {
		t.Errorf("defaults: got %+v, %v", pt, err)
	}
	// slowmode
	pt, err = processThreadArgs(true, parent, "slowmode", 30)
	if err != nil || *pt.RateLimitPerUser != 30 {
		t.Errorf("slowmode: got %+v, %v", pt, err)
	}
	// tags by name, by ID and as a slice, dropping unknown names and duplicates
	pt, err = processThreadArgs(true, parent, "tags", "news")
	if err != nil || len(*pt.AppliedTags) != 1 || (*pt.AppliedTags)[0] != 101 {
		t.Errorf("tags by name: got %+v, %v", pt, err)
	}
	pt, err = processThreadArgs(true, parent, "tags", "102")
	if err != nil || len(*pt.AppliedTags) != 1 || (*pt.AppliedTags)[0] != 102 {
		t.Errorf("tags by id: got %+v, %v", pt, err)
	}
	pt, err = processThreadArgs(true, parent, "tags", []interface{}{"news", "nope", "news", "meta"})
	if err != nil || len(*pt.AppliedTags) != 2 || (*pt.AppliedTags)[0] != 101 || (*pt.AppliedTags)[1] != 102 {
		t.Errorf("tags slice: got %+v, %v", pt, err)
	}
	// an unknown tag name applies nothing silently
	pt, err = processThreadArgs(true, parent, "tags", "nope")
	if err != nil || len(*pt.AppliedTags) != 0 {
		t.Errorf("unknown tag: got %+v, %v", pt, err)
	}
	// without declared tags, tags applies nothing silently (nil AvailableTags)
	pt, err = processThreadArgs(true, types.CtxChannel{Type: channelTypeForum}, "tags", "news")
	if err != nil || len(*pt.AppliedTags) != 0 {
		t.Errorf("no available tags: got %+v, %v", pt, err)
	}
	// auto_archive_duration and invitable are parsed... (60/1440/4320/10080; a bool)
	pt, err = processThreadArgs(true, parent, "auto_archive_duration", 1440, "invitable", true)
	if err != nil || pt.AutoArchiveDuration == nil || *pt.AutoArchiveDuration != 1440 ||
		pt.Invitable == nil || !*pt.Invitable {
		t.Errorf("parsed keys: got %+v, %v", pt, err)
	}

	bad := []struct {
		values []interface{}
		err    string
	}{
		{[]interface{}{"tags", 42}, "`tags` must be of type string or cslice"},
		{[]interface{}{"auto_archive_duration", 30}, "'auto_archive_duration' must be 60, 1440, 4320, or 10080"},
		{[]interface{}{"invitable", "yes"}, "'invitable' must be a boolean"},
		{[]interface{}{"nope", 1}, `invalid key "nope"`},
	}
	for _, c := range bad {
		_, err := processThreadArgs(true, parent, c.values...)
		if err == nil || err.Error() != c.err {
			t.Errorf("%v: got %v, want %q", c.values, err, c.err)
		}
	}
}

// A post with thread arguments applies its tags to the created thread, as the returned
// channel shows. Vendor dedupes by the names SUPPLIED ("meta" and "102" name the same
// tag but both apply), not by resolved ID.
func TestCreateForumPostAppliesTags(t *testing.T) {
	ctx := forumCtx(true)
	out, err := run(t, ctx, `{{$p := createForumPost 77 "t" "x" "tags" (cslice "news" "meta")}}{{$p.AppliedTags}}`)
	if err != nil || out != "[101 102]" {
		t.Errorf("got %q, %v; want [101 102]", out, err)
	}
	if c := ctx.ChannelDetails[firstThreadID]; len(c.AppliedTags) != 2 || c.AppliedTags[0] != 101 || c.AppliedTags[1] != 102 {
		t.Errorf("thread details: got AppliedTags %v, want [101 102]", c.AppliedTags)
	}

	ctx = forumCtx(true)
	out, err = run(t, ctx, `{{$p := createForumPost 77 "t" "x" "tags" (cslice "meta" "102")}}{{$p.AppliedTags}}`)
	if err != nil || out != "[102 102]" { // same tag twice: different supplied names
		t.Errorf("got %q, %v; want [102 102] (vendor dedupes by supplied name)", out, err)
	}
}

// The first message goes through the send path: an over-2000-character content is
// Discord's refusal, with nothing created
func TestCreateForumPostMessageLimits(t *testing.T) {
	ctx := forumCtx(true)
	_, err := run(t, ctx, fmt.Sprintf(`{{createForumPost 77 "t" %q}}`, strings.Repeat("a", 2001)))
	if err == nil || !strings.Contains(err.Error(), "Invalid Form Body") {
		t.Errorf("strict: want Discord's content refusal, got %v", err)
	}
	// the error carries Discord's text; the emulator's detail is a warning
	if w := kinds(ctx, KindLimit); len(w) == 0 || !strings.Contains(w[0], "content is 2001 characters (max 2000)") {
		t.Errorf("strict: want a warning naming the content problem, got %q", w)
	}
	if len(ctx.Threads) != 1 || sentOutside(ctx, ctx.ChannelID) != 0 {
		t.Errorf("nothing must be created; sent %+v, threads %v", ctx.SentMessages, ctx.Threads)
	}
}

// A thread of a text channel is created empty (nothing recorded as sent — the observable
// difference from createForumPost), registered in Threads and resolvable as a thread is
// (ChannelArg, getChannelOrThread; getChannel, channels-only, doesn't find it)
func TestCreateThreadCreatesResolvableThread(t *testing.T) {
	ctx := forumCtx(true)
	out, err := run(t, ctx, `{{$t := createThread 78 0 "my thread"}}`+
		`{{$t.ID}} {{$t.Name}} {{$t.IsThread}} {{$t.ParentID}}`)
	want := fmt.Sprintf("%d my thread true 78", firstThreadID)
	if err != nil || out != want {
		t.Fatalf("got %q, %v; want %q", out, err, want)
	}
	if len(ctx.SentMessages) != 0 {
		t.Errorf("the thread is created empty; sent %+v", ctx.SentMessages)
	}
	if _, ok := ctx.Threads[firstThreadID]; !ok {
		t.Errorf("thread %d not registered in Threads (%v)", firstThreadID, ctx.Threads)
	}
	if got, ok := ctx.Channels[firstThreadID]; ok {
		t.Errorf("a thread must not land in .Guild.Channels; Channels[%d] = %q", firstThreadID, got)
	}

	// resolvable by ID and by name afterwards, as a declared thread is
	ctx = forumCtx(true)
	out, err = run(t, ctx, `{{$t := createThread 78 0 "my thread"}}`+
		`{{(getChannelOrThread $t.ID).Name}} {{(getChannelOrThread "my thread").ID}} {{(getChannelOrThread "My Thread").ID}}`)
	want = fmt.Sprintf("my thread %d %d", firstThreadID, firstThreadID)
	if err != nil || out != want {
		t.Errorf("lookup: got %q, %v; want %q", out, err, want)
	}
	// getChannel never finds a thread (GS.GetChannel is channels-only)
	ctx = forumCtx(true)
	_, err = run(t, ctx, `{{$t := createThread 78 0 "t"}}{{(getChannel $t.ID).Name}}`)
	if err == nil || !strings.Contains(err.Error(), "channel not in state") {
		t.Errorf("getChannel of the thread: got %v, want \"channel not in state\"", err)
	}
}

// The thread's type: public by default, private with the first positional optional true,
// and in an announcement channel always a news thread — vendor's news check runs after
// the optionals, overwriting private
func TestCreateThreadTypes(t *testing.T) {
	cases := []struct {
		src  string
		kind int
	}{
		{`createThread 78 0 "t"`, channelTypeGuildPublicThread},
		{`createThread 78 0 "t" false`, channelTypeGuildPublicThread},
		{`createThread 78 0 "t" true`, channelTypeGuildPrivateThread},
		{`createThread 80 0 "t"`, channelTypeGuildNewsThread},
		{`createThread 80 0 "t" true`, channelTypeGuildNewsThread},
	}
	for _, c := range cases {
		ctx := forumCtx(true)
		out, err := run(t, ctx, `{{$t := `+c.src+`}}{{$t.Type}}`)
		want := fmt.Sprintf("%d", c.kind)
		if err != nil || out != want {
			t.Errorf("%s: got %q, %v; want type %s", c.src, out, err, want)
		}
	}
}

// The positional optionals: every valid auto-archive duration parses (positional, not a
// dict), and a wrong-typed one carries YAGPDB's text verbatim
func TestCreateThreadOptionals(t *testing.T) {
	for _, d := range []string{"60", "1440", "4320", "10080"} {
		ctx := forumCtx(true)
		if _, err := run(t, ctx, `{{createThread 78 0 "t" false `+d+` false}}`); err != nil {
			t.Errorf("duration %s: got %v, want no error", d, err)
		}
	}
	cases := []struct{ src, err string }{
		{`{{createThread 78 0 "t" "yes"}}`, "createThread 'private' must be a boolean"},
		{`{{createThread 78 0 "t" nil}}`, "createThread 'private' must be a boolean"},
		{`{{createThread 78 0 "t" true 30}}`, "createThread 'auto_archive_duration' must be 60, 1440, 4320, or 10080"},
		{`{{createThread 78 0 "t" true 1440 "yes"}}`, "createThread 'invitable' must be a boolean"},
		{`{{createThread 78 0 "t" true 1440 true "extra"}}`, "createThread: Too many arguments"},
	}
	for _, c := range cases {
		if _, err := run(t, forumCtx(true), c.src); err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: got %v, want it to contain %q", c.src, err, c.err)
		}
	}
}

// A thread can attach to a message (msgID > 0): a known one works, an unknown one is
// Discord's 10008 Unknown Message — an error in strict mode, a warning (and no thread)
// otherwise, as with the reactions
func TestCreateThreadOnMessage(t *testing.T) {
	ctx := forumCtx(true)
	ctx.Messages = []types.CtxMessage{{ID: 500, ChannelID: 78, Author: botUser}}
	out, err := run(t, ctx, `{{$t := createThread 78 500 "from msg"}}{{$t.ID}} {{$t.ParentID}}`)
	if err != nil || out != fmt.Sprintf("%d 78", firstThreadID) {
		t.Fatalf("got %q, %v; want a thread of 78", out, err)
	}

	// unknown message: strict refuses with 10008, and nothing is created
	ctx = forumCtx(true)
	_, err = run(t, ctx, `{{createThread 78 999 "x"}}`)
	if err == nil || !strings.Contains(err.Error(), `"code": 10008`) ||
		!strings.Contains(err.Error(), "Unknown Message") {
		t.Errorf("strict: want Discord's 10008, got %v", err)
	}
	if len(ctx.Threads) != 1 {
		t.Errorf("strict: no thread must exist; threads %v", ctx.Threads)
	}

	// non-strict: a warning naming the message, and no thread
	ctx = forumCtx(false)
	out, err = run(t, ctx, `{{createThread 78 999 "x"}}`)
	if err != nil || out != "<nil>" {
		t.Errorf("non-strict: got %q, %v; want <nil>, no error", out, err)
	}
	if w := kinds(ctx, KindLimit); len(w) == 0 || !strings.Contains(w[0], "no message 999 in channel 78") {
		t.Errorf("non-strict: want a warning about message 999, got %q", w)
	}
	if len(ctx.Threads) != 1 {
		t.Errorf("non-strict: no thread must exist; threads %v", ctx.Threads)
	}
}

// One create_thread call per run, shared with createForumPost: whichever comes second is
// ErrTooManyCalls (both orders), premium or not, and the first one happened
func TestCreateThreadLimitSharedWithForumPost(t *testing.T) {
	for _, premium := range []bool{true, false} {
		for _, tc := range []struct{ src, second string }{
			{`{{createForumPost 77 "a" "x"}}{{createThread 78 0 "b"}}`,
				"createThread: over the limit of 1 create_thread calls per run"},
			{`{{createThread 78 0 "b"}}{{createForumPost 77 "a" "x"}}`,
				"createForumPost: over the limit of 1 create_thread calls per run"},
		} {
			forum := forumCtx(true)
			ctx := newCtx(true, premium)
			ctx.ChannelDetails, ctx.Channels, ctx.ChannelOrder = forum.ChannelDetails, forum.Channels, forum.ChannelOrder
			ctx.Threads, ctx.ThreadOrder = forum.Threads, forum.ThreadOrder
			_, err := run(t, ctx, tc.src)
			if err == nil || !strings.Contains(err.Error(), ErrTooManyCalls.Error()) ||
				!strings.Contains(explained(ctx, err), tc.second) {
				t.Errorf("premium %v, %s: want ErrTooManyCalls naming the second call, got %v (%s)",
					premium, tc.src, err, explained(ctx, err))
			}
			if len(ctx.Threads) != 2 {
				t.Errorf("premium %v, %s: only the first thread exists, got %v", premium, tc.src, ctx.Threads)
			}
		}
	}
}

// An empty or over-100-character thread name is Discord's 50035 Invalid Form Body: an
// error in strict mode, a warning (and no thread) otherwise — YAGPDB doesn't check it
func TestCreateThreadNameRefused(t *testing.T) {
	long := strings.Repeat("a", 101)
	for _, src := range []string{
		`{{createThread 78 0 ""}}`,
		fmt.Sprintf(`{{createThread 78 0 %q}}`, long),
	} {
		ctx := forumCtx(true)
		_, err := run(t, ctx, src)
		if err == nil || !strings.Contains(err.Error(), `"code": 50035`) ||
			!strings.Contains(err.Error(), "Invalid Form Body") {
			t.Errorf("%s: strict: want Discord's 50035, got %v", src, err)
		}
		if len(ctx.Threads) != 1 {
			t.Errorf("%s: strict: the thread must not exist; threads %v", src, ctx.Threads)
		}

		ctx = forumCtx(false)
		out, err := run(t, ctx, src)
		if err != nil || out != "<nil>" {
			t.Errorf("%s: non-strict: got %q, %v; want <nil>, no error", src, out, err)
		}
		if len(ctx.Threads) != 1 {
			t.Errorf("%s: non-strict: no thread must exist; threads %v", src, ctx.Threads)
		}
		if w := kinds(ctx, KindLimit); len(w) == 0 || !strings.Contains(w[0], "the thread name") {
			t.Errorf("%s: non-strict: want a warning about the thread name, got %q", src, w)
		}
	}
}

// YAGPDB's channel errors: a resolved thread is "not in state", and an unresolvable
// channel is no error at all (nil, nil), nothing created
func TestCreateThreadErrors(t *testing.T) {
	if _, err := run(t, forumCtx(true), `{{createThread 79 0 "t"}}`); err == nil ||
		!strings.Contains(err.Error(), "channel not in state") {
		t.Errorf("thread parent: got %v, want \"channel not in state\"", err)
	}

	ctx := forumCtx(true)
	out, err := run(t, ctx, `{{createThread 999 0 "t"}}`)
	if err != nil || out != "<nil>" || len(ctx.Threads) != 1 {
		t.Errorf("unknown channel: got %q, %v, threads %v; want <nil>, no error, nothing created", out, err, ctx.Threads)
	}
}
