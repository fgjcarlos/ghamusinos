//go:build integration

// Integration test del feed de actividades del calendario (#313, T3):
// scoping por usuario real y bandera linked aunque la sesion vinculada
// caiga fuera del rango visible. Necesita las migraciones aplicadas:
//
//	TEST_DATABASE_URL=... go test -tags integration -race ./internal/workouts/...
package workouts_test

import (
	"context"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
	"github.com/fgjcarlos/ghamusinos/internal/workouts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestIntegration_CalendarActivities_ScopedLinkFlag comprueba contra
// PostgreSQL las dos fronteras del feed: solo se devuelven las
// actividades del usuario, y linked es cierto solo si la sesion
// vinculada tambien es del usuario (el EXISTS cualifica a los dos
// duenos), incluso cuando esa sesion esta fuera del rango pedido.
func TestIntegration_CalendarActivities_ScopedLinkFlag(t *testing.T) {
	pool := openCompareDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	newUser := func(name string) sqlc.User {
		t.Helper()
		u, err := q.CreateUser(ctx, sqlc.CreateUserParams{
			ClerkUserID:  "clerk_calendar_" + name + "_" + uuid.NewString(),
			Email:        "calendar-" + name + "-" + uuid.NewString() + "@example.com",
			InviteStatus: status.InviteStatusActive,
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", u.ID) })
		return u
	}

	owner := newUser("owner")
	other := newUser("other")
	if _, err := q.UpdateUserPreferences(ctx, sqlc.UpdateUserPreferencesParams{ID: owner.ID, Timezone: "Europe/Madrid"}); err != nil {
		t.Fatalf("UpdateUserPreferences: %v", err)
	}

	activity := func(userID pgtype.UUID, externalID int64, startedAt time.Time, name string) sqlc.Activity {
		t.Helper()
		a, err := q.UpsertActivity(ctx, sqlc.UpsertActivityParams{
			UserID:         userID,
			ExternalSource: "strava",
			ExternalID:     externalID,
			Name:           name,
			SportType:      "Run",
			StartedAt:      pgtype.Timestamptz{Time: startedAt, Valid: true},
			ElapsedSeconds: 1800,
			MovingSeconds:  1800,
			RawPayload:     []byte(`{}`),
		})
		if err != nil {
			t.Fatalf("UpsertActivity: %v", err)
		}
		return a
	}

	// 23:30Z del 25: en Madrid es el dia 26 local.
	linkedOutsideRange := activity(owner.ID, 88001, time.Date(2026, 11, 25, 23, 30, 0, 0, time.UTC), "Vinculada lejos")
	freeInRange := activity(owner.ID, 88002, time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC), "Libre")
	otherUserActivity := activity(other.ID, 88003, time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC), "De otro")

	// Sesion del owner FUERA del rango visible (diciembre): la bandera
	// linked es global, no del rango.
	outside, err := q.CreatePlannedSession(ctx, sqlc.CreatePlannedSessionParams{
		UserID:      owner.ID,
		ScheduledOn: pgtype.Date{Time: time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC), Valid: true},
		Title:       "Lejana",
		WorkoutType: "rodaje",
	})
	if err != nil {
		t.Fatalf("CreatePlannedSession: %v", err)
	}
	if _, err := q.LinkPlannedSessionActivity(ctx, sqlc.LinkPlannedSessionActivityParams{ID: outside.ID, UserID: owner.ID, ActivityID: linkedOutsideRange.ID}); err != nil {
		t.Fatalf("LinkPlannedSessionActivity: %v", err)
	}

	// Enlace cruzado forzado (sin pasar por la API, que lo impide): una
	// sesion de OTRO usuario apuntando a una actividad del owner. El
	// EXISTS cualifica a los dos duenos: para el owner no es linked.
	foreign, err := q.CreatePlannedSession(ctx, sqlc.CreatePlannedSessionParams{
		UserID:      other.ID,
		ScheduledOn: pgtype.Date{Time: time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC), Valid: true},
		Title:       "Ajena",
		WorkoutType: "rodaje",
	})
	if err != nil {
		t.Fatalf("CreatePlannedSession: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE planned_sessions SET activity_id = $1 WHERE id = $2", freeInRange.ID, foreign.ID); err != nil {
		t.Fatalf("enlace cruzado: %v", err)
	}

	store := workouts.NewStore(q)
	ownerID := uuid.UUID(owner.ID.Bytes)
	got, err := store.CalendarActivities(ctx, ownerID,
		time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("CalendarActivities: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("CalendarActivities devolvio %d actividades, want 2 (solo las del owner): %+v", len(got), got)
	}
	byID := map[uuid.UUID]workouts.CalendarActivity{}
	for _, a := range got {
		byID[a.ID] = a
	}
	vinculada := byID[uuid.UUID(linkedOutsideRange.ID.Bytes)]
	if !vinculada.Linked {
		t.Fatalf("la actividad con sesion fuera de rango debe ser linked")
	}
	if vinculada.ScheduledOn != "2026-11-26" {
		t.Fatalf("scheduled_on = %q, want 2026-11-26 (23:30Z del 25 es el 26 en Madrid)", vinculada.ScheduledOn)
	}
	if vinculada.Name != "Vinculada lejos" {
		t.Fatalf("name = %q, want Vinculada lejos", vinculada.Name)
	}
	libre := byID[uuid.UUID(freeInRange.ID.Bytes)]
	if libre.Linked {
		t.Fatalf("una sesion de otro usuario no marca linked para el owner (EXISTS cualifica ambos duenos)")
	}
	if _, ok := byID[uuid.UUID(otherUserActivity.ID.Bytes)]; ok {
		t.Fatalf("se devolvio una actividad de otro usuario")
	}
}
