package yagstd

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// StandardFuncs returns YAGPDB's standard template functions that don't need Discord,
// keyed and wired exactly as in StandardFuncMap (common/templates/context.go, yagpdb
// 0cf2ec5). The emulator adds its own mocks for the Discord-dependent ones (cembed,
// complexMessage, ...) and for context functions.
//
// Not included, because they need YAGPDB's bot or third-party data: sanitizeText,
// componentBuilder and the other component builders, roleAbove, adjective/noun/verb,
// snowflakeToTime, humanizeDuration*, humanizeTimeSinceDays.
func StandardFuncs() map[string]interface{} {
	return map[string]interface{}{
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
		"dict":              Dictionary,
		"sdict":             StringKeyDictionary,
		"structToSdict":     StructToSdict,
		"cslice":            CreateSlice,
		"kindOf":            KindOf,
		"in":                in,
		"inFold":            inFold,
		"json":              tmplJson,
		"jsonToSdict":       tmplJSONToSDict,
		"randInt":           randInt,
		"seq":               sequence,
		"shuffle":           shuffle,

		// time functions
		"currentTime":     tmplCurrentTime,
		"parseTime":       tmplParseTime,
		"formatTime":      tmplFormatTime,
		"loadLocation":    time.LoadLocation,
		"newDate":         tmplNewDate,
		"timestampToTime": tmplTimestampToTime,
		"weekNumber":      tmplWeekNumber,
	}
}
