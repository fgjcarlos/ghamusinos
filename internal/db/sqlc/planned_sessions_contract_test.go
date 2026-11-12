// Static contract tests for the planned_sessions storage (fase 2.1,
// issue #298).
//
// The persistence contract of the table is pinned statically, the same way
// the ai_analysis storage is pinned in ai_analysis_contract_test.go: the
// migration, the sqlc schema and the query file must agree, and the
// generated code must expose the ownership-scoped queries the store
// reuses.
//
// These tests read the files from disk; they do not need a database. The
// live behaviour is covered by planned_sessions_integration_test.go (build
// tag `integration`).
package sqlc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plannedSessionsMigration is the migration that creates the table.
const plannedSessionsMigration = "00016_planned_sessions.sql"

// readPlannedSessionsFile reads a file relative to this package directory
// (internal/db/sqlc).
func readPlannedSessionsFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(parts...)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// plannedSessionsGooseSection returns the Up or Down half of a migration.
func plannedSessionsGooseSection(t *testing.T, migration, section string) string {
	t.Helper()
	if section == "Down" {
		idx := strings.Index(migration, "-- +goose Down")
		if idx < 0 {
			return ""
		}
		return migration[idx:]
	}
	idx := strings.Index(migration, "-- +goose Down")
	if idx < 0 {
		return migration
	}
	return migration[:idx]
}

// requiredPlannedSessionsColumns are the columns the store relies on.
var requiredPlannedSessionsColumns = []string{
	"id",
	"user_id",
	"scheduled_on",
	"title",
	"notes",
	"workout_type",
	"target_duration_sec",
	"target_distance_m",
	"target_elevation_gain_m",
	"target_intensity",
	"created_at",
	"updated_at",
}

// plannedSessionsWorkoutTypes is the closed vocabulary of workout types.
// internal/workouts repeats it as Go constants; the domain validation test
// there proves the two lists agree.
var plannedSessionsWorkoutTypes = []string{
	"rodaje",
	"tirada_larga",
	"series",
	"cuestas",
	"tempo",
	"fartlek",
	"fuerza",
	"recuperacion",
	"otro",
}

// plannedSessionsIntensities is the closed vocabulary of target intensity
// zones (z1..z5, the same one the heart-rate zones already use).
var plannedSessionsIntensities = []string{"z1", "z2", "z3", "z4", "z5"}

// TestMigration00016_CreatesPlannedSessionsTable proves the storage
// contract of the table: ownership by cascade, a calendar date, bounded
// titles and notes, and the closed set of workout types and intensities.
func TestMigration00016_CreatesPlannedSessionsTable(t *testing.T) {
	migration := readPlannedSessionsFile(t, "..", "migrations", plannedSessionsMigration)
	up := plannedSessionsGooseSection(t, migration, "Up")

	if !strings.Contains(up, "CREATE TABLE planned_sessions") {
		t.Fatalf("expected the migration to create the planned_sessions table, got:\n%s", up)
	}
	for _, column := range requiredPlannedSessionsColumns {
		if !strings.Contains(up, column) {
			t.Errorf("expected the planned_sessions table to declare the column %q, got:\n%s", column, up)
		}
	}
	if !strings.Contains(up, "REFERENCES users(id) ON DELETE CASCADE") {
		t.Errorf("expected user_id to reference users(id) ON DELETE CASCADE, got:\n%s", up)
	}
	// scheduled_on es un dia civil, no un instante.
	if !strings.Contains(up, "scheduled_on            DATE") {
		t.Errorf("expected scheduled_on to be a DATE, got:\n%s", up)
	}
	for _, workoutType := range plannedSessionsWorkoutTypes {
		if !strings.Contains(up, "'"+workoutType+"'") {
			t.Errorf("expected the workout_type constraint to admit %q, got:\n%s", workoutType, up)
		}
	}
	for _, intensity := range plannedSessionsIntensities {
		if !strings.Contains(up, "'"+intensity+"'") {
			t.Errorf("expected the target_intensity constraint to admit %q, got:\n%s", intensity, up)
		}
	}
	// Los objetivos no pueden ser negativos.
	for _, constraint := range []string{
		"planned_sessions_target_duration_no_negativo",
		"planned_sessions_target_distance_no_negativo",
		"planned_sessions_target_elevation_no_negativo",
	} {
		if !strings.Contains(up, constraint) {
			t.Errorf("expected the migration to declare the constraint %q, got:\n%s", constraint, up)
		}
	}
	if !strings.Contains(up, "CREATE INDEX idx_planned_sessions_user_scheduled") {
		t.Errorf("expected the migration to index (user_id, scheduled_on), got:\n%s", up)
	}
}

