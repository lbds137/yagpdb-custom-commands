package funcs

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/types"
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

// ToRune converts a string to a rune slice.
func ToRune(s string) []rune {
	return []rune(s)
}

// ToByte converts a string to a byte slice.
func ToByte(s string) []byte {
	return []byte(s)
}

// String Manipulation Functions

// JoinStrings joins strings with a separator.
func JoinStrings(sep string, args ...interface{}) string {
	strs := make([]string, len(args))
	for i, arg := range args {
		strs[i] = ToString(arg)
	}
	return strings.Join(strs, sep)
}

// SliceFunc extracts a substring or slice portion.
func SliceFunc(item interface{}, indices ...int) (interface{}, error) {
	switch v := item.(type) {
	case string:
		runes := []rune(v)
		start := 0
		end := len(runes)
		if len(indices) > 0 {
			start = indices[0]
		}
		if len(indices) > 1 {
			end = indices[1]
		}
		if start < 0 {
			start = 0
		}
		if end > len(runes) {
			end = len(runes)
		}
		if start > end {
			return "", nil
		}
		return string(runes[start:end]), nil
	case []interface{}:
		start := 0
		end := len(v)
		if len(indices) > 0 {
			start = indices[0]
		}
		if len(indices) > 1 {
			end = indices[1]
		}
		if start < 0 {
			start = 0
		}
		if end > len(v) {
			end = len(v)
		}
		if start > end {
			return []interface{}{}, nil
		}
		return v[start:end], nil
	case types.Slice:
		start := 0
		end := len(v)
		if len(indices) > 0 {
			start = indices[0]
		}
		if len(indices) > 1 {
			end = indices[1]
		}
		if start < 0 {
			start = 0
		}
		if end > len(v) {
			end = len(v)
		}
		if start > end {
			return types.Slice{}, nil
		}
		return v[start:end], nil
	default:
		return nil, fmt.Errorf("slice: unsupported type %T", item)
	}
}

// URLEscape escapes a string for use in URLs.
func URLEscape(s string) string {
	return url.PathEscape(s)
}

// URLUnescape unescapes a URL-encoded string.
func URLUnescape(s string) string {
	result, _ := url.PathUnescape(s)
	return result
}

// Math Functions

// Add adds numbers.
func Add(args ...interface{}) interface{} {
	if len(args) == 0 {
		return 0
	}

	// Check if any argument is float
	hasFloat := false
	for _, arg := range args {
		if _, ok := arg.(float64); ok {
			hasFloat = true
			break
		}
		if _, ok := arg.(float32); ok {
			hasFloat = true
			break
		}
	}

	if hasFloat {
		var sum float64
		for _, arg := range args {
			sum += ToFloat64(arg)
		}
		return sum
	}

	var sum int64
	for _, arg := range args {
		sum += ToInt64(arg)
	}
	return int(sum)
}

// Sub subtracts numbers.
func Sub(a, b interface{}) interface{} {
	af, bf := ToFloat64(a), ToFloat64(b)
	result := af - bf
	if result == float64(int64(result)) {
		return int(result)
	}
	return result
}

// Mult multiplies numbers.
func Mult(args ...interface{}) interface{} {
	if len(args) == 0 {
		return 0
	}

	result := ToFloat64(args[0])
	for _, arg := range args[1:] {
		result *= ToFloat64(arg)
	}

	if result == float64(int64(result)) {
		return int(result)
	}
	return result
}

// Div divides numbers (integer division).
func Div(a, b interface{}) (interface{}, error) {
	bf := ToFloat64(b)
	if bf == 0 {
		return nil, fmt.Errorf("division by zero")
	}
	return int(ToFloat64(a) / bf), nil
}

// FDiv divides numbers (floating point).
func FDiv(a, b interface{}) (float64, error) {
	bf := ToFloat64(b)
	if bf == 0 {
		return 0, fmt.Errorf("division by zero")
	}
	return ToFloat64(a) / bf, nil
}

// Mod returns the modulo of two numbers.
func Mod(a, b interface{}) (int, error) {
	bi := ToInt(b)
	if bi == 0 {
		return 0, fmt.Errorf("modulo by zero")
	}
	return ToInt(a) % bi, nil
}

// Abs returns the absolute value.
func Abs(n interface{}) float64 {
	return math.Abs(ToFloat64(n))
}

// Sqrt returns the square root.
func Sqrt(n interface{}) float64 {
	return math.Sqrt(ToFloat64(n))
}

// Cbrt returns the cube root.
func Cbrt(n interface{}) float64 {
	return math.Cbrt(ToFloat64(n))
}

// Pow returns x^y.
func Pow(x, y interface{}) float64 {
	return math.Pow(ToFloat64(x), ToFloat64(y))
}

// Log returns the natural logarithm.
func Log(n interface{}) float64 {
	return math.Log(ToFloat64(n))
}

// Round rounds to the nearest integer.
func Round(n interface{}) float64 {
	return math.Round(ToFloat64(n))
}

// RoundCeil rounds up.
func RoundCeil(n interface{}) float64 {
	return math.Ceil(ToFloat64(n))
}

// RoundFloor rounds down.
func RoundFloor(n interface{}) float64 {
	return math.Floor(ToFloat64(n))
}

