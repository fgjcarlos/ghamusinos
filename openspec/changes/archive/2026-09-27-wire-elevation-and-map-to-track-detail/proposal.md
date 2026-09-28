# Proposal: Wire elevation profile and map to the track-detail page

> Phase 1.3 — Laboratorio GPX (continuation). This proposal is
> authored against the exploration in
> `openspec/changes/wire-elevation-and-map-to-track-detail/exploration.md`
> and against the on-disk truth verified during proposal work. The
> slice is small enough to ship as one PR; the chain-strategy
> decision is intentionally deferred to `/sdd-tasks`.

## Why

The user-visible lab track-detail page is broken end-to-end despite
the backend being feature-complete. Three independent gaps combine
into a single product defect:

1. **The MapLibre polyline never appears on `/rutas/:id` /
   `/lab/:id`.** `web/src/main.tsx` mounts
   `RouteDetailContainer` for those routes, but the container
   renders `RouteDetail` directly instead of the already-built
   `TrackDetailPage` that wraps `MapView`. The map component is
   built and tested; it just is not wired in.
2. **The single-track elevation profile renders an empty area.**
   `RouteDetail.tsx` hard-codes `projectTrack([])` (line 29). The
   inline TODO on lines 22–28 claims "the backend doesn't yet
   return the raw points array" — that comment is stale; the
   backend does return points via
   `internal/gpx/store.go` `subsamplePoints` (line 354) and the
   response marshals them through `Track.Points`
   (`internal/gpx/types.go:21`).
3. **The comparator elevation profile misrenders.**
   `ComparisonElevationProfile.tsx:50` passes raw `{ lat, lng,
   ele }` points through `t.points as unknown as
   Parameters<typeof projectTrack>[0]`. `projectTrack` reads
   `points[i].distance_m`, which is `undefined` on that shape, so
   every comparator profile computes `NaN` ticks. The TODO above
   the cast (lines 46–50) is the same stale comment as #2.

Independently, the **"no D+ lie" policy introduced in issue #171
(A8) is invisible to API consumers.** Migration
`internal/db/migrations/00010_gpx_tracks_elevation_coverage.sql`
adds `elevation_coverage NUMERIC`, SQLC selects it on every query
(`internal/db/sqlc/gpx_tracks.sql.go`), and the Go `Analysis`
struct declares it (`internal/gpx/types.go:48`,
`json:"elevation_coverage,omitempty"`). Yet
`internal/gpx/store.go` `storedTrack` builds the `Analysis`
literal without copying `row.ElevationCoverage` — the value is
discarded, so the API never publishes a coverage number for any
track. The frontend therefore cannot render the
"partial / insufficient elevation" badge that #160/#158/#126
explicitly depend on.

User-visible impact:

- Selecting a track from `/lab` opens a page with no map, an empty
  elevation chart, and metrics that may read `0 m` for D+ even
  when the track genuinely has positive elevation (because the
  frontend has no signal that the backend suppressed a non-zero
  value for low coverage).
- The comparator page renders three stacked elevation SVGs whose
  axis labels are `NaN`.
- API consumers (and any future mobile / CLI client) cannot
  distinguish "track has no elevation data" from "track has
  elevation data, D+/D- trustworthy" without re-implementing the
  coverage heuristic the backend already encodes.

This slice closes all four gaps in one coherent change.

## What Changes

Project roots in scope: **`web/` (frontend)** and **`.` (Go
backend)**. Each delta below is tagged with the project it lives in.

### Web (`web/`)

1. **New file: `web/src/features/gpx/rawPointsToTrackPoints.ts`.**
   Pure helper with no React or DOM imports. Accepts the raw
   `{ lat: number; lng: number; ele?: number | null }` shape (the
   same shape `RouteComparator.isFlatPoint` already narrows to)
   and returns `TrackPoint[]` (`{ distance_m, elevation_m |
   null }`) with cumulative haversine distance, mirroring the
   formula in `internal/gpx/analysis.go` (`earthRadiusM = 6371e3`,
   atan2-based haversine). The duplication is intentional and
   documented: the TS helper must run in the browser, so sharing
   the formula across language boundaries would require a new
   build pipeline. Drift between the two implementations is
   surfaced by per-implementation tests asserting the canonical
   Madrid → Cercedilla reference track within ±1 m cumulative
   distance. Lines: ~40 implementation + ~50 tests.
