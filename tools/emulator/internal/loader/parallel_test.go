package loader

import (
	"fmt"
	"testing"
)

// 24 tests on a CI runner (the parallel path) come back in input order, each
// result its own test's, all passing. The templates call title — the function
// that used to share a package-level x/text caser — so the pool exercises (and
// under -race watches) shared template code, not just literal output.
func TestRunTestsParallelKeepsInputOrder(t *testing.T) {
	const n = 24
	tests := make([]*TestCase, n)
	for i := range tests {
		tests[i] = &TestCase{
			Name:           fmt.Sprintf("pool-%02d", i),
			TemplateSource: fmt.Sprintf(`{{print "out-%02d " (title "hello world")}}`, i),
			Expected:       ExpectedResult{OutputEquals: str(fmt.Sprintf("out-%02d Hello World", i))},
		}
		tests[i].applyDefaults()
	}
	r := NewRunner(RunnerConfig{CI: true})
	results := r.RunTests(tests)
	if !r.parallelTaken {
		t.Fatal("CI without UpdateSnapshots or StopOnFail must take the parallel path")
	}
	if len(results) != n {
		t.Fatalf("want %d results, got %d", n, len(results))
	}
	for i, res := range results {
		if res.Name != tests[i].Name || res.Output != fmt.Sprintf("out-%02d Hello World", i) || !res.Passed {
			t.Errorf("result %d: name %q output %q passed=%v; want %q, out-%02d Hello World, passed",
				i, res.Name, res.Output, res.Passed, tests[i].Name, i)
		}
	}
}

// StopOnFail keeps the sequential loop: the run stops at the first failure and
// the tests after it never run
func TestRunTestsStopOnFailStopsAtFirstFailure(t *testing.T) {
	const n = 8
	const failAt = 3
	tests := make([]*TestCase, n)
	for i := range tests {
		tc := &TestCase{
			Name:           fmt.Sprintf("seq-%02d", i),
			TemplateSource: fmt.Sprintf("out-%02d", i),
		}
		if i == failAt {
			tc.Expected.OutputEquals = str("never printed")
		}
		tc.applyDefaults()
		tests[i] = tc
	}
	r := NewRunner(RunnerConfig{StopOnFail: true})
	results := r.RunTests(tests)
	if r.parallelTaken {
		t.Fatal("StopOnFail must keep the sequential loop")
	}
	if len(results) != failAt+1 {
		t.Fatalf("want the run to stop at test %d, got %d results", failAt, len(results))
	}
	if last := results[failAt]; last.Passed {
		t.Errorf("the last result should be the failing test: %+v", last)
	}
	for i, res := range results[:failAt] {
		if !res.Passed {
			t.Errorf("result %d should have passed: %q", i, res.Failures)
		}
	}
}
