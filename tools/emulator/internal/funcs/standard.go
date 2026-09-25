package funcs

import (
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/yagstd"
)

// Type Conversion Functions

// ToString is YAGPDB's ToString (yagstd), so mocks convert values as production does.
func ToString(v interface{}) string {
	return yagstd.ToString(v)
}

// ToInt converts a value to int.
func ToInt(v interface{}) int {
	return int(ToInt64(v))
}

// ToInt64 is YAGPDB's ToInt64 (yagstd), so mocks convert values as production does.
func ToInt64(v interface{}) int64 {
	return yagstd.ToInt64(v)
}

// ToFloat64 is YAGPDB's ToFloat64 (yagstd), so mocks convert values as production does.
func ToFloat64(v interface{}) float64 {
	return yagstd.ToFloat64(v)
}

// ToDuration converts a value to time.Duration.
func ToDuration(v interface{}) time.Duration {
	switch val := v.(type) {
	case time.Duration:
		return val
	case int:
		return time.Duration(val)
	case int64:
		return time.Duration(val)
	case float64:
		return time.Duration(val)
	case string:
		d, _ := time.ParseDuration(val)
		return d
	default:
		return 0
	}
}

// String Manipulation Functions

// Math Functions

// Time Functions

// Regex Functions
// Note: These accept interface{} to handle nil values gracefully

// Utility Functions
