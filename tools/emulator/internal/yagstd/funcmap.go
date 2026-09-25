package templates

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/lbds137/yagpdb-custom-commands/tools/emulator/internal/confusables"
)

// StandardFuncs returns YAGPDB's standard template functions that don't need Discord,
// keyed and wired exactly as in StandardFuncMap (common/templates/context.go, yagpdb
// 0cf2ec5). The emulator adds its own mocks for the Discord-dependent ones (cembed,
// complexMessage, ...) and for context functions.
//
// Not included, because they need Discord data: componentBuilder and the other component
// builders, and roleAbove (the emulator's roles are its own type).
//
// The clock and random functions (RunFuncs) read the system clock and math/rand here; the
// emulator's runs replace them with their own.
func StandardFuncs() map[string]interface{} {
	m := map[string]interface{}{
		// conversion functions
		"str":        ToString,
		"toString":   ToString,
		"toInt":      tmplToInt,
		"toInt64":    ToInt64,
		"toFloat":    ToFloat64,
		"toDuration": ToDuration,
		"toRune":     ToRune,
		"toByte":     ToByte,

		// string manipulation
		"hasPrefix":   strings.HasPrefix,
		"hasSuffix":   strings.HasSuffix,
		"joinStr":     joinStrings,
		"lower":       strings.ToLower,
		"slice":       slice,
		"split":       strings.Split,
		"title":       titleCaser.String,
		"trimSpace":   strings.TrimSpace,
		"upper":       strings.ToUpper,
		"urlescape":   url.PathEscape,
		"urlunescape": url.PathUnescape,
		"print":       withOutputLimit(fmt.Sprint, MaxStringLength),
		"println":     withOutputLimit(fmt.Sprintln, MaxStringLength),
		"printf":      withOutputLimitF(fmt.Sprintf, MaxStringLength),

		// regexp
		"reQuoteMeta": regexp.QuoteMeta,

		// math
		"abs":        tmplAbs,
		"add":        add,
		"cbrt":       tmplCbrt,
		"div":        tmplDiv,
		"fdiv":       tmplFDiv,
		"log":        tmplLog,
		"mathConst":  tmplMathConstant,
		"max":        tmplMax,
		"min":        tmplMin,
		"mod":        tmplMod,
		"mult":       tmplMult,
		"pow":        tmplPow,
		"round":      tmplRound,
		"roundCeil":  tmplRoundCeil,
		"roundEven":  tmplRoundEven,
		"roundFloor": tmplRoundFloor,
		"sqrt":       tmplSqrt,
		"sub":        tmplSub,

		// bitwise ops
		"bitwiseAnd":        tmplBitwiseAnd,
		"bitwiseOr":         tmplBitwiseOr,
		"bitwiseXor":        tmplBitwiseXor,
		"bitwiseNot":        tmplBitwiseNot,
		"bitwiseAndNot":     tmplBitwiseAndNot,
		"bitwiseLeftShift":  tmplBitwiseLeftShift,
		"bitwiseRightShift": tmplBitwiseRightShift,

		// misc
		"humanizeThousands": tmplHumanizeThousands,

		"humanizeDurationHours":   tmplHumanizeDurationHours,
		"humanizeDurationMinutes": tmplHumanizeDurationMinutes,
		"humanizeDurationSeconds": tmplHumanizeDurationSeconds,

		"sanitizeText":  confusables.SanitizeText,
		"dict":          Dictionary,
		"sdict":         StringKeyDictionary,
		"structToSdict": StructToSdict,
		"cslice":        CreateSlice,
		"kindOf":        KindOf,
		"in":            in,
		"inFold":        inFold,
		"json":          tmplJson,
		"jsonToSdict":   tmplJSONToSDict,
		"seq":           sequence,

		// time functions
		"parseTime":       tmplParseTime,
		"formatTime":      tmplFormatTime,
		"loadLocation":    time.LoadLocation,
		"newDate":         tmplNewDate,
		"snowflakeToTime": tmplSnowflakeToTime,
		"timestampToTime": tmplTimestampToTime,
		"weekNumber":      tmplWeekNumber,
	}
	for name, fn := range RunFuncs(time.Now, GlobalRandom{}) {
		m[name] = fn
	}
	return m
}

// CallVariadic is callVariadic, for the emulator's copies of YAGPDB's variadic context
// functions (addReactions and the like).
func CallVariadic(f func([]reflect.Value) (reflect.Value, error), skipNil bool, values ...reflect.Value) (reflect.Value, error) {
	return callVariadic(f, skipNil, values...)
}