2. **New file: `web/src/features/gpx/rawPointsToTrackPoints.test.ts`.**
   Vitest unit tests covering: empty input, single-point input,
   a 2-point zero-distance case, the Madrid → Cercedilla
   reference (assertion against a pre-computed ground truth), null
   `ele` mid-track, and `NaN` coordinates are dropped without
   poisoning the accumulator. Drives the helper under TDD.
3. **Edit: `web/src/features/gpx/RouteDetailContainer.tsx`.**
   When `status.kind === 'ready'`, render
   `TrackDetailPage data={status.data}` instead of
   `RouteDetail data={status.data}`. No other state-machine
   branches change. `TrackDetailPage` already wraps the
   `MapView` on top and composes `RouteDetail` below
   (`web/src/features/lab/TrackDetailPage/TrackDetailPage.tsx:72-85`).
4. **Edit: `web/src/features/gpx/RouteDetail/RouteDetail.tsx`.**
   Feed real points into `projectTrack` by calling
   `rawPointsToTrackPoints(data.track.points)`. Remove the stale
   TODO comment on lines 22–28. Keep all presentational
   composition untouched.
5. **Edit: `web/src/features/lab/ComparisonElevationProfile/ComparisonElevationProfile.tsx`.**
   Replace the `t.points as unknown as Parameters<typeof
   projectTrack>[0]` cast (line 50) with a typed call to
   `rawPointsToTrackPoints(t.points)`. Remove the stale TODO
   comment on lines 46–50. The `RawPoint` type alias already
   declared at the top of the file (line 22) is the input shape.
6. **Edit: `web/src/lib/api/types.ts`.**
   Add `elevation_coverage: number | null` to the `GpxAnalysis`
   interface so the new server-side field is reachable from
   TypeScript without `as unknown` casts. Mirrors the
   `omitempty` JSON contract in `internal/gpx/types.go:48`
   (where `nil` → field absent, `*float64` value → present). The
   frontend does not need to *consume* this value in this slice
   (the visible-elevation product decision is deferred), but the
   type MUST be present so propagation is type-safe end-to-end
   and so any future badge / warning component can pick it up
   without a second type pass.
7. **Delete: `web/src/routes/lab-detail.tsx`.**
   The file is a placeholder for the (now-shipped) `TrackDetailPage`
   work. `main.tsx` mounts `/lab/:id` to `RouteDetailContainer`,
   not to this file; no router or test references it. Deletion
   keeps the file tree honest. Irreversible without git history.

### Backend (`.`)

8. **Edit: `internal/gpx/store.go` `storedTrack`.**
   Add one line to the `Analysis` literal:
   `ElevationCoverage: numericPointer(row.ElevationCoverage),`.
   `numericPointer` already exists in the same file (line 583)
   and returns `*float64` / nil exactly as the JSON contract
   expects. No new SQLC regen — the column was already added in
   migration 00010 and SQLC already selects it. Tests in
   `internal/gpx/store_test.go` MUST pin the propagation:
   extend `databaseTrack` (line 198) to set a non-zero
   `ElevationCoverage` and extend
   `TestSQLCStoreGetByIDScopesByUser` (or add a new test) to
   assert `stored.Track.Analysis.ElevationCoverage` matches.
9. **No new migrations.** Migration 00010 is already applied.
   No SQLC regen. The schema delta in this slice is purely an
   in-memory propagation fix.

## Impact

Approximate authored-line counts per project:

| Project | Files touched                                     | Lines (approx) |
|---------|---------------------------------------------------|----------------|
| `web/`  | 1 new helper + 1 new test + 5 edits + 1 delete   | ~140           |
| `.` (Go)| 1 store.go edit + 1 store_test.go edit            | ~40            |
| **Total** |                                                   | **~180–220**   |

This is well within the 400-line review budget; **no chained-PR
split is required**. The decision is deferred to `/sdd-tasks`
solely because the cache holds `delivery_strategy: ask-on-risk`
and the chain-vs-single choice must be confirmed there.

### Existing tests that already cover this slice

- `web/src/features/gpx/ElevationProfile/projectTrack.test.ts`
  (8+ TDD tests for `projectTrack`; reused unchanged).
- `web/src/features/gpx/RouteDetail/RouteDetail.test.tsx`
  (header / metrics / climbs / risks / elevation profile
  composition; the `renders the elevation profile svg` test on
  the last block continues to pass with real points flowing
  through).
- `web/src/features/lab/TrackDetailPage/TrackDetailPage.test.tsx`
  (5 TDD tests for map-empty / map-present / malformed-points
  fallback; all continue to pass).
