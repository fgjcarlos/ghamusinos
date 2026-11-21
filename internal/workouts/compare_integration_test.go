//go:build integration

// Integration test end-to-end del endpoint de comparacion (#312, T2):
// sesiones + actividades reales + zona horaria guardada contra
// PostgreSQL. Necesita las migraciones aplicadas y se ejecuta con:
//
//	TEST_DATABASE_URL=... go test -tags integration -race ./internal/workouts/...
package workouts_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// openCompareDB abre un pool contra TEST_DATABASE_URL (o DATABASE_URL),
// igual que el helper de internal/db/sqlc.
func openCompareDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL (o DATABASE_URL) no definida; saltando integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestIntegration_Compare_BucketsWeeksInUserZoneOverDST reproduce el
// limite de semana en Europe/Madrid cruzando el fin del horario de
// verano (2026-10-25): 22:30Z sigue en la semana del 19; 23:30Z ya es
// lunes 26 local. La actividad vinculada cuenta igual que la libre.
func TestIntegration_Compare_BucketsWeeksInUserZoneOverDST(t *testing.T) {
	pool := openCompareDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ClerkUserID:  "clerk_compare_" + uuid.NewString(),
		Email:        "compare-" + uuid.NewString() + "@example.com",
		InviteStatus: status.InviteStatusActive,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	userID := user.ID
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	})
	if _, err := q.UpdateUserPreferences(ctx, sqlc.UpdateUserPreferencesParams{
		ID:       userID,
		Timezone: "Europe/Madrid",
	}); err != nil {
		t.Fatalf("UpdateUserPreferences: %v", err)
	}

	monday := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	session := func(day time.Time, duration, distance, gain int32) sqlc.PlannedSession {
		t.Helper()
		created, cErr := q.CreatePlannedSession(ctx, sqlc.CreatePlannedSessionParams{
			UserID:               userID,
			ScheduledOn:          pgtype.Date{Time: day, Valid: true},
			Title:                "Sesion",
			WorkoutType:          "rodaje",
			TargetDurationSec:    pgtype.Int4{Int32: duration, Valid: true},
			TargetDistanceM:      pgtype.Int4{Int32: distance, Valid: true},
			TargetElevationGainM: pgtype.Int4{Int32: gain, Valid: true},
		})
		if cErr != nil {
			t.Fatalf("CreatePlannedSession: %v", cErr)
		}
		return created
	}

	completed := session(monday, 3600, 10000, 300)
	skipped := session(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), 5400, 20000, 500)
	if _, err := pool.Exec(ctx, "UPDATE planned_sessions SET status = 'skipped' WHERE id = $1", skipped.ID); err != nil {
		t.Fatalf("marcar skipped: %v", err)
	}

	activity := func(externalID int64, startedAt time.Time, moving int32, distance, gain string) sqlc.Activity {
		t.Helper()
		d := pgtype.Numeric{}
		g := pgtype.Numeric{}
		if err := d.Scan(distance); err != nil {
			t.Fatalf("Numeric.Scan(%q): %v", distance, err)
		}
		if err := g.Scan(gain); err != nil {
			t.Fatalf("Numeric.Scan(%q): %v", gain, err)
		}
		created, aErr := q.UpsertActivity(ctx, sqlc.UpsertActivityParams{
			UserID:         userID,
			ExternalSource: "strava",
			ExternalID:     externalID,
			Name:           "Actividad",
			SportType:      "Run",
			StartedAt:      pgtype.Timestamptz{Time: startedAt, Valid: true},
			ElapsedSeconds: moving,
			MovingSeconds:  moving,
			DistanceMeters: d,
			ElevationGainM: g,
			RawPayload:     []byte(`{}`),
		})
		if aErr != nil {
			t.Fatalf("UpsertActivity: %v", aErr)
		}
		return created
	}

	activity(99001, time.Date(2026, 10, 25, 22, 30, 0, 0, time.UTC), 1800, "5000", "100")
	linked := activity(99002, time.Date(2026, 10, 25, 23, 30, 0, 0, time.UTC), 3600, "15000.5", "250.5")
	if _, err := q.LinkPlannedSessionActivity(ctx, sqlc.LinkPlannedSessionActivityParams{
		ID:         completed.ID,
		UserID:     userID,
		ActivityID: linked.ID,
	}); err != nil {
		t.Fatalf("LinkPlannedSessionActivity: %v", err)
	}

	store := workouts.NewStore(q)
	periods, err := store.Compare(ctx, uuid.UUID(userID.Bytes),
		time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		workouts.CompareGroupWeek)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	want := []workouts.ComparePeriod{
		{
			PeriodStart: "2026-10-19",
			Actual:      workouts.CompareTotals{DurationSeconds: 1800, DistanceM: 5000, ElevationGainM: 100},
		},
		{
			PeriodStart: "2026-10-26",
			Planned:     workouts.CompareTotals{DurationSeconds: 3600, DistanceM: 10000, ElevationGainM: 300},
			Actual:      workouts.CompareTotals{DurationSeconds: 3600, DistanceM: 15000.5, ElevationGainM: 250.5},
			Compliance:  workouts.CompareCompliance{Total: 2, Completed: 1, Skipped: 1},
		},
	}
	if len(periods) != len(want) {
		t.Fatalf("Compare() = %+v, want %d periods", periods, len(want))
	}
	for i := range want {
		if periods[i] != want[i] {
			t.Errorf("period[%d] = %+v, want %+v", i, periods[i], want[i])
		}
	}
}
