//go:build integration

// Integration tests del estado y la vinculación de planned_sessions (fase
// 2.2, issue #311). Necesitan un PostgreSQL con las migraciones aplicadas
// (incluida 00017_planned_sessions_status_activity.sql) y se ejecutan con:
//
//	go test -tags integration -race ./internal/db/sqlc/...
//
// La mitad estática del mismo contrato vive en
// planned_sessions_contract_test.go y siempre corre.
package sqlc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// createPlannedSessionsActivity inserta una actividad Strava del usuario y
// devuelve la fila persistida.
func createPlannedSessionsActivity(t *testing.T, q *sqlc.Queries, userID pgtype.UUID, externalID int64, name string) sqlc.Activity {
	t.Helper()
	activity, err := q.UpsertActivity(context.Background(), sqlc.UpsertActivityParams{
		UserID:         userID,
		ExternalSource: "strava",
		ExternalID:     externalID,
		Name:           name,
		SportType:      "Run",
		StartedAt:      pgtype.Timestamptz{Time: time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC), Valid: true},
		ElapsedSeconds: 3600,
		MovingSeconds:  3500,
		RawPayload:     []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("UpsertActivity: %v", err)
	}
	return activity
}

// createSessionFor inserta una sesion planificada del usuario.
func createSessionFor(t *testing.T, q *sqlc.Queries, userID pgtype.UUID, title string) sqlc.PlannedSession {
	t.Helper()
	session, err := q.CreatePlannedSession(context.Background(), sqlc.CreatePlannedSessionParams{
		UserID:      userID,
		ScheduledOn: pgtype.Date{Time: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Valid: true},
		Title:       title,
		WorkoutType: "rodaje",
	})
	if err != nil {
		t.Fatalf("CreatePlannedSession: %v", err)
	}
	return session
}

// TestIntegration_PlannedSessions_LinkSetsCompletedAndKeepsModified proves
// vincular una actividad propia completa la sesion, salvo que ya este
// marcada como modified.
func TestIntegration_PlannedSessions_LinkSetsCompletedAndKeepsModified(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createPlannedSessionsUser(t, pool, q, "link_status")
	activity := createPlannedSessionsActivity(t, q, user.ID, 910001, "Rodaje matinal")

	planned := createSessionFor(t, q, user.ID, "Sesion planificada")
	linked, err := q.LinkPlannedSessionActivity(ctx, sqlc.LinkPlannedSessionActivityParams{
		ID: planned.ID, UserID: user.ID, ActivityID: activity.ID,
	})
	if err != nil {
		t.Fatalf("LinkPlannedSessionActivity: %v", err)
	}
	if linked.Status != "completed" {
		t.Errorf("status = %q, want %q after linking a planned session", linked.Status, "completed")
	}
	if !linked.ActivityID.Valid || linked.ActivityID.Bytes != activity.ID.Bytes {
		t.Errorf("activity_id no quedo enlazada a la actividad creada")
	}

	// La sesion marcada modified conserva su estado al vincular.
	modified := createSessionFor(t, q, user.ID, "Sesion modificada")
	if _, err := q.UpdatePlannedSession(ctx, sqlc.UpdatePlannedSessionParams{
		ID:          modified.ID,
		UserID:      user.ID,
		ScheduledOn: modified.ScheduledOn,
		Title:       "Sesion modificada",
		WorkoutType: "rodaje",
		Status:      "modified",
	}); err != nil {
		t.Fatalf("UpdatePlannedSession: %v", err)
	}
	second := createPlannedSessionsActivity(t, q, user.ID, 910002, "Tirada larga real")
	linkedModified, err := q.LinkPlannedSessionActivity(ctx, sqlc.LinkPlannedSessionActivityParams{
		ID: modified.ID, UserID: user.ID, ActivityID: second.ID,
	})
	if err != nil {
		t.Fatalf("LinkPlannedSessionActivity (modified): %v", err)
	}
	if linkedModified.Status != "modified" {
		t.Errorf("status = %q, want %q: vincular debe conservar modified", linkedModified.Status, "modified")
	}
}

// TestIntegration_PlannedSessions_LinkRejectsForeignOrMissingActivity
// proves the atomic ownership check: una sesion o actividad ajena, o una
// actividad inexistente, no producen ninguna fila (el 404 del handler).
func TestIntegration_PlannedSessions_LinkRejectsForeignOrMissingActivity(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	userA := createPlannedSessionsUser(t, pool, q, "link_owner")
	userB := createPlannedSessionsUser(t, pool, q, "link_intruder")

	sessionA := createSessionFor(t, q, userA.ID, "Sesion de A")
	activityB := createPlannedSessionsActivity(t, q, userB.ID, 910003, "Actividad de B")

	cases := []struct {
		name   string
		params sqlc.LinkPlannedSessionActivityParams
	}{
		{
			name:   "actividad de otro usuario",
			params: sqlc.LinkPlannedSessionActivityParams{ID: sessionA.ID, UserID: userA.ID, ActivityID: activityB.ID},
		},
		{
			name:   "actividad inexistente",
			params: sqlc.LinkPlannedSessionActivityParams{ID: sessionA.ID, UserID: userA.ID, ActivityID: pgtypeUUID(t, uuid.NewString())},
		},
		{
			name:   "sesion de otro usuario",
			params: sqlc.LinkPlannedSessionActivityParams{ID: sessionA.ID, UserID: userB.ID, ActivityID: activityB.ID},
		},
		{
			name:   "sesion inexistente",
			params: sqlc.LinkPlannedSessionActivityParams{ID: pgtypeUUID(t, uuid.NewString()), UserID: userA.ID, ActivityID: activityB.ID},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := q.LinkPlannedSessionActivity(ctx, tc.params); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("LinkPlannedSessionActivity() error = %v, want pgx.ErrNoRows", err)
			}
		})
	}

	// Ningun enlace quedo escrito.
	after, err := q.GetPlannedSession(ctx, sqlc.GetPlannedSessionParams{ID: sessionA.ID, UserID: userA.ID})
	if err != nil {
		t.Fatalf("GetPlannedSession: %v", err)
	}
	if after.ActivityID.Valid {
		t.Error("un intento rechazado dejo un enlace escrito")
	}
}