- `web/src/features/lab/RouteComparator/RouteComparator.tsx`
  end-to-end via `pnpm -C web test`.
- `internal/gpx/store_test.go` covers `storedTrack` via
  `TestSQLCStoreGetByIDScopesByUser`,
  `TestSQLCStoreFindByHashScopesDuplicateToUser`,
  `TestSQLCStoreGetDetailHydratesChildren`,
  `TestSQLCStoreCreateDetailPersistsClimbsAndRiskZones`, and
  the subsampling suite (`TestSubsamplePoints_*`). These tests
  already exercise the code path that will now propagate
  `ElevationCoverage`.

### New tests added by this slice

- `web/src/features/gpx/rawPointsToTrackPoints.test.ts` — ~50
  lines, 6–8 cases (RED-GREEN-REFACTOR).
- One new case in `internal/gpx/store_test.go` (or extension of
  the existing `databaseTrack` fixture + assertion) to pin
  `ElevationCoverage` propagation through `storedTrack`. ~10
  lines.

### Per-slice test command pinning

The workspace has `strict_tdd: false`; this slice nevertheless
uses RED-GREEN-REFACTOR discipline for the two logic-bearing
changes (the haversine helper and the `ElevationCoverage`
propagation) and gates on both per-project test commands:

- Go: **`make test`** (raw:
  `GOTOOLCHAIN=local go test -race ./...`).
- Web: **`pnpm -C web test`** (alt: `pnpm -C web test:run` for
  the non-watch variant).
- Optionally `make lint` (`golangci-lint run ./...`) and
  `pnpm -C web typecheck` (`tsc --noEmit`) to keep quality green.

`sdd-apply` MUST invoke both `make test` and `pnpm -C web test`
explicitly because there is no workspace-aggregator target —
this is enforced in `openspec/config.yaml` (`apply.test_command`
is intentionally empty).

## Out of Scope

The following are intentionally excluded from this slice and
must not be touched by `sdd-apply`:

- A new `GET /api/v1/gpx/{id}/points` endpoint that returns
  points decoupled from the rest of the detail payload. Listed
  as Approach 3 in the exploration; would push the slice over
  400 lines alone (sqlc regen + handler + route + tests + ≥1 web
  consumer). Surface as a follow-up if list-payload pressure
  emerges.
- Wider TODO-comment cleanup in `web/src/features/lab/*` (the
  `RouteComparator.tsx` and `TrackDetailPage.tsx` comments are
  the ones this slice needs to address; the broader TODO sweep
  is a separate concern).
- Visible UI for the elevation-coverage signal (badge, warning,
  partial-estimate label). The slice only makes the value
  observable end-to-end so future UI work can read it without
  further plumbing. The product decision on what to render is
  deferred — it is the natural next slice.
- Any modification of `internal/gpx/analysis.go` (the Go
  haversine). It is already correct; we mirror the formula in
  TS by spec.
- Chain-strategy selection. The slice fits one PR, but the
  final `delivery_strategy` (and the implicit
  `chain_strategy` if `ask-on-risk` stops the gate) is the
  parent's call at `/sdd-tasks` time.

## Rollback Plan

Per change, in execution order. The slice is one PR; a `git
revert` of the merge commit removes every delta in one
operation.

1. **`web/src/features/gpx/RouteDetailContainer.tsx`** —
   revert the import + render swap. Falls back to the previous
   behaviour (no map). Stateless reversion; no data migration.
2. **`web/src/features/gpx/RouteDetail/RouteDetail.tsx`** —
   revert to `projectTrack([], …)`. The empty-points profile
   state is exactly what the previous behaviour produced.
3. **`web/src/features/lab/ComparisonElevationProfile/ComparisonElevationProfile.tsx`**
   — revert to the `as unknown as` cast + TODO comment. The
   comparator returns to its pre-fix (broken-chart) state;
   nothing else depends on the helper.
4. **`web/src/features/gpx/rawPointsToTrackPoints.ts`** and
   its test file — delete. No other file imports it after the
   reverts in steps 2 and 3; safe to remove.
5. **`web/src/lib/api/types.ts`** — drop the
   `elevation_coverage` field. The Go side will continue to
   emit the field (it is `omitempty` so clients that don't
   read it are unaffected); the TS type simply stops
   declaring it. Re-add it in a future slice if/when the UI
   consumes it.
6. **`web/src/routes/lab-detail.tsx`** — restore via
   `git checkout HEAD~1 -- web/src/routes/lab-detail.tsx` (or
   `git revert`). No router or test references it; restoring
   does not change runtime behaviour.
