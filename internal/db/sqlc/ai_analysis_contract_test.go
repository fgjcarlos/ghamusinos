// Static contract tests for the ai_analysis storage (phase 1.5, issue #17,
// unit T4).
//
// The persistence half of the optional AI feature is pinned statically, the
// same way the consent migration (00014) is pinned in
// internal/http/handlers/me_preferences_test.go: the migration, the sqlc
// schema and the query file must agree, and the generated code must expose
// the ownership-scoped queries the job and API units reuse.
//
// These tests read the files from disk; they do not need a database. The
// live behaviour is covered by ai_analysis_integration_test.go (build tag
// `integration`).
package sqlc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// aiAnalysisMigration is the migration that creates the ai_analysis table.
const aiAnalysisMigration = "00015_ai_analysis.sql"

// readDBFile reads a file relative to this package directory (internal/db/sqlc).
func readDBFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(parts...)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// gooseUpSection returns the Up half of a goose migration.
func gooseUpSection(t *testing.T, migration string) string {
	t.Helper()
	idx := strings.Index(migration, "-- +goose Down")
	if idx < 0 {
		return migration
	}
	return migration[:idx]
}

// gooseDownSection returns the Down half of a goose migration.
func gooseDownSection(t *testing.T, migration string) string {
	t.Helper()
	idx := strings.Index(migration, "-- +goose Down")
	if idx < 0 {
		return ""
	}
	return migration[idx:]
}

// requiredAIAnalysisColumns are the columns the job and API units rely on.
var requiredAIAnalysisColumns = []string{
	"id",
	"user_id",
	"kind",
	"scope",
	"status",
	"provider",
	"model",
	"payload",
	"output",
	"error_code",
	"error_message",
	"created_at",
	"updated_at",
}

// TestMigration00015_CreatesAIAnalysisTable proves the storage contract of
// the table: one row per user and analysis target, constrained kind and
// status, cascade delete with the user, and safe error columns.
func TestMigration00015_CreatesAIAnalysisTable(t *testing.T) {
	migration := readDBFile(t, "..", "migrations", aiAnalysisMigration)
	up := gooseUpSection(t, migration)

	if !strings.Contains(up, "CREATE TABLE ai_analysis") {
		t.Fatalf("expected the migration to create the ai_analysis table, got:\n%s", up)
	}
	for _, column := range requiredAIAnalysisColumns {
		if !strings.Contains(up, column) {
			t.Errorf("expected the ai_analysis table to declare the column %q, got:\n%s", column, up)
		}
	}
	if !strings.Contains(up, "REFERENCES users(id) ON DELETE CASCADE") {
		t.Errorf("expected user_id to reference users(id) ON DELETE CASCADE, got:\n%s", up)
	}
	if !strings.Contains(up, "UNIQUE (user_id, kind, scope)") {
		t.Errorf("expected a UNIQUE (user_id, kind, scope) constraint, got:\n%s", up)
	}
	for _, kind := range []string{"activity", "weekly", "load"} {
		if !strings.Contains(up, "'"+kind+"'") {
			t.Errorf("expected the kind constraint to admit %q, got:\n%s", kind, up)
		}
	}
	for _, status := range []string{"pending", "completed", "failed"} {
		if !strings.Contains(up, "'"+status+"'") {
			t.Errorf("expected the status constraint to admit %q, got:\n%s", status, up)
		}
	}
	// The error columns must be safe by construction: a lowercase token and
	// a bounded message. A provider body or a credential must not fit.
	if !strings.Contains(up, "error_code ~") {
		t.Errorf("expected the error_code column to be constrained to a safe token, got:\n%s", up)
	}
	if !strings.Contains(up, "length(error_message)") {
		t.Errorf("expected the error_message column to have a length bound, got:\n%s", up)
	}
	// The queries list the analyses of a user, so the table needs an index
	// that supports that read.
	if !strings.Contains(up, "CREATE INDEX") {
		t.Errorf("expected the migration to create the indexes the queries need, got:\n%s", up)
	}
}

