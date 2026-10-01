# Proposal: phase-1-4-performance-dashboard

> Phase 1.4 close-out slice. Closes issue epic #16
> ("Fase 1.4 — Dashboard de rendimiento y salud/fatiga") by
> delivering six ported performance/health metrics
> (TSS, IF, GAP, EF, Cardiac Drift, CTL/ATL/TSB), a
> TimescaleDB-backed daily training-load series, three
> `/api/v1/dashboard/*` endpoints, an extended
> `/healthz` and a web `/dashboard` view rendered with
> ECharts. User-confirmed delivery strategy: **4-PR
> stacked-to-main chain**, ~400 LoC per PR ceiling,
> each PR mergeable with green CI and no residual
> dependents. Forecast ~1,400 LoC total: PR1 ~280
> (pure metrics), PR2 ~360 (TimescaleDB persistence +
> users column), PR3 ~340 (API + extended health),
> PR4 ~340 (web dashboard).

## Intent

Issue #16 (OPEN, label `phase:1.4`) defines "Fase 1.4 —
Dashboard de rendimiento y salud/fatiga" as the slice
that exposes six performance/health metrics to the
athlete. The architecture inventory
(`docs/architecture/feature-inventory.md:§7`) lists
them: **TSS** (cycling + running), **Intensity Factor**,
**Grade Adjusted Pace**, **Efficiency Factor**,
**Cardiac Drift**, **CTL/ATL/TSB** (PMC of Coggan
with empty-day fill). §8 specifies the dashboard
surface (weekly volume, elevation, activity count,
trend deltas, line chart of fitness/fatigue/form,
stacked-bar HR-zone distribution, sparklines) and
mandates **ECharts** (the inventory moves the chart
library off Recharts). §11 adds an extended `/healthz`
surface exposing DB liveness, last-recalc timestamp,
and training-load row counts.

Today the codebase ships the **data substrate** that
makes those metrics computable — `activities`
(`internal/db/migrations/00004_strava_activities.sql:30-44`
carries `avg_hr`, `max_hr`, `avg_power`, `elapsed_seconds`,
`moving_seconds`, `started_at`, `sport_type`),
`activity_streams` (JSONB with `stream_type ∈ {heartrate,
watts, cadence, altitude, latlng, ...}` per line 48-53),
`hr_zones` (migration `00005_hr_zones.sql`),
`user_preferences` with `hr_max` / `lthr` / `ftp`
(migration `00003_user_preferences.sql:23-26`), and
TimescaleDB already loaded
(`00007_timescaledb_extension.sql:7` literally names
`training_load_daily` as a future use case) — but **no
metric implementation**, no training-load series, no
dashboard endpoints, no dashboard UI. The closest
existing slice is `openspec/changes/archive/2026-09-30-phase-1-3-km-vertical-and-king-climb/`
which established the "three tables, not JSONB" pattern
for persistence (`gpx_muros`, `gpx_recovery_zones`,
`gpx_km_vertical` in migration `00011`); PR2 mirrors
that pattern with `training_load_daily` (and a
hypertable on top).

This proposal is therefore not a "fix a bug" slice but
a "build the missing layer" slice, scoped strictly to
the inventory's §7/§8/§11 surface. Every metric
implementation is a pure function (no I/O) so PR1
itself has zero migration or HTTP surface — that is
what makes the 4-PR split cheap to review.

## Scope

### In scope

- **Six ported metrics in pure Go** (`internal/metrics/`):
  - `TSSCycling(ftp, durationSec, npWatts)` — TrainingPeaks
    formula.
  - `TSSRunning(thresholdSecPerKm, durationSec, actualSecPerKm)`
    — canonical formula: `(durationSec / 3600) ×
    (thresholdSecPerKm / actualSecPerKm)^2 × 100` (M-002). A
    faster-than-threshold pace therefore has intensity factor > 1.
  - `IntensityFactor(ftp, npWatts)` — Coggan IF.
  - `GradeAdjustedPace(grade, paceSecPerKm)` — Minetti
    curve, normalized against Strava GAP convention
    (E6 risk).
  - `EfficiencyFactor(npWatts, avgHR)` — Coggan EF.
  - `CardiacDrift(hrStart, hrEnd)` and
    `CardiacDriftSeries([]int)` — Minetti/Cardiac-Drift
    convention (Pauley, Laukkanen).
  - `CTL([]DailyLoad)`, `ATL([]DailyLoad)`, `TSB(ctl, atl)`
    — PMC of Coggan (42-day / 7-day EMA) with empty-day
    fill of `FillMissingDays(from, to, raw) []DailyLoad`
    returning zeros for missing days.
- **TimescaleDB persistence** (PR2): migration
  `00012_training_load_daily.sql` with hypertable on
  `training_load_daily (user_id, day, tss, activity_count,
  distance_m, elevation_gain_m, computed_at)`; SQLC
  queries `UpsertDailyLoad`, `ListDailyLoadByUserRange`,
  `ListDailyLoadFromFirstActivity`,
  `ListUserIdsWithActivities`; service
  `ComputeDailyLoad(ctx, db, userID, from, to)`;
  River job `recalc_training_load` (deduplicated per
  E7); hook post-`UpsertActivity` enqueuing a
  ±3-day window recalc.
- **Users column** (PR2, G1): migration `00013` adds
  `users.running_threshold_sec_per_km SMALLINT NULL`
  so PR3 can read it from the users table.
- **Dashboard metadata table** (PR2, G2): migration
  `00012` also creates
  `dashboard_metadata (user_id PK, last_recalc_at,
  last_recalc_status, training_load_rows)` so PR3's
  `/healthz` reads `last_recalc_at` and
  `training_load_rows` directly instead of guessing.
- **Three dashboard endpoints** (PR3):
  - `GET /api/v1/dashboard/summary` → `{ weekly_volume_m,
    weekly_elevation_m, weekly_activities_count,
    trend_7d_pct, trend_30d_pct }`.
  - `GET /api/v1/dashboard/load?from=&to=` → daily
    `{ day, ctl, atl, tsb }` series.
  - `GET /api/v1/dashboard/hr-zones?from=&to=` →
    per-zone minutes aggregated from `activity_streams`
    (`heartrate`) when available, fallback to
    `activities.avg_hr * elapsed_seconds` partition.
  - `POST /api/v1/dashboard/recalc` (G4) — manual
    full-history trigger for users with >1 year of
    activities.