7. **`internal/gpx/store.go` `storedTrack`** — remove the
   `ElevationCoverage` line. Propagation reverts to
   `zero-value *float64` (always nil), which is exactly the
   pre-fix behaviour. **The field is `omitempty`** on the JSON
   contract, so clients that don't read it continue to be
   unaffected by its presence or absence. Reverting
   `ElevationCoverage` propagation does not break any other
   consumer — it just stops surfacing the value. There is no
   data migration, no schema change, no row-level effect.

**No migrations are added or removed in this slice.** Migration
00010 stays applied. No SQLC regen is needed.

## Risks

- **Review surprise: backend `ElevationCoverage` propagation
  looks like an existing feature.** The JSON tag
  (`elevation_coverage,omitempty`) and the SQLC column have
  been there since #171/A8 landed. Reviewers may assume the
  field already populates correctly. Mitigation: the
  propagation test added in `internal/gpx/store_test.go` MUST
  pin a non-zero value (e.g. `0.87`) and assert
  `stored.Track.Analysis.ElevationCoverage != nil` so the test
  fails if a future refactor accidentally reverts to
  zero-value. Low risk; called out so reviewers do not skip
  the test.
- **Haversine formula is intentionally duplicated between TS
  and Go.** The two implementations will drift unless both
  are pinned to the same reference track (Madrid →
  Cercedilla, ~42 km) within a tolerance band. The TS test
  suite asserts the same reference; the Go test suite
  already pins it via `internal/gpx/analysis_test.go`. Add a
  shared `// see also:` comment in each file so the next
  person reading either one knows to update the other. Low
  risk; ~4 lines of comments.
- **`web/src/routes/lab-detail.tsx` deletion is safe but
  irreversible without git.** Verified via grep across `web/`
  and `main.tsx` that no router entry, test, or import
  references it. If a future change reintroduces a router
  entry by mistake, restore from `git log`. Low risk.
- **List endpoint payload size.** Today the list also returns
  up to ~4000 points per track (default of `subsamplePoints`).
  With 20 tracks per page that is up to ~80 000 points in a
  single JSON. Not in scope for this slice; if the list page
  becomes slow, Approach 3 (a separate
  `/api/v1/gpx/{id}/points?resolution=N` endpoint) is the
  natural follow-up. Surfaced here as a known pressure
  signal; not a blocker.
- **TS type drift: `GpxAnalysis` did not previously declare
  `elevation_coverage`.** Adding the field to the
  TypeScript interface is required for type-safe consumption
  even though this slice does not consume the value. If a
  reviewer prefers to defer the type change to the next UI
  slice, that is acceptable as long as the slice is
  documented as not yet observable from TypeScript.
  (Recommended: include it now so the propagation is
  verifiable end-to-end.)

### Drift discovered during this proposal

- **Drift (minor, no action):** The exploration refers to the
  dead placeholder file as `routes/lab-detail.tsx`. On disk
  the path is `web/src/routes/lab-detail.tsx`. Description
  and intent match; only the path prefix differs. Surfacing
  here so `sdd-tasks` uses the correct path when emitting
  the delete task.
- **Drift (minor, no action):** The exploration reports the
  stale TODO comments as being on `TrackDetailPage.tsx`
  lines 17–18. On disk the inline TODO body sits inside the
  `useMemo` at lines 17–20. Line numbers shifted slightly
  because of unrelated edits, but the claim ("two stale
  TODO comments") and the fix (delete them once the wiring
  is done) are unchanged. No action needed; flagged so the
  apply task uses current line numbers if it inlines an edit.
- **Drift (new, scope-relevant):** The exploration does NOT
  call out that `web/src/lib/api/types.ts` `GpxAnalysis`
  lacks the `elevation_coverage` field. Even after
  `storedTrack` propagates it, the TypeScript client cannot
  read it without a type pass. The What Changes section
  above includes the type edit (item 6) so the slice is
  observable end-to-end. Not a blocker; surfaced so
  reviewers know why the `lib/api/types.ts` change is in
  this slice.

## Open Questions

None. Every product decision needed for this slice has either
been confirmed by the prior exploration or is scoped out
explicitly (visible-elevation UI, `/points` endpoint, broader
TODO cleanup, chain strategy). Adding manufactured open
questions here would only delay `/sdd-tasks`.

The chain-strategy decision is intentionally **not** an open
question for this phase; it belongs to `/sdd-tasks` per the
cached `delivery_strategy: ask-on-risk`.