// TestMigration00015_DownDropsTable proves the rollback removes the table.
func TestMigration00015_DownDropsTable(t *testing.T) {
	migration := readDBFile(t, "..", "migrations", aiAnalysisMigration)
	down := gooseDownSection(t, migration)
	if down == "" {
		t.Fatal("expected the migration to declare a Down section")
	}
	if !strings.Contains(down, "DROP TABLE") || !strings.Contains(down, "ai_analysis") {
		t.Errorf("expected the Down section to drop the ai_analysis table, got:\n%s", down)
	}
}

// TestSchema00015_MirrorsTheMigration proves the sqlc schema (the source of
// truth for the generated code) agrees with the migration: same columns,
// same constraints, same uniqueness.
func TestSchema00015_MirrorsTheMigration(t *testing.T) {
	schema := readDBFile(t, "..", "schema.sql")
	if !strings.Contains(schema, "CREATE TABLE ai_analysis") {
		t.Fatal("expected schema.sql to declare the ai_analysis table")
	}
	for _, column := range requiredAIAnalysisColumns {
		if !strings.Contains(schema, column) {
			t.Errorf("expected schema.sql to declare the column %q", column)
		}
	}
	if !strings.Contains(schema, "REFERENCES users(id) ON DELETE CASCADE") {
		t.Error("expected schema.sql to declare user_id with ON DELETE CASCADE")
	}
	if !strings.Contains(schema, "UNIQUE (user_id, kind, scope)") {
		t.Error("expected schema.sql to declare UNIQUE (user_id, kind, scope)")
	}
	for _, kind := range []string{"activity", "weekly", "load"} {
		if !strings.Contains(schema, "'"+kind+"'") {
			t.Errorf("expected the schema kind constraint to admit %q", kind)
		}
	}
	for _, status := range []string{"pending", "completed", "failed"} {
		if !strings.Contains(schema, "'"+status+"'") {
			t.Errorf("expected the schema status constraint to admit %q", status)
		}
	}
}

// expectedAIAnalysisQueries are the queries the job unit (T5) and the API
// unit (T6) reuse. Renaming one of them is a breaking change for those
// units, so the names are pinned here.
var expectedAIAnalysisQueries = []string{
	"CreateAIAnalysis",
	"GetAIAnalysis",
	"ListAIAnalyses",
	"CompleteAIAnalysis",
	"FailAIAnalysis",
}

// TestQueries00015_DeclareExpectedQueries proves the query file declares
// the contract the later units consume.
func TestQueries00015_DeclareExpectedQueries(t *testing.T) {
	queries := readDBFile(t, "..", "queries", "ai_analysis.sql")
	for _, name := range expectedAIAnalysisQueries {
		if !strings.Contains(queries, "-- name: "+name+" ") {
			t.Errorf("expected the query file to declare the query %q, got:\n%s", name, queries)
		}
	}
	// Every read and every write is scoped by user_id: one user can never
	// see or overwrite the analysis of another one.
	for _, name := range expectedAIAnalysisQueries {
		if name == "CreateAIAnalysis" {
			continue
		}
		start := strings.Index(queries, "-- name: "+name+" ")
		if start < 0 {
			continue
		}
		end := strings.Index(queries[start+1:], "-- name: ")
		block := queries[start:]
		if end >= 0 {
			block = queries[start : start+1+end]
		}
		if !strings.Contains(block, "user_id = $1") {
			t.Errorf("expected the query %q to filter by user_id, got:\n%s", name, block)
		}
	}
}

// TestGenerated00015_ExposeTheQueryContract proves the generated code
// exposes the queries (sqlc ran over the new query file) and that the
// ownership filter survives the generation.
func TestGenerated00015_ExposeTheQueryContract(t *testing.T) {
	generated := readDBFile(t, "ai_analysis.sql.go")
	for _, name := range expectedAIAnalysisQueries {
		if !strings.Contains(generated, "func (q *Queries) "+name+"(") {
			t.Errorf("expected the generated code to expose %s, got:\n%s", name, generated)
		}
	}
	normalized := strings.Join(strings.Fields(generated), " ")
	if !strings.Contains(normalized, "WHERE user_id = $1 AND kind = $2 AND scope = $3") {
		t.Error("expected the generated queries to filter by (user_id, kind, scope)")
	}
	if !strings.Contains(normalized, "ON CONFLICT (user_id, kind, scope)") {
		t.Error("expected the generated insert to reuse the row of a repeated request")
	}
}