- **Extended `/healthz`** (PR3, G3): public body
  remains `{ status: "ok" }`. With the
  `X-Internal-Health: 1` header the response gains
  `{ db: { ok, latency_ms }, strava: { ok, last_sync_at },
  last_recalc_at, training_load_rows, ai: { placeholder } }`.
- **Web dashboard** (PR4):
  - `web/src/lib/api/dashboard.ts` — types and
    fetcher mirroring the Go handlers.
  - `useDashboardSummary`, `useTrainingLoad(from, to)`,
    `useHRZones(from, to)` hooks.
  - `DashboardSummaryCard.tsx` (4 KPIs + sparkline),
    `TrainingLoadChart.tsx` (ECharts line CTL/ATL/TSB
    with period selector), `HRZonesChart.tsx`
    (ECharts stacked-bar).
  - Page `/dashboard` mounted in the existing shell;
    `E1` resolved by adding `echarts` to
    `web/package.json` as the very first PR4 commit.
  - `G5` enforced: Cardiac Drift panel renders only
    when HR streams exist for the queried range; otherwise
    the panel is hidden with the empty-state label
    "necesita streams HR para calcular".
- **Tests at every boundary the chain touches**:
  metric unit tests (table-driven with literature
  goldens + degenerate cases), service unit tests
  against SQLC mocks, handler tests with
  `httptest` + chi, web component tests with
  Vitest + Testing Library, plus the
  TimescaleDB migration forward/back smoke.

### Out of scope

- **AI applied to metrics** (Fase 1.5 per
  `feature-inventory.md:§9`). The `ai` block in
  `/healthz` is a placeholder only.
- **Planning (V2).** Separate roadmap slice.
- **Advanced 1.6 GPX lab features.** Out of
  `phase:1.4` scope even though migration `00011`
  is in this repo.
- **3D MapLibre + slope heatmap.** Explicit Fase
  1.6 per `feature-inventory.md:108`. Not touched.
- **Backfilling pre-existing tracks with metrics.**
  Metrics compute from activities, not GPX tracks,
  so this concern does not apply for 1.4. Pre-1.2
  activities compute on first recalc, gated by
  `POST /api/v1/dashboard/recalc`.
- **Comparator (`computeDiff`) extension.** Same
  posture as `closes-159-strava-disconnect` and
  `phase-1-3-km-vertical-and-king-climb`: comparator
  stays at its current surface.
- **OAuth/Strava lifecycle.** Separate change.
- **Re-deriving metrics from `activity_streams`
  beyond what `TSSRunning` and HR-zones already need.**
  Stream parsing beyond the four uses already in the
  inventory is out.
- **Visible UI for `elevation_coverage`.** Separate
  concern.

## Approach

### Column / table placement decisions

**`training_load_daily` as its own hypertable, not as
JSONB inside `users` or `activities`.** Rationale
mirrors the decision rationale from
`openspec/changes/archive/2026-09-30-phase-1-3-km-vertical-and-king-climb/proposal.md:Column
placement decision`:

1. **Shape consistency.** The repo's pattern
   (`gpx_climbs`, `gpx_risk_zones`, `gpx_muros`,
   `gpx_recovery_zones`, `gpx_km_vertical` in
   migrations `00006` and `00011`) is "one feature,
   one relational table, foreign-keyed to the
   aggregate root." JSONB on `activities` would break
   the convention.
2. **Time-series semantics.** TimescaleDB hypertables
   need `(user_id, day)` as the time index and chunk
   by `INTERVAL '7 days'`. JSONB cannot model that.
3. **HR-zone distribution is per-user per-zone per-day.**
   The `/dashboard/hr-zones` aggregation collapses to
   the response shape, so it does not need its own
   table; it computes from `activity_streams` on
   demand. Same posture as `closes-159-strava-disconnect`'s
   "compute on read" stance for hot data.

**`dashboard_metadata` as a separate table, not as
columns on `users`.** Rationale (G2): `users` is
already a 7-table-wide collection (`00001_users_invites.sql`
+ `00003_user_preferences.sql`); a `last_recalc_at`
column would invite every other future "last X" to
land on `users` too. `dashboard_metadata` keys by
`user_id` PK and carries exactly the four fields the
extended `/healthz` needs (E2). Adding future
per-user dashboard state (e.g. last-viewed period,
preferences cached) does not touch `users`.

**`users.running_threshold_sec_per_km` (120..1800) as a
`SMALLINT NULL` column, not derived from HR max.**
Rationale (G1): Running threshold pace is not a
function of HR max or LTHR alone — it requires a
field test (or race-result anchor). The nullable
column accepts a future "I haven't set this yet"
state without forcing PR1/PR3 to invent a proxy
formula. The valid range is **120..1800 seconds/km**,
aligned with migration 00013 and TL-003.

**`X-Internal-Health` header, not a `/healthz/full`
endpoint.** Rationale (G3): keeps the URL space
flat (one health endpoint, two contract depths),
avoids leaking implementation depth in the route
table, and matches the inventory's "extend `/healthz`"
phrasing. Public shape is unchanged.

### Risk-bearing decisions (carried from exploration E1–E9)

These are documented in
`openspec/changes/phase-1-4-performance-dashboard/exploration.md:§Riesgos`
and are consigned here without rewriting them:

- **E1 — ECharts absent from `web/package.json`.** PR4's
  first commit adds `echarts` before any import. The
  proposal makes this an explicit ordering rule so the
  PR4 diff is green on first CI run.
- **E2 — `last_recalc_at` resolution.** Resolved by G2
  (PR2 creates `dashboard_metadata`).
- **E3 — Cardiac Drift without streams.** Resolved by
  G5 (panel hidden or labelled empty when no HR
  streams).
- **E4 — `users.running_threshold_sec_per_km`.**
  Resolved by G1 (PR2 adds the column).
