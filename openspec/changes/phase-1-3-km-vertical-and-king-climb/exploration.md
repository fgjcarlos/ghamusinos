# Exploration: phase-1-3-km-vertical-and-king-climb

> Phase 1.3 — Laboratorio GPX (base). Investigation of the gap
> between issue #15's "Entregables" and the code as it ships today.
> Issue #15's five sub-issues (#122, #123, #124, #125, #126) are all
> CLOSED, the verifications have been archived (`wire-elevation-and-
> map-to-track-detail` on 2026-09-27), and the feature inventory in
> `docs/architecture/feature-inventory.md` describes the 1.3 surface
> as complete. This exploration is the evidence base for the next
> proposal/design/spec chain (if a gap is found) or for closing the
> epic (if the gap is illusory).

## Problem statement

Issue #15 ("Fase 1.3 — Laboratorio GPX (base)") enumerates a long
list of deliverables in its body. Most of them are already shipped
under the five sub-issues, but three feature detection routines
compute a value at upload time and then **discard it**:

- `FindKmVertical(track)` → `*KmVerticalResult`
- `FindMuros(track)` → `[]Muro`
- `FindRecoveryZones(track, climbs)` → `[]RecoveryZone`

All three are reachable via `POST /api/v1/gpx/upload` (the handler
returns them in the response body under the keys `km_vertical`,
`muros`, `recovery_zones`), but **none** are persisted, **none** are
included in the `GET /api/v1/gpx/{id}` detail response, and the
frontend's `uploadGpx` (`web/src/lib/api/gpx.ts:101`) parses only
`{ id: string }` from the upload body. After a page refresh, the
Km Vertical, muros and recovery zones are gone from the user view.

The roadmap (`docs/roadmap/roadmap.md:38`) treats all three as 1.3
deliverables, alongside King Climb (which IS persisted) and the
already-shipped climbs/risk zones UI. So this is a real, narrow
gap, not a misreading.

The other 1.3 deliverables from #15 (Effort Index, ITRA, VAM,
Leg-Breaker, Runnability, Difficulty, persistence/file hash, map)
are all live in main as of `02e6c41` (PR #225, archive verify
2026-09-27).

The 3D map + slope heatmap that the issue body and roadmap
(`roadmap.md:38`) mention are explicitly Fase 1.6 deliverables per
`feature-inventory.md:108-110` and the roadmap section "Fase 1.6 —
Laboratorio GPX avanzado" — out of scope here.

## Current State

### What shipped for phase 1.3 already (verified on disk)

**Backend (`internal/`, `internal/db/`):**

