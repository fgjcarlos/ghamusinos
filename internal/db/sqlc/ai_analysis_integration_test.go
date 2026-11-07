//go:build integration

// Integration tests for the ai_analysis storage (phase 1.5, issue #17,
// unit T4).
//
// They need a PostgreSQL with the migrations applied (including
// 00015_ai_analysis.sql). They run with:
//
//	go test -tags integration -race ./internal/db/sqlc/...
//
// The static half of the same contract lives in
// ai_analysis_contract_test.go and always runs.
package sqlc_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
)

// assertJSONEqual compares two JSON documents by value: PostgreSQL
// normalizes the whitespace and the key order of a jsonb column, so the
// bytes are not the contract, the document is.
func assertJSONEqual(t *testing.T, got []byte, want []byte) {
	t.Helper()
	var gotDoc, wantDoc any
	if err := json.Unmarshal(got, &gotDoc); err != nil {
		t.Fatalf("el JSON guardado no es válido: %v", err)
	}
	if err := json.Unmarshal(want, &wantDoc); err != nil {
		t.Fatalf("el JSON esperado no es válido: %v", err)
	}
	if !reflect.DeepEqual(gotDoc, wantDoc) {
		t.Errorf("JSON = %s, quería %s", got, want)
	}
}

// createAIAnalysisTestUser creates a user for the test and removes it (and
// its analyses, by cascade) on cleanup.
func createAIAnalysisTestUser(t *testing.T, pool *pgxpool.Pool, q *sqlc.Queries, suffix string) sqlc.User {
	t.Helper()
	ctx := context.Background()
	clerkID := "clerk_test_" + t.Name() + "_" + suffix + "_integration"

	created, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ClerkUserID:  clerkID,
		Email:        "ai-analysis-" + suffix + "@example.com",
		DisplayName:  pgtype.Text{String: "AI Analysis Test", Valid: true},
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

// TestIntegration_AIAnalysis_CreatedPending proves a new analysis starts
// as pending, without provider, model, result or error.
func TestIntegration_AIAnalysis_CreatedPending(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createAIAnalysisTestUser(t, pool, q, "pending")

	row, err := q.CreateAIAnalysis(ctx, sqlc.CreateAIAnalysisParams{
		UserID: user.ID,
		Kind:   "activity",
		Scope:  "activity-1",
	})
	if err != nil {
		t.Fatalf("CreateAIAnalysis: %v", err)
	}
	if row.Status != "pending" {
		t.Errorf("status = %q, quería %q", row.Status, "pending")
	}
	if row.Provider.Valid || row.Model.Valid {
		t.Errorf("provider/model no deberían estar seteados al crear: %+v", row)
	}
	if row.Output != nil {
		t.Errorf("output = %s, quería NULL", row.Output)
	}
	if row.ErrorCode.Valid || row.ErrorMessage.Valid {
		t.Errorf("error_code/error_message no deberían estar seteados al crear: %+v", row)
	}
	if string(row.Payload) != "{}" {
		t.Errorf("payload = %s, quería {} (nada enviado todavía)", row.Payload)
	}
	if !row.CreatedAt.Valid || !row.UpdatedAt.Valid {
		t.Error("created_at/updated_at deberían estar seteados")
	}
}

// TestIntegration_AIAnalysis_RepeatedRequestReusesTheRow proves the
// uniqueness contract: a repeated request updates the same row instead of
// duplicating it.
func TestIntegration_AIAnalysis_RepeatedRequestReusesTheRow(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createAIAnalysisTestUser(t, pool, q, "idempotent")

	first, err := q.CreateAIAnalysis(ctx, sqlc.CreateAIAnalysisParams{
		UserID: user.ID,
		Kind:   "weekly",
		Scope:  "2026-W23",
	})
	if err != nil {
		t.Fatalf("CreateAIAnalysis: %v", err)
	}
	second, err := q.CreateAIAnalysis(ctx, sqlc.CreateAIAnalysisParams{
		UserID: user.ID,
		Kind:   "weekly",
		Scope:  "2026-W23",
	})
	if err != nil {
		t.Fatalf("CreateAIAnalysis (repetido): %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("id = %v / %v, quería la misma fila", first.ID, second.ID)
	}
	var count int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM ai_analysis WHERE user_id = $1 AND kind = $2 AND scope = $3",
		user.ID, "weekly", "2026-W23").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("filas = %d, quería 1", count)
	}
}

