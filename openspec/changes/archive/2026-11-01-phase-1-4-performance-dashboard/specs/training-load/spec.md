# Spec Delta: phase-1-4-performance-dashboard

> Domain: training-load
> Source: openspec/changes/phase-1-4-performance-dashboard/{proposal,exploration}.md
> Cross-project: `.` (Go)
> Stack: Go 1.26.0 + chi/v5 + pgx/v5 + goose/v3 + riverqueue + sqlc
> Gating test command: `make test`
> Quality gates: `make lint`, `make vet`, `make fmt`

This delta covers the persistence slice (PR2): two new
relational tables (`training_load_daily` as a TimescaleDB
hypertable, `dashboard_metadata` as a per-user singleton),
a new column on `users`, the TimescaleDB-guarded migration,
and the River job that recalculates the daily series with
in-flight deduplication. No canonical
`openspec/specs/training-load/spec.md` exists yet; archive will
copy this file into the canonical location.

## ADDED Requirements

### Requirement: `training_load_daily` is a TimescaleDB hypertable (`TL-001`)

`internal/db/migrations/00012_training_load_daily.sql`
SHALL create the table
`training_load_daily (user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE, day DATE NOT NULL, tss NUMERIC(8,2) NOT NULL DEFAULT 0, activity_count INT NOT NULL DEFAULT 0, distance_m NUMERIC(10,2) NOT NULL DEFAULT 0, elevation_gain_m NUMERIC(10,2) NOT NULL DEFAULT 0, computed_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY (user_id, day))`,
convert it into a hypertable via
`SELECT create_hypertable('training_load_daily', 'day', chunk_time_interval => INTERVAL '7 days', if_not_exists => TRUE)`,
and SHALL create the supporting index
`CREATE INDEX idx_training_load_daily_user_day_desc ON training_load_daily (user_id, day DESC)`.
The `goose Down` migration MUST drop both objects in
reverse order (index first, hypertable conversion unwound via
`DROP TABLE` since `create_hypertable` is implicit on the table
itself).

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/migrations/00012_training_load_daily.sql`

#### Scenario: hypertable is created with 7-day chunks and the supporting index

- GIVEN a clean database with the `timescaledb` extension
  already loaded by migration 00007
- WHEN `goose Up` runs migration 00012
- THEN the `training_load_daily` table exists with the
  columns and PRIMARY KEY listed above
- AND `_timescaledb_catalog.hypertable` contains a row for
  `training_load_daily` with `chunk_time_interval = INTERVAL '7 days'`
- AND the index `idx_training_load_daily_user_day_desc` exists
  on `(user_id, day DESC)`

#### Scenario: `goose Down` drops the table and the index symmetrically

- GIVEN migration 00012 is currently applied
- WHEN `goose Down` runs for migration 00012
- THEN the `idx_training_load_daily_user_day_desc` index no
  longer exists
- AND the `training_load_daily` table no longer exists
- AND `goose Up` re-applying it returns the table to the same
  state (idempotent re-run)

### Requirement: `dashboard_metadata` is a per-user singleton table (`TL-002`)

The same migration `00012_training_load_daily.sql` SHALL
also create
`dashboard_metadata (user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE, last_recalc_at TIMESTAMPTZ NULL, last_recalc_status TEXT NULL, training_load_rows INT NOT NULL DEFAULT 0)`.
The PRIMARY KEY enforces one row per user (the singleton
invariant). `goose Down` SHALL drop it in reverse order
before dropping the hypertable.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/migrations/00012_training_load_dashboard_metadata` block

#### Scenario: dashboard_metadata singleton invariant is enforced by PRIMARY KEY

- GIVEN a user `U` with no row in `dashboard_metadata`
- WHEN two raw `INSERT INTO dashboard_metadata (user_id,
  ...) VALUES (U, ...)` statements run in succession
- THEN the second INSERT fails with a unique-constraint /
  primary-key violation
- AND only the upsert path (`ON CONFLICT (user_id) DO UPDATE`)
  can replace the row for the same user

#### Scenario: cascade delete removes dashboard_metadata when the user is removed

- GIVEN a row in `dashboard_metadata` for user `U`
- WHEN `DELETE FROM users WHERE id = U` runs
- THEN the corresponding `dashboard_metadata` row is also
  removed (ON DELETE CASCADE)