- **E5 — User timezone.** PR1 metric math is
  timezone-free (pure functions of durations and
  paces). PR2 groups `training_load_daily` by the
  user's local day: PR2 converts `started_at` to
  `users.timezone` (column already in `00003`) before
  bucketing. PR3 endpoints return ISO dates without
  TZ suffix and the web layer formats them client-side
  using the same `users.timezone`.
- **E6 — GAP normalization.** PR1 uses the Strava
  (Minetti-normalized) convention so the metric
  composes with Strava-sourced `avg_pace` and
  `avg_speed`. The choice is documented in
  `internal/metrics/SPEC.md` alongside the formula.
- **E7 — River job deduplication.** PR2's
  `recalc_training_load` River job uses
  `riverqueue/river`'s `UniqueOpts` (job key
  `recalc_training_load:{user_id}:{window_hash}`) so a
  flood of post-upsert enqueues collapses to one
  in-flight job per user. The window hash includes the
  ±3-day range so two distinct ranges remain distinct
  jobs.
- **E8 — `/healthz` extension breaking.** Resolved by G3
  (header-gated extension; public body unchanged).
- **E9 — TimescaleDB guard.** The migration runner
  `internal/db/migrations.go` already guards on
  `SELECT extname FROM pg_extension WHERE extname =
  'timescaledb'` and skips `00012` cleanly if the
  extension is missing — verified against the existing
  CI smoke. The CI matrix runs the migrations container
  with `timescaledb/timescaledb-ha:pg16` so 00012's
  `create_hypertable` is exercised in the smoke.

### PR1 — Pure-Go metrics (TDD)

**Project root:** `.` (Go). **Gating test command:**
`make test`. **Quality gates:** `make lint`, `make vet`,
`make fmt`.

1. **`internal/metrics/SPEC.md`** — one paragraph per
   metric: source (TrainingPeaks / Coggan / Minetti /
   Pauley), formula, unit contract, sentinel for
   degenerate inputs, and at least one golden test
   case from literature. Single source of truth for
   "what does this function promise".
2. **`internal/metrics/performance.go`** —
   `TSSCycling`, `TSSRunning`, `IntensityFactor`,
   `GradeAdjustedPace`, `EfficiencyFactor`. All
   pure, all unit-testable in isolation. Sentinel
   behavior: `ftp <= 0` or `durationSec <= 0` returns
   `0` (no NaN, no panic).
3. **`internal/metrics/health.go`** —
   `CardiacDrift(hrStart, hrEnd) float64` returns
   `(hrEnd - hrStart) / hrStart * 100` (Pauley
   convention). `CardiacDriftSeries([]int) float64`
   averages per-pair across the slice; returns `0`
   when the slice has fewer than two samples.
4. **`internal/metrics/fatigue.go`** — `CTL`,
   `ATL`, `TSB`, `FillMissingDays(from, to, raw)`. The
   empty-day filler guarantees one row per calendar
   day in `[from, to]` inclusive, with `TSS = 0` for
   any missing day; downstream EMA never sees a hole.
5. **`internal/metrics/*_test.go`** — table-driven
   per metric with three cases each: golden from
   literature, degenerate (zero / negative inputs),
   and a documented edge (e.g. very long activity,
   very short activity, FTP zero).
6. **No migrations, no SQL, no HTTP, no web.** PR1's
   diff is contained to `internal/metrics/`. Reviewers
   read 6 files of pure math.

#### PR1 file-by-file

| File | Purpose |
|---|---|
| `internal/metrics/SPEC.md` | Formula reference |
| `internal/metrics/performance.go` | TSS/IF/GAP/EF |
| `internal/metrics/health.go` | Cardiac Drift |
| `internal/metrics/fatigue.go` | CTL/ATL/TSB + filler |
| `internal/metrics/performance_test.go` | 4 metrics |
| `internal/metrics/health_test.go` | Cardiac Drift |
| `internal/metrics/fatigue_test.go` | CTL/ATL/TSB |
| `internal/metrics/SPEC_test.go` | Executable godoc examples |

### PR2 — TimescaleDB persistence + users column

**Project root:** `.` (Go). **Gating test command:**
`make test`. **Quality gates:** `make lint`, `make vet`,
`make fmt`, plus CI migration smoke with
`timescaledb/timescaledb-ha:pg16`.

1. **Migration `internal/db/migrations/00012_training_load_daily.sql`:**
   - `training_load_daily (user_id UUID NOT NULL REFERENCES
     users(id) ON DELETE CASCADE, day DATE NOT NULL,
     tss NUMERIC(8,2) NOT NULL DEFAULT 0, activity_count
     INT NOT NULL DEFAULT 0, distance_m NUMERIC(10,2) NOT
     NULL DEFAULT 0, elevation_gain_m NUMERIC(10,2) NOT
     NULL DEFAULT 0, computed_at TIMESTAMPTZ NOT NULL
     DEFAULT now(), PRIMARY KEY (user_id, day))`.
   - `SELECT create_hypertable('training_load_daily',
     'day', chunk_time_interval => INTERVAL '7 days',
     if_not_exists => TRUE);`.
   - `CREATE INDEX idx_training_load_daily_user_day_desc ON
     training_load_daily (user_id, day DESC);`.
   - `dashboard_metadata (user_id UUID PRIMARY KEY REFERENCES
     users(id) ON DELETE CASCADE, last_recalc_at TIMESTAMPTZ
     NULL, last_recalc_status TEXT NULL, training_load_rows
     INT NOT NULL DEFAULT 0)`.
   - `goose Up`/`Down` symmetric; `Down` drops both
     objects in reverse order. Tables are net-new; no
     backfill needed.
   - **TimescaleDB guard** (E9): the migration wraps the
     `create_hypertable` call in `DO $$ BEGIN IF EXISTS
     (SELECT 1 FROM pg_extension WHERE extname =
     'timescaledb') THEN … END IF; END $$;` so a
     non-timescaledb dev DB boots cleanly. CI smoke
     with the timescaledb image still exercises the
     `create_hypertable` happy path.
