//go:build integration

// Integration tests for the planned_sessions storage (fase 2.1, issue
// #298).
//
// They need a PostgreSQL with the migrations applied (including
// 00016_planned_sessions.sql). They run with:
//
//	go test -tags integration -race ./internal/db/sqlc/...
//
// The static half of the same contract lives in
// planned_sessions_contract_test.go and always runs.
package sqlc_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// quoteLiteral quotes a SQL string literal.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// createPlannedSessionsUser creates a user for the test and removes it
// (and its sessions, by cascade) on cleanup.
func createPlannedSessionsUser(t *testing.T, pool *pgxpool.Pool, q *sqlc.Queries, suffix string) sqlc.User {
	t.Helper()
	ctx := context.Background()
	clerkID := "clerk_planned_sessions_" + t.Name() + "_" + suffix

	created, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ClerkUserID:  clerkID,
		Email:        "planned-sessions-" + suffix + "@example.com",
		DisplayName:  pgtype.Text{String: "Planned Sessions Test", Valid: true},
		InviteStatus: status.InviteStatusActive,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE clerk_user_id = $1", clerkID)
	})
	return created
}

// TestIntegration_PlannedSessions_RejectsInvalidValues proves the database
// is the last line of defence: un objetivo negativo o un tipo desconocido
// no se guardan.
func TestIntegration_PlannedSessions_RejectsInvalidValues(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createPlannedSessionsUser(t, pool, q, "invalid")
	userID := user.ID
	day := pgtype.Date{Time: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Valid: true}
	negative := pgtype.Int4{Int32: -1, Valid: true}

	tests := []struct {
		name   string
		params sqlc.CreatePlannedSessionParams
	}{
		{
			name: "duracion negativa",
			params: sqlc.CreatePlannedSessionParams{
				UserID:            userID,
				ScheduledOn:       day,
				Title:             "Rodaje",
				WorkoutType:       "rodaje",
				TargetDurationSec: negative,
			},
		},
		{
			name: "distancia negativa",
			params: sqlc.CreatePlannedSessionParams{
				UserID:          userID,
				ScheduledOn:     day,
				Title:           "Rodaje",
				WorkoutType:     "rodaje",
				TargetDistanceM: negative,
			},
		},
		{
			name: "desnivel negativo",
			params: sqlc.CreatePlannedSessionParams{
				UserID:               userID,
				ScheduledOn:          day,
				Title:                "Rodaje",
				WorkoutType:          "rodaje",
				TargetElevationGainM: negative,
			},
		},
		{
			name: "tipo de entrenamiento desconocido",
			params: sqlc.CreatePlannedSessionParams{
				UserID:      userID,
				ScheduledOn: day,
				Title:       "Sesion",
				WorkoutType: "nope",
			},
		},
		{
			name: "intensidad desconocida",
			params: sqlc.CreatePlannedSessionParams{
				UserID:          userID,
				ScheduledOn:     day,
				Title:           "Sesion",
				WorkoutType:     "rodaje",
				TargetIntensity: pgtype.Text{String: "z9", Valid: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := q.CreatePlannedSession(ctx, tt.params); err == nil {
				t.Fatal("CreatePlannedSession() expected the database to reject the row, got nil error")
			}
		})
	}
}

// TestIntegration_PlannedSessions_OtherUserCannotTouchSession proves the
// ownership filter: el usuario B no puede leer, editar ni borrar las
// sesiones del usuario A.
func TestIntegration_PlannedSessions_OtherUserCannotTouchSession(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	userA := createPlannedSessionsUser(t, pool, q, "owner")
	userB := createPlannedSessionsUser(t, pool, q, "intruder")

	created, err := q.CreatePlannedSession(ctx, sqlc.CreatePlannedSessionParams{
		UserID:      userA.ID,
		ScheduledOn: pgtype.Date{Time: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Valid: true},
		Title:       "Tirada larga",
		WorkoutType: "tirada_larga",
	})
	if err != nil {
		t.Fatalf("CreatePlannedSession: %v", err)
	}

	// B no puede leer la sesion de A.
	if _, err := q.GetPlannedSession(ctx, sqlc.GetPlannedSessionParams{
		ID:     created.ID,
		UserID: userB.ID,
	}); err == nil {
		t.Fatal("GetPlannedSession() for another user expected an error, got nil")
	}

	// B no puede editar la sesion de A.
	if _, err := q.UpdatePlannedSession(ctx, sqlc.UpdatePlannedSessionParams{
		ID:          created.ID,
		UserID:      userB.ID,
		ScheduledOn: created.ScheduledOn,
		Title:       "Secuestrada",
		WorkoutType: "rodaje",
	}); err == nil {
		t.Fatal("UpdatePlannedSession() for another user expected an error, got nil")
	}

	// B no puede borrar la sesion de A.
	rows, err := q.DeletePlannedSession(ctx, sqlc.DeletePlannedSessionParams{
		ID:     created.ID,
		UserID: userB.ID,
	})
	if err != nil {
		t.Fatalf("DeletePlannedSession: %v", err)
	}
	if rows != 0 {
		t.Errorf("DeletePlannedSession() for another user deleted %d rows, want 0", rows)
	}

	// La sesion de A sigue intacta.
	after, err := q.GetPlannedSession(ctx, sqlc.GetPlannedSessionParams{
		ID:     created.ID,
		UserID: userA.ID,
	})
	if err != nil {
		t.Fatalf("GetPlannedSession() for the owner: %v", err)
	}
	if after.Title != "Tirada larga" {
		t.Errorf("title = %q, want %q: la sesion cambio pese al intento de otro usuario", after.Title, "Tirada larga")
	}
}

// TestIntegration_PlannedSessions_ListRangeIsScopedToUser proves el
// listado solo devuelve las sesiones del usuario.
func TestIntegration_PlannedSessions_ListRangeIsScopedToUser(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	userA := createPlannedSessionsUser(t, pool, q, "list_owner")
	userB := createPlannedSessionsUser(t, pool, q, "list_other")

	day := pgtype.Date{Time: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Valid: true}
	if _, err := q.CreatePlannedSession(ctx, sqlc.CreatePlannedSessionParams{
		UserID:      userA.ID,
		ScheduledOn: day,
		Title:       "Sesion de A",
		WorkoutType: "rodaje",
	}); err != nil {
		t.Fatalf("CreatePlannedSession: %v", err)
	}

	rows, err := q.ListPlannedSessionsByRange(ctx, sqlc.ListPlannedSessionsByRangeParams{
		UserID:   userB.ID,
		FromDate: day,
		ToDate:   day,
		Limit:    50,
		Offset:   0,
	})
	if err != nil {
		t.Fatalf("ListPlannedSessionsByRange: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("user B sees %d sessions of user A, want 0", len(rows))
	}
}

// TestIntegration_PlannedSessions_KeepsTheCalendarDay proves the stored
// day is a calendar day: el mismo registro se lee igual bajo cualquier
// zona horaria de sesion.
func TestIntegration_PlannedSessions_KeepsTheCalendarDay(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createPlannedSessionsUser(t, pool, q, "calendar_day")
	day := pgtype.Date{Time: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Valid: true}

	created, err := q.CreatePlannedSession(ctx, sqlc.CreatePlannedSessionParams{
		UserID:      user.ID,
		ScheduledOn: day,
		Title:       "Tirada larga",
		WorkoutType: "tirada_larga",
	})
	if err != nil {
		t.Fatalf("CreatePlannedSession: %v", err)
	}

	for _, tz := range []string{"UTC", "Europe/Madrid", "Pacific/Kiritimati"} {
		if _, err := pool.Exec(ctx, "SET TIME ZONE "+quoteLiteral(tz)); err != nil {
			t.Fatalf("SET TIME ZONE %s: %v", tz, err)
		}
		read, err := q.GetPlannedSession(ctx, sqlc.GetPlannedSessionParams{
			ID:     created.ID,
			UserID: user.ID,
		})
		if err != nil {
			t.Fatalf("GetPlannedSession() under %s: %v", tz, err)
		}
		got := read.ScheduledOn.Time.Format("2006-01-02")
		if got != "2026-11-20" {
			t.Errorf("under %s the session reads as %s, want 2026-11-20", tz, got)
		}
	}
}

// TestIntegration_PlannedSessions_CascadeDelete proves que al borrar el
// usuario caen sus sesiones (ON DELETE CASCADE).
func TestIntegration_PlannedSessions_CascadeDelete(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createPlannedSessionsUser(t, pool, q, "cascade")
	day := pgtype.Date{Time: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Valid: true}

	if _, err := q.CreatePlannedSession(ctx, sqlc.CreatePlannedSessionParams{
		UserID:      user.ID,
		ScheduledOn: day,
		Title:       "Sesion",
		WorkoutType: "rodaje",
	}); err != nil {
		t.Fatalf("CreatePlannedSession: %v", err)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM planned_sessions WHERE user_id = $1", user.ID).Scan(&count); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if count != 0 {
		t.Errorf("planned_sessions rows = %d, want 0 after the owner was deleted", count)
	}
	_ = uuid.Nil
}
