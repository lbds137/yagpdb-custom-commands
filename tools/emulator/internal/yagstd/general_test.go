package templates

import "testing"

// tmplToInt/ToInt64 take an optional base argument (vendor c579722, common/templates/
// general.go:1218-1258); default base stays 10.
func TestToIntBase(t *testing.T) {
	if got := tmplToInt("ff", 16); got != 255 {
		t.Errorf("toInt \"ff\" 16 = %d, want 255", got)
	}
	if got := ToInt64("-10", 2); got != -2 {
		t.Errorf("toInt64 \"-10\" 2 = %d, want -2", got)
	}
	if got := tmplToInt("12"); got != 12 {
		t.Errorf("toInt \"12\" (default base) = %d, want 12", got)
	}
}