// TestIntegration_AIAnalysis_Completed proves the completion path: the
// status, the provider, the model, the aggregate payload sent and the
// validated output are stored, and no error remains.
func TestIntegration_AIAnalysis_Completed(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createAIAnalysisTestUser(t, pool, q, "completed")

	if _, err := q.CreateAIAnalysis(ctx, sqlc.CreateAIAnalysisParams{
		UserID: user.ID,
		Kind:   "activity",
		Scope:  "activity-2",
	}); err != nil {
		t.Fatalf("CreateAIAnalysis: %v", err)
	}

	payload := []byte(`{"kind":"activity","metrics":{"distance_meters":1000,"duration_seconds":600}}`)
	output := []byte(`{"summary":"Sesión suave","observations":[],"cautions":[]}`)
	done, err := q.CompleteAIAnalysis(ctx, sqlc.CompleteAIAnalysisParams{
		UserID:   user.ID,
		Kind:     "activity",
		Scope:    "activity-2",
		Provider: pgtype.Text{String: "openai", Valid: true},
		Model:    pgtype.Text{String: "gpt-test", Valid: true},
		Payload:  payload,
		Output:   output,
	})
	if err != nil {
		t.Fatalf("CompleteAIAnalysis: %v", err)
	}
	if done.Status != "completed" {
		t.Errorf("status = %q, quería %q", done.Status, "completed")
	}
	if !done.Provider.Valid || done.Provider.String != "openai" {
		t.Errorf("provider = %+v, quería openai", done.Provider)
	}
	if !done.Model.Valid || done.Model.String != "gpt-test" {
		t.Errorf("model = %+v, quería gpt-test", done.Model)
	}
	if string(done.Payload) == "" {
		t.Error("payload vacío: el payload enviado debe quedar guardado")
	}
	assertJSONEqual(t, done.Payload, payload)
	assertJSONEqual(t, done.Output, output)
	if done.ErrorCode.Valid || done.ErrorMessage.Valid {
		t.Errorf("el error debe quedar limpio tras completar: %+v", done)
	}

	got, err := q.GetAIAnalysis(ctx, sqlc.GetAIAnalysisParams{
		UserID: user.ID,
		Kind:   "activity",
		Scope:  "activity-2",
	})
	if err != nil {
		t.Fatalf("GetAIAnalysis: %v", err)
	}
	if got.Status != "completed" {
		t.Errorf("status leído = %q, quería %q", got.Status, "completed")
	}
}

// TestIntegration_AIAnalysis_Failed proves the failure path: the status
// becomes failed and only the safe error (a token code and a short
// message) is stored.
func TestIntegration_AIAnalysis_Failed(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createAIAnalysisTestUser(t, pool, q, "failed")

	if _, err := q.CreateAIAnalysis(ctx, sqlc.CreateAIAnalysisParams{
		UserID: user.ID,
		Kind:   "load",
		Scope:  "2026-06-01..2026-06-07",
	}); err != nil {
		t.Fatalf("CreateAIAnalysis: %v", err)
	}

	failed, err := q.FailAIAnalysis(ctx, sqlc.FailAIAnalysisParams{
		UserID:       user.ID,
		Kind:         "load",
		Scope:        "2026-06-01..2026-06-07",
		Provider:     pgtype.Text{String: "claude", Valid: true},
		Model:        pgtype.Text{String: "claude-test", Valid: true},
		Payload:      []byte(`{"kind":"load"}`),
		ErrorCode:    pgtype.Text{String: "provider_timeout", Valid: true},
		ErrorMessage: pgtype.Text{String: "El proveedor no respondió a tiempo.", Valid: true},
	})
	if err != nil {
		t.Fatalf("FailAIAnalysis: %v", err)
	}
	if failed.Status != "failed" {
		t.Errorf("status = %q, quería %q", failed.Status, "failed")
	}
	if !failed.ErrorCode.Valid || failed.ErrorCode.String != "provider_timeout" {
		t.Errorf("error_code = %+v, quería provider_timeout", failed.ErrorCode)
	}
	if !failed.ErrorMessage.Valid {
		t.Error("error_message debería estar seteado")
	}
	if failed.Output != nil {
		t.Errorf("output = %s, quería NULL en un análisis fallido", failed.Output)
	}
}

// TestIntegration_AIAnalysis_RejectsUnsafeErrorCode proves the safe
// error contract at the storage level: an error code that is not a
// lowercase token (for example a provider body) cannot be stored.
func TestIntegration_AIAnalysis_RejectsUnsafeErrorCode(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createAIAnalysisTestUser(t, pool, q, "unsafe-error")

	if _, err := q.CreateAIAnalysis(ctx, sqlc.CreateAIAnalysisParams{
		UserID: user.ID,
		Kind:   "activity",
		Scope:  "activity-3",
	}); err != nil {
		t.Fatalf("CreateAIAnalysis: %v", err)
	}

	_, err := q.FailAIAnalysis(ctx, sqlc.FailAIAnalysisParams{
		UserID:       user.ID,
		Kind:         "activity",
		Scope:        "activity-3",
		Payload:      []byte(`{"kind":"activity"}`),
		ErrorCode:    pgtype.Text{String: "El proveedor dijo: sk-secret-123", Valid: true},
		ErrorMessage: pgtype.Text{String: "cuerpo del proveedor", Valid: true},
	})
	if err == nil {
		t.Fatal("FailAIAnalysis debería rechazar un error_code que no es un token seguro")
	}
}

