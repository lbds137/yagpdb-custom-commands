// Copied from YAGPDB (github.com/botlabs-gg/yagpdb, commit 0cf2ec5), common/util.go (RandomAdjective to HumanizeDuration), common/templates/general.go
// (tmplHumanize*, tmplSnowflakeToTime) and bot/util.go (SnowflakeToTime).
// MIT license, see LICENSE-YAGPDB. Changes: package name; common. prefixes dropped; SnowflakeToTime computes Discord's snowflake
// time directly instead of through the snowflake package (same epoch, 1420070400000);
// Random* and tmplHumanizeTimeSinceDays are built for a run from its random source and
// clock (see runfuncs.go).

package templates

import (
	"fmt"
	"time"
)

func RandomAdjective(random Random) func() string {
	return func() string {
		return Adjectives[random.Intn(len(Adjectives))]
	}
}

func RandomNoun(random Random) func() string {
	return func() string {
		return Nouns[random.Intn(len(Nouns))]
	}
}

func RandomVerb(random Random) func() string {
	return func() string {
		return Verbs[random.Intn(len(Verbs))]
	}
}

type DurationFormatPrecision int

const (
	DurationPrecisionSeconds DurationFormatPrecision = iota
	DurationPrecisionMinutes
	DurationPrecisionHours
	DurationPrecisionDays
	DurationPrecisionWeeks
	DurationPrecisionYears
)

func (d DurationFormatPrecision) String() string {
	switch d {
	case DurationPrecisionSeconds:
		return "second"
	case DurationPrecisionMinutes:
		return "minute"
	case DurationPrecisionHours:
		return "hour"
	case DurationPrecisionDays:
		return "day"
	case DurationPrecisionWeeks:
		return "week"
	case DurationPrecisionYears:
		return "year"
	}
	return "Unknown"
}

func (d DurationFormatPrecision) FromSeconds(in int64) int64 {
	switch d {
	case DurationPrecisionSeconds:
		return in % 60
	case DurationPrecisionMinutes:
		return (in / 60) % 60
	case DurationPrecisionHours:
		return ((in / 60) / 60) % 24
	case DurationPrecisionDays:
		days := (((in / 60) / 60) / 24)
		// 365 % 7 == 1, meaning calculating days based on remainder after dividing
		// into weeks for a year would leave us with 1 extra day. This wouldn't be
		// a problem if we stopped at weeks, since 365 days == 52 weeks and 1 day,
		// however the weeks mod out to 0 so that 365 days properly becomes a year.
		// 52 weeks ≠ 1 year. Resultingly, we need to skip every 365th day.
		return (days - days/365) % 7
	case DurationPrecisionWeeks:
		// There's 52 weeks + 1 day per year (techically +1.25... but were doing +1)
		// Make sure 364 days isnt 0 weeks and 0 years
		days := (((in / 60) / 60) / 24) % 365
		return days / 7
	case DurationPrecisionYears:
		return (((in / 60) / 60) / 24) / 365
	}

	panic("We shouldn't be here")
}

func pluralize(val int64) string {
	if val == 1 {
		return ""
	}
	return "s"
}

func HumanizeDuration(precision DurationFormatPrecision, in time.Duration) string {
	seconds := int64(in.Seconds())

	out := make([]string, 0)

	for i := int(precision); i < int(DurationPrecisionYears)+1; i++ {
		curPrec := DurationFormatPrecision(i)
		units := curPrec.FromSeconds(seconds)
		if units > 0 {
			out = append(out, fmt.Sprintf("%d %s%s", units, curPrec.String(), pluralize(units)))
		}
	}

	outStr := ""

	for i := len(out) - 1; i >= 0; i-- {
		if i == 0 && i != len(out)-1 {
			outStr += " and "
		} else if i != len(out)-1 {
			outStr += " "
		}
		outStr += out[i]
	}

	if outStr == "" {
		outStr = "less than 1 " + precision.String()
	}

	return outStr
}

func tmplHumanizeDurationHours(in interface{}) string {
	return HumanizeDuration(DurationPrecisionHours, ToDuration(in))
}

func tmplHumanizeDurationMinutes(in interface{}) string {
	return HumanizeDuration(DurationPrecisionMinutes, ToDuration(in))
}

func tmplHumanizeDurationSeconds(in interface{}) string {
	return HumanizeDuration(DurationPrecisionSeconds, ToDuration(in))
}

func tmplHumanizeTimeSinceDays(now func() time.Time) func(in time.Time) string {
	return func(in time.Time) string {
		return HumanizeDuration(DurationPrecisionDays, now().Sub(in))
	}
}

func tmplSnowflakeToTime(v interface{}) time.Time {
	return SnowflakeToTime(ToInt64(v)).UTC()
}

// SnowflakeToTime is bot.SnowflakeToTime: snowflake.ID(i).Time() with Discord's epoch,
// truncated to the second.
func SnowflakeToTime(i int64) time.Time {
	ms := (i >> 22) + 1420070400000
	t := time.Unix(ms/1000, 0)
	return t
}
