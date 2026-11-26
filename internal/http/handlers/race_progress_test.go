package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/races"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type progressQueries struct {
	sqlc.Querier
	owner      pgtype.UUID
	raceDate   pgtype.Date
	withData   bool
	reportRead bool
}

func (q *progressQueries) GetRace(_ context.Context, arg sqlc.GetRaceParams) (sqlc.Race, error) {
	if arg.UserID != q.owner {
		return sqlc.Race{}, pgx.ErrNoRows
	}
	return sqlc.Race{ID: arg.ID, RaceDate: q.raceDate, DistanceM: 42000, ElevationGainM: 1800}, nil
}

func (q *progressQueries) GetUserPreferencesByID(context.Context, pgtype.UUID) (sqlc.GetUserPreferencesByIDRow, error) {
	q.reportRead = true
	return sqlc.GetUserPreferencesByIDRow{Timezone: "Europe/Madrid"}, nil
}

func (q *progressQueries) ListRaceSessions(context.Context, sqlc.ListRaceSessionsParams) ([]sqlc.ListRaceSessionsRow, error) {
	if !q.withData {
		return nil, nil
	}
	return []sqlc.ListRaceSessionsRow{{Status: "planned"}, {Status: "completed"}, {Status: "skipped"}, {Status: "modified"}}, nil
}

func (q *progressQueries) ListActivitiesInRange(context.Context, sqlc.ListActivitiesInRangeParams) ([]sqlc.ListActivitiesInRangeRow, error) {
	if !q.withData {
		return nil, nil
	}
	rows := []sqlc.ListActivitiesInRangeRow{}
	for _, value := range []string{"2026-10-27T22:30:00Z", "2026-10-27T23:30:00Z"} {
		// In Madrid these are Oct 27 and 28 respectively; only the latter belongs to the report.
		started, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, err
		}
		var distance, elevation pgtype.Numeric
		if err := distance.Scan("12000"); err != nil {
			return nil, err
		}
		if err := elevation.Scan("250"); err != nil {
			return nil, err
		}
		rows = append(rows, sqlc.ListActivitiesInRangeRow{StartedAt: pgtype.Timestamptz{Time: started, Valid: true}, DistanceMeters: distance, ElevationGainM: elevation})
	}
	return rows, nil
}

func (q *progressQueries) ListTrainingLoadRange(_ context.Context, arg sqlc.ListTrainingLoadRangeParams) ([]sqlc.TrainingLoadDaily, error) {
	if !q.withData {
		return nil, nil
	}
	return []sqlc.TrainingLoadDaily{{Day: arg.Day, Tss: 40}, {Day: arg.Day_2, Tss: 20}}, nil
}

func TestRaceProgress(t *testing.T) {
	owner := uuid.MustParse(workoutTestUser)
	now := time.Date(2026, 11, 23, 23, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		race     time.Time
		user     uuid.UUID
		withData bool
		status   int
		days     int
		past     bool
	}{
		{"future", time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC), owner, true, 200, 8, false},
		{"past", time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), owner, true, 200, 0, true},
		{"empty", time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC), owner, false, 200, 0, false},
		{"foreign", time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC), uuid.New(), true, 404, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := &progressQueries{owner: pgtype.UUID{Bytes: owner, Valid: true}, raceDate: pgtype.Date{Time: tc.race, Valid: true}, withData: tc.withData}
			router := chi.NewRouter()
			router.Get("/api/v1/races/{id}/progress", raceProgressHandler(races.NewStore(q), func() time.Time { return now }))
			rec := httptest.NewRecorder()
			ctx := auth.WithAuthUser(context.Background(), &auth.User{ID: tc.user.String()})
			req := httptest.NewRequestWithContext(ctx, "GET", "/api/v1/races/"+workoutTestID+"/progress", nil)
			router.ServeHTTP(rec, req)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			if tc.status == 404 {
				require.JSONEq(t, `{"type":"about:blank","title":"Not Found","status":404,"detail":"race or track not found"}`, rec.Body.String())
				require.False(t, q.reportRead)
				return
			}
			var p races.Progress
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
			require.True(t, p.Orientative)
			require.Equal(t, "2026-10-28", p.WindowStart)
			require.Equal(t, "2026-11-24", p.AsOf)
			require.Equal(t, tc.days, p.DaysRemaining)
			require.Equal(t, tc.days/7, p.WeeksRemaining)
			require.Equal(t, tc.past, p.Past)
			require.Equal(t, int32(42000), p.TargetDistanceM)
			require.Equal(t, int32(1800), p.TargetElevationGainM)
			if !tc.withData {
				require.Zero(t, p.ActivityCount)
				require.Zero(t, p.DistanceM)
				require.Zero(t, p.ElevationGainM)
				require.Equal(t, races.SessionCounts{}, p.Sessions)
				require.Equal(t, []races.LoadPoint{}, p.Load)
				return
			}
			require.Equal(t, 1, p.ActivityCount)
			require.Equal(t, 12000.0, p.DistanceM)
			require.Equal(t, 250.0, p.ElevationGainM)
			require.Equal(t, races.SessionCounts{Total: 4, Planned: 1, Completed: 1, Skipped: 1, Modified: 1}, p.Sessions)
			require.Len(t, p.Load, 28)
			require.Equal(t, races.LoadPoint{Day: p.WindowStart, CTL: 40, ATL: 40}, p.Load[0])
			last := p.Load[27]
			require.Equal(t, p.AsOf, last.Day)
			require.InDelta(t, last.CTL-last.ATL, last.TSB, 1e-9)
		})
	}
}
