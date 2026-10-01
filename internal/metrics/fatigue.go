package metrics

import (
	"math"
	"time"
)

const (
	// TauCTL is the chronic training-load time constant in days.
	TauCTL = 42
	// TauATL is the acute training-load time constant in days.
	TauATL = 7
)

// DailyLoad contains the training stress score recorded for a calendar day.
type DailyLoad struct {
	Day time.Time
	TSS float64
}

// CTL returns the final 42-day exponential moving average of daily TSS.
// The EMA cold-starts at zero and processes only the supplied values.
func CTL(daily []DailyLoad) float64 {
	return ema(daily, TauCTL)
}

// ATL returns the final 7-day exponential moving average of daily TSS.
// The EMA cold-starts at zero and processes only the supplied values.
func ATL(daily []DailyLoad) float64 {
	return ema(daily, TauATL)
}

// TSB returns training stress balance, chronic load minus acute load.
func TSB(ctl, atl float64) float64 {
	return ctl - atl
}

// FillMissingDays returns one row per inclusive UTC calendar date, preserving
// observed TSS and filling absent days with zero. Reversed ranges are empty.
func FillMissingDays(from, to time.Time, raw []DailyLoad) []DailyLoad {
	start := utcDay(from)
	end := utcDay(to)
	if start.After(end) {
		return nil
	}

	loads := make(map[time.Time]float64, len(raw))
	for _, entry := range raw {
		loads[utcDay(entry.Day)] = entry.TSS
	}

	filled := make([]DailyLoad, 0, int(end.Sub(start).Hours()/24)+1)
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		filled = append(filled, DailyLoad{Day: day, TSS: loads[day]})
	}
	return filled
}

func utcDay(day time.Time) time.Time {
	day = day.UTC()
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
}

func ema(daily []DailyLoad, tau int) float64 {
	if len(daily) == 0 {
		return 0
	}
	alpha := 1 - math.Exp(-1/float64(tau))
	load := daily[0].TSS
	for _, day := range daily[1:] {
		load += (day.TSS - load) * alpha
	}
	return load
}
