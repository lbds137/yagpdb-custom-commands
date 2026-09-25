package template

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// EMULATOR PATCH tests: OnMaxOps.

const manyOps = `{{range $i := seq}}{{$i}}{{end}}`

func seqFunc() []int { return make([]int, 200) }

func TestMaxOpsStopsWithoutCallback(t *testing.T) {
	tmpl := Must(New("t").Funcs(FuncMap{"seq": seqFunc}).MaxOps(100).Parse(manyOps))
	err := tmpl.Execute(&bytes.Buffer{}, nil)
	if err == nil || !strings.Contains(err.Error(), "exceeded max operations") {
		t.Fatalf("want the operation limit error, got %v", err)
	}
}

func TestOnMaxOpsReportsOnceAndContinues(t *testing.T) {
	calls, gotMax := 0, 0
	tmpl := Must(New("t").Funcs(FuncMap{"seq": seqFunc}).MaxOps(100).OnMaxOps(func(ops, max int) {
		calls++
		gotMax = max
	}).Parse(manyOps))
	var out bytes.Buffer
	if err := tmpl.Execute(&out, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || gotMax != 100 {
		t.Errorf("callback calls=%d max=%d, want 1 and 100", calls, gotMax)
	}
	if out.Len() != 200 {
		t.Errorf("execution should finish, printed %d bytes", out.Len())
	}
}

// EMULATOR PATCH tests: OnCall.

func TestOnCallReportsWhetherTheCallIsInsideTry(t *testing.T) {
	var got []bool
	f := func(s string) string { return s }
	src := `{{f "out"}}{{define "d"}}{{f "tmpl"}}{{end}}` +
		`{{try}}{{f "in"}}{{template "d"}}{{index 1 1}}{{catch}}{{f "catch"}}{{end}}{{f "after"}}`
	tmpl := Must(New("t").Funcs(FuncMap{"f": f}).OnCall(func(inTry bool) { got = append(got, inTry) }).Parse(src))
	if err := tmpl.Execute(&bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	// out, in, the template's call, index (in try), catch, after
	want := []bool{false, true, true, true, false, false}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