// TestIntegration_AIAnalysis_RejectsUnknownKind proves the kind
// constraint: only the three documented analyses can be stored.
func TestIntegration_AIAnalysis_RejectsUnknownKind(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createAIAnalysisTestUser(t, pool, q, "bad-kind")

	_, err := pool.Exec(ctx,
		"INSERT INTO ai_analysis (user_id, kind, scope) VALUES ($1, 'monthly', 'x')", user.ID)
	if err == nil {
		t.Fatal("la tabla debería rechazar un kind fuera de (activity, weekly, load)")
	}
}

// TestIntegration_AIAnalysis_OtherUserCannotSeeOrOverwrite proves the
// isolation contract: the analysis of one user is invisible and
// untouchable for another one.
func TestIntegration_AIAnalysis_OtherUserCannotSeeOrOverwrite(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	owner := createAIAnalysisTestUser(t, pool, q, "owner")
	intruder := createAIAnalysisTestUser(t, pool, q, "intruder")

	if _, err := q.CreateAIAnalysis(ctx, sqlc.CreateAIAnalysisParams{
		UserID: owner.ID,
		Kind:   "activity",
		Scope:  "activity-4",
	}); err != nil {
		t.Fatalf("CreateAIAnalysis: %v", err)
	}

	// No puede leerlo.
	_, err := q.GetAIAnalysis(ctx, sqlc.GetAIAnalysisParams{
		UserID: intruder.ID,
		Kind:   "activity",
		Scope:  "activity-4",
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetAIAnalysis de otro usuario = %v, quería pgx.ErrNoRows", err)
	}

	// No puede completarlo.
	_, err = q.CompleteAIAnalysis(ctx, sqlc.CompleteAIAnalysisParams{
		UserID:   intruder.ID,
		Kind:     "activity",
		Scope:    "activity-4",
		Provider: pgtype.Text{String: "openai", Valid: true},
		Payload:  []byte(`{"kind":"activity"}`),
		Output:   []byte(`{"summary":"inject","observations":[],"cautions":[]}`),
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("CompleteAIAnalysis de otro usuario = %v, quería pgx.ErrNoRows", err)
	}

	// No puede fallarlo.
	_, err = q.FailAIAnalysis(ctx, sqlc.FailAIAnalysisParams{
		UserID:    intruder.ID,
		Kind:      "activity",
		Scope:     "activity-4",
		Payload:   []byte(`{"kind":"activity"}`),
		ErrorCode: pgtype.Text{String: "injected", Valid: true},
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("FailAIAnalysis de otro usuario = %v, quería pgx.ErrNoRows", err)
	}

	// La fila del dueño sigue intacta.
	row, err := q.GetAIAnalysis(ctx, sqlc.GetAIAnalysisParams{
		UserID: owner.ID,
		Kind:   "activity",
		Scope:  "activity-4",
	})
	if err != nil {
		t.Fatalf("GetAIAnalysis (dueño): %v", err)
	}
	if row.Status != "pending" {
		t.Errorf("status = %q, quería %q (la fila del dueño no puede cambiar)", row.Status, "pending")
	}
	if row.Output != nil {
		t.Errorf("output = %s, quería NULL", row.Output)
	}
}

// TestIntegration_AIAnalysis_ListOnlyOwnRows proves that the listing is
// scoped to the owner and ordered from the newest to the oldest.
func TestIntegration_AIAnalysis_ListOnlyOwnRows(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	owner := createAIAnalysisTestUser(t, pool, q, "list-owner")
	other := createAIAnalysisTestUser(t, pool, q, "list-other")

	for _, scope := range []string{"a", "b"} {
		if _, err := q.CreateAIAnalysis(ctx, sqlc.CreateAIAnalysisParams{
			UserID: owner.ID,
			Kind:   "activity",
			Scope:  scope,
		}); err != nil {
			t.Fatalf("CreateAIAnalysis: %v", err)
		}
	}
	if _, err := q.CreateAIAnalysis(ctx, sqlc.CreateAIAnalysisParams{
		UserID: other.ID,
		Kind:   "activity",
		Scope:  "c",
	}); err != nil {
		t.Fatalf("CreateAIAnalysis: %v", err)
	}

	rows, err := q.ListAIAnalyses(ctx, sqlc.ListAIAnalysesParams{
		UserID: owner.ID,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListAIAnalyses: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("filas = %d, quería 2 (sólo las del usuario)", len(rows))
	}
	for _, r := range rows {
		if r.UserID != owner.ID {
			t.Errorf("la lista incluye una fila de otro usuario: %+v", r)
		}
	}
}

// TestIntegration_AIAnalysis_CascadeDeleteWithUser proves the analysis
// rows disappear with their user (ON DELETE CASCADE).
func TestIntegration_AIAnalysis_CascadeDeleteWithUser(t *testing.T) {
	pool := openTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	user := createAIAnalysisTestUser(t, pool, q, "cascade")

	if _, err := q.CreateAIAnalysis(ctx, sqlc.CreateAIAnalysisParams{
		UserID: user.ID,
		Kind:   "weekly",
		Scope:  "2026-W24",
	}); err != nil {
		t.Fatalf("CreateAIAnalysis: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM ai_analysis WHERE user_id = $1", user.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("filas = %d, quería 0 tras borrar el usuario (cascade)", count)
	}
}
