# Exploration: Wire elevation profile and map to the GPX track-detail page

> Phase 1.3 — Laboratorio GPX (continuation). This exploration was
> produced by `sdd-explore` against the workspace as it stands after
> the prior session on 2026-08-04. No source code was modified; only
> this file was written.

## Current State

### What shipped for phase 1.3 already (verified on disk)

**Backend (`internal/`, `cmd/`, `internal/db/`):**

- `internal/gpx/` is a complete Go package: `parser.go`, `analysis.go`
  (with `CalculateElevation`, `CoverageStatus`, `ElevationResult`,
  `CoverageFullThreshold`/`CoverageMinThreshold` policy from #171/A8),
  `climbs.go` (FindAllClimbs / FindKingClimb / FindMuros /
  FindRecoveryZones / FindKmVertical), `risk_zones.go`, `track_type.go`,
  `validation.go`, `hash.go`, `store.go` (with transactional
  `CreateDetail`, `subsamplePoints` from #170/M9, `ErrNumericConversion`
  for +Inf/-Inf/NaN from #170/M8), `types.go`.
- `internal/http/handlers/`: `gpx_upload.go`, `gpx_get.go`,
  `gpx_list.go`, `gpx_delete` (inside `gpx_list.go`), `gpx_compare.go`
  (max 3 tracks, with nullable D+/D-).
- `internal/http/router.go`: all five GPX routes mounted under
  `/api/v1/gpx/*`, plus the optional GPX dependency wiring via
  `WithGPX(...)`.
- `internal/db/migrations/00006_gpx_tracks.sql`: schema for
  `gpx_tracks` + `gpx_climbs` + `gpx_risk_zones` (with `coordinates`
  JSONB, full metric columns, `king_climb` JSONB, FK cascade).
- `internal/db/migrations/00010_gpx_tracks_elevation_coverage.sql`:
  makes `d_plus_m` / `d_minus_m` nullable and adds
  `elevation_coverage NUMERIC` (issue #171/A8 — branch
  `fix/gpx-elevation-coverage-171`).
- `internal/db/queries/`: `gpx_tracks.sql` (Create / Get / List with
  `COUNT(*) OVER() AS total_count` from #172/M6 / Hash / Delete),
  `gpx_climbs.sql`, `gpx_risk_zones.sql`. SQLC generated code reflects
  all columns including `ElevationCoverage` (`models.go`, sqlc
  generated).

**Frontend (`web/src/`):**

- `routes/lab.tsx` — list + upload + delete + select-to-compare.
- `routes/lab-compare.tsx` — `/lab/compare?ids=...` page.
- `routes/lab-detail.tsx` — placeholder file present but **not wired**
  into `main.tsx` (it is dead code; `main.tsx` mounts
  `RouteDetailContainer` for `/rutas/:id` and `/lab/:id`).
- `features/gpx/`:
  - `RouteDetailContainer.tsx` — fetch + state machine
    (loading / no-token / not-found / error / ready).
  - `RouteDetail/RouteDetail.tsx` — composes header + metrics + climbs
    + risks + elevation profile.
  - `RouteHeader/`, `RouteMetrics/`, `RouteClimbs/`, `RouteRisks/`,
    `ElevationProfile/` (the SVG hand-rolled renderer from #156 with
    `projectTrack.ts` and its 8+ TDD tests), plus a full unit-test
    suite for each.
  - `normalize.ts` — pgtype → plain shapes boundary.
- `features/lab/`:
  - `TrackDetailPage/TrackDetailPage.tsx` — **exists but not wired**
    into `RouteDetailContainer`. This is the page from issue #125
    that wraps `MapView` on top of `RouteDetail`.
  - `MapView/MapView.tsx` — MapLibre GL polyline renderer (OSM raster
    tiles, cleanup in `useEffect`).
  - `ComparisonMode/ComparisonMode.tsx`,
    `RouteComparator/RouteComparator.tsx`,
    `ComparisonMap/ComparisonMap.tsx`,
    `MetricsDiffTable/MetricsDiffTable.tsx`,
    `ComparisonElevationProfile/ComparisonElevationProfile.tsx`,
    `RiskZonesPanel/RiskZonesPanel.tsx` — full comparator from issue
    #126.

### Drift vs the prior plan (issues #156 / #157 / #125 + #126)

The trio described in `odd/tasks/track-detail-page-156-157-125.md` is
**mostly landed but not closed end-to-end**:

| Issue | Component | Status |
|-------|-----------|--------|
| #156 | `ElevationProfile` + `projectTrack` | ✅ Merged |
| #157 | `RouteDetailContainer` + `RouteDetail` (header / metrics / climbs / risks) | ✅ Merged |
| #125 | `TrackDetailPage` + `MapView` (compose map above detail) | ⚠️ Files merged; **NOT wired into the container** |
| #126 | `ComparisonMode` + comparator (map + diff + risk + elevation) | ✅ Merged |

Two specific gaps block the lab from feeling finished:

1. **`RouteDetailContainer` does not use `TrackDetailPage`.** It
   renders `RouteDetail` directly (no MapLibre polyline, no start/end
   markers on the detail page). The two TODO comments on
   `TrackDetailPage.tsx` lines 17–18 and `RouteDetail.tsx` lines 22–28
   both say the backend has not returned points — **the backend does
   return points** (via `subsamplePoints` in
   `internal/gpx/store.go:354`, JSON-marshalled through `Track.Points`
   in `internal/gpx/types.go:21`). The TODO comments are stale.

2. **`projectTrack` (the SVG elevation renderer) is fed an empty array
   on the single-track detail page and is fed a type-asserted
   `unknown[]` on the comparison page.** In `RouteDetail.tsx:29`, the
   `points` argument to `projectTrack` is hardcoded to `[]`. In
   `ComparisonElevationProfile.tsx:50`, raw `{ lat, lng, ele }` points
   are passed through `t.points as unknown as
   Parameters<typeof projectTrack>[0]` — a type-assertion bypass that
   produces `NaN` at runtime (because `projectTrack` reads
   `points[i].distance_m`, which is `undefined`).

3. **There is no client-side haversine distance accumulator.** Both
   renderers need `TrackPoint[]` (`distance_m` + `elevation_m`), but
   the backend serialises raw `{ lat, lon, ele }` and the frontend has
   no helper to convert one to the other. The `RouteComparator` page
   already filters with `isFlatPoint` (lat / lng / ele keys) and feeds
   coords to `MapView` directly, but no caller currently produces the
   `TrackPoint[]` shape.

### Backend bug found during reconciliation

`internal/gpx/store.go:357–366` (the `storedTrack` function) reads
`row.ElevationCoverage` exists on the sqlc-generated struct but
**discards it** when building `Analysis`:

```go
Analysis: Analysis{
    DistanceM: numericValue(row.DistanceM), ...
    DPlusM: numericPointer(row.DPlusM),
    DMinusM: numericPointer(row.DMinusM),
    MaxElevationM: numericPointer(row.MaxElevationM), ...
    // NOTE: row.ElevationCoverage is never propagated.
},
```

The migration 00010 persists the column, the SQLC SELECTs include it
(`SELECT *` and the explicit column lists in
`internal/db/sqlc/gpx_tracks.sql.go` lines 189, 237), and the Go
`gpx.Analysis` struct has the matching field
(`internal/gpx/types.go:48`). Only `storedTrack` fails to copy it.

This means the API exposes `d_plus_m: null` for tracks with
insufficient elevation coverage but **never exposes the coverage
value itself**, which is the signal the frontend needs to render a
"partial / insufficient" badge or substitute its heuristic (#160/#158/
#126 — explicitly listed in
`odd/tasks/gpx-elevation-coverage-171.md` § "Out of scope").

### Affected areas

- `web/src/features/gpx/RouteDetailContainer.tsx` — swap `RouteDetail`
  for `TrackDetailPage` when status is `ready`. Strip the stale
  `lab-detail.tsx` placeholder.
- `web/src/features/gpx/RouteDetail/RouteDetail.tsx` — feed real
  points into `projectTrack`; remove the misleading TODO comment.
- `web/src/features/lab/ComparisonElevationProfile/ComparisonElevationProfile.tsx`
  — replace the `as unknown as` type assertion with a proper converter
  call; remove the stale TODO comment.
- `web/src/features/gpx/` (new) — pure helper `rawPointsToTrackPoints`
  that takes `{ lat, lon, ele }[]` and returns `TrackPoint[]` (with
  cumulative haversine distance), plus TDD unit tests.
- `internal/gpx/store.go` (line ~357) — copy `row.ElevationCoverage`
  into `Analysis.ElevationCoverage` in `storedTrack`. Single-line
  semantic change; one store test extension.
- `internal/http/handlers/gpx_compare.go` (optional, low-cost) —
  expose `elevation_coverage` in the diff map so the comparator can
  warn per-track.

### Approaches

1. **Frontend-only (haversine + wiring, no backend fix)** — ~150
   lines of web code + tests. Closes the TODO comments and wires the
   map. Leaves `ElevationCoverage` un-propagated; the frontend still
   cannot surface a "partial elevation" badge. Pros: smallest blast
   radius; one PR. Cons: leaves the backend drift visible to API
   consumers and forces the frontend to fall back to its existing
   "D+==0 && D-==0 → warn" heuristic.
2. **Both (haversine + wiring + ElevationCoverage propagation)** —
   ~180–220 lines total (≈140 web + ≈40 Go incl. test). Closes the
   TODO comments, wires the map, AND makes the elevation coverage
   observable end-to-end. Pros: a single coherent "elevation data
   goes end-to-end" slice. Cons: cross-project (Go + web), so
   `make test` AND `pnpm -C web test` both gate it.
3. **Add a new `/api/v1/gpx/{id}/points` endpoint** — separate
   detail-points fetch. Pros: keeps list payload small, gives
   clients control over point resolution. Cons: ~400 lines including
   migration smoke, sqlc regen, handler + route + tests + 1–2 web
   integration points; pushes the slice over the 400-line review
   budget alone.

### Recommendation

**Approach 2** (frontend wiring + backend `ElevationCoverage`
propagation) is the natural next slice. Reasoning:

- It is what the existing code is reaching for. `TrackDetailPage`
  was built, `ElevationProfile` was built, the `ElevationCoverage`
  field exists on the Go struct, the column is in the DB — only the
  last 5 % of glue is missing.
- It is small enough to fit one PR (well under the 400-line review
  budget: ~140 web + ~40 Go).
- It is self-contained: each side is testable in isolation
  (`pnpm -C web test` for the converter + wiring; `make test` for the
  store change).
- It aligns with phase 1.3 product intent: "subir un GPX produce un
  análisis completo, visualizado en mapa 3D y persistido" (per
  `docs/roadmap/roadmap.md`). The map (MapLibre 2D; 3D is fase 1.6)
  and elevation profile are the visible centerpiece of the lab; this
  slice is what makes the lab page honest.

**Project(s) in scope**: **both** Go (`.`) and web (`web/`).

**Per-project test commands that gate this slice**:

- Go: `make test` (raw: `GOTOOLCHAIN=local go test -race ./...`).
- Web: `pnpm -C web test` (Vitest).
- Optionally `make lint` and `pnpm -C web typecheck` to keep quality
  green.

**Line-budget forecast**: ~180–220 authored lines (excluding
generated sqlc output, which is not touched by this slice). Well
within the 400-line budget. **No chained-PR split is required;**
Approach 3 (a separate `/points` endpoint) is documented in
`risks` below as a follow-up if list-payload pressure emerges later.

**Change candidate name** for `/sdd-new`:

```text
wire-elevation-and-map-to-track-detail
```

Verified to not be in use: `openspec/changes/` only contains
`.gitkeep` and the empty `archive/` directory.

### Risks

- **Risk: backend `ElevationCoverage` propagation may surprise
  reviewers.** The field already exists on the API JSON shape
  (`gpx.Analysis.ElevationCoverage *float64 \`json:"elevation_coverage,omitempty"\``)
  but is never populated by `storedTrack`. The fix is to add
  `ElevationCoverage: numericPointer(row.ElevationCoverage)` in
  `storedTrack`. Tests should pin the propagation explicitly. Low
  risk; mention it in the proposal so reviewers do not assume it
  already worked.
- **Risk: client-side haversine adds JS surface.** ~40 lines for the
  helper + ~50 lines of tests. Use the same earth-radius constant and
  formula already in `internal/gpx/analysis.go` (`earthRadiusM =
  6371e3`, haversine) for consistency; consider documenting in the
  proposal that we duplicate the formula in TS (or extract to a
  shared math note) so the next person does not refactor it
  inconsistently.
- **Risk: list endpoint payload size.** Today the list also returns
  up to ~4000 points per track (subsamplePoints default). With 20
  tracks per page the JSON can be ~80 000 points. Not in scope for
  this slice, but if the list page becomes slow, a follow-up slice
  could introduce `/api/v1/gpx/{id}/points?resolution=N` (the
  Approach 3 above). Note this in the proposal as a known
  follow-up, do not pre-select chain strategy.
- **Risk: `routes/lab-detail.tsx` is now dead code.** Delete it
  during the slice to keep the file tree honest. The
  `main.tsx`-mounted route is `/lab/:id` → `RouteDetailContainer`;
  `routes/lab-detail.tsx` is not referenced anywhere in `main.tsx`.
- **Decision needed before tasks: chained-PR strategy.** Not
  applicable — the slice fits one PR. Surface this in the proposal
  so the user can confirm `delivery_strategy` at `sdd-tasks` time
  (`single-pr` is the natural default; `auto-chain` and
  `size:exception` are not needed).
- **Risk: TODO-comment style drift in the project.** Several files
  carry TODO comments that drifted from the actual code state (the
  two above plus a few in `features/lab/*`). The slice includes
  removing the specific TODO comments that block the wiring;
  general TODO-cleanup is out of scope.

### Ready for Proposal

**Yes** — `nextRecommended: propose`. The next phase should be
`/sdd-new wire-elevation-and-map-to-track-detail`, followed by
`sdd-proposal` (which builds the proposal.md), then `sdd-spec` and
`sdd-design` against the proposal, then `sdd-tasks`, then `sdd-apply`
gated by both `make test` and `pnpm -C web test`.
