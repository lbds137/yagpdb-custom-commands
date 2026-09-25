package funcs

import (
	"fmt"
	"strconv"
	"time"
)

// Type Conversion Functions

// ToString converts any value to a string.
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// ToInt converts a value to int.
func ToInt(v interface{}) int {
	return int(ToInt64(v))
}

// ToInt64 converts a value to int64.
func ToInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case int32:
		return int64(val)
	case float64:
		return int64(val)
	case float32:
		return int64(val)
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// ToFloat64 converts a value to float64.
func ToFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
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