// TestMigration00016_DownDropsTable proves the rollback removes the table.
func TestMigration00016_DownDropsTable(t *testing.T) {
	migration := readPlannedSessionsFile(t, "..", "migrations", plannedSessionsMigration)
	down := plannedSessionsGooseSection(t, migration, "Down")
	if down == "" {
		t.Fatal("expected the migration to declare a Down section")
	}
	if !strings.Contains(down, "DROP TABLE") || !strings.Contains(down, "planned_sessions") {
		t.Errorf("expected the Down section to drop the planned_sessions table, got:\n%s", down)
	}
}

// TestSchema00016_MirrorsTheMigration proves the sqlc schema (the source of
// truth for the generated code) agrees with the migration.
func TestSchema00016_MirrorsTheMigration(t *testing.T) {
	schema := readPlannedSessionsFile(t, "..", "schema.sql")
	if !strings.Contains(schema, "CREATE TABLE planned_sessions") {
		t.Fatal("expected schema.sql to declare the planned_sessions table")
	}
	for _, column := range requiredPlannedSessionsColumns {
		if !strings.Contains(schema, column) {
			t.Errorf("expected schema.sql to declare the column %q", column)
		}
	}
	if !strings.Contains(schema, "REFERENCES users(id) ON DELETE CASCADE") {
		t.Error("expected schema.sql to declare user_id with ON DELETE CASCADE")
	}
	if !strings.Contains(schema, "planned_sessions_target_duration_no_negativo") {
		t.Error("expected schema.sql to declare the non-negative target constraints")
	}
	if !strings.Contains(schema, "CREATE INDEX idx_planned_sessions_user_scheduled") {
		t.Error("expected schema.sql to declare the (user_id, scheduled_on) index")
	}
}

// expectedPlannedSessionsQueries are the queries the store consumes.
// Renaming one of them is a breaking change for the store, so the names are
// pinned here.
var expectedPlannedSessionsQueries = []string{
	"CreatePlannedSession",
	"GetPlannedSession",
	"ListPlannedSessionsByRange",
	"UpdatePlannedSession",
	"DeletePlannedSession",
}

// TestQueries00016_DeclareExpectedQueries proves the query file declares
// the contract the store consumes, and that every read and every write is
// scoped by user_id: one user can never see or overwrite the session of
// another one.
func TestQueries00016_DeclareExpectedQueries(t *testing.T) {
	queries := readPlannedSessionsFile(t, "..", "queries", "planned_sessions.sql")
	for _, name := range expectedPlannedSessionsQueries {
		if !strings.Contains(queries, "-- name: "+name+" ") {
			t.Errorf("expected the query file to declare the query %q, got:\n%s", name, queries)
		}
	}
	// Create lleva el user_id en el INSERT: es el aislamiento de escritura.
	for _, name := range []string{"GetPlannedSession", "UpdatePlannedSession", "DeletePlannedSession"} {
		block := plannedSessionsQueryBlock(t, queries, name)
		if !strings.Contains(block, "user_id") {
			t.Errorf("expected the query %q to filter by user_id, got:\n%s", name, block)
		}
	}
	list := plannedSessionsQueryBlock(t, queries, "ListPlannedSessionsByRange")
	if !strings.Contains(list, "WHERE user_id") {
		t.Errorf("expected the list query to filter by user_id, got:\n%s", list)
	}
}

// plannedSessionsQueryBlock returns the text of a single query block.
func plannedSessionsQueryBlock(t *testing.T, queries, name string) string {
	t.Helper()
	start := strings.Index(queries, "-- name: "+name+" ")
	if start < 0 {
		return ""
	}
	end := strings.Index(queries[start+1:], "-- name: ")
	if end < 0 {
		return queries[start:]
	}
	return queries[start : start+1+end]
}

// TestGenerated00016_ExposeTheQueryContract proves the generated code
// exposes the queries (sqlc ran over the new query file).
func TestGenerated00016_ExposeTheQueryContract(t *testing.T) {
	generated := readPlannedSessionsFile(t, "planned_sessions.sql.go")
	for _, name := range expectedPlannedSessionsQueries {
		if !strings.Contains(generated, "func (q *Queries) "+name+"(") {
			t.Errorf("expected the generated code to expose %s, got:\n%s", name, generated)
		}
	}
}
