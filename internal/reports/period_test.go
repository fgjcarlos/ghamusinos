package reports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// zone resoluciona una zona IANA y falla el test si no existe en el sistema.
func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	return loc
}

// at parsea "2006-01-02 15:04" en la zona indicada (hora civil del usuario).
func at(t *testing.T, local string, loc *time.Location) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02 15:04", local, loc)
	require.NoError(t, err)
	return parsed
}

func TestWeekPeriod(t *testing.T) {
	madrid := zone(t, "Europe/Madrid")
	cases := []struct {
		name, day, wantStart, wantEnd string
	}{
		{"lunes", "2026-10-26 00:00", "2026-10-26", "2026-11-01"},
		{"miercoles", "2026-10-28 15:04", "2026-10-26", "2026-11-01"},
		{"domingo por la noche", "2026-11-01 23:59", "2026-10-26", "2026-11-01"},
		{"semana que cruza un mes", "2026-11-29 08:00", "2026-11-23", "2026-11-29"},
		{"semana del cambio de hora", "2026-10-25 12:00", "2026-10-19", "2026-10-25"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := WeekPeriod(at(t, tc.day, madrid), madrid)
			require.Equal(t, tc.wantStart, start.Format(time.DateOnly))
			require.Equal(t, tc.wantEnd, end.Format(time.DateOnly))
			require.Equal(t, 0, start.Hour())
			require.Equal(t, 0, end.Hour())
		})
	}
}

func TestMonthPeriod(t *testing.T) {
	madrid := zone(t, "Europe/Madrid")
	cases := []struct {
		name, day, wantStart, wantEnd string
	}{
		{"mitad de mes", "2026-02-15 12:00", "2026-02-01", "2026-02-28"},
		{"febrero bisiesto", "2028-02-29 12:00", "2028-02-01", "2028-02-29"},
		{"ultimo dia del año", "2026-12-31 23:00", "2026-12-01", "2026-12-31"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := MonthPeriod(at(t, tc.day, madrid), madrid)
			require.Equal(t, tc.wantStart, start.Format(time.DateOnly))
			require.Equal(t, tc.wantEnd, end.Format(time.DateOnly))
			require.Equal(t, 0, start.Hour())
			require.Equal(t, 0, end.Hour())
			require.Equal(t, 1, start.Day())
		})
	}
}

func TestPreviousPeriod(t *testing.T) {
	madrid := zone(t, "Europe/Madrid")
	weekStart, weekEnd := WeekPeriod(at(t, "2026-11-01 12:00", madrid), madrid)
	blockStart, blockEnd := at(t, "2027-01-04 00:00", madrid), at(t, "2027-01-10 00:00", madrid)
	monthStart, monthEnd := MonthPeriod(at(t, "2026-01-31 12:00", madrid), madrid)
	cases := []struct {
		name               string
		kind               Kind
		start, end         time.Time
		wantStart, wantEnd string
	}{
		{"semana", KindWeek, weekStart, weekEnd, "2026-10-19", "2026-10-25"},
		{"mes calendario anterior", KindMonth, monthStart, monthEnd, "2025-12-01", "2025-12-31"},
		{"bloque de siete dias", KindBlock, blockStart, blockEnd, "2026-12-28", "2027-01-03"},
		{"bloque de tres dias", KindBlock, blockStart, blockStart.AddDate(0, 0, 2), "2027-01-01", "2027-01-03"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := PreviousPeriod(tc.kind, tc.start, tc.end)
			require.Equal(t, tc.wantStart, start.Format(time.DateOnly))
			require.Equal(t, tc.wantEnd, end.Format(time.DateOnly))
		})
	}
}

// TestPeriodsAcrossSkippedMidnight usa Azores, donde el 2026-03-29 las 00:00 no
// existen: el cálculo civil no puede perder un día al medir el rango.
func TestPeriodsAcrossSkippedMidnight(t *testing.T) {
	azores := zone(t, "Atlantic/Azores")
	start, end := WeekPeriod(at(t, "2026-03-25 12:00", azores), azores)
	require.Equal(t, "2026-03-23", start.Format(time.DateOnly))
	require.Equal(t, "2026-03-29", end.Format(time.DateOnly))
	require.Equal(t, "2026-03-29T01:00:00Z", end.UTC().Format(time.RFC3339))

	prevStart, prevEnd := PreviousPeriod(KindWeek, start, end)
	require.Equal(t, "2026-03-16", prevStart.Format(time.DateOnly))
	require.Equal(t, "2026-03-22", prevEnd.Format(time.DateOnly))

	monthStart, monthEnd := MonthPeriod(at(t, "2026-03-25 12:00", azores), azores)
	require.Equal(t, "2026-03-01", monthStart.Format(time.DateOnly))
	require.Equal(t, "2026-03-31", monthEnd.Format(time.DateOnly))
}
