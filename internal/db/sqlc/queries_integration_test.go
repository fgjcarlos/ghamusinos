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
