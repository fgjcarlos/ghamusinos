# Spec Delta: phase-1-4-performance-dashboard

> Domain: dashboard-api
> Source: openspec/changes/phase-1-4-performance-dashboard/{proposal,exploration}.md
> Cross-project: `.` (Go)
> Stack: Go 1.26.0 + chi/v5 + pgx/v5 + sqlc + testify + httptest
> Gating test command: `make test`
> Quality gates: `make lint`, `make vet`, `make fmt`

This delta covers PR3: the four `/api/v1/dashboard/*`
endpoints (three GETs and one POST) plus the
`X-Internal-Health`-gated extension of `/healthz`. The handlers
follow the existing pattern from
`internal/http/handlers/activities.go` (factory returning
`http.Handler` with `sqlc.Querier` + `auth.AuthUser` +
`WriteProblem`). No canonical
`openspec/specs/dashboard-api/spec.md` exists yet; archive
will copy this file into the canonical location.

## ADDED Requirements

### Requirement: `GET /api/v1/dashboard/summary` returns weekly KPIs and trends (`DA-001`)

`internal/http/handlers/dashboard.go` `GetDashboardSummary(q
sqlc.Querier, pool *pgxpool.Pool) http.Handler` SHALL return
`200 OK` with body
`{ weekly_volume_m: number, weekly_elevation_m: number, weekly_activities_count: number, trend_7d_pct: number | null, trend_30d_pct: number | null }`.
The handler MUST require an authenticated user via
`auth.AuthUser` and return `401` otherwise. Volume and
elevation SHALL aggregate from `training_load_daily WHERE day
>= date_trunc('week', now())`; activities count SHALL
aggregate from `activities WHERE started_at >= date_trunc('week',
now())`. Trends SHALL compute
`(current_window_avg − previous_window_avg) / previous_window_avg`
and MUST be `null` when the previous window has zero rows
(avoids `NaN` / divide-by-zero).

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/dashboard.go` `GetDashboardSummary`

#### Scenario: authenticated user with 4 activities this week gets the expected shape

- GIVEN an authenticated user `U` with 4 activities in the
  current ISO week summing `volume_m = 42500`,
  `elevation_gain_m = 850`
- WHEN `GET /api/v1/dashboard/summary` is invoked with a
  valid Bearer JWT
- THEN the response status is `200`
- AND the body has `weekly_volume_m = 42500`,
  `weekly_elevation_m = 850`, `weekly_activities_count = 4`
- AND `trend_7d_pct` and `trend_30d_pct` are either numeric
  or `null` (no `NaN` literal)

#### Scenario: anonymous request is rejected with 401

- GIVEN a request with no `Authorization` header
- WHEN `GET /api/v1/dashboard/summary` is invoked
- THEN the response status is `401`
- AND the body follows the existing RFC 9457 problem
  document shape (consistent with other auth-gated handlers)

### Requirement: `GET /api/v1/dashboard/load?from=&to=` returns a filled daily series (`DA-002`)

`internal/http/handlers/dashboard.go` `GetTrainingLoad(q
sqlc.Querier) http.Handler` SHALL read `from` and `to` query
params as ISO dates, default `to` to today and `from` to
`to − 30 days` when absent, return `200 OK` with body
`{ from, to, series: [{ day, ctl, atl, tsb }, ...] }`. The
returned `series` MUST contain exactly one entry per calendar
day in `[from, to]` inclusive, with `ctl/atl/tsb = 0` for any
day the user has no activities (delegating to
`metrics.FillMissingDays` from PR1). If `from` predates the
user's first activity, the pre-history days SHALL be filled
with zeros. The response SHALL include the header
`Cache-Control: private, max-age=60` so intermediate caches
respect the per-user authorization.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/dashboard.go` `GetTrainingLoad`

#### Scenario: 7-day range returns 7 contiguous entries with zero-fill

- GIVEN an authenticated user with activities only on days
  `2026-09-03` and `2026-09-06`
