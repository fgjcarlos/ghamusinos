package races

import (
	"context"
	"math"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/metrics"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const progressDays = 28

type SessionCounts struct {
	Total     int `json:"total"`
	Planned   int `json:"planned"`
	Completed int `json:"completed"`
	Skipped   int `json:"skipped"`
	Modified  int `json:"modified"`
}

type LoadPoint struct {
	Day string  `json:"day"`
	CTL float64 `json:"ctl"`
	ATL float64 `json:"atl"`
	TSB float64 `json:"tsb"`
}

type Progress struct {
	Orientative          bool          `json:"orientative"`
	AsOf                 string        `json:"as_of"`
	WindowStart          string        `json:"window_start"`
	DaysRemaining        int           `json:"days_remaining"`
	WeeksRemaining       int           `json:"weeks_remaining"`
	Past                 bool          `json:"past"`
	Sessions             SessionCounts `json:"sessions"`
	ActivityCount        int           `json:"activity_count"`
	DistanceM            float64       `json:"distance_m"`
	ElevationGainM       float64       `json:"elevation_gain_m"`
	TargetDistanceM      int32         `json:"target_distance_m"`
	TargetElevationGainM int32         `json:"target_elevation_gain_m"`
	Load                 []LoadPoint   `json:"load"`
}

func (s *Store) Progress(ctx context.Context, user, id uuid.UUID, now time.Time) (Progress, error) {
	goal, err := s.get(ctx, user, id)
	if err != nil {
		return Progress{}, err
	}
	uid := dbID(user)
	prefs, err := s.q.GetUserPreferencesByID(ctx, uid)
	if err != nil {
		return Progress{}, err
	}
	loc := workouts.UserLocation(prefs.Timezone)
	today := workouts.LocalDate(now, loc)
	start := today.AddDate(0, 0, 1-progressDays)
	date, err := time.Parse(time.DateOnly, goal.RaceDate)
	if err != nil {
		return Progress{}, err
	}
	days := int((date.Unix() - today.Unix()) / 86400)
	p := Progress{Orientative: true, AsOf: today.Format(time.DateOnly), WindowStart: start.Format(time.DateOnly),
		DaysRemaining: max(0, days), WeeksRemaining: max(0, days/7), Past: days < 0,
		TargetDistanceM: goal.DistanceM, TargetElevationGainM: goal.ElevationGainM, Load: []LoadPoint{}}

	sessions, err := s.q.ListRaceSessions(ctx, sqlc.ListRaceSessionsParams{UserID: uid, RaceID: dbID(id)})
	if err != nil {
		return Progress{}, err
	}
	p.Sessions.Total = len(sessions)
	for _, row := range sessions {
		switch workouts.Status(row.Status) {
		case workouts.StatusPlanned:
			p.Sessions.Planned++
		case workouts.StatusCompleted:
			p.Sessions.Completed++
		case workouts.StatusSkipped:
			p.Sessions.Skipped++
		case workouts.StatusModified:
			p.Sessions.Modified++
		}
	}

	// The UTC query starts a day early so the exact civil-date filter also works across DST.
	activity, err := s.q.ListActivitiesInRange(ctx, sqlc.ListActivitiesInRangeParams{UserID: uid,
		StartedAt:   pgtype.Timestamptz{Time: start.AddDate(0, 0, -1), Valid: true},
		StartedAt_2: pgtype.Timestamptz{Time: now, Valid: true}})
	if err != nil {
		return Progress{}, err
	}
	for _, row := range activity {
		if workouts.LocalDate(row.StartedAt.Time, loc).Before(start) {
			continue
		}
		p.ActivityCount++
		p.DistanceM += finiteNumeric(row.DistanceMeters)
		p.ElevationGainM += finiteNumeric(row.ElevationGainM)
	}

	rows, err := s.q.ListTrainingLoadRange(ctx, sqlc.ListTrainingLoadRangeParams{UserID: uid,
		Day: pgtype.Date{Time: start, Valid: true}, Day_2: pgtype.Date{Time: today, Valid: true}})
	if err != nil {
		return Progress{}, err
	}
	if len(rows) > 0 {
		raw := make([]metrics.DailyLoad, 0, len(rows))
		for _, row := range rows {
			raw = append(raw, metrics.DailyLoad{Day: row.Day.Time, TSS: row.Tss})
		}
		filled := metrics.FillMissingDays(start, today, raw)
		for i, day := range filled {
			// The fixed 28-day window is small; reuse the same EMA as the dashboard.
			ctl, atl := metrics.CTL(filled[:i+1]), metrics.ATL(filled[:i+1])
			p.Load = append(p.Load, LoadPoint{Day: day.Day.Format(time.DateOnly), CTL: ctl, ATL: atl, TSB: metrics.TSB(ctl, atl)})
		}
	}
	return p, nil
}

func finiteNumeric(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid || math.IsNaN(f.Float64) || math.IsInf(f.Float64, 0) {
		return 0
	}
	return f.Float64
}
