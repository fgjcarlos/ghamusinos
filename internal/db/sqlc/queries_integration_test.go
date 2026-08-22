//go:build integration

// Integration tests para las queries SQLC de users + invites.
// Requieren una base de datos PostgreSQL accesible vía TEST_DATABASE_URL
// (o DATABASE_URL como fallback). El job de CI que ejecuta estos tests
// usa el servicio Postgres que ya está declarado en
// .github/workflows/ci.yml, gracias a #28 (TimescaleDB).
//
// Se ejecuta con: go test -tags integration -race ./internal/db/sqlc/...
package sqlc_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
)

// openTestDB abre un pool de pgx contra TEST_DATABASE_URL (o
// DATABASE_URL como fallback). t.Skip si la variable no está seteada:
// el CI de PR normales (sin tag integration) no debería fallar.
func openTestDB(t *testing.T) *pgxpool.Pool {
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

// pgtypeUUID parsea un UUID string estándar (8-4-4-4-12) al tipo
// pgtype.UUID que esperan las queries SQLC.
func pgtypeUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		t.Fatalf("pgtype.UUID.Scan(%q): %v", s, err)
	}
	return u
}

// TestIntegration_CreateUserAndGetByClerkID cubre el flujo mínimo de
// users: CreateUser → GetUserByClerkID round-trip. Cada test usa un
// clerk_user_id único (sufijo de t.Name() + timestamp) para no chocar
// con runs concurrentes.
func TestIntegration_CreateUserAndGetByClerkID(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	clerkID := "clerk_test_" + t.Name() + "_integration"

	created, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ClerkUserID:  clerkID,
		Email:        "test@example.com",
		DisplayName:  pgtype.Text{String: "Test User", Valid: true},
		InviteStatus: status.InviteStatusPending,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.ID.String() == "" {
		t.Fatalf("CreateUser devolvió ID vacío")
	}

	got, err := q.GetUserByClerkID(ctx, clerkID)
	if err != nil {
		t.Fatalf("GetUserByClerkID: %v", err)
	}
	if got.ClerkUserID != clerkID {
		t.Errorf("ClerkUserID = %q, quería %q", got.ClerkUserID, clerkID)
	}
	if got.Email != "test@example.com" {
		t.Errorf("Email = %q, quería test@example.com", got.Email)
	}
	if got.InviteStatus != status.InviteStatusPending {
		t.Errorf("InviteStatus = %v, quería pending", got.InviteStatus)
	}

	// Limpiar para no acumular rows en runs repetidos.
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE clerk_user_id = $1", clerkID)
	})
}

// TestIntegration_CreateInviteAndMarkAccepted cubre el flujo de
// invites: CreateInvite → MarkInviteAccepted y verifica que el status
// pasa de 'pending' a 'accepted' tras la aceptación.
func TestIntegration_CreateInviteAndMarkAccepted(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	tokenHash := "test-hash-" + t.Name()

	invite, err := q.CreateInvite(ctx, sqlc.CreateInviteParams{
		Email:     "invitee-" + t.Name() + "@example.com",
		TokenHash: tokenHash,
		Status:    status.StatusPending,
		// expires_at: NULL es válido (el CHECK sólo lo exige < created_at + 7d
		// cuando está presente; sin expires_at el CHECK pasa). pgtype.Timestamptz
		// con Valid=false envía NULL.
		ExpiresAt: pgtype.Timestamptz{Valid: false},
	})
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	if err := q.MarkInviteAccepted(ctx, pgtypeUUID(t, invite.ID.String())); err != nil {
		t.Fatalf("MarkInviteAccepted: %v", err)
	}

	got, err := q.GetInviteByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetInviteByTokenHash: %v", err)
	}
	if got.Status != status.StatusAccepted {
		t.Errorf("status tras MarkInviteAccepted = %v, quería accepted", got.Status)
	}
	if !got.AcceptedAt.Valid {
		t.Errorf("accepted_at debería estar set tras MarkInviteAccepted")
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM invites WHERE id = $1", pgtypeUUID(t, invite.ID.String()))
	})
}

