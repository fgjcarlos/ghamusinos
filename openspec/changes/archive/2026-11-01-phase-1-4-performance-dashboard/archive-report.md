# Archive Report: phase-1-4-performance-dashboard

## Summary

Closed the epic **#16 — Fase 1.4 (Dashboard de rendimiento y salud/fatiga)**. The
slice spans six PRs over `main`:

- **PR #240** (merge `bf1db5c`) — PR1: pure-Go metrics package
  (`internal/metrics`): TSS cycling/running, intensity factor, grade-adjusted
  pace, efficiency factor, cardiac drift and the CTL/ATL/TSB EMA with
  day filling (M-001..M-010).
- **PR #242** (merge `d8e920f`) — PR2: TimescaleDB persistence
  (`training_load_daily` hypertable + `dashboard_metadata`, migrations 00012
  and 00013), the `ComputeDailyLoad` orchestrator and the River
  `RecalcTrainingLoad` worker (TL-001..TL-010).
- **PR #252** (merge `7455684`) — PR3: the HTTP slice. Four
  `/api/v1/dashboard/*` endpoints (`summary`, `load`, `hr-zones`, `recalc`) plus
  the `/healthz` extension behind `X-Internal-Health: 1` (DA-001..DA-006).
- **PR #267** (merge `549c3e4`) — the CI blocker that kept every open PR red:
  `govulncheck` failed on the pinned Go 1.26.8 toolchain; the workflow now
  pins 1.26.10.
- **PR #268** (merge `01b6e8f`) — the prerequisite for the web slice: the
  Vitest suite was broken in all 40 files by a `react`/`react-dom` version
  mismatch (19.3.0 vs 19.3.1) and the frontend CI job never ran the tests.
  React is aligned at 19.3.1 and the CI now runs `pnpm test:run`.
- **PR #269** (merge `5299bdb`) — **DA-007**: `GET /api/v1/dashboard/cardiac-drift`,
  the endpoint DW-005 needs. It reads the raw `heartrate` streams already
  persisted in `activity_streams` and delegates the formula to
  `internal/metrics.CardiacDriftSeries`.
- **PR #270** (merge `283faf6`) — PR4: the `/dashboard` SPA screen with ECharts
  (DW-001..DW-008): KPI tiles plus weekly sparkline, CTL/ATL/TSB line chart,
  stacked HR-zone bars, the conditional cardiac-drift panel, the 7d/30d/90d/1y
  period selector and the empty state.

The gap the epic opened — *metrics computed but never persisted, never
summarised and never shown* — is closed end to end: from the Strava stream
ingestion through `training_load_daily` up to the dashboard the user opens.

## Lineage

Branches (all merged and deleted upstream):

- `feat/phase-1.4-pr1-metrics-pure-go` → PR #240
- `feat/phase-1.4-pr2-timescaledb-recalc` → PR #242
- `feat/phase-1.4-pr3-api-dashboard` → PR #252
- `chore/web-react-19.3.1` → PR #268
- `feat/dashboard-cardiac-drift` → PR #269
- `feat/phase-1.4-pr4-web-dashboard` → PR #270

## Canonical specs

The four delta specs of this change are now the canonical specs:

| Delta | Canonical spec | Requirements |
|---|---|---|
| `specs/metrics-go/spec.md` | `openspec/specs/metrics-go/spec.md` | M-001..M-010 |
| `specs/training-load/spec.md` | `openspec/specs/training-load/spec.md` | TL-001..TL-010 |
| `specs/dashboard-api/spec.md` | `openspec/specs/dashboard-api/spec.md` | DA-001..DA-007 |
| `specs/dashboard-web/spec.md` | `openspec/specs/dashboard-web/spec.md` | DW-001..DW-008 |

## Verification

- Backend: `go build ./...`, `go vet ./...`, `gofmt -l .`,
  `golangci-lint run` (0 issues) and `go test ./...` green in CI on every
  merged PR (the `Backend (lint + test + build + migrate)` job also runs the
  migration smoke and the integration tests).
- Web: `pnpm --dir web test:run` (54 files / 276 tests), `typecheck`, `lint`,
  `format:check` and `build` green; the frontend CI job now runs the test
  suite as well.
- Native RDD review: three lenses (risk, resilience, reliability) on the
  endpoint slice (#269) and on the web slice (#270) — both **approved**. The
  two non-blocking resilience advisories of the web review (no error boundary
  around the lazy chunk; charts mounting before the summary resolved) were
  fixed inside #270.

## Risks and known follow-ups

- **R1 — the endpoint range bound is exclusive**: `cardiac-drift` uses
  `started_at < 00:00 UTC (to + 1 day)` so the last day of the range is
  included whole; `/hr-zones` still uses an inclusive midnight bound. Worth
  aligning in a future change.
- **R2 — ECharts is 185 kB gzipped**: it lives in its own lazily loaded chunk
  (the initial bundle did not grow), but it is still the heaviest dependency
  of the SPA.
- **R3 — cardiac drift needs re-ingestion**: the drift is computed from the
  raw `heartrate` streams, so activities ingested before this change have no
  drift row until the Streams worker runs again for them.
- **R4 — pre-existing flaky test**: `internal/jobs.TestBackfillWindow_CalculatesCorrectSpan`
  fails outside UTC because `AddDate` crosses the DST shift (span 1009 h vs a
  strict 1 h tolerance). It passes in CI (UTC) and does not touch this change.
- **R5 — the `/rendimiento` alias**: the screen moved to `/dashboard`;
  `/rendimiento` redirects, so old links keep working.

## Outcome

The epic #16 can be closed: every deliverable of the issue body shipped
(pure-Go metrics, TimescaleDB series with recalculation, the dashboard API,
the ECharts charts and the detailed health endpoint), and the delta specs are
promoted to canonical specs.