- `internal/gpx/` is a complete Go package: `parser.go`, `analysis.go`
  (`CalculateElevation` with `CoverageStatus` and
  `ElevationResult` from issue #171/A8), `climbs.go`
  (`FindAllClimbs`, `FindKingClimb` with VAM, `FindMuros`,
  `FindRecoveryZones`, `FindKmVertical`), `risk_zones.go`
  (`DetectSteep`, `DetectTechnical`, `DetectExposure`),
  `track_type.go` (`Detect` circular/point-to-point with direction),
  `validation.go`, `hash.go` (sha256), `store.go` (with
  `subsamplePoints` from #170/M9, `ErrNumericConversion` from
  #170/M8, transactional `CreateDetail`).
- `internal/gpx/analysis.go` carries every metric named in #15:
  Effort Index (`:188`), ITRA Points (`:196`), Leg-Breaker Index
  (`:207`), Estimated VAM (`:213`), Difficulty
  (`CalculateDifficulty` + `difficultyScore` in handler, `:225`),
  Runnability Index (`:257`).
- `internal/http/handlers/`: `gpx_upload.go` (with the now-orphaned
  muros/km_vertical/recovery_zones JSON shape, `:143-146`),
  `gpx_get.go`, `gpx_list.go` (incl. delete), `gpx_compare.go`
  (max 3 tracks, `computeDiff` covers 6 metrics: distance, D+,
  D-, difficulty, leg-breaker, ITRA, moving_time_s).
- `internal/http/router.go:194-201`: all five GPX routes mounted
  under `/api/v1/gpx/*`.
- `internal/db/migrations/00006_gpx_tracks.sql`: schema for
  `gpx_tracks` + `gpx_climbs` + `gpx_risk_zones`. **Climbs and
  risk zones only — no muros, no km_vertical, no recovery_zones.**
- `internal/db/migrations/00010_gpx_tracks_elevation_coverage.sql`:
  makes `d_plus_m`/`d_minus_m` nullable, adds `elevation_coverage
  NUMERIC` (issue #171/A8).
- `internal/db/queries/`: `gpx_tracks.sql` (Create / Get / List /
  Hash / Delete), `gpx_climbs.sql`, `gpx_risk_zones.sql`. No
  muros/km/recovery queries exist.

**Frontend (`web/src/`):**

- `routes/lab.tsx` — list + upload + delete + select-to-compare.
- `routes/lab-compare.tsx` — `/lab/compare?ids=...` page
  (mounted at `/lab/compare`, also reachable from `/lab`).
- `features/gpx/`:
  - `RouteDetailContainer.tsx` — fetch + state machine (loading /
    no-token / not-found / error / ready) and renders
    `TrackDetailPage` when ready.
  - `RouteDetail/` (`RouteDetail.tsx`) composes header + metrics +
    climbs + risks + elevation profile.
  - `RouteHeader/`, `RouteMetrics/` (renders 10 metrics:
    Distancia, D+/-, Tiempo, Pendiente media/máx, ITRA, Effort,
    VAM, Corribilidad), `RouteClimbs/` (King Climb badge, VAM per
    climb), `RouteRisks/`, `ElevationProfile/` (SVG hand-rolled,
    `projectTrack.ts` with its own TDD suite), `normalize.ts`,
    `rawPointsToTrackPoints.ts`.
- `features/lab/`:
  - `TrackDetailPage/` — wraps `MapView` over `RouteDetail`,
    mounted from `RouteDetailContainer.tsx:98`.
  - `MapView/` — MapLibre GL 2D polyline renderer (OSM raster
    tiles, cleanup in `useEffect`).
  - `ComparisonMode/`, `RouteComparator/`,
    `ComparisonElevationProfile/`, `ComparisonMap/`,
    `MetricsDiffTable/`, `RiskZonesPanel/`.
- `lib/api/types.ts`: `GpxAnalysis` declares `elevation_coverage:
  number | null` (and every other #15 metric — `effort_index`,
  `itra_points`, `leg_breaker_index`, `estimated_vam`,
  `difficulty_score`/`difficulty_label`, `runnability_pct`).
- `lib/api/gpx.ts`: `uploadGpx` returns `Promise<{ id: string }>`
  (the muros/km_vertical/recovery_zones from the backend body are
  parsed and discarded).

### Drift vs the prior plan (issues #156, #157, #125, #126)

The closed change `wire-elevation-and-map-to-track-detail` shipped
the elevation profile + map wiring end-to-end. The only drift is
the three orphaned detection routines. Specifically:

| Source | Status |
|---|---|
| `internal/gpx/climbs.go:108` `FindMuros` | Computed; returned in upload body; **not persisted; not returned by GET** |
| `internal/gpx/climbs.go:142` `FindRecoveryZones` | Computed; returned in upload body; **not persisted; not returned by GET** |
| `internal/gpx/climbs.go:183` `FindKmVertical` | Computed; returned in upload body; **not persisted; not returned by GET** |
| `internal/gpx/climbs.go:84` `FindKingClimb` | Computed; **persisted twice** (as JSONB `king_climb` column on `gpx_tracks` AND as a `gpx_climbs` row with `is_king_climb=true`); exposed via GET; rendered with badge in `RouteClimbs.tsx` |
| `internal/gpx/climbs.go:36` `FindAllClimbs` | Computed; persisted in `gpx_climbs` table; exposed; rendered |

The three orphaned routines are not failing — they work correctly
in the upload response, with unit tests
(`internal/gpx/climbs_test.go:97-213`) and a handler-level test
(`gpx_upload_test.go:132-134`). They simply have no home after
upload completes. The user's Km Vertical, muros and recovery
zones exist only in the upload POST body for the duration of that
request and are then forgotten.

The test that pins the current (lossy) shape is
`gpx_upload_test.go:117-134`:

```go
require.Contains(t, recorder.Body.String(), `"muros":[]`)
require.Contains(t, recorder.Body.String(), `"recovery_zones":[]`)
require.Contains(t, recorder.Body.String(), `"km_vertical":null`)
```

Any change that persists and re-serves these must update those
assertions or the test breaks.

### Backend bug (or design choice) found during reconciliation

`internal/gpx/store.go:224-262` `createDetail` accepts
`climbs []Climb` and `riskZones []RiskZone` but **does not**
accept `muros`, `recoveryZones`, or `kmVertical` — those are
computed by `gpx_upload.go:94-105` and never reach the store:

```go
// gpx_upload.go (excerpt)
muros, _ := climbDetector.FindMuros(track)
recoveryZones, _ := climbDetector.FindRecoveryZones(track, climbs)
kmVertical, _ := climbDetector.FindKmVertical(track)
...
detail, err := store.CreateDetail(r.Context(), track, analysis, climbs, riskZones, kingClimb)
// ^ muros / recoveryZones / kmVertical are passed only to escribirJSON, never persisted
```

The signature on the `gpx.GPXStore` interface (`types.go:223`)
matches: `CreateDetail(ctx, *Track, *Analysis, []Climb, []RiskZone,
*Climb)`. The three orphaned outputs hang off the side of the
upload pipeline.

### Affected areas (if a slice is opened)

- `internal/gpx/types.go` — `StoredTrackDetail` adds muros,
  recovery_zones, km_vertical fields.
- `internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql`
  (new) — three new tables, or one wide table, or fold recovery
  zones into the existing `gpx_risk_zones` (cleaner: new tables,
  recovery zones are NOT risk zones — semantically distinct).
- `internal/db/queries/gpx_muros.sql` (new),
  `gpx_recovery_zones.sql` (new), `gpx_km_vertical.sql` (new).
- `internal/db/sqlc/*` regenerated.
- `internal/gpx/store.go`:
  - `CreateDetail` signature grows; `gpxQuerier` interface grows.
  - `GetDetail` re-hydrates muros/recovery/km_vertical.
- `internal/gpx/climbs.go` — `ClimbDetector` interface unchanged;
  callers learn to consume the new fields.
- `internal/http/handlers/gpx_upload.go` — pass the three slices
  into `CreateDetail`; upload response shape no longer needs the
  duplicated top-level keys (mover a `StoredTrackDetail`).
- `internal/http/handlers/gpx_compare.go` — `computeDiff` may want
  to include muros/km_vertical/recovery comparisons (lower
  priority).
- `internal/http/handlers/gpx_get_test.go`,
  `internal/http/handlers/gpx_upload_test.go` — assertions
  update.
- `web/src/lib/api/types.ts` — `StoredTrackDetail` adds muros/
  recovery_zones/km_vertical; mirror types in
  `features/gpx/normalize.ts`.
- `web/src/features/lab/ComparisonElevationProfile.tsx` and
  `web/src/features/lab/RouteComparator.tsx` — pass-through.
- New presentational components (mirrors of `RouteClimbs.tsx` /
  `RouteRisks.tsx`):
  - `web/src/features/gpx/RouteMuros/RouteMuros.tsx`
  - `web/src/features/gpx/RouteKmVertical/RouteKmVertical.tsx`
  - `web/src/features/gpx/RouteRecovery/RouteRecovery.tsx` (or
    fold into RouteRisks if product decides recovery ≈ risk-flat).
- `web/src/features/gpx/RouteDetail/RouteDetail.tsx` — compose the
  new panels alongside `RouteClimbs` / `RouteRisks`.
- `web/src/lib/api/gpx.ts` — `uploadGpx` may stop discarding the
  full response body (or keep the `{ id: string }` narrowing — both
  are acceptable).

### Approaches

1. **Persist muros + km_vertical + recovery_zones, return them on
   GET, render them.** ~300–400 LoC of backend (1 migration +
   3 SQLC queries + store plumbing + 4 handler tests) + ~250 LoC
   of web (3 presentational components + normalize types +
   RouteDetail composition + 3 component tests). **One user-visible
   gap closed: the user can now see their Km Vertical after a
   refresh.** Does NOT add the 3D map or slope heatmap (those are
   explicit Fase 1.6 deliverables).
2. **Same as 1 + add a `/gpx/{id}/summary` endpoint that returns
   only the climb-derived features (king_climb, muros, km_vertical,
   recovery_zones).** Lower-coupling alternative; keeps the detail
   endpoint stable for clients that don't care. Probably 100 LoC
   more than Approach 1 with no clear user benefit (the existing
   detail endpoint already carries `climbs`/`risk_zones`; adding
   three more siblings is cheap).
3. **Same as 1 + comparison-side integration (`computeDiff`
   includes muros/km_vertical/recovery comparisons).** Adds ~80
   LoC of compute diff + a column in `MetricsDiffTable` and a
   section in `RiskZonesPanel` / equivalent. Nice-to-have for the
   comparator page but not the smallest valuable slice — the
   single-track gap is independent of the comparator gap.

### Recommendation

**Approach 1** (persist + return + render the three orphaned
features; no 3D map, no heatmap, no comparator integration this
slice). Reasoning:

- It is the smallest slice that makes the existing analysis
  persistent, which is the user-valuable delta. After this lands,
  reloading `/rutas/:id` after an upload does not silently drop
  features the user already saw once.
- It is bounded. Three new DB tables + a small store change +
  three small presentational components. The backend blast
  radius is the `CreateDetail` signature and the `gpxQuerier`
  interface — both already documented to grow
  (`store.go:99-108`).
- It does NOT touch the 3D map / slope heatmap, which is the
  feature inventory's explicit 1.6 deliverable and the epic body
  has been shipped as the "laboratorio avanzado" line.
- It is self-contained: backend gates on `make test` (no SQLC regen
  surprises); web gates on `pnpm -C web test`. No cross-project
  coupling.
- A future comparator-side slice (Approach 3) becomes a clean
  diff-extension on top of the persisted fields without needing
  to re-derive them at compare time.

**Project(s) in scope**: **both** Go (`.`) and web (`web/`).

**Per-project test commands that gate this slice**:

- Go: `make test` (raw: `GOTOOLCHAIN=local go test -race ./...`).
- Web: `pnpm -C web test` (Vitest).
- Optionally `make lint`, `make vet`, `make fmt`,
  `pnpm -C web typecheck`, `pnpm -C web lint` to keep quality
  green.

**Line-budget forecast**: ~550–650 authored lines
(≈300 backend + ≈250 web + ≈80 tests). **Exceeds the 400-line
review budget** by ~150–250 LoC. **Chain-PR split is
recommended** (see Risks).

**Change candidate name** for `/sdd-new`:

```text
phase-1-3-km-vertical-and-king-climb
```

Verified to not be in use: `openspec/changes/` only contains
`.gitkeep` and the empty `archive/` directory (which itself is
out-of-bounds per the change-naming rules).

### Risks

- **Risk: SQLC regen touches `internal/db/sqlc/*` broadly.** A new
  migration + new queries means `make generate` rewrites
  `gpx_tracks.sql.go`, `models.go`, `querier.go` etc. Reviewers
  should verify only the expected additions show up in the diff.
  Low risk; the existing CI already runs SQLC in CI.
- **Risk: `CreateDetail` signature change ripples.** Every mock
  that implements `UploadGPXStore.CreateDetail` (currently
  `internal/http/handlers/gpx_upload_test.go:24` and any router
  test) needs to add the three new params. Mechanical. Low risk
  if the slice carries a one-liner migration in tests.
- **Risk: muros and recovery zones may be stored on the wrong
  side of the schema boundary.** Recovery zones are NOT risk
  zones (they have no severity; semantically "post-climb flat").
  Putting them in `gpx_risk_zones` would conflate them with
  steep/technical/exposure. Recommend **a new
  `gpx_recovery_zones` table**, mirroring `gpx_risk_zones` minus
  `severity`/`risk_type`. Low risk if reviewers catch the design
  early in the proposal.
- **Risk: km_vertical is a singleton (one row per track).** The
  detection routine returns `*KmVerticalResult`; the schema should
  be `gpx_km_vertical` with `track_id` and UNIQUE, NOT a generic
  table. Easy to get right; flagging it.
- **Risk: comparator (Approach 3) temptation.** Avoid growing the
  scope to include `computeDiff` extensions. Defer to a follow-up
  change; the existing comparator already works.
- **Risk: 3D map + slope heatmap creep.** If reviewers ask "but
  the issue body says 3D map", the answer is "the issue body is
  out of sync with the architecture decision (Fase 1.6 per
  `feature-inventory.md:108-110`)". Mention it in the proposal so
  the user can confirm `delivery_strategy` at `sdd-tasks` time.
- **Decision needed before tasks: chained-PR strategy.** Likely
  **yes** — the slice exceeds 400 LoC. Two reasonable splits:
  - **PR1 (backend)** — migration + SQLC + store + handler
    + Go tests. ~300–350 LoC. Closes the gap on the wire; GET
    returns the three features. No UI yet.
  - **PR2 (web)** — type + normalize + 3 presentational
    components + RouteDetail composition + 3 component tests.
    ~200–250 LoC. Closes the user-visible gap.
  Either is a clean review; PR2 has a clean UI-only diff on top
  of PR1's stable wire shape. The user can pick `auto-chain` at
  `sdd-tasks` time.
- **Risk: TODO-comment style drift in the project.** No TODO/FIXME
  comments remain in `internal/gpx/` or `web/src/features/lab/`
  (grep verified). The slice adds fresh code; no comment cleanup
  needed in scope.

### Closing the epic instead?

If the user prefers, the alternative is to **close #15 as already
shipped** and document the orphaned-feature caveat in a follow-up
issue that this change would address. Reasoning either way:

- **Close #15 now**: every #15 entregable EXCEPT the three
  orphaned routines + the 1.6-deferred 3D/heatmap is shipped.
  The persisted-vs-orphaned distinction is invisible to a user
  who only sees the upload response once. The product could ship
  as-is.
- **Keep #15 open and ship this slice**: the orphaned routines
  are already computed and tested — the cost to make them
  persistent is small (~600 LoC). After this slice, every
  analytical feature the backend produces is observable from
  the SPA, end-to-end, which is the spirit of phase 1.3's
  closing criterion ("subir un GPX produce un análisis completo,
  visualizado en mapa 3D y persistido" — the *persisted* part is
  currently false for 3 of the 7 climb-derived features).

**My recommendation: keep #15 open and ship this slice as the
last 1.3 deliverable, with the 3D/heatmap pieces explicitly
deferred to 1.6 per the feature inventory.** The cost is
modest, the user-visible delta is real (refresh survival), and
the slice closes the only remaining 1.3 gap.

## Gap matrix

The columns are: **Implemented?** (✅ shipped end-to-end, ⚠️
partial — computed but not persisted/served/rendered, ❌ absent),
**Where** (file paths and line ranges proving the state),
**Gap** (what is missing if not ✅).

| Item from #15 body | Implemented? | Where | Gap |
|---|---|---|---|
| Parsing GPX | ✅ | `internal/gpx/parser.go`; `internal/http/handlers/gpx_upload.go:71-86` | — |
| Core analysis (distance, D+/D-, slope, max/min ele) | ✅ | `internal/gpx/analysis.go:46-176`; persisted `gpx_tracks.distance_m`, `d_plus_m`, `d_minus_m`, `avg_slope_pct`, `max_slope_pct`, `max_elevation_m`, `min_elevation_m` (`internal/db/migrations/00006_gpx_tracks.sql:9-18`) | — |
| Effort Index | ✅ | `internal/gpx/analysis.go:188` `CalculateEffortIndex`; persisted `effort_index`; rendered `RouteMetrics.tsx:50` "Índice esfuerzo"; in compare diff `gpx_compare.go:122` | — |
| ITRA | ✅ | `internal/gpx/analysis.go:196` `CalculateITRAPoints`; persisted `itra_points`; rendered `RouteMetrics.tsx:48` "ITRA"; in compare diff `gpx_compare.go:140` | — |
| VAM | ✅ | Per-track: `internal/gpx/analysis.go:213` `CalculateEstimatedVAM`; persisted `estimated_vam`; rendered `RouteMetrics.tsx:51`. Per-king-climb: `internal/gpx/climbs.go:84-103` `FindKingClimb` computes `vam` from track timestamps; exposed via `GpxClimb.vam`; rendered `RouteClimbs.tsx:55-60` | — |
| Leg-Breaker | ✅ | `internal/gpx/analysis.go:207` `CalculateLegBreakerIndex`; persisted `leg_breaker_index`; in compare diff `gpx_compare.go:136`. **No metric tile in RouteMetrics** — present in API but not rendered as a tile. | Cosmetic: add a tile in `RouteMetrics.tsx` (low-priority polish, in scope as part of this slice or a follow-up). |
| Runnability | ✅ | `internal/gpx/analysis.go:257` `CalculateRunnabilityIndex`; persisted `runnability_pct`; rendered `RouteMetrics.tsx:52` "Corribilidad" | — |
| Difficulty | ✅ | `internal/gpx/analysis.go:225` `CalculateDifficulty` + handler `difficultyScore` in `gpx_upload.go:255`; persisted `difficulty_score` + `difficulty_label`; rendered `RouteHeader.tsx:46` DifficultyBadge | — |
| Km Vertical | ⚠️ Partial | Computed: `internal/gpx/climbs.go:183` `FindKmVertical` with tests `climbs_test.go:141-163`. Returned in upload body: `internal/http/handlers/gpx_upload.go:104,145` (key `km_vertical`). **Not persisted. Not in `GET /gpx/{id}` (StoredTrackDetail has no field). Not in any web component.** | Migrate + SQLC + store + handler shape + new `RouteKmVertical` panel + `RouteDetail` composition. ~250 LoC total. |
| King Climb | ✅ | `internal/gpx/climbs.go:84` `FindKingClimb` with VAM computation; persisted as JSONB `king_climb` (`store.go:432-447`) AND as a `gpx_climbs` row with `is_king_climb=true`; exposed via GET; rendered with badge in `RouteClimbs.tsx:33-43`. | — |
| Muros | ⚠️ Partial | Computed: `internal/gpx/climbs.go:108` `FindMuros` with tests `climbs_test.go:97-113`. Returned in upload body: `gpx_upload.go:94,143` (key `muros`). **Not persisted. Not in `GET /gpx/{id}`. Not in any web component.** | Same as Km Vertical. ~150 LoC total (smaller surface — muros is a list, not a singleton). |
| Recovery zones | ⚠️ Partial | Computed: `internal/gpx/climbs.go:142` `FindRecoveryZones` with tests `climbs_test.go:116-138`. Returned in upload body: `gpx_upload.go:99,144` (key `recovery_zones`). **Not persisted. Not in `GET /gpx/{id}`. Not in any web component.** (Note: risk zones, which already ship, are semantically distinct — see Risks.) | Same as Km Vertical. ~150 LoC total. |
| Risk zones | ✅ | `internal/gpx/risk_zones.go` `Detect`/`DetectSteep`/`DetectTechnical`/`DetectExposure`; persisted `gpx_risk_zones` table (`migrations/00006_gpx_tracks.sql:48-58`); exposed via GET; rendered `RouteRisks.tsx` and `RiskZonesPanel.tsx` | — |
| Persistence and file hash | ✅ | `internal/gpx/hash.go` (sha256); persisted `gpx_tracks.file_hash`; duplicate detection `gpx_upload.go:62-70` returns 409 Conflict with existing `track_id` | — |
| 2D Map (MapLibre) | ✅ | `web/src/features/lab/MapView/MapView.tsx` (polyline + start/end markers); mounted by `TrackDetailPage.tsx:44`; ComparisonMap for up to 3 tracks `web/src/features/lab/ComparisonMap/ComparisonMap.tsx` | — |
| 3D MapLibre + slope heatmap | ❌ Deferred to 1.6 | `docs/roadmap/roadmap.md:38` lists this in 1.3 entregables, but `docs/architecture/feature-inventory.md:108` (and the roadmap's own "Fase 1.6 — Laboratorio GPX avanzado" section) move it to Fase 1.6. Current MapView renders polyline only — no terrain, no slope coloring. | **Out of scope for this slice.** If the user wants 1.3 to claim 3D/heatmap, that's a separate phase 1.3 → 1.6 scope decision, not a 1.3 deliverable. |
| Route comparator (max 3) | ✅ | `internal/http/handlers/gpx_compare.go` POST `/api/v1/gpx/compare`; `web/src/features/lab/ComparisonMode/` + `RouteComparator/` | — |

### Sub-issue-to-deliverable mapping (for audit)

| Sub-issue | Title (from PR / commit log) | Maps to #15 deliverable(s) | Verdict |
|---|---|---|---|
| #122 | parser + core analysis + DB schema (foundational, merged before PR #225) | Parsing, core analysis, Effort/ITRA/VAM/Leg-Breaker/Runnability/Difficulty, persistence/hash | ✅ Shipped |
| #123 | climb detection + risk zones + upload handler | AllClimbs, FindKingClimb, FindMuros, FindRecoveryZones, FindKmVertical, Risk zones (Detect) | ✅ Detectors shipped + tested; muros/KmVertical/recovery zones returned in upload but not persisted |
| #124 | list + compare APIs + frontend upload UI | List/Compare endpoints, Lab route with FileUploadZone, select-to-compare | ✅ Shipped |
| #125 | track detail page + 2D map + elevation profile | MapView (2D), TrackDetailPage, ElevationProfile, RouteDetail, RouteHeader/Metrics/Climbs/Risks | ✅ Shipped |
| #126 | route comparator + risk zones panel | ComparisonMode, RouteComparator, ComparisonMap, MetricsDiffTable, ComparisonElevationProfile, RiskZonesPanel | ✅ Shipped |

## Recommended change scope

**Phase 1.3 close-out slice — persist + serve + render the three
orphaned detection routines (Km Vertical, Muros, Recovery Zones)
so every analytical feature the backend produces is observable
end-to-end after a page refresh.**

- **Backend** (`.`):
  - Migration `00011_gpx_muros_recovery_kmvertical.sql` (new):
    three new tables — `gpx_muros`, `gpx_recovery_zones`,
    `gpx_km_vertical` (with `track_id` UNIQUE for the last).
  - SQLC queries: `gpx_muros.sql`, `gpx_recovery_zones.sql`,
    `gpx_km_vertical.sql`; regen via `make generate`.
  - `internal/gpx/store.go`: `CreateDetail` grows to accept the
    three new slices; `GetDetail` re-hydrates them; the
    `gpxQuerier` interface mirrors.
  - `internal/gpx/types.go`: `StoredTrackDetail` adds `Muros`,
    `RecoveryZones`, `KmVertical`.
  - `internal/http/handlers/gpx_upload.go`: pass the three slices
    into `CreateDetail`; drop the duplicated top-level keys from
    the upload body (they now live under `stored_track_detail`,
    matching what GET returns).
  - Test updates: `gpx_upload_test.go:132-134` asserts move into
    `StoredTrackDetail`; new `gpx_get_test.go` cases cover muros
    re-hydration; `store_test.go` extends the mock to satisfy the
    new `gpxQuerier` interface.
  - LoC forecast (Go): **~300–350** including tests.

- **Web** (`web/`):
  - `web/src/lib/api/types.ts`: `StoredTrackDetail` adds the three
    fields (parallel Go struct).
  - `web/src/features/gpx/normalize.ts`: forward the three fields.
  - Three new presentational components, mirroring
    `RouteClimbs.tsx` / `RouteRisks.tsx`:
    - `RouteKmVertical/` — singleton (gain_m, distance_m,
      avg_slope_pct; one card).
    - `RouteMuros/` — list of walls (gain_m, distance_m,
      avg_slope_pct; `MuroCard`).
    - `RouteRecovery/` — list of post-climb flat zones
      (distance_m; subtle "flat recovery" framing).
  - `web/src/features/gpx/RouteDetail/RouteDetail.tsx`: compose
    the three new panels alongside `RouteClimbs`/`RouteRisks`.
  - TDD per component: minimum one test per component asserting
    the panel renders when the data is present and the empty
    state otherwise.
  - `web/src/lib/api/gpx.ts`: `uploadGpx` may keep the narrow
    `{ id: string }` return type (acceptable — the SPA navigates
    to the detail page after upload and reads from there).
  - LoC forecast (web): **~200–250** including tests.

- **Total LoC forecast**: ~550–650. **Exceeds the 400-line review
  budget by 150–250 LoC.**

### Chain-PR recommendation

**Two PRs**, decided with the user up front:

- **PR1 — backend persistence + wire shape**: migration, SQLC
  queries, store changes, handler changes, Go tests. Closes the
  gap on the wire (`GET /gpx/{id}` returns muros/km_vertical/
  recovery_zones). No UI yet. ~300–350 LoC, fits under budget.
- **PR2 — web UI**: types, normalize, three presentational
  components, `RouteDetail` composition, component tests.
  ~200–250 LoC, fits under budget. Clean diff on top of PR1's
  stable wire shape.

PR1 is testable end-to-end with `curl` + a bearer token (the
detail response body now carries the three fields). PR2 is a
pure UI slice. Either order works; PR1 first is conservative
(wire shape is locked before any UI depends on it).

Alternative: a single PR with `size:exception` accepted by the
user, mirroring what `wire-elevation-and-map-to-track-detail`
did. Both are reasonable; the choice is a `sdd-tasks`-time
decision, not an exploration decision.

## Files and call-sites that prove the gap

| File | Line(s) | Evidence |
|---|---|---|
| `internal/gpx/climbs.go` | 183-208 | `FindKmVertical` exists with full TDD coverage but no consumer beyond upload |
| `internal/gpx/climbs.go` | 108-139 | `FindMuros` exists with TDD coverage but no consumer beyond upload |
| `internal/gpx/climbs.go` | 142-179 | `FindRecoveryZones` exists with TDD coverage but no consumer beyond upload |
| `internal/gpx/climbs_test.go` | 97-213 | Three full TDD suites proving the detectors work |
| `internal/gpx/types.go` | 112-129 | `Muro`, `RecoveryZone`, `KmVerticalResult` types exist with JSON tags |
| `internal/gpx/types.go` | 151-156 | `StoredTrackDetail` has only `Track`, `Climbs`, `RiskZones` — no muros/km/recovery fields |
| `internal/gpx/types.go` | 208-210 | `ClimbDetector` interface declares all three `Find*` methods (caller-side proof) |
| `internal/gpx/types.go` | 223 | `CreateDetail` signature has no muros/km/recovery params |
| `internal/http/handlers/gpx_upload.go` | 94-105 | Computes muros/recovery/km and assigns to local vars |
| `internal/http/handlers/gpx_upload.go` | 143-146 | Returns the three values in the upload body via duplicated top-level keys |
| `internal/http/handlers/gpx_upload_test.go` | 132-134 | Pins the current loss: `"muros":[]`, `"recovery_zones":[]`, `"km_vertical":null` |
| `internal/http/handlers/gpx_get.go` | 17-19, 53 | `GPXDetailStore.GetDetail` returns `StoredTrackDetail` — no muros/km/recovery |
| `internal/db/migrations/00006_gpx_tracks.sql` | 1-60 | Tables: `gpx_tracks`, `gpx_climbs`, `gpx_risk_zones`. No muros/km/recovery tables. |
| `internal/db/queries/` | gpx_climbs.sql, gpx_risk_zones.sql | No queries for muros/km/recovery. |
| `internal/gpx/store.go` | 224-262 | `createDetail` writes `climbs` and `risk_zones`; no muros/km/recovery writes |
| `internal/gpx/store.go` | 99-108 | `gpxQuerier` interface — no muros/km/recovery methods |
| `internal/gpx/store.go` | 432-447 | `marshalKingClimb` — only king climb is special-persisted |
| `web/src/lib/api/types.ts` | 219-224 | `StoredTrackDetail` has only `track`, `climbs`, `risk_zones` |
| `web/src/lib/api/gpx.ts` | 95-115 | `uploadGpx` discards the upload body, returning only `{ id: string }` |
| `web/src/features/gpx/normalize.ts` | 150-156 | `normalizeTrackDetail` flattens the three missing fields as undefined |
| `web/src/features/gpx/RouteClimbs.tsx` | — | Renders climbs only; no muros/km/recovery panels nearby |
| `web/src/features/gpx/RouteRisks.tsx` | — | Renders risks only; recovery zones are semantically distinct (flat post-climb) so they cannot piggy-back here without confusing the UI |

## What the existing code already supports (no work needed)

- **HTTP plumbing**: `router.go:194-201` already mounts `/gpx/*`
  under auth; adding new tables requires no router change.
- **SQLC generation pipeline**: `make generate` already wires
  the codegen for `gpx_tracks`/`gpx_climbs`/`gpx_risk_zones`;
  three new SQL files follow the same pattern.
- **The store's existing helper `storedTrack`**: the central
  conversion point for `GetByID`/`GetDetail`/`FindByHash`/`List`
  is the natural place to copy the new fields into
  `StoredTrackDetail` — same pattern as the ElevationCoverage
  propagation that just shipped.
- **Handler test pattern**: `gpx_upload_test.go` already has
  `uploadGPXStore` mock with a tiny per-call interface — the
  slice extends it cleanly.
- **TDD scaffolding**: `internal/gpx/climbs_test.go` already
  has the full unit-test coverage for the three detectors. The
  slice adds **integration tests at the store/handler boundary**
  rather than re-deriving the detector tests.
- **Web test patterns**: `RouteClimbs.tsx` and `RouteRisks.tsx`
  already follow the same shape (`<section>` with empty state,
  `<ul>` of items, severity/elevation framing). The three new
  panels follow the same template.

## Constraints / risks

- **Wire-shape compatibility**: `StoredTrackDetail` adds three
  optional fields. Clients that already consume `climbs`/
  `risk_zones` are unaffected (additive change). The upload
  response shape change (the duplicated `muros` / `recovery_zones`
  / `km_vertical` top-level keys) is a breaking change for
  anything that reads them — only the SPA consumes this endpoint
  and the SPA's `uploadGpx` already discards the body, so the
  blast radius is zero.
- **3D map + slope heatmap out of scope**: per the architecture
  decision in `feature-inventory.md`. If the user disagrees, the
  slice grows to 1.6 territory and should be re-scoped.
- **Auth + multi-tenancy**: no change. All three new tables
  inherit the existing FK to `gpx_tracks(id)` with
  `ON DELETE CASCADE`.
- **Recovery zones vs risk zones design**: kept in a separate
  table (semantic distinction — recovery zones are flat
  post-climb, not risky). The reviewer should accept this early
  in the proposal.
- **KmVertical as singleton**: schema choice. A unique
  `track_id` index on `gpx_km_vertical` is sufficient; no need
  for a start_idx/end_idx pair (the detector returns a single
  interval per track).
- **Muros de-duplication**: the detector may emit multiple muros
  per track. Each is a row in `gpx_muros`; the list endpoint
  orders by `start_idx` (mirroring `gpx_climbs`).
- **Comparator integration**: out of scope. `computeDiff` keeps
  its current 6-metric surface; adding muros/km/recovery to the
  comparator is a follow-up.

## Out-of-scope (explicit non-goals)

- **3D MapLibre + slope heatmap** — explicit Fase 1.6 per
  `feature-inventory.md`. If a reviewer asks, point them there.
- **Comparator diff extensions** — `computeDiff` keeps its 6
  metrics; a future slice can extend it once muros/km/recovery
  are persistent.
- **Re-running analysis on existing tracks** — the detectors are
  deterministic; the slice does not re-analyze pre-existing data
  (out of scope per the Epic's "future separate change"
  convention).
- **ElevationCoverage UI** — already shipped as a typed
  `number | null`; no visible badge is in scope (separate
  concern, belongs to its own slice).
- **Web coverage / chained PR strategy** — covered by
  `openspec/config.yaml` rules and the 400-line budget. The
  recommendation here is **two PRs** (PR1 backend, PR2 web) with
  `auto-chain` chosen at `sdd-tasks` time.

## Drift discovered

None at the architecture level. The drift is at the **API
contract** level: the upload response exposes muros/km_vertical/
recovery_zones, but the GET response does not. This is a contract
gap (additive on the GET side, removal on the upload side),
solvable in one cohesive slice.

## Decision summary

This exploration recommends:

- **Open a new change** named `phase-1-3-km-vertical-and-king-
  climb` with the scope in "Recommended change scope" above.
- **Two PRs** chained: backend first (persistence + wire shape),
  web second (UI).
- **3D map + slope heatmap** stay in 1.6; mention the deferral
  in the proposal so the user can confirm.
- **Closing the epic** is the alternative if the user prefers
  to ship as-is and document the orphaned-feature caveat in a
  follow-up issue; my recommendation is to keep #15 open and
  ship this slice as the final 1.3 deliverable.

The slice is small enough to fit two PRs under the 400-line
budget each. The blast radius is contained to `internal/gpx/`,
`internal/db/migrations/`, `internal/db/queries/`,
`internal/db/sqlc/`, `internal/http/handlers/gpx_*`, and the
web layer's `lab/` + `gpx/` features. No router, auth, or
config changes. No SQLC semantic surprises (the new tables
mirror existing ones).

## Ready for Proposal

**Yes** — `nextRecommended: propose`. The next phase should be
`/sdd-new phase-1-3-km-vertical-and-king-climb`, followed by
`sdd-proposal` (which builds the proposal.md), then `sdd-spec`
and `sdd-design` against the proposal, then `sdd-tasks` (which
will pick the chain strategy), then `sdd-apply` gated by both
`make test` and `pnpm -C web test`.