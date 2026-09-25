package runtime

import (
	"strings"
	"testing"
)

// A failed child's error message quotes the lines around the error as YAGPDB does: tabs as
// four spaces, the common indent removed, long lines cut, backticks broken with a ZWS
func TestFailedChildMessageQuotesTheSource(t *testing.T) {
	src := "\t{{$a := 1}}\n\t\t{{.ExecData.X.Y}} `c`\n\t{{/* a comment long enough to be cut off */}}\n"
	dir := t.TempDir()
	writeFile(t, dir+"/child.gohtml", src)
	writeFile(t, dir+"/bad.gohtml", "{{if}}")
	ctx := newCtx(false, true)
	ctx.TemplateBaseDir = dir
	ctx.CommandIDMap = map[int64]string{7: "child.gohtml", 8: "bad.gohtml"}
	if _, err := run(t, ctx, `{{execCC 7 nil 0 nil}}{{execCC 8 nil 0 nil}}`); err != nil {
		t.Fatal(err)
	}
	if len(ctx.SentMessages) != 2 {
		t.Fatalf("sent %+v", ctx.SentMessages)
	}
	stop := "\nAn error caused the execution of the custom command template to stop:\n"
	got := ctx.SentMessages[0].Content
	wantStart := stop + "`Failed executing CC #7, line 2, row "
	wantEnd := "nil pointer evaluating interface {}.X`\n```" +
		"1    {{$a := 1}}\n2        {{.ExecData.X.Y}} `​c`​\n3    {{/* a comment long enough to ...\n```"
	if !strings.HasPrefix(got, wantStart) || !strings.HasSuffix(got, wantEnd) {
		t.Errorf("got %q\nwant %q ... %q", got, wantStart, wantEnd)
	}
	// An execution error the formatter can't place quotes the whole error, with YAGPDB's prefix
	if _, err := run(t, newCtx(true, true), `{{printf "%030000d" 1}}`); err == nil ||
		err.Error() != "Failed executing template: response grew too big (>25k)" {
		t.Errorf("got %v", err)
	}
	// Not an execution error (here a parse error): the error as it is, in backticks
	want := stop + "`Failed parsing template: template: CC #8:1: missing value for if`"
	if got := ctx.SentMessages[1].Content; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
