package main

import (
	"reflect"
	"testing"
)

func TestRunPaths(t *testing.T) {
	codes := map[string]int{"a": 0, "b": 1, "c": 0}
	for _, tc := range []struct {
		stopOnFail bool
		wantRun    []string
	}{
		{false, []string{"a", "b", "c"}},
		{true, []string{"a", "b"}},
	} {
		var ran []string
		run := func(opts testOptions) int {
			ran = append(ran, opts.path)
			return codes[opts.path]
		}
		code := runPaths(testOptions{stopOnFail: tc.stopOnFail}, []string{"a", "b", "c"}, run)
		if code != 1 {
			t.Errorf("stopOnFail=%v: exit code %d, want 1", tc.stopOnFail, code)
		}
		if !reflect.DeepEqual(ran, tc.wantRun) {
			t.Errorf("stopOnFail=%v: ran %v, want %v", tc.stopOnFail, ran, tc.wantRun)
		}
	}

	// A later passing path doesn't clear an earlier failure; passing paths alone pass
	run := func(opts testOptions) int { return codes[opts.path] }
	if code := runPaths(testOptions{}, []string{"b", "a"}, run); code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	if code := runPaths(testOptions{stopOnFail: true}, []string{"a", "c"}, run); code != 0 {
		t.Errorf("exit code %d, want 0", code)
	}
}
