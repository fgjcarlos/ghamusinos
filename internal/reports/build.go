package reports

import (
	"errors"
	"math"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/metrics"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrInvalid = errors.New("invalid report input")

// Input is everything Build needs, already loaded and scoped to one user.
// Location is the user's zone, used for the civil dates of the period.
type Input struct {
	Now         time.Time
	Location    *time.Location
	Kind        Kind
	PeriodStart time.Time
	PeriodEnd   time.Time
	Block       *sqlc.TrainingBlock
	Race        *sqlc.Race
	Load        []sqlc.TrainingLoadDaily
	Activities  []sqlc.ListActivitiesInRangeRow
	Sessions    []sqlc.ListPlannedSessionsByRangeRow
}

// Build assembles a report from already loaded data. It is pure: no clock, no
// database; Now decides the days left to the race and Location the civil dates.
func Build(in Input) (Snapshot, error) {
	loc := in.Location
	if loc == nil {
		loc = time.UTC
	}
	start, end := workouts.LocalDate(in.PeriodStart, loc), workouts.LocalDate(in.PeriodEnd, loc)
	switch in.Kind {
	case KindWeek, KindMonth, KindBlock:
	default:
		return Snapshot{}, ErrInvalid
	}
	if end.Before(start) {
		return Snapshot{}, ErrInvalid
	}

	s := Snapshot{SchemaVersion: SchemaVersion, Kind: in.Kind,
		PeriodStart: start.Format(time.DateOnly), PeriodEnd: end.Format(time.DateOnly), Days: daySpan(start, end)}
	if in.Block != nil {
		s.Block = &BlockRef{ID: uuidOf(in.Block.ID), Name: in.Block.Name}
	}
	if in.Race != nil {
		raceDate, today := workouts.LocalDate(in.Race.RaceDate.Time, time.UTC), workouts.LocalDate(in.Now, loc)
		s.Race = &RaceRef{ID: uuidOf(in.Race.ID), Name: in.Race.Name, RaceDate: raceDate.Format(time.DateOnly),
			Priority: in.Race.Priority, DaysToRace: int(raceDate.Sub(today).Hours() / 24)}
	}

	series, counts := fillLoad(in.Load, start, end)
	s.Load = buildLoad(series, counts, start)
	var perf perfTotals
	s.Volume, perf = aggregate(in.Activities, start, end, loc)
	s.Performance = Performance{AvgEF: mean(perf.efSum, perf.efSamples), EFSamples: perf.efSamples,
		AvgGAPSecPerKm: mean(perf.gapSum, perf.gapSamples), GAPSamples: perf.gapSamples}
	prevStart, prevEnd := PreviousPeriod(in.Kind, start, end)
	prevVolume, prevPerf := aggregate(in.Activities, prevStart, prevEnd, loc)
	s.Previous = Previous{PeriodStart: prevStart.Format(time.DateOnly), PeriodEnd: prevEnd.Format(time.DateOnly),
		Activities: prevVolume.Activities, DistanceM: prevVolume.DistanceM, ElevationGainM: prevVolume.ElevationGainM}
	s.Previous.AvgEF, s.Previous.AvgGAPSecPerKm = prevPerf.averages()
	for _, day := range series {
		if !day.Day.Before(prevStart) && !day.Day.After(prevEnd) {
			s.Previous.TSS += day.TSS
		}
	}
	s.Previous.TSS = round2(s.Previous.TSS)
	s.Plan = plan(in.Sessions, start, end)
	return s, nil
}

// fillLoad returns the zero-filled daily series from the user's first recorded
// day (or the start of the period) to its last day, plus the activity count of
// each recorded day. Days are UTC, as the dashboard treats them, and later days
// are dropped so Now never leaks in.
func fillLoad(rows []sqlc.TrainingLoadDaily, start, end time.Time) ([]metrics.DailyLoad, map[time.Time]int) {
	raw := make([]metrics.DailyLoad, 0, len(rows))
	counts := make(map[time.Time]int, len(rows))
	origin := start
	for _, row := range rows {
		day := workouts.LocalDate(row.Day.Time, time.UTC)
		if day.After(end) {
			continue
		}
		if day.Before(origin) {
			origin = day
		}
		raw = append(raw, metrics.DailyLoad{Day: day, TSS: row.Tss})
		counts[day] = int(row.ActivityCount)
	}
	return metrics.FillMissingDays(origin, end, raw), counts
}

// buildLoad summarizes the daily series: total TSS of the period, the series
// itself and CTL/ATL/TSB at the close of the day before and of the last day.
func buildLoad(series []metrics.DailyLoad, counts map[time.Time]int, start time.Time) Load {
	// Everything before the first day of the period is the history the start
	// of the period is measured on.
	first := int(start.Sub(series[0].Day).Hours() / 24)
	ctlStart, atlStart := metrics.CTL(series[:first]), metrics.ATL(series[:first])
	ctlEnd, atlEnd := metrics.CTL(series), metrics.ATL(series)
	load := Load{Daily: make([]DailyLoad, 0, len(series)-first),
		CTLStart: round2(ctlStart), ATLStart: round2(atlStart), TSBStart: round2(metrics.TSB(ctlStart, atlStart)),
		CTLEnd: round2(ctlEnd), ATLEnd: round2(atlEnd), TSBEnd: round2(metrics.TSB(ctlEnd, atlEnd))}
	for _, day := range series[first:] {
		load.TSS += day.TSS
		load.Daily = append(load.Daily, DailyLoad{Date: day.Day.Format(time.DateOnly), TSS: round2(day.TSS),
			ActivityCount: counts[day.Day]})
	}
	load.TSS = round2(load.TSS)
	return load
}

// perfTotals accumulates the orientative performance samples of a period.
type perfTotals struct {
	efSum, gapSum         float64
	efSamples, gapSamples int
}

func (p perfTotals) averages() (*float64, *float64) {
	return mean(p.efSum, p.efSamples), mean(p.gapSum, p.gapSamples)
}

// aggregate sums the volume of the activities of the period and its performance
// samples. The period is filtered by the user's civil dates, so an activity
// near a boundary never falls in two reports.
func aggregate(activities []sqlc.ListActivitiesInRangeRow, start, end time.Time, loc *time.Location) (Volume, perfTotals) {
	volume := Volume{}
	var perf perfTotals
	for _, a := range activities {
		day := workouts.LocalDate(a.StartedAt.Time, loc)
		if day.Before(start) || day.After(end) {
			continue
		}
		distance, elevation := numericOrZero(a.DistanceMeters), numericOrZero(a.ElevationGainM)
		volume.Activities++
		volume.DistanceM += distance
		volume.ElevationGainM += elevation
		volume.MovingSeconds += int64(a.MovingSeconds)
		if metrics.IsCyclingSport(a.SportType) && a.AvgHr.Valid && a.AvgPower.Valid {
			perf.efSum += metrics.EfficiencyFactor(int(a.AvgPower.Int16), int(a.AvgHr.Int16))
			perf.efSamples++
		}
		if metrics.IsRunningSport(a.SportType) && distance > 0 && a.MovingSeconds > 0 {
			moving := float64(a.MovingSeconds)
			if gap := metrics.GradeAdjustedPace(elevation/distance, moving/(distance/1000)); gap > 0 {
				perf.gapSum += gap
				perf.gapSamples++
			}
			// EF running: speed in m/min over average heart rate.
			if a.AvgHr.Valid {
				perf.efSum += distance * 60 / moving / float64(a.AvgHr.Int16)
				perf.efSamples++
			}
		}
	}
	volume.DistanceM, volume.ElevationGainM = round2(volume.DistanceM), round2(volume.ElevationGainM)
	return volume, perf
}

// plan counts the planned sessions of the period by status. Adherence is the
// share of sessions completed or modified, null without sessions.
func plan(sessions []sqlc.ListPlannedSessionsByRangeRow, start, end time.Time) Plan {
	p := Plan{}
	for _, row := range sessions {
		day := workouts.LocalDate(row.ScheduledOn.Time, time.UTC)
		if day.Before(start) || day.After(end) {
			continue
		}
		switch workouts.Status(row.Status) {
		case workouts.StatusPlanned:
			p.Planned++
		case workouts.StatusCompleted:
			p.Completed++
		case workouts.StatusSkipped:
			p.Skipped++
		case workouts.StatusModified:
			p.Modified++
		}
	}
	if total := p.Planned + p.Completed + p.Skipped + p.Modified; total > 0 {
		p.Adherence = f(round2(float64(p.Completed+p.Modified) / float64(total)))
	}
	return p
}

func mean(sum float64, samples int) *float64 {
	if samples == 0 {
		return nil
	}
	return f(round2(sum / float64(samples)))
}

func f(v float64) *float64 { return &v }

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func uuidOf(id pgtype.UUID) uuid.UUID { return uuid.UUID(id.Bytes) }

// numericOrZero maps a nullable NUMERIC to float64; NULL or a non-finite value
// becomes 0.
func numericOrZero(n pgtype.Numeric) float64 {
	value, err := n.Float64Value()
	if err != nil || !value.Valid || math.IsNaN(value.Float64) || math.IsInf(value.Float64, 0) {
		return 0
	}
	return value.Float64
}
