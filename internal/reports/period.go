package reports

import (
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/workouts"
)

// WeekPeriod returns the Monday and Sunday of the week that contains day, as
// civil dates at midnight in loc.
func WeekPeriod(day time.Time, loc *time.Location) (start, end time.Time) {
	date := civilDate(day, loc)
	date = date.AddDate(0, 0, -((int(date.Weekday()) + 6) % 7))
	return localDay(date, loc), localDay(date.AddDate(0, 0, 6), loc)
}

// MonthPeriod returns the first and last day of the month that contains day.
func MonthPeriod(day time.Time, loc *time.Location) (start, end time.Time) {
	date := civilDate(day, loc)
	first := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
	return localDay(first, loc), localDay(first.AddDate(0, 1, -1), loc)
}

// PreviousPeriod returns the comparison period: the previous calendar month for
// month reports, and the same number of days right before the period for week
// and block reports. Days are counted on civil dates, never on elapsed hours,
// because a week across a DST change is 167 or 169 hours long.
func PreviousPeriod(kind Kind, start, end time.Time) (prevStart, prevEnd time.Time) {
	loc := start.Location()
	from, to := civilDate(start, loc), civilDate(end, loc)
	if kind == KindMonth {
		return localDay(time.Date(from.Year(), from.Month()-1, 1, 0, 0, 0, 0, time.UTC), loc), localDay(from.AddDate(0, 0, -1), loc)
	}
	prevEnd = from.AddDate(0, 0, -1)
	return localDay(prevEnd.AddDate(0, 0, 1-daySpan(from, to)), loc), localDay(prevEnd, loc)
}

// civilDate carries the civil date of an instant in loc as a UTC value, so date
// arithmetic never depends on the zone's offset.
func civilDate(t time.Time, loc *time.Location) time.Time { return workouts.LocalDate(t, loc) }

// localDay returns 00:00 of a civil date in loc. A zone that skips midnight
// (Azores, 2026-03-29) has no 00:00 that day: the first hour that exists starts
// it, so the date never moves backwards.
func localDay(date time.Time, loc *time.Location) time.Time {
	at := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	if at.Format(time.DateOnly) != date.Format(time.DateOnly) {
		at = time.Date(date.Year(), date.Month(), date.Day(), 1, 0, 0, 0, loc)
	}
	return at
}

// daySpan counts the inclusive civil days from start to end.
func daySpan(start, end time.Time) int {
	from, to := civilDate(start, start.Location()), civilDate(end, end.Location())
	return int(to.Sub(from).Hours()/24) + 1
}
