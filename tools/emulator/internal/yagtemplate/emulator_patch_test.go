package template

import (
	"bytes"
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
