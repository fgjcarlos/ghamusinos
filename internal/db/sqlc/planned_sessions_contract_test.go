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

// ─── Fase 2.2 (issue #311): estado y vinculación de actividad ────────────

// plannedSessionsStatusMigration añade status y activity_id.
const plannedSessionsStatusMigration = "00017_planned_sessions_status_activity.sql"

// plannedSessionsStatuses es el vocabulario cerrado de estados.
var plannedSessionsStatuses = []string{"planned", "completed", "skipped", "modified"}

// expectedPlannedSessionsLinkQueries son las dos queries nuevas del store.
var expectedPlannedSessionsLinkQueries = []string{
	"LinkPlannedSessionActivity",
	"UnlinkPlannedSessionActivity",
}

// TestMigration00017_AddsStatusAndActivityLink proves the storage contract
// of migration 00017: estado cerrado con DEFAULT 'planned', actividad
// opcional con FK que no borra la actividad y UNIQUE de un solo enlace por
// actividad.
func TestMigration00017_AddsStatusAndActivityLink(t *testing.T) {
	migration := readPlannedSessionsFile(t, "..", "migrations", plannedSessionsStatusMigration)
	up := plannedSessionsGooseSection(t, migration, "Up")

	if !strings.Contains(up, "ADD COLUMN status") {
		t.Fatalf("expected the migration to add the status column, got:\n%s", up)
	}
	if !strings.Contains(up, "DEFAULT 'planned'") {
		t.Errorf("expected status to default to 'planned', got:\n%s", up)
	}
	if !strings.Contains(up, "planned_sessions_status_valido") {
		t.Errorf("expected a named status CHECK constraint, got:\n%s", up)
	}
	for _, s := range plannedSessionsStatuses {
		if !strings.Contains(up, "'"+s+"'") {
			t.Errorf("expected the status constraint to admit %q, got:\n%s", s, up)
		}
	}
	if !strings.Contains(up, "ADD COLUMN activity_id") {
		t.Errorf("expected the migration to add the activity_id column, got:\n%s", up)
	}
	if !strings.Contains(up, "REFERENCES activities(id) ON DELETE SET NULL") {
		t.Errorf("expected activity_id to reference activities(id) ON DELETE SET NULL, got:\n%s", up)
	}
	if !strings.Contains(up, "planned_sessions_activity_unica") || !strings.Contains(up, "UNIQUE") {
		t.Errorf("expected a named UNIQUE constraint on activity_id (one link per activity), got:\n%s", up)
	}
}

// TestMigration00017_DownDropsOnlyTheNewColumns proves el rollback es
// reversible: caen las columnas nuevas, la tabla y sus sesiones siguen.
func TestMigration00017_DownDropsOnlyTheNewColumns(t *testing.T) {
	migration := readPlannedSessionsFile(t, "..", "migrations", plannedSessionsStatusMigration)
	down := plannedSessionsGooseSection(t, migration, "Down")
	if down == "" {
		t.Fatal("expected the migration to declare a Down section")
	}
	for _, column := range []string{"activity_id", "status"} {
		if !strings.Contains(down, "DROP COLUMN IF EXISTS "+column) {
			t.Errorf("expected the Down section to drop the column %q, got:\n%s", column, down)
		}
	}
	if strings.Contains(down, "DROP TABLE") {
		t.Errorf("the Down section must keep the planned_sessions table, got:\n%s", down)
	}
}

// TestSchema00017_MirrorsTheMigration proves the sqlc schema agrees with
// migration 00017.
func TestSchema00017_MirrorsTheMigration(t *testing.T) {
	schema := readPlannedSessionsFile(t, "..", "schema.sql")
	if !strings.Contains(schema, "status                  TEXT        NOT NULL DEFAULT 'planned'") {
		t.Error("expected schema.sql to declare status with DEFAULT 'planned'")
	}
	if !strings.Contains(schema, "planned_sessions_status_valido") {
		t.Error("expected schema.sql to declare the status CHECK constraint")
	}
	if !strings.Contains(schema, "planned_sessions_activity_unica UNIQUE") {
		t.Error("expected schema.sql to declare the UNIQUE constraint on activity_id")
	}
	if !strings.Contains(schema, "REFERENCES activities(id) ON DELETE SET NULL") {
		t.Error("expected schema.sql to declare activity_id with ON DELETE SET NULL")
	}
	for _, s := range plannedSessionsStatuses {
		if !strings.Contains(schema, "'"+s+"'") {
			t.Errorf("expected schema.sql to admit the status %q", s)
		}
	}
}

// TestQueries00017_LinkUnlinkAreOwnershipScoped proves the two new queries
// check the ownership of the session AND the activity in the same atomic
// statement, and that unlinking only clears the link.
func TestQueries00017_LinkUnlinkAreOwnershipScoped(t *testing.T) {
	queries := readPlannedSessionsFile(t, "..", "queries", "planned_sessions.sql")
	for _, name := range expectedPlannedSessionsLinkQueries {
		if !strings.Contains(queries, "-- name: "+name+" ") {
			t.Errorf("expected the query file to declare the query %q, got:\n%s", name, queries)
		}
	}

	link := plannedSessionsQueryBlock(t, queries, "LinkPlannedSessionActivity")
	if !strings.Contains(link, "a.user_id = $2") {
		t.Errorf("expected the link query to check the activity ownership atomically, got:\n%s", link)
	}
	if !strings.Contains(link, "s.user_id = $2") {
		t.Errorf("expected the link query to check the session ownership atomically, got:\n%s", link)
	}
	if !strings.Contains(link, "CASE WHEN s.status = 'modified' THEN 'modified' ELSE 'completed' END") {
		t.Errorf("expected linking to complete the session but keep 'modified', got:\n%s", link)
	}

	unlink := plannedSessionsQueryBlock(t, queries, "UnlinkPlannedSessionActivity")
	if !strings.Contains(unlink, "SET activity_id = NULL") {
		t.Errorf("expected the unlink query to clear activity_id, got:\n%s", unlink)
	}
	if strings.Contains(unlink, "status =") {
		t.Errorf("the unlink query must not touch the status, got:\n%s", unlink)
	}
	if !strings.Contains(unlink, "user_id = $2") {
		t.Errorf("expected the unlink query to filter by user_id, got:\n%s", unlink)
	}
}

// TestGenerated00017_ExposeTheQueryContract proves the generated code
// exposes the link queries.
func TestGenerated00017_ExposeTheQueryContract(t *testing.T) {
	generated := readPlannedSessionsFile(t, "planned_sessions.sql.go")
	for _, name := range expectedPlannedSessionsLinkQueries {
		if !strings.Contains(generated, "func (q *Queries) "+name+"(") {
			t.Errorf("expected the generated code to expose %s, got:\n%s", name, generated)
		}
	}
}