2. **Migration `internal/db/migrations/00013_users_running_threshold.sql`:**
   - `ALTER TABLE users ADD COLUMN running_threshold_sec_per_km
     SMALLINT NULL;`.
   - `CHECK (running_threshold_sec_per_km IS NULL OR
     (running_threshold_sec_per_km BETWEEN 120 AND 1800))`
     (a sane 2:00/km – 30:00/km range; reject obvious
     garbage at the DB level).
   - `goose Up`/`Down` symmetric.
3. **SQLC queries** under `internal/db/queries/`:
   - `training_load.sql`:
     `UpsertDailyLoad(:one)`,
     `ListDailyLoadByUserRange(:many ORDER BY day ASC)`,
     `ListDailyLoadFromFirstActivity(:many)`,
     `ListUserIdsWithActivities(:many)`.
   - `dashboard_metadata.sql`:
     `UpsertDashboardMetadata(:one)`,
     `GetDashboardMetadata(:one)`.
   - `users.sql` extension: `GetUserRunningThreshold(:one)`.
   - `make generate` regens `internal/db/sqlc/*`.
4. **`internal/metrics/training_load.go`** — service
   layer (not pure): `ComputeDailyLoad(ctx, db, userID,
   from, to) (rows []DailyLoadRow, err error)` iterates
   `activities` in the range, groups by
   `started_at` converted to `users.timezone`, computes
   per-day TSS via PR1's `TSSCycling` /
   `TSSRunning`. Idempotent: re-running for the same
   window yields the same rows; the service updates
   `computed_at` to `now()` and overwrites `tss`,
   `activity_count`, `distance_m`, `elevation_gain_m`
   via `UpsertDailyLoad`.
5. **`internal/jobs/recalc_training_load.go`** —
   `RiverJob` arg `(user_id UUID, from *time.Time, to
   *time.Time)`. When `from`/`to` are nil, defaults to
   "first activity → now". Job execution calls
   `ComputeDailyLoad` for the window and updates
   `dashboard_metadata.last_recalc_at`,
   `last_recalc_status`, `training_load_rows`. Uses
   `riverqueue/river`'s `UniqueOpts` keyed
   `recalc_training_load:{user_id}:{window_hash}` with
   a 30-second TTL for in-flight deduplication (E7).
6. **`internal/strava` post-`UpsertActivity` hook**
   (wherever that event currently emits, per the
   `closes-159-strava-disconnect` wiring): after a
   successful upsert, enqueue the job for `(user_id,
   started_at - 3d, started_at + 3d)` so the EMA
   window stays locally consistent without full-history
   re-scan.
7. **Tests**:
   - `00012` forward/back CI smoke (already wired in
     `internal/db/migrations_test.go` template; PR2
     adds the test for `00012` specifically).
   - `00013` forward/back CI smoke (analogous).
   - `TestComputeDailyLoadGroupsByDay` with synthetic
     activities in mixed timezones — verifies E5
     (local-day bucketing).
   - `TestUpsertDailyLoadIdempotent` — second run
     yields identical row count.
   - `TestRecalcTrainingLoadRiverJobDeduplicates` —
     mock River client receives one enqueue despite N
     identical (user_id, window) enqueue attempts.
   - Mock runner — no live River needed for
     `go test ./...`.

### PR3 — Dashboard API + extended `/healthz`

**Project root:** `.` (Go). **Gating test command:**
`make test`. **Quality gates:** `make lint`, `make vet`,
`make fmt`.

1. **`internal/http/handlers/dashboard.go`** — three
   handlers:
   - `Summary`: returns weekly volume/elevation/
     activity count from `training_load_daily WHERE
     day >= date_trunc('week', now())`, plus 7d and
     30d trends (`(current_window_avg -
     previous_window_avg) / previous_window_avg`,
     `null` when previous window has zero rows).
   - `Load(from, to)`: returns daily `{ day, ctl,
     atl, tsb }`. If `from` predates the user's first
     activity, fill with zeros; compute CTL/ATL/TSB
     from the filled series via PR1 functions. Cache
     header `Cache-Control: private, max-age=60` (60
     seconds; the data refreshes on job completion
     and `/recalc`).
   - `HRZones(from, to)`: read `activity_streams`
     rows where `stream_type = 'heartrate'` and
     `activity_id IN (SELECT id FROM activities
     WHERE started_at BETWEEN from AND to AND
     user_id = $1)`. Aggregate minutes in
     `[hr_max * pct_i, hr_max * pct_{i+1})` using the
     `hr_zones` table's bounds. When the streams
     query returns zero rows, fall back to
     `activities.avg_hr * elapsed_seconds / 60`
     bucketed into the same zone, **and** set the
     `degraded: true` flag in the response so the UI
     can label the chart accordingly.
2. **`internal/http/handlers/dashboard_recalc.go`** —
   `POST /api/v1/dashboard/recalc` (G4): enqueue
   `recalc_training_load` for the requesting user
   with `from = nil, to = nil` (full history).
   Response: `202 Accepted` with
   `{ job_id, queued_at, will_recompute_from }`. Auth:
   same JWT middleware as the rest of the API.
3. **`internal/http/handlers/health.go`** — extend
   `/healthz`:
   - Public shape unchanged: `{ "status": "ok" }` for
     the unadorned request.
   - With header `X-Internal-Health: 1` (G3):
     ```json
     {
       "status": "ok",
       "db": { "ok": true, "latency_ms": 4 },
       "strava": { "ok": true, "last_sync_at": "..." },
       "last_recalc_at": "2026-10-01T...",
       "training_load_rows": 12345,
       "ai": { "enabled": false }
     }
     ```
   - When the DB ping fails, response is `503` with
     `{ "status": "degraded", "db": { "ok": false } }`
     and other sections omitted. Header requirement
     is independent of the 503 path.
   - `ai.enabled` reads `users.ai_enabled` (column
     already present from migration 00003); the
     section stays present regardless so the schema
     is stable when AI lands in 1.5.
4. **Router wiring** in `internal/app/router.go`: the
   three GET endpoints mount under
   `/api/v1/dashboard/*` with the same auth middleware
   as `/api/v1/activities`. `POST /recalc` lives next
   to them.
