//go:build integration

package app

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fgjcarlos/ghamusinos/internal/blocks"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/fgjcarlos/ghamusinos/internal/db/status"
	"github.com/fgjcarlos/ghamusinos/internal/reports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestIntegration_ReportSnapshotBlockIDMatchesKind(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("integration requires a migrated test database")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	q := sqlc.New(pool)
	key := uuid.NewString()
	user, err := q.CreateUser(ctx, sqlc.CreateUserParams{ClerkUserID: key, Email: key + "@example.test", InviteStatus: status.InviteStatusActive})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		require.NoError(t, err)
	})
	owner := uuid.UUID(user.ID.Bytes)
	block, err := blocks.NewStore(pool).Create(ctx, owner, blocks.Draft{Name: "Invariant", StartDate: "2026-10-05", EndDate: "2026-10-11"})
	require.NoError(t, err)
	start := pgtype.Date{Time: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), Valid: true}
	end := pgtype.Date{Time: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC), Valid: true}
	payload := []byte(`{"schema_version":1}`)

	// A block report without block_id and a non-block report with block_id are
	// both invalid: kind and block_id describe the same report identity.
	for _, tc := range []struct {
		name string
		kind string
		id   pgtype.UUID
	}{
		{name: "block without block_id", kind: "block"},
		{name: "week with block_id", kind: "week", id: pgtype.UUID{Bytes: block.ID, Valid: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, `INSERT INTO report_snapshots (user_id, kind, period_start, period_end, block_id, payload)
VALUES ($1, $2, $3, $4, $5, $6)`, user.ID, tc.kind, start, end, tc.id, payload)
			require.Error(t, err, "PostgreSQL must reject a report whose block_id presence does not match kind")
		})
	}
}

func TestIntegration_ReportSnapshotsUpsertAndScoping(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("integration requires a migrated test database")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	q := sqlc.New(pool)
	madrid, err := time.LoadLocation("Europe/Madrid")
	require.NoError(t, err)

	newUser := func() uuid.UUID {
		key := uuid.NewString()
		u, err := q.CreateUser(ctx, sqlc.CreateUserParams{ClerkUserID: key, Email: key + "@example.test", InviteStatus: status.InviteStatusActive})
		require.NoError(t, err)
		id := uuid.UUID(u.ID.Bytes)
		t.Cleanup(func() {
			_, err := pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", u.ID)
			require.NoError(t, err)
		})
		_, err = q.UpdateUserPreferences(ctx, sqlc.UpdateUserPreferencesParams{
			ID: pgtype.UUID{Bytes: id, Valid: true}, Timezone: "Europe/Madrid"})
		require.NoError(t, err)
		return id
	}
	// storedRows counts the snapshot rows of a user; the package has no list
	// query yet (#320 adds it), so the test reads the table directly.
	type storedRow struct {
		ID   uuid.UUID
		Kind string
	}
	storedRows := func(user uuid.UUID) []storedRow {
		t.Helper()
		rows, err := pool.Query(ctx, "SELECT id, kind FROM report_snapshots WHERE user_id = $1 ORDER BY kind, period_start", user)
		require.NoError(t, err)
		defer rows.Close()
		out := []storedRow{}
		for rows.Next() {
			var id pgtype.UUID
			var kind string
			require.NoError(t, rows.Scan(&id, &kind))
			out = append(out, storedRow{ID: uuid.UUID(id.Bytes), Kind: kind})
		}
		require.NoError(t, rows.Err())
		return out
	}

	store := reports.NewStore(q)
	owner, other := newUser(), newUser()
	week := time.Date(2026, 10, 28, 12, 0, 0, 0, madrid)

	// Regenerar el mismo periodo sustituye la fila: sigue habiendo una sola y
	// guarda el payload nuevo.
	first, err := store.GenerateWeek(ctx, owner, week)
	require.NoError(t, err)
	require.Equal(t, reports.KindWeek, first.Kind)
	require.Equal(t, "2026-10-26", first.PeriodStart)
	require.Equal(t, "2026-11-01", first.PeriodEnd)
	require.Zero(t, first.Load.TSS)
	require.Len(t, first.Load.Daily, 7)
	require.Len(t, storedRows(owner), 1)

	_, err = q.UpsertTrainingLoadDaily(ctx, sqlc.UpsertTrainingLoadDailyParams{
		UserID: pgtype.UUID{Bytes: owner, Valid: true}, Day: pgtype.Date{Time: time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC), Valid: true},
		Tss: 50, ActivityCount: 1, ComputedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}})
	require.NoError(t, err)
	second, err := store.GenerateWeek(ctx, owner, week)
	require.NoError(t, err)
	require.Equal(t, 50.0, second.Load.TSS)
	rows := storedRows(owner)
	require.Len(t, rows, 1)
	stored, err := store.Get(ctx, owner, rows[0].ID)
	require.NoError(t, err)
	require.Equal(t, reports.KindWeek, stored.Kind)
	require.Equal(t, second, stored.Payload)
	require.False(t, stored.GeneratedAt.IsZero())

	// Una semana, un mes y un bloque del mismo usuario son tres filas.
	month, err := store.GenerateMonth(ctx, owner, time.Date(2026, 10, 15, 12, 0, 0, 0, madrid))
	require.NoError(t, err)
	require.Equal(t, reports.KindMonth, month.Kind)
	require.Equal(t, "2026-10-01", month.PeriodStart)
	require.Equal(t, "2026-10-31", month.PeriodEnd)
	require.Len(t, month.Load.Daily, 31)

	block, err := blocks.NewStore(pool).Create(ctx, owner, blocks.Draft{Name: "Base", StartDate: "2026-10-05", EndDate: "2026-10-11"})
	require.NoError(t, err)
	blockReport, err := store.GenerateBlock(ctx, owner, block.ID, week)
	require.NoError(t, err)
	require.Equal(t, reports.KindBlock, blockReport.Kind)
	require.Equal(t, "2026-10-05", blockReport.PeriodStart)
	require.Equal(t, "2026-10-11", blockReport.PeriodEnd)
	require.Equal(t, &reports.BlockRef{ID: block.ID, Name: "Base"}, blockReport.Block)
	require.Nil(t, blockReport.Race)
	require.Equal(t, []string{"block", "month", "week"}, []string{storedRows(owner)[0].Kind, storedRows(owner)[1].Kind, storedRows(owner)[2].Kind})

	// Un bloque ajeno responde ErrNotFound y no escribe nada.
	_, err = store.GenerateBlock(ctx, other, block.ID, week)
	require.ErrorIs(t, err, reports.ErrNotFound)
	require.Empty(t, storedRows(other))
	require.Len(t, storedRows(owner), 3)

	// Borrar el bloque borra su reporte; semana y mes siguen.
	blockReportID := uuid.Nil
	for _, row := range storedRows(owner) {
		if row.Kind == string(reports.KindBlock) {
			blockReportID = row.ID
		}
	}
	require.NotEqual(t, uuid.Nil, blockReportID)
	require.NoError(t, blocks.NewStore(pool).Delete(ctx, owner, block.ID))
	require.Len(t, storedRows(owner), 2)
	_, err = store.Get(ctx, owner, blockReportID)
	require.ErrorIs(t, err, reports.ErrNotFound)
}