### Requirement: `users.running_threshold_sec_per_km` is added with a CHECK guard (`TL-003`)

`internal/db/migrations/00013_users_running_threshold.sql`
SHALL add the column
`ALTER TABLE users ADD COLUMN running_threshold_sec_per_km SMALLINT NULL`;
`running_threshold_sec_per_km` accepts 120..1800 seconds/km,
with the constraint
`CHECK (running_threshold_sec_per_km IS NULL OR (running_threshold_sec_per_km BETWEEN 120 AND 1800))`
(2:00/km to 30:00/km — a sane physiological range; rejects
obvious garbage at the DB level). The `goose Down` migration
SHALL drop the column (and the CHECK) symmetrically. This
column resolves the proposal's G1 decision: running threshold
pace is not derivable from HR max alone and must be settable
by the user or anchored to a race result.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/migrations/00013_users_running_threshold.sql`

#### Scenario: a valid threshold within range is accepted

- GIVEN the column exists with the CHECK constraint
- WHEN `UPDATE users SET running_threshold_sec_per_km = 240 WHERE id = U` (4:00/km)
- THEN the row updates successfully
- AND reading the column back returns `240`

#### Scenario: an out-of-range threshold is rejected at the DB layer

- GIVEN the column exists with the CHECK constraint
- WHEN `UPDATE users SET running_threshold_sec_per_km = 30 WHERE id = U` (0:30/km — implausible)
- THEN the UPDATE fails with a CHECK-constraint violation
- AND the column value for `U` is unchanged

#### Scenario: NULL threshold is allowed (user has not set it yet)

- GIVEN the column exists with the CHECK constraint
- WHEN `UPDATE users SET running_threshold_sec_per_km = NULL WHERE id = U`
- THEN the row updates successfully
- AND PR3's `/dashboard/load` and `TSSRunning` callers can
  detect "not set" via `NULL` without inventing a proxy

### Requirement: Migration 00012 is guarded against non-timescaledb dev DBs (`TL-004`)

The `create_hypertable` block in `00012_training_load_daily.sql`
SHALL be wrapped in
`DO $$ BEGIN IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb') THEN … create_hypertable … END IF; END $$;`
so that a developer database without TimescaleDB boots cleanly
(skipping the hypertable conversion while still creating the
plain table). The CI smoke runs against
`timescaledb/timescaledb-ha:pg16` so the `create_hypertable`
happy path is exercised there. This guard resolves the
exploration's E9 risk.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/migrations/00012_training_load_daily.sql` TimescaleDB guard

#### Scenario: migration applies cleanly without TimescaleDB

- GIVEN a developer database with `CREATE EXTENSION timescaledb` not invoked
- WHEN `goose Up` runs migration 00012
- THEN the table `training_load_daily` is created
- AND the `_timescaledb_catalog.hypertable` table has no row
  for `training_load_daily` (the guard skipped the call)
- AND the migration records itself as applied in `goose_db_version`

#### Scenario: migration applies with hypertable when TimescaleDB is loaded

- GIVEN a CI database with `timescaledb` loaded
- WHEN `goose Up` runs migration 00012
- THEN `_timescaledb_catalog.hypertable` contains a row for
  `training_load_daily` (the guard satisfied and the call ran)

### Requirement: Migration 00013 is symmetrically reversible (`TL-005`)