5. **Tests**:
   - `TestDashboardSummaryReturnsWeeklyCounts` —
     table-driven across empty / 7-day / 30-day /
     trend-zero / trend-positive fixtures.
   - `TestDashboardLoadFillsPreFirstActivity` —
     `from` predates user, response has zero
     `ctl/atl/tsb` for those days.
   - `TestDashboardHRZonesStreamsPreferredOverAvg` —
     when streams exist for the range, `degraded` is
     `false`; when only `avg_hr` is present,
     `degraded` is `true`.
   - `TestDashboardRecalcReturns202WithJobID` —
     mock River enqueue receives the right args.
   - `TestHealthzPublicUnchanged` — no header →
     `{ "status": "ok" }`.
   - `TestHealthzInternalIncludesExtendedSections` —
     header present → full body.
   - `TestHealthzInternalDegradedWhenDBDown` — DB
     mock returns error → 503 with `db.ok = false`.
   - All handlers pinned to `httptest` +
     `chi` + SQLC mock; no live PostgreSQL.

### PR4 — Web dashboard

**Project root:** `web/`. **Gating test command:**
`pnpm -C web test:run`. **Quality gates:**
`pnpm -C web typecheck`, `pnpm -C web lint`,
`pnpm -C web format:check` over changed files.

1. **First commit (E1 fix):**
   `pnpm add echarts @types/echarts` (or whatever the
   inventory's pinned version is) and commit
   independently before any code that imports it. CI
   proves `package.json` and `pnpm-lock.yaml` are
   consistent.
2. **`web/src/lib/api/dashboard.ts`** — types and
   fetchers mirroring the Go handlers:
   - `DashboardSummary`, `TrainingLoadPoint`,
     `HRZonesPoint`, `RecalcResponse`.
   - `getDashboardSummary()`, `getTrainingLoad(from,
     to)`, `getHRZones(from, to)`, `postRecalc()`.
3. **Hooks** under `web/src/features/dashboard/`:
   - `useDashboardSummary()` — `useSWR` or the
     existing fetcher hook (whichever the
     `closes-159-strava-disconnect` slice standardized
     on).
   - `useTrainingLoad(from, to)` — re-fetch on
     `(from, to)` change.
   - `useHRZones(from, to)`.
4. **Components**:
   - `DashboardSummaryCard.tsx` — four KPI tiles
     with sparkline per tile, pulled from the
     summary endpoint. Empty state when no activities.
   - `TrainingLoadChart.tsx` — ECharts line chart
     rendering CTL / ATL / TSB with a 7d / 30d / 90d /
     1y period selector (matches the existing
     `PeriodSelector` if there is one in the SPA).
     Tokens via `var(--gh-*)`; no hardcoded colors.
   - `HRZonesChart.tsx` — ECharts stacked-bar over
     activities or aggregated by period. When the
     server response has `degraded: true`, the chart
     header labels itself "aproximación (sin streams
     HR)".
   - `CardiacDriftPanel.tsx` — conditionally
     rendered (G5): the panel queries
     `activity_streams` existence via a small
     dedicated endpoint OR via a header in the HR
     zones response; when no HR streams are present,
     the panel is hidden with the empty-state label
     "necesita streams HR para calcular". PR4 ships
     the conditional render and the empty state;
     PR3 does not need to change to accommodate it
     because the panel reads its own dependency.
5. **Page mount** — `/dashboard` route added to the
   existing shell layout; navigation entry in the
   sidebar / nav bar following the existing pattern.
6. **i18n / tokens** — all colors via `var(--gh-*)`
   from `web/src/styles/tokens.css`. Format dates
   with the user timezone already cached on the
   client (no new timezone logic in web).
7. **Tests**:
   - `DashboardSummaryCard.test.tsx` — empty state
     + populated state.
   - `TrainingLoadChart.test.tsx` — renders
     CTL/ATL/TSB lines; period selector flips
     between 7d/30d/90d/1y by re-querying the hook.
   - `HRZonesChart.test.tsx` — `degraded: true`
     shows the "aproximación" label; `degraded:
     false` does not.
   - `CardiacDriftPanel.test.tsx` — `hasHRStreams:
     false` renders the empty-state label;
     `hasHRStreams: true` renders the panel.

## Wire-shape delta

### GET `/api/v1/dashboard/summary` — added (no breaking)

```json
{
  "weekly_volume_m": 42500,
  "weekly_elevation_m": 850,
  "weekly_activities_count": 4,
  "trend_7d_pct": 12.5,
  "trend_30d_pct": -3.2
}
```

`trend_7d_pct` and `trend_30d_pct` are `null` when the
previous window has zero rows. Endpoint is net-new; no
prior shape to preserve.

### GET `/api/v1/dashboard/load?from=&to=` — added

```json
{
  "from": "2026-09-01",
  "to": "2026-10-01",
  "series": [
    { "day": "2026-09-01", "ctl": 42.1, "atl": 31.0, "tsb": 11.1 },
    { "day": "2026-09-02", "ctl": 42.4, "atl": 30.5, "tsb": 11.9 }
  ]
}
```

`series` always has exactly one entry per calendar day
in `[from, to]`. Days without activity show
`ctl/atl/tsb = 0`. Endpoint is net-new.

### GET `/api/v1/dashboard/hr-zones?from=&to=` — added

```json
{
  "from": "2026-09-01",
  "to": "2026-10-01",
  "degraded": false,
  "zones": [
    { "zone": "z1", "minutes": 120 },
    { "zone": "z2", "minutes": 80 },
    { "zone": "z3", "minutes": 45 },
    { "zone": "z4", "minutes": 30 },
    { "zone": "z5", "minutes": 15 }
  ]
}
```

`degraded: true` means the response was built from
`activities.avg_hr * elapsed_seconds`, not from
`activity_streams`. Endpoint is net-new.

### POST `/api/v1/dashboard/recalc` — added

```json
{ "job_id": "...", "queued_at": "2026-10-01T...", "will_recompute_from": "..." }
```

Endpoint is net-new.