- WHEN `GET /api/v1/dashboard/load?from=2026-09-01&to=2026-09-07` is invoked
- THEN the body has `from = "2026-09-01"`, `to = "2026-09-07"`
- AND `series` has exactly 7 entries
- AND every entry's `day` field is one of the seven dates in
  order
- AND the entries on `2026-09-01`, `2026-09-02`, `2026-09-04`,
  `2026-09-05`, `2026-09-07` have `ctl = atl = tsb = 0`
- AND the response includes `Cache-Control: private, max-age=60`

#### Scenario: `from` predates the user's first activity and gets zero-filled

- GIVEN a user whose first activity is on `2025-06-01`
- WHEN `GET /api/v1/dashboard/load?from=2024-01-01&to=2025-12-31` is invoked
- THEN the body has one entry per calendar day in
  `[2024-01-01, 2025-12-31]` inclusive
- AND all days before `2025-06-01` have `ctl = atl = tsb = 0`

### Requirement: `GET /api/v1/dashboard/hr-zones` aggregates HR minutes per zone (`DA-003`)

`internal/http/handlers/dashboard.go` `GetHRZones(q
sqlc.Querier) http.Handler` SHALL read `from`/`to` (defaults
identical to `DA-002`) and return `200 OK` with body
`{ from, to, degraded: boolean, zones: [{ zone: "z1" | "z2" | "z3" | "z4" | "z5", minutes: number }] }`.
The handler SHALL first try to aggregate from
`activity_streams WHERE stream_type = 'heartrate'` (using the
per-zone bounds from the `hr_zones` table). When the streams
query returns zero rows for the range, the handler SHALL
fall back to `activities.avg_hr × elapsed_seconds / 60`
partitioned uniformly across the same zones, and SHALL set
`degraded: true` so the UI can label the chart
"aproximación (sin streams HR)". When streams are present,
`degraded` SHALL be `false`.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/dashboard.go` `GetHRZones`

#### Scenario: HR streams are preferred and degrade=false

- GIVEN a user with two activities in the range, both having
  `activity_streams` rows where `stream_type = 'heartrate'`
- WHEN `GET /api/v1/dashboard/hr-zones?from=...&to=...` is invoked
- THEN `degraded = false`
- AND `zones` contains exactly 5 entries (`z1`..`z5`) with
  `minutes` sums derived from the stream samples bucketed by
  the user's `hr_zones` table bounds

#### Scenario: missing streams falls back to avg_hr with degraded=true

- GIVEN a user with activities in the range whose
  `activity_streams` table has zero `heartrate` rows
- WHEN `GET /api/v1/dashboard/hr-zones?from=...&to=...` is invoked
- THEN `degraded = true`
- AND `zones` is populated from
  `activities.avg_hr × elapsed_seconds / 60` bucketed against
  the same zone bounds (even when `avg_hr` is `NULL` for a
  given activity, that activity contributes 0 minutes)

### Requirement: `POST /api/v1/dashboard/recalc` enqueues a full-history job (`DA-004`)

`internal/http/handlers/dashboard_recalc.go`
`PostDashboardRecalc(q sqlc.Querier, enqueuer RiverEnqueuer)
http.Handler` SHALL enqueue
`RecalcTrainingLoadArgs{UserID: <auth user>, From: nil, To: nil}`
via the existing `RiverEnqueuerAdapter` pattern
(`internal/jobs/river.go:127-141`) and return `202 Accepted`
with body
`{ job_id: string, queued_at: ISO8601 timestamp, will_recompute_from: ISO8601 date | null }`.
The handler MUST require an authenticated user (same JWT
middleware as the other `/api/v1/dashboard/*` endpoints) and
return `401` otherwise. `will_recompute_from` SHALL be the
first-activity date when activities exist, else `null`.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/dashboard_recalc.go` `PostDashboardRecalc`

#### Scenario: authenticated POST returns 202 with the job metadata

- GIVEN an authenticated user with first activity on `2025-01-01`
- WHEN `POST /api/v1/dashboard/recalc` is invoked
- THEN the response status is `202`
- AND the body has a non-empty `job_id`, a `queued_at` ISO
  timestamp, and `will_recompute_from = "2025-01-01"`
- AND the mock River enqueuer recorded exactly one
  `RecalcTrainingLoadArgs` call with `From = nil`,
  `To = nil`, and the auth user's `UserID`

#### Scenario: anonymous POST is rejected with 401

- GIVEN a request with no `Authorization` header
- WHEN `POST /api/v1/dashboard/recalc` is invoked
- THEN the response status is `401`
- AND the mock River enqueuer recorded zero calls

### Requirement: `/healthz` is extended only behind the `X-Internal-Health: 1` header (`DA-005`)

`internal/http/handlers/health.go` SHALL keep the public
shape unchanged (`{ "status": "ok" }` for an unadorned
request). When the request carries the header
`X-Internal-Health: 1`, the response SHALL additionally
include sections
`db: { ok: boolean, latency_ms: number }`,
`strava: { ok: boolean, last_sync_at: ISO8601 timestamp | null }`,
`last_recalc_at: ISO8601 timestamp | null`,
`training_load_rows: number`,
`ai: { enabled: boolean }`.
The header requirement is independent of the DB-OK vs DB-down
branch (DA-006). A DB failure always returns a degraded body
including `db.ok = false`, whether or not the header is present.
This keeps the healthy public `/healthz` shape stable while
making DB failure explicit.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/health.go`

#### Scenario: public request without the header keeps the minimal shape

- GIVEN a healthy DB and a valid `/healthz` handler
- WHEN `GET /healthz` is invoked with no `X-Internal-Health`
  header
- THEN the response status is `200`
- AND the body is exactly `{ "status": "ok" }` (no additional
  keys are present)

#### Scenario: internal request with the header returns the extended shape

- GIVEN a healthy DB, a Strava client with a known
  `last_sync_at`, a `dashboard_metadata` row with
  `last_recalc_at = "2026-10-01T08:00:00Z"` and
  `training_load_rows = 12345`, and `users.ai_enabled = false`
- WHEN `GET /healthz` is invoked with header
  `X-Internal-Health: 1`
- THEN the response status is `200`
- AND the body has `status = "ok"` plus the keys
  `db.ok = true`, `db.latency_ms` numeric,
  `strava.ok = true`, `strava.last_sync_at` ISO string,
  `last_recalc_at = "2026-10-01T08:00:00Z"`,
  `training_load_rows = 12345`, and `ai.enabled = false`

### Requirement: `/healthz` returns 503 with `status: "degraded"` when the DB ping fails (`DA-006`)

When the DB ping (shared helper from
`internal/db/status/`) fails, the handler SHALL return
`503 Service Unavailable` with body
`{ "status": "degraded", "db": { "ok": false } }`, with or
without the internal header. The remaining extended sections
(`strava`, `last_recalc_at`, `training_load_rows`, `ai`) SHALL
be omitted because they depend on DB state.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/health.go` degraded branch

#### Scenario: public degraded response reports DB failure

- GIVEN the DB ping returns an error
- WHEN `GET /healthz` is invoked with no `X-Internal-Health` header
- THEN the response status is `503`
- AND the body is `{ "status": "degraded", "db": { "ok": false } }`

#### Scenario: internal degraded response carries db.ok=false and omits DB-dependent sections

- GIVEN the DB ping returns an error
- WHEN `GET /healthz` is invoked with header `X-Internal-Health: 1`
- THEN the response status is `503`
- AND the body has `status = "degraded"` and `db.ok = false`
- AND `strava`, `last_recalc_at`, `training_load_rows`, and
  `ai` keys are absent (or `null`) because they cannot be
  computed without a working DB