// TestIntegration_ActivityEventEnqueueAndGet verifies the real Strava event
// shape, UUID lookup, athlete resolution, and natural-key idempotency.
func TestIntegration_ActivityEventEnqueueAndGet(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	// Crear usuario + tokens Strava para que GetUserIDByAthleteID resuelva.
	clerkID := "clerk_test_" + t.Name() + "_integration"
	created, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ClerkUserID:  clerkID,
		Email:        "strava-test@example.com",
		DisplayName:  pgtype.Text{String: "Strava Test", Valid: true},
		InviteStatus: status.InviteStatusActive,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE clerk_user_id = $1", clerkID)
	})

	athleteID := int64(7777777)
	if _, err := pool.Exec(ctx, `INSERT INTO strava_tokens
		(user_id, access_cipher, refresh_cipher, expires_at, athlete_id, scopes, created_at, updated_at)
		VALUES ($1, $2, $3, now() + interval '1 hour', $4, $5, now(), now())`,
		created.ID, "dGVzdA==", "dGVzdA==", athleteID, "activity:read"); err != nil {
		t.Fatalf("insert strava_tokens: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM strava_tokens WHERE user_id = $1", created.ID)
	})

	// Resolver user_id por athlete_id.
	uid, err := q.GetUserIDByAthleteID(ctx, athleteID)
	if err != nil {
		t.Fatalf("GetUserIDByAthleteID: %v", err)
	}
	if !uid.Valid {
		t.Fatalf("GetUserIDByAthleteID devolvió UUID sin Valid=true; esperaba user_id real")
	}
	if uid != created.ID {
		t.Errorf("GetUserIDByAthleteID = %v, quería %v", uid, created.ID)
	}

	// Encolar un evento con los campos del payload real.
	eventTime := pgtype.Timestamptz{Time: time.Now().UTC().Truncate(time.Microsecond), Valid: true}
	inserted, err := q.EnqueueActivityEvent(ctx, sqlc.EnqueueActivityEventParams{
		UserID:         created.ID,
		ObjectType:     "activity",
		AspectType:     "create",
		ObjectID:       1234567890,
		OwnerID:        pgtype.Int8{Int64: athleteID, Valid: true},
		SubscriptionID: pgtype.Int8{Int64: 99999, Valid: true},
		EventTime:      eventTime,
		RawPayload:     []byte(`{"object_type":"activity","aspect_type":"create","object_id":1234567890}`),
	})
	if err != nil {
		t.Fatalf("EnqueueActivityEvent: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM activity_events WHERE id = $1", inserted.ID)
	})

	// Recuperar por UUID interno.
	got, err := q.GetActivityEventByID(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("GetActivityEventByID: %v", err)
	}
	if got.ObjectID != 1234567890 {
		t.Errorf("ObjectID = %d, quería 1234567890", got.ObjectID)
	}
	if got.AspectType != "create" {
		t.Errorf("AspectType = %q, quería create", got.AspectType)
	}
	if !got.OwnerID.Valid || got.OwnerID.Int64 != athleteID {
		t.Errorf("OwnerID = %+v, quería {Int64:%d Valid:true}", got.OwnerID, athleteID)
	}
	if !got.EventTime.Valid {
		t.Errorf("EventTime debería estar set, es NULL")
	}

	// Re-enqueuing the same natural event key must return the existing row.
	if _, err := q.EnqueueActivityEvent(ctx, sqlc.EnqueueActivityEventParams{
		UserID:         created.ID,
		ObjectType:     "activity",
		AspectType:     "create",
		ObjectID:       1234567890,
		OwnerID:        pgtype.Int8{Int64: athleteID, Valid: true},
		SubscriptionID: pgtype.Int8{Int64: 99999, Valid: true},
		EventTime:      eventTime,
		RawPayload:     []byte(`{"retry":true}`),
	}); err != nil {
		t.Fatalf("EnqueueActivityEvent (retry): %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM activity_events
		WHERE object_id = $1 AND aspect_type = $2 AND event_time = $3`,
		int64(1234567890), "create", eventTime).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("natural-key idempotency broken: got %d rows, want 1", count)
	}
}

// TestIntegration_GetUserIDByAthleteID_UnknownAthlete devuelve UUID sin
// Valid=true cuando el athlete_id no está en strava_tokens. El handler
// (PR B) lo trata como "no conozco a este atleta" y responde 200 sin
// encolar el evento.
func TestIntegration_GetUserIDByAthleteID_UnknownAthlete(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	uid, err := q.GetUserIDByAthleteID(ctx, 999999999999)
	if err != nil {
		t.Fatalf("GetUserIDByAthleteID (unknown): %v", err)
	}
	if uid.Valid {
		t.Errorf("GetUserIDByAthleteID (unknown) devolvió UUID válido; esperaba Valid=false")
	}
}