// TestIntegration_PlannedSessions_LinkDuplicateRejected proves the UNIQUE
// constraint: una actividad no puede enlazarse a dos sesiones.
func TestIntegration_PlannedSessions_LinkDuplicateRejected(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createPlannedSessionsUser(t, pool, q, "link_dup")
	activity := createPlannedSessionsActivity(t, q, user.ID, 910004, "Unica actividad")
	first := createSessionFor(t, q, user.ID, "Primera sesion")
	second := createSessionFor(t, q, user.ID, "Segunda sesion")

	if _, err := q.LinkPlannedSessionActivity(ctx, sqlc.LinkPlannedSessionActivityParams{
		ID: first.ID, UserID: user.ID, ActivityID: activity.ID,
	}); err != nil {
		t.Fatalf("LinkPlannedSessionActivity (first): %v", err)
	}

	_, err := q.LinkPlannedSessionActivity(ctx, sqlc.LinkPlannedSessionActivityParams{
		ID: second.ID, UserID: user.ID, ActivityID: activity.ID,
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("duplicate link error = %v, want 23505 unique violation", err)
	}

	// El enlace original sigue intacto y la segunda sesion sigue libre.
	kept, err := q.GetPlannedSession(ctx, sqlc.GetPlannedSessionParams{ID: first.ID, UserID: user.ID})
	if err != nil {
		t.Fatalf("GetPlannedSession (first): %v", err)
	}
	if !kept.ActivityID.Valid || kept.ActivityID.Bytes != activity.ID.Bytes {
		t.Error("el enlace original no se conservo tras el duplicado")
	}
	free, err := q.GetPlannedSession(ctx, sqlc.GetPlannedSessionParams{ID: second.ID, UserID: user.ID})
	if err != nil {
		t.Fatalf("GetPlannedSession (second): %v", err)
	}
	if free.ActivityID.Valid {
		t.Error("la segunda sesion quedo enlazada pese al rechazo")
	}
}

// TestIntegration_PlannedSessions_UnlinkPreservesStatusAndActivity proves
// desvincular deja activity_id a NULL, no cambia el estado y no borra la
// actividad.
func TestIntegration_PlannedSessions_UnlinkPreservesStatusAndActivity(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createPlannedSessionsUser(t, pool, q, "unlink")
	activity := createPlannedSessionsActivity(t, q, user.ID, 910005, "Rodaje real")
	session := createSessionFor(t, q, user.ID, "Sesion a desvincular")

	if _, err := q.LinkPlannedSessionActivity(ctx, sqlc.LinkPlannedSessionActivityParams{
		ID: session.ID, UserID: user.ID, ActivityID: activity.ID,
	}); err != nil {
		t.Fatalf("LinkPlannedSessionActivity: %v", err)
	}

	unlinked, err := q.UnlinkPlannedSessionActivity(ctx, sqlc.UnlinkPlannedSessionActivityParams{
		ID: session.ID, UserID: user.ID,
	})
	if err != nil {
		t.Fatalf("UnlinkPlannedSessionActivity: %v", err)
	}
	if unlinked.ActivityID.Valid {
		t.Error("activity_id no quedo a NULL tras desvincular")
	}
	if unlinked.Status != "completed" {
		t.Errorf("status = %q, want %q: desvincular no cambia el estado", unlinked.Status, "completed")
	}

	// La actividad sigue existiendo y pertenece al usuario.
	still, err := q.GetActivityByExternalID(ctx, sqlc.GetActivityByExternalIDParams{
		UserID: user.ID, ExternalSource: "strava", ExternalID: 910005,
	})
	if err != nil {
		t.Fatalf("GetActivityByExternalID: %v (la actividad no debe borrarse al desvincular)", err)
	}
	if still.ID.Bytes != activity.ID.Bytes {
		t.Error("la actividad cambio de identidad tras desvincular")
	}

	// Desvincular una sesion ajena o inexistente no toca nada.
	if _, err := q.UnlinkPlannedSessionActivity(ctx, sqlc.UnlinkPlannedSessionActivityParams{
		ID: session.ID, UserID: pgtypeUUID(t, uuid.NewString()),
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("UnlinkPlannedSessionActivity() for another user = %v, want pgx.ErrNoRows", err)
	}
}