// RoundEven rounds to nearest even.
func RoundEven(n interface{}) float64 {
	return math.RoundToEven(ToFloat64(n))
}

// Min returns the minimum value.
func Min(args ...interface{}) interface{} {
	if len(args) == 0 {
		return nil
	}
	min := ToFloat64(args[0])
	for _, arg := range args[1:] {
		v := ToFloat64(arg)
		if v < min {
			min = v
		}
	}
	if min == float64(int64(min)) {
		return int(min)
	}
	return min
}

// Max returns the maximum value.
func Max(args ...interface{}) interface{} {
	if len(args) == 0 {
		return nil
	}
	max := ToFloat64(args[0])
	for _, arg := range args[1:] {
		v := ToFloat64(arg)
		if v > max {
			max = v
		}
	}
	if max == float64(int64(max)) {
		return int(max)
	}
	return max
}

// Time Functions

// CurrentTime returns the current time.
func CurrentTime() time.Time {
	return time.Now()
}

// FormatTime formats a time value.
func FormatTime(t time.Time, layout string) string {
	return t.Format(layout)
}

// ParseTime parses a time string.
func ParseTime(layout, value string) (time.Time, error) {
	return time.Parse(layout, value)
}

// NewDate creates a new time.Time. Location is optional (defaults to UTC).
func NewDate(year, month, day, hour, min, sec int, loc ...*time.Location) time.Time {
	var location *time.Location = time.UTC
	if len(loc) > 0 && loc[0] != nil {
		location = loc[0]
	}
	return time.Date(year, time.Month(month), day, hour, min, sec, 0, location)
}

// Regex Functions
// Note: These accept interface{} to handle nil values gracefully

// ReFind finds the first match.
func ReFind(pattern, s interface{}) string {
	patternStr := ToString(pattern)
	sStr := ToString(s)
	re, err := regexp.Compile(patternStr)
	if err != nil {
		return ""
	}
	return re.FindString(sStr)
}

// ReFindAll finds all matches.
func ReFindAll(pattern, s interface{}, n ...int) []string {
	patternStr := ToString(pattern)
	sStr := ToString(s)
	re, err := regexp.Compile(patternStr)
	if err != nil {
		return nil
	}
	limit := -1
	if len(n) > 0 {
		limit = n[0]
	}
	return re.FindAllString(sStr, limit)
}

// ReReplace replaces matches.
func ReReplace(pattern, s, repl interface{}) string {
	patternStr := ToString(pattern)
	sStr := ToString(s)
	replStr := ToString(repl)
	re, err := regexp.Compile(patternStr)
	if err != nil {
		return sStr
	}
	return re.ReplaceAllString(sStr, replStr)
}

// ReSplit splits a string by regex pattern.
func ReSplit(pattern, s interface{}, n ...int) []string {
	patternStr := ToString(pattern)
	sStr := ToString(s)
	re, err := regexp.Compile(patternStr)
	if err != nil {
		return []string{sStr}
	}
	limit := -1
	if len(n) > 0 {
		limit = n[0]
	}
	return re.Split(sStr, limit)
}

// ReQuoteMeta escapes regex metacharacters.
func ReQuoteMeta(s interface{}) string {
	return regexp.QuoteMeta(ToString(s))
}

// Utility Functions

// In checks if a value is in a slice.
func In(needle interface{}, haystack ...interface{}) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// InFold checks if a string is in a slice (case-insensitive).
func InFold(needle string, haystack ...string) bool {
	needle = strings.ToLower(needle)
	for _, item := range haystack {
		if strings.ToLower(item) == needle {
			return true
		}
	}
	return false
}

// KindOf returns the kind/type of a value.
func KindOf(v interface{}) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", v)
}

// Seq generates a sequence of integers.
func Seq(args ...int) []int {
	var start, end, step int
	switch len(args) {
	case 1:
		end = args[0]
		step = 1
	case 2:
		start = args[0]
		end = args[1]
		step = 1
	case 3:
		start = args[0]
		end = args[1]
		step = args[2]
	default:
		return nil
	}

	if step == 0 {
		return nil
	}

	var result []int
	if step > 0 {
		for i := start; i < end; i += step {
			result = append(result, i)
		}
	} else {
		for i := start; i > end; i += step {
			result = append(result, i)
		}
	}
	return result
}

// RandInt generates a random integer (stub - should use proper random).
func RandInt(args ...int) int {
	// In a real implementation, this would use crypto/rand or math/rand
	// For testing, we return a predictable value
	switch len(args) {
	case 1:
		return args[0] / 2
	case 2:
		return (args[0] + args[1]) / 2
	default:
		return 0
	}
}

// Sort sorts a slice by a field specified in options.
// Usage: {{ sort $slice (sdict "key" "fieldName" "reverse" false) }}
func Sort(slice interface{}, options interface{}) (interface{}, error) {
	// For now, return the slice unchanged - full implementation would sort by field
	// This is sufficient for templates to parse and execute without error
	switch s := slice.(type) {
	case []interface{}:
		// Return a copy
		result := make([]interface{}, len(s))
		copy(result, s)
		return result, nil
	default:
		// Return as-is for other types
		return slice, nil
	}
}