### GET `/healthz` — extended behind header (G3)

Public shape **unchanged**:

```json
{ "status": "ok" }
```

With `X-Internal-Health: 1`:

```json
{
  "status": "ok",
  "db": { "ok": true, "latency_ms": 4 },
  "strava": { "ok": true, "last_sync_at": "..." },
  "last_recalc_at": "...",
  "training_load_rows": 12345,
  "ai": { "enabled": false }
}
```

When DB ping fails (header present or not), the
response is `503 Service Unavailable` with
`{ "status": "degraded", "db": { "ok": false } }`.

**Zero breaking change.** Public consumers (load
balancers, k8s liveness probes, uptime checkers)
continue to see `{ "status": "ok" }`.

### Web types — added

`web/src/lib/api/dashboard.ts`:

```ts
export interface DashboardSummary {
  weekly_volume_m: number;
  weekly_elevation_m: number;
  weekly_activities_count: number;
  trend_7d_pct: number | null;
  trend_30d_pct: number | null;
}

export interface TrainingLoadPoint {
  day: string;       // ISO date
  ctl: number;
  atl: number;
  tsb: number;
}

export interface TrainingLoadSeries {
  from: string;
  to: string;
  series: TrainingLoadPoint[];
}

export interface HRZoneBucket {
  zone: 'z1' | 'z2' | 'z3' | 'z4' | 'z5';
  minutes: number;
}

export interface HRZonesResponse {
  from: string;
  to: string;
  degraded: boolean;
  zones: HRZoneBucket[];
}

export interface RecalcResponse {
  job_id: string;
  queued_at: string;
  will_recompute_from: string;
}
```

All additive.

## Verification commands per PR

- **PR1**: `make test` (≈ backend suite green),
  `make lint`, `make vet`, `make fmt`.
  `make coverage` should report
  `internal/metrics` ≥ 90% lines (informational,
  not enforced).
- **PR2**: `make test`, `make lint`, `make vet`,
  `make fmt`. CI migration smoke runs the full
  goose chain forward and back with
  `timescaledb/timescaledb-ha:pg16`. PR2 also
  adds an integration test
  `internal/db/migrations_00012_test.go` exercising
  the hypertable creation against the
  timescaledb image only (skipped otherwise).
- **PR3**: `make test`, `make lint`, `make vet`,
  `make fmt`. Manual smoke: `curl -H "Authorization:
  Bearer …" /api/v1/dashboard/summary` and verify
  the body; `curl -H "X-Internal-Health: 1" /healthz`
  for the extended shape.
- **PR4**: `pnpm -C web test:run`, `pnpm -C web
  typecheck`, `pnpm -C web lint`, `pnpm -C web
  format:check` over changed files. Manual smoke:
  `/dashboard` renders with seed data; period
  selector flips between 7d/30d/90d/1y; cardiac
  drift panel hides for users without HR streams.

## Risk

- **Risk: 4-PR stacked-to-main chain.** PR1 ships
  pure code with no dependents; PR2 builds on PR1's
  metrics package and adds persistence; PR3 builds
  on PR2's persistence; PR4 builds on PR3's API.
  Each PR is independently revertible. **Severity:
  low** when PR2's diff stays under 400 LoC (the
  service + job + migration + tests is the
  long pole). If PR2 trends over budget at
  implementation time, split PR2 into PR2a
  (migrations + SQLC + dashboard_metadata) and PR2b
  (service + job + hook), as the feature doc's
  contingency plan covers. **Mitigation flagged in
  proposal; not exercised yet.**

- **Risk: River `UniqueOpts` semantics across the
  riverqueue API.** The `UniqueOpts` API exists in
  the River library as used in 1.2
  (`internal/jobs/*` already exists), but the
  specific TTL parameter and the key composition
  need verification at implementation time.
  **Severity: low**; flagged so PR2's TDD verifies
  that a flood of identical enqueues collapses to
  one job. E7 mitigates.

- **Risk: TimescaleDB hypertable creation on
  existing rows.** The `00012` migration runs on a
  fresh schema (no prior `training_load_daily` rows
  exist), so `create_hypertable` is the standard
  "create empty hypertable" path. **Severity: very
  low**. The `if_not_exists => TRUE` clause makes
  the migration idempotent against re-runs.

- **Risk: GAP formula divergence from Strava's
  published GAP.** E6 documents the Strava
  (Minetti-normalized) choice. A reviewer may ask
  "why not the canonical Minetti?" — the answer
  lives in `internal/metrics/SPEC.md` (PR1 deliverable)
  and explains that the Strava-normalized curve is
  the only one that composes with Strava-sourced
  pace/speed columns. **Severity: low** if the SPEC
  is rigorous.