`00013_users_running_threshold.sql` SHALL have a `goose Up`
that adds the column with its CHECK, and a `goose Down` that
drops the column (the CHECK is dropped with the column).
Re-applying `Up` after `Down` SHALL be idempotent. No
TimescaleDB guard is required for 00013 since it does not
touch TimescaleDB primitives.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/migrations/00013_users_running_threshold.sql`

#### Scenario: 00013 Up + Down + Up re-applies cleanly

- GIVEN migration 00013 has not been applied
- WHEN `goose Up; goose Down; goose Up` runs in sequence
- THEN the column exists at the end of the sequence
- AND the CHECK constraint is intact
- AND `goose_db_version` records 00013 as applied exactly once

### Requirement: `RecalcTrainingLoad` River job deduplicates in-flight enqueues (`TL-006`)

`internal/jobs/recalc_training_load.go` SHALL register a
River worker with args
`RecalcTrainingLoadArgs{UserID uuid.UUID, From *time.Time, To *time.Time}`.
The enqueue path SHALL pass `river.UniqueOpts` with key
`recalc_training_load:{user_id}:{window_hash}` (where
`window_hash` includes the ±3-day range) and a 30-second TTL
so a flood of identical post-upsert enqueues collapses to one
in-flight job per user. Execution SHALL call
`metrics.ComputeDailyLoad(ctx, pool, userID, from, to)` and
update `dashboard_metadata.last_recalc_at`,
`last_recalc_status`, `training_load_rows` via the
`UpsertDashboardMetadata` SQLC query. When `From`/`To` are
nil, the job SHALL default to "first activity → now" via
`ListDailyLoadFromFirstActivity` (or the equivalent
first-activity query). The post-`UpsertActivity` hook in
`internal/strava` SHALL enqueue with
`From = started_at − 3d`, `To = started_at + 3d` so the EMA
window stays locally consistent without a full-history re-scan.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/jobs/recalc_training_load.go`

#### Scenario: 50 identical post-upsert enqueues collapse to one in-flight job (E7)

- GIVEN a mock River client that records enqueue calls
- WHEN the post-`UpsertActivity` hook enqueues
  `RecalcTrainingLoadArgs{UserID, From: t-3d, To: t+3d}` 50
  times for the same `UserID` within the 30-second TTL
- THEN the mock River client records at most one effective
  job (UniqueOpts coalesces the duplicates) for that
  `(UserID, window_hash)` key
- AND `dashboard_metadata.training_load_rows` reflects one
  recalc, not 50

#### Scenario: the default nil window expands to "first activity → now"

- GIVEN a user whose earliest activity is at `2025-01-01` and
  the current time is `2026-09-30`
- WHEN `RecalcTrainingLoad{UserID, From: nil, To: nil}` runs
- THEN the worker queries `ListDailyLoadFromFirstActivity(UserID)`
  to discover the first-activity date
- AND it executes `ComputeDailyLoad(ctx, pool, UserID,
  2025-01-01, 2026-09-30)`
- AND it writes `last_recalc_at = now()`, `last_recalc_status
  = "ok"` (or `"failed"` on error), and
  `training_load_rows = <count>` to `dashboard_metadata`

### Requirement: Full-history recalculation is driven by `POST /api/v1/dashboard/recalc` (`TL-007`)

`POST /api/v1/dashboard/recalc` (see `dashboard-api` `DA-004`)
SHALL enqueue `RecalcTrainingLoadArgs{UserID, From: nil,
To: nil}` for the authenticated user so a 5-year-history
recalc runs asynchronously. The job writes back the
resulting row count and status to
`dashboard_metadata.training_load_rows` /
`last_recalc_status` so that `GET /healthz` (with
`X-Internal-Health: 1`) and `GET /api/v1/dashboard/summary`
can observe progress. The endpoint returns `202 Accepted`
immediately; the user does not block on the work.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/jobs/recalc_training_load.go` + `internal/http/handlers/dashboard_recalc.go`

#### Scenario: an enqueued full-history recalc updates dashboard_metadata on completion

- GIVEN a user with 5 years of activities and an empty
  `dashboard_metadata` row
- WHEN `POST /api/v1/dashboard/recalc` is invoked (handler
  enqueues the job with `From = nil, To = nil`) and the
  worker eventually runs to completion
- THEN `dashboard_metadata.last_recalc_at` is set to the
  completion timestamp
- AND `dashboard_metadata.last_recalc_status` is `"ok"`
- AND `dashboard_metadata.training_load_rows` equals the
  number of distinct calendar days with at least one
  activity in the user's history

#### Scenario: failed recalc records an error status

- GIVEN a recalc that fails mid-execution (e.g.
  `ComputeDailyLoad` returns an error)
- WHEN the worker's `defer` (or equivalent error branch) runs
- THEN `dashboard_metadata.last_recalc_status` is `"failed"`
  (not `"ok"`)
- AND `last_recalc_at` still records the attempt timestamp so
  observability tools can detect the failure