- **Risk: HR-zones fallback degrades user trust.**
  When `activity_streams` is missing, the
  `degraded: true` flag and the chart label
  "aproximación (sin streams HR)" keep users
  informed. A reviewer may push for hiding the
  chart entirely in degraded mode. **Severity:
  low** if the decision (label, don't hide) is
  documented in PR3 tests and PR4 component tests.

- **Risk: Cardiac Drift without streams hiding the
  panel might confuse users.** G5's empty state
  ("necesita streams HR para calcular") is
  intentionally not a 500 / not a crash; it is a
  soft affordance. A reviewer may want a banner
  instead of an empty card. **Severity: very low**
  if the empty-state copy is clear; flagged for
  product review at `sdd-tasks`.

- **Risk: `/healthz` header inspection in CI
  k6/smoke tests.** The release CI smokes `/healthz`
  without the header; the new internal-only path is
  not part of release smoke. **Severity: very low**
  if the release job is left untouched.

- **Risk: 5-year user triggers long recalc.** PR3's
  `/recalc` endpoint (G4) is the explicit lever;
  PR2's default post-upsert window is ±3 days. A
  user with 5 years of activities who hits `/recalc`
  sees a 202; the job runs asynchronously. **Severity:
  low**; mitigated by the River job's
  `training_load_rows` write-back so progress is
  observable in `/healthz`.

- **Risk: ECharts bundle size.** E1 mitigation adds
  `echarts` as the first PR4 commit, so reviewers
  see the bundle-size impact up front. Tree-shaking
  by ECharts module (use `echarts/core` plus the
  specific chart/component imports) keeps the
  per-route chunk lean. **Severity: low**.

- **Risk: Migration numbering with parallel work.**
  `00012` and `00013` are reserved for PR2. If
  another slice lands between proposal and PR2, the
  goose numbering must be reconciled at apply time.
  **Severity: very low**; the
  `closes-159-strava-disconnect` and
  `phase-1-3-km-vertical-and-king-climb` archives
  show that the repo standard is "PR author renumbers
  if needed."

## Rollback

### PR1 rollback

PR1 is the pure-Go metrics slice. `git revert` of the
merge commit removes every PR1 delta in one operation:

1. **`internal/metrics/*.go`** — revert. The metrics
   package disappears.
2. **`internal/metrics/SPEC.md`** — revert.
3. **Test files** — revert.

After revert: the codebase is byte-equivalent to the
pre-PR1 state. PR2 (which depends on PR1's metrics) is
not yet applied, so no dependents break.

### PR2 rollback

PR2 is the persistence + users column + dashboard
metadata slice. `git revert` of the merge commit:

1. **`internal/db/migrations/00012_training_load_daily.sql`**
   — revert. `goose Down` drops `training_load_daily`
   and `dashboard_metadata`. The TimescaleDB guard
   ensures the `Down` is symmetric even on a
   non-timescaledb dev DB (it drops the dashboard
   table first, then the hypertable creation block
   is a no-op on rollback because the table is
   already gone).
2. **`internal/db/migrations/00013_users_running_threshold.sql`**
   — revert. `goose Down` drops the
   `running_threshold_sec_per_km` column.
3. **`internal/db/queries/training_load.sql`** and
   **`dashboard_metadata.sql`** — revert (delete).
   `make generate` regenerates `internal/db/sqlc/*`
   without them; in practice the reverted commit
   already carries the pre-PR2 regen.
4. **`internal/metrics/training_load.go`** — revert.
5. **`internal/jobs/recalc_training_load.go`** — revert.
6. **Post-`UpsertActivity` hook** — revert (the
   call site returns to its pre-PR2 form).
7. **Test files** — revert.

After revert: no `training_load_daily` table, no
`dashboard_metadata` table, no `users.running_threshold_sec_per_km`
column, no River job. PR3 is not yet applied, so
the API endpoints it adds are absent; the
`/healthz` extended shape is absent; no consumer
relies on the rolled-back state.

### PR3 rollback

PR3 is the API + extended `/healthz` slice. `git revert`:

1. **`internal/http/handlers/dashboard.go`** and
   **`dashboard_recalc.go`** — revert. The three GET
   endpoints and the POST `/recalc` disappear from
   the router.
2. **`internal/http/handlers/health.go`** — revert
   the `X-Internal-Health` branching. `/healthz`
   returns to its public shape
   `{ "status": "ok" }` only.
3. **Router wiring** in `internal/app/router.go` —
   revert the dashboard mounts.
4. **Test files** — revert.

After revert: no `/api/v1/dashboard/*` endpoints,
`/healthz` is back to its 1.1 contract. PR4 is not
yet applied, so no web code references these
endpoints.

### PR4 rollback

PR4 is the web dashboard slice. `git revert`:

1. **`web/src/lib/api/dashboard.ts`** — revert.
2. **`web/src/features/dashboard/`** — revert (delete
   the new feature folder including its components,
   hooks, and tests).
3. **`web/package.json`** and **`pnpm-lock.yaml`** —
   revert the `echarts` addition. (If the revert
   leaves other packages depending on `echarts`, the
   revert will fail at `pnpm install` time; PR4 is
   the only consumer, so this is safe.)
4. **Router/nav entry** in `web/src/` — revert.

After revert: no `/dashboard` page, no ECharts
dependency. The backend (PR1–PR3) continues to serve
the dashboard endpoints, but no UI consumes them.
Recoverable by re-applying PR4.

### Worst-case combined rollback

Revert PR4 first (UI depends on PR3's endpoints),
then PR3 (API + health depends on PR2's persistence),
then PR2 (persistence depends on PR1's metrics),
then PR1 (pure metrics). Order matters: PR2's
`ComputeDailyLoad` imports `internal/metrics/*` from
PR1, so reverting PR1 first would leave PR2 unable
to compile; reverting PR2 first would leave PR3
unable to compile; reverting PR3 first would leave
PR4 unable to compile.

## PR boundary forecast

Per the user-confirmed 4-PR stacked-to-main chain, the
LoC budget per PR is the 400-line review budget defined
in `openspec/config.yaml:rules.proposal`. Forecasts below
are authored lines (added or changed), not "lines of diff"
— the project standard from the previous slices'
apply-progress reports.

### PR1 — Pure-Go metrics (~280 LoC)

| Change | LoC |
|---|---|
| `internal/metrics/SPEC.md` | ~50 |
| `internal/metrics/performance.go` (4 metrics) | ~60 |
| `internal/metrics/health.go` | ~25 |
| `internal/metrics/fatigue.go` (CTL/ATL/TSB + filler) | ~45 |
| `internal/metrics/performance_test.go` (table-driven × 4 metrics × ~3 cases) | ~50 |
| `internal/metrics/health_test.go` | ~20 |
| `internal/metrics/fatigue_test.go` | ~30 |
| **PR1 total** | **~280** |

Under the 400-line budget by ~120 LoC. No
contingency split needed.

### PR2 — TimescaleDB persistence + users column (~360 LoC)

| Change | LoC |
|---|---|
| Migration `00012_training_load_daily.sql` | ~40 |
| Migration `00013_users_running_threshold.sql` | ~15 |
| SQLC queries (`training_load.sql`, `dashboard_metadata.sql`, `users.sql` extension) | ~40 |
| `internal/db/sqlc/*` regenerated | (mechanical, not authored) |
| `internal/metrics/training_load.go` (service) | ~70 |
| `internal/jobs/recalc_training_load.go` (River job) | ~50 |
| Post-`UpsertActivity` hook wiring | ~15 |
| `internal/metrics/training_load_test.go` | ~50 |
| `internal/jobs/recalc_training_load_test.go` | ~40 |
| `internal/db/migrations_00012_test.go` (timescaledb smoke) | ~40 |
| **PR2 total** | **~360** |

Under the 400-line budget by ~40 LoC. Tight; the
contingency split (`PR2a` migrations + SQLC,
`PR2b` service + job) is documented in the feature
doc but not exercised yet.

### PR3 — Dashboard API + extended `/healthz` (~340 LoC)

| Change | LoC |
|---|---|
| `internal/http/handlers/dashboard.go` (3 GET handlers) | ~110 |
| `internal/http/handlers/dashboard_recalc.go` (POST `/recalc`) | ~25 |
| `internal/http/handlers/health.go` (extended `/healthz`) | ~40 |
| Router wiring (`internal/app/router.go`) | ~15 |
| `internal/http/handlers/dashboard_test.go` | ~70 |
| `internal/http/handlers/dashboard_recalc_test.go` | ~25 |
| `internal/http/handlers/health_test.go` (extension + header branches) | ~55 |
| **PR3 total** | **~340** |

Under the 400-line budget by ~60 LoC.

### PR4 — Web dashboard (~340 LoC)

| Change | LoC |
|---|---|
| `web/package.json` + `pnpm-lock.yaml` (`pnpm add echarts`) | (mechanical) |
| `web/src/lib/api/dashboard.ts` (types + fetchers) | ~50 |
| `web/src/features/dashboard/useDashboardSummary.ts` | ~15 |
| `web/src/features/dashboard/useTrainingLoad.ts` | ~20 |
| `web/src/features/dashboard/useHRZones.ts` | ~15 |
| `web/src/features/dashboard/DashboardSummaryCard.tsx` + test | ~40 |
| `web/src/features/dashboard/TrainingLoadChart.tsx` + test | ~60 |
| `web/src/features/dashboard/HRZonesChart.tsx` + test | ~50 |
| `web/src/features/dashboard/CardiacDriftPanel.tsx` + test | ~35 |
| Page mount `/dashboard` + nav entry | ~25 |
| `web/src/features/dashboard/periodSelector.tsx` (or reuse) | ~15 |
| Style tokens additions in `web/src/styles/tokens.css` | ~15 |
| **PR4 total** | **~340** |

Under the 400-line budget by ~60 LoC.

### Combined forecast

~1,320 LoC total across four PRs. Each PR is
independently revertible (see Rollback). Each PR fits
the 400-line budget. No `size:exception` is required.
The chain strategy is the **stacked-to-main** shape
used by `closes-159-strava-disconnect` and
`phase-1-3-km-vertical-and-king-climb`: PR1 lands
first on `main`, PR2 rebases on `main` after PR1
merges, and so on. Branch naming convention:
`feat/phase-1.4-pr{N}-{slug}` per the user-confirmed
naming pattern from the feature doc.

## Exploration caveats

Three minor items to flag for `sdd-tasks` and beyond:

1. **Legacy TS absence.** The exploration confirms
   `ghamusinos_/__`, `ghamusinos__`, and
   `old_ghamusinos` do not exist in this repo
   (`find ghamusinos_*` returns 0 results; `ls`
   root shows only `cmd, docker-compose.yml, docs,
   go.mod, go.sum, internal, Makefile, odd,
   openspec, README.md, scripts, sqlc.yaml, web`).
   The "PORT" framing in the feature doc is therefore
   a port against literature, not against code. PR1's
   `internal/metrics/SPEC.md` documents each formula's
   canonical source (TrainingPeaks, Coggan, Minetti,
   Pauley) so the lack of a TS reference is not a
   documentation gap.

2. **Column placement decisions.** The exploration's
   "Affected areas" section enumerates candidate
   schemas; the proposal must pick one and justify.
   The proposal picks separate tables with the
   rationale in the "Column / table placement
   decisions" section above; this matches the
   exploration's intent and the
   `phase-1-3-km-vertical-and-king-climb` pattern
   (three separate tables in migration `00011`).

3. **Hook insertion point.** The exploration lists the
   post-`UpsertActivity` hook as a desired behavior
   but does not name the exact file in `internal/strava`
   where the upsert returns. PR2's implementation will
   locate it; the proposal documents the desired
   behavior ("after a successful upsert, enqueue with
   `started_at ± 3d`") and lets `sdd-design` identify
   the precise call site. This is a low-risk
   discovery item that surfaces here so the
   implementation step does not block on it.

## Out of scope (whole change)

The following are intentionally excluded from this slice
chain and MUST NOT be touched by `sdd-apply`:

- AI applied to metrics (Fase 1.5 per
  `feature-inventory.md:§9`).
- Planning (V2).
- Advanced 1.6 GPX lab features.
- 3D MapLibre + slope heatmap (Fase 1.6).
- Comparator (`computeDiff`) extension.
- OAuth/Strava lifecycle.
- Backfilling pre-existing activities — PR3's
  `/recalc` endpoint is the explicit user-driven
  lever (G4); automatic backfill is not shipped.
- Migration `00014` or later — `00013` is the
  highest number this slice reserves.
- Any modification of `internal/gpx/climbs.go` or
  any GPX code (the slice is dashboard, not GPX).
- Re-deriving metrics from `activity_streams`
  beyond the four inventory-cited uses (TSS running,
  HR zones, cardiac drift, GAP).

## Ready for Spec

**Yes** — `nextRecommended: spec`. The next phase
should be `sdd-spec` for the backend
(`openspec/specs/dashboard-backend/spec.md`) and the
web (`openspec/specs/dashboard-web/spec.md`) deltas,
then `sdd-design`, then `sdd-tasks` (which will
confirm the 4-PR chain and may exercise the
contingency split for PR2 if PR2a + PR2b is the
cleaner cut), then `sdd-apply` gated by `make test`
(PR1, PR2, PR3) and `pnpm -C web test:run` (PR4),
then `sdd-verify`, then archive. This proposal does
NOT write the spec deltas; that is `sdd-spec`'s job.
