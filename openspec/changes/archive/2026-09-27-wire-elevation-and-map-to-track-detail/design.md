# Design: Wire elevation and map to track detail

## Goals and Non-Goals

**Goals**

- Make the existing map polyline appear on track-detail by rendering `TrackDetailPage` from the ready state.
- Render the single-track elevation profile from real track points with cumulative distance.
- Stop the comparator elevation profile from producing `NaN` ticks by converting raw points to the shape `projectTrack` consumes.
- Propagate `Analysis.ElevationCoverage` from the persisted row through the Go API and expose it as a type-visible field to the TypeScript client.
- Delete the dead placeholder route file and remove the stale missing-points TODOs in the two profile components.

**Non-Goals**

The following are intentionally excluded from this slice and must not be touched during apply:

- A new `GET /api/v1/gpx/{id}/points` endpoint that returns points decoupled from the rest of the detail payload. Listed as Approach 3 in the exploration; would push the slice over 400 lines alone (sqlc regen + handler + route + tests + ≥1 web consumer). Surface as a follow-up if list-payload pressure emerges.
- Wider TODO-comment cleanup in `web/src/features/lab/*` (the `RouteComparator.tsx` and `TrackDetailPage.tsx` comments are the ones this slice needs to address; the broader TODO sweep is a separate concern).
- Visible UI for the elevation-coverage signal (badge, warning, partial-estimate label). The slice only makes the value observable end-to-end so future UI work can read it without further plumbing. The product decision on what to render is deferred — it is the natural next slice.
- Any modification of `internal/gpx/analysis.go` (the Go haversine). It is already correct; we mirror the formula in TS by spec.
- Chain-strategy selection. The slice fits one PR, but the final `delivery_strategy` (and the implicit `chain_strategy` if `ask-on-risk` stops the gate) is the parent's call at `/sdd-tasks` time.
- No new migrations. Migration 00010 is already applied. No SQLC regen.

## Architecture Overview

Current state: GPX storage and analysis are feature-complete, including stored track points, the SQLC elevation-coverage column, and Go's `Analysis.ElevationCoverage` field. The visible lab/detail page remains broken at the wiring boundary: `RouteDetailContainer` renders bare `RouteDetail`, whose elevation profile receives an empty point list; the comparator passes raw points through an unsafe cast and gets missing distance values. In addition, `storedTrack` drops the stored elevation-coverage value, so it is invisible to API clients and absent from the TS `GpxAnalysis` interface.

Target state: the ready branch renders the already-built `TrackDetailPage`, which shows the map above the route detail. A pure browser-safe converter computes cumulative Haversine distance for the profile consumers; both profiles receive the expected `TrackPoint[]` shape. Go's shared `storedTrack` conversion propagates nullable coverage and the TS API type declares the additive field. The unreferenced placeholder is deleted and stale TODOs are removed.

```text
DB gpx_tracks row
  (coordinates, elevation_coverage)
          │
          ▼
  storedTrack(row)
  ├─ Track.Points ── JSON response ── raw coordinates ──┬─ TrackDetailPage / MapView
  │                                                     └─ RouteDetail → rawPointsToTrackPoints → RouteDetail profile
  └─ Analysis.ElevationCoverage ── JSON: "elevation_coverage" ── TS GpxAnalysis.elevation_coverage
                                                               ├─ RouteDetail
                                                               ├─ RouteDetailContainer → TrackDetailPage
                                                               └─ ComparisonElevationProfile
```

`elevation_coverage` is the additive JSON field on `Analysis`; it is nullable/omitted when Go holds a nil pointer. `RouteDetail`, `TrackDetailPage`, and `ComparisonElevationProfile` are named here as API/page consumers in the end-to-end path; this slice does not add UI consumption of the coverage value.

## Cross-Project Wiring Contract

- **Wire name and language names:** the JSON field is exactly `elevation_coverage` (snake_case). Go represents it as `Analysis.ElevationCoverage`; the TS `GpxAnalysis` interface represents it as `elevation_coverage: number | null`.
- **Go contract:** `internal/gpx/types.go` already declares `ElevationCoverage *float64` with JSON tag `json:"elevation_coverage,omitempty"`. `internal/gpx/store.go`'s `storedTrack` conversion MUST copy `row.ElevationCoverage` using the existing `numericPointer` helper. A valid SQL numeric becomes a non-nil `*float64`; SQL NULL or failed/invalid conversion becomes nil, and `omitempty` omits the wire property for nil. SQLC already has `ElevationCoverage` in `internal/db/sqlc/gpx_tracks.sql.go`; migration `internal/db/migrations/00010_gpx_tracks_elevation_coverage.sql` already adds `elevation_coverage NUMERIC`. Neither generated code nor schema changes are part of this slice.
- **TS contract:** add `elevation_coverage: number | null` to `GpxAnalysis` in `web/src/lib/api/types.ts`. Since Go omits the field when nil, consumers/helpers that read it must tolerate absence as well as explicit `null` (normalizing absent to `null` if a consumer needs a stable normalized shape). No consumer reads this value in this slice by design. The comparator's existing `isFlatPoint` filter must continue accepting the raw point shape; the converter is applied after that defensive filter, not as a replacement for it.
- **Track points contract across project roots:** the converter and specified TS component call sites use `{ lat, lng, ele }` points and produce `TrackPoint[]` (`distance_m`, `elevation_m`). The Go `Point` JSON tag currently names longitude `lon`, while existing TS comparator narrowing expects `lng`. This is a newly observed wire-shape mismatch and is a material integration risk; task planning must make the actual point-shape adaptation explicit and keep it within the approved scope before apply. Do not silently weaken `isFlatPoint` or assume Go emits `lng`.
- **Stability:** `elevation_coverage` is additive and optional in spirit at this contract boundary. A future change that wants to make it required MUST have its own delta spec.

## Component Design

### `web/src/features/gpx/rawPointsToTrackPoints.ts` (new pure helper)

- **Purpose:** convert raw latitude/longitude/elevation points into `projectTrack`'s distance/elevation shape, computing cumulative great-circle distance.
- **Location and key types:** new module beside the GPX detail components; input `Array<{ lat: number; lng: number; ele?: number | null }>`; output `TrackPoint[]` imported as a type from `ElevationProfile/projectTrack.ts` (`distance_m: number`, `elevation_m: number | null`). Empty input returns an empty array. Round each cumulative distance to 3 decimal places (millimetre precision).
- **Dependencies:** no React, DOM, or runtime UI dependencies; same Haversine earth radius (`6371e3`) and atan2-based formula as `internal/gpx/analysis.go`. Add a code-level cross-reference comment in this TS module pointing to `internal/gpx/analysis.go`, and a corresponding comment in Go's Haversine implementation pointing to this TS helper.
- **Input handling:** retain valid negative coordinates (they are geographic coordinates, not invalid values); omit points with non-finite/non-numeric latitude or longitude without poisoning subsequent accumulation. Preserve an absent/null elevation as `null` in the output while still including that valid coordinate in cumulative distance. This supports gaps in elevation without collapsing the distance axis.
- **Test surface:** `rawPointsToTrackPoints.test.ts` covers empty and single-point arrays, equatorial 1 km tolerance, Madrid → Cercedilla reference tolerance, negative coordinates, and non-finite coordinate omission/accumulator stability.
- **Rationale:** this small duplication keeps browser math independent of the Go build and API; comments plus a canonical fixture make algorithm drift discoverable without creating a cross-language build pipeline. Sharing the reference fixture across Go and TS is explicitly out of scope for this slice.

### `web/src/features/gpx/RouteDetailContainer.tsx` (modified)

- **Purpose:** select the full track-detail page on successful fetch while preserving the existing request and state machine.
- **Location and key types:** the `ready` branch replaces `RouteDetail` with `TrackDetailPage`, passing `status.data` unchanged. `TrackDetailPage` consumes `NormalizedTrackDetail` and renders map and route content.
- **Dependencies:** existing `normalizeTrackDetail`, `TrackDetailPage`, and `RouteDetailStatus` contracts; loading, no-token, not-found, and error branches stay unchanged.
- **Test surface:** a focused container regression test asserts the `track-detail-page` testid renders in ready state, alongside existing state tests.
- **Rationale:** reuse the previously implemented map composition rather than duplicate map logic in the container.

### `web/src/features/gpx/RouteDetail/RouteDetail.tsx` (modified)

- **Purpose:** render the single-track profile from real point data.
- **Location and key types:** replace `projectTrack([])` with `projectTrack(rawPointsToTrackPoints(raw))`, where the input is the track's validated/contracted raw point list and output is `TrackPoint[]`; retain the current climb and risk-zone projection inputs. Remove the stale TODO block.
- **Dependencies:** new converter and existing `projectTrack`, `NormalizedTrackDetail`, climbs, and risk zones.
- **Test surface:** component test checks real points produce a multi-segment, non-NaN path and that empty points still render the empty profile without throwing.
- **Rationale:** the projection consumes cumulative distance, so the raw API shape must be converted rather than passed as a structurally unrelated object.

### `web/src/features/lab/ComparisonElevationProfile/ComparisonElevationProfile.tsx` (modified)

- **Purpose:** make each comparator profile receive real cumulative distance and eliminate NaN axis/path output.
- **Location and key types:** replace `t.points as unknown as Parameters<typeof projectTrack>[0]` with `rawPointsToTrackPoints(t.points)`, and remove the stale TODO block. Preserve the existing `isFlatPoint` validation at the `RouteComparator` boundary.
- **Dependencies:** new converter, existing `projectTrack`, `RawPoint`, `Climb`, `RiskZone`, and comparator's current defensive point filter.
- **Test surface:** component tests cover three populated tracks (no `NaN` in ticks or paths) and an empty-point track; comparator regression tests retain coverage of malformed-point filtering.
- **Rationale:** one shared converter keeps single-track and comparison calculations consistent; filtering remains at the boundary where untrusted API values are narrowed.

### `web/src/lib/api/types.ts` (modified)

- **Purpose:** make the propagated API field discoverable and type-checkable in TS.
- **Location and key types:** add `elevation_coverage: number | null` to `GpxAnalysis`.
- **Dependencies:** Go's additive JSON field; no new API call or UI consumer.
- **Test surface:** `pnpm -C web typecheck` validates the interface addition and existing fixtures. The wire may omit the field when nil, so future readers must handle absent values as well as `null`.
- **Rationale:** type visibility is part of the end-to-end propagation contract; adding it now avoids hiding the server-side change behind an untyped boundary. A consumer is intentionally deferred.

### `internal/gpx/store.go` (`storedTrack`, modified)

- **Purpose:** preserve coverage read from SQLC rows in the returned `Analysis`.
- **Location and key types:** add `ElevationCoverage: numericPointer(row.ElevationCoverage)` to the `Analysis` literal. `numericPointer(pgtype.Numeric) *float64` already handles valid and NULL numeric values.
- **Dependencies:** existing SQLC row field, `numericPointer`, `gpx.Analysis`; shared `storedTrack` conversion serves GetByID, GetDetail, FindByHash, and List paths.
- **Test surface:** `internal/gpx/store_test.go`'s store read test pins equality for a valid non-null numeric; a NULL fixture asserts nil. The equality assertion must fail if propagation is removed.
- **Rationale:** use the existing conversion helper and single row-to-domain boundary; no new query or schema work is required.

### `web/src/routes/lab-detail.tsx` (deleted)

- **Purpose:** remove a dead placeholder that is not registered by the router.
- **Location and key types:** delete this file only; `/lab/:id` remains routed to `RouteDetailContainer` through the existing main router.
- **Dependencies:** none; no imports, tests, or router references were found in the reconciled artifacts.
- **Test surface:** web typecheck and reference search verify the deletion does not leave imports behind.
- **Rationale:** the live page is the existing container plus `TrackDetailPage`; retaining a placeholder invites confusion without adding behavior.

## Test Design

- **Per-slice TDD posture:** use RED-GREEN-REFACTOR for the Haversine converter and `ElevationCoverage` propagation, as specified in the proposal. Wiring changes (`RouteDetailContainer`, type addition, and dead-code deletion) are mechanical and gated by the existing test surface plus the new minimal container regression test.
- **Haversine helper tests:** empty input returns empty; single point has cumulative distance `0`; approximately 1 km east on the equator is within ±0.5 m; Madrid → Cercedilla reference fixture cumulative distances are within ±1 m; valid negative coordinates are retained and computed; non-finite coordinates are omitted without poisoning the accumulator; null elevation is preserved while distance still accumulates. Round cumulative outputs to 3 decimals.
- **`ElevationCoverage` propagation tests:** non-null SQL numeric `0.87` propagates to a non-nil pointer with equal value; SQL NULL yields a nil pointer; the store test covers the read path that originally drops the field. Pin the equality assertion so removing the assignment fails CI. Shared `storedTrack` conversion is the propagation point for all store reads.
- **Wiring regression test:** `RouteDetailContainer` test asserts the `track-detail-page` testid is rendered when ready; existing `TrackDetailPage` tests cover map and empty-map behavior. Keep non-ready branches' existing assertions unchanged.
- **Profile regressions:** single-track real points produce a non-degenerate SVG path without `NaN`; empty input still renders safely. Comparator tests verify real track paths and tick labels are `NaN`-free; its existing `isFlatPoint` filter remains in place.
- **Type assertion:** `pnpm -C web typecheck` must pass after adding `GpxAnalysis.elevation_coverage`. Add this command explicitly to the apply/verify gate if project CI does not run it separately; the spec phase already raised this gate concern.
- **Pinned commands (match the spec matrix):**
  - Go: `make test`
  - Web unit/component: `pnpm -C web test` (interactive) / `pnpm -C web test:run` (CI)
  - Web typecheck: `pnpm -C web typecheck`
  - Web lint: `pnpm -C web lint`
  - Web format: `pnpm -C web format:check`

## Risks and Mitigations

- **Haversine duplication may drift between Go and TS.** Add one-line cross-reference comments in both implementations. Sharing the Madrid → Cercedilla fixture as a Go-side/cross-language test is out of scope because it would expand the budget.
- **`ElevationCoverage` propagation could regress silently.** The store test pins the non-null value through `numericPointer(row.ElevationCoverage)` and separately covers SQL NULL.
- **Web typecheck may not be in the project gate.** Apply must run `pnpm -C web typecheck` explicitly. Consider a follow-up issue to add it to `openspec/config.yaml` `verify.web_test_command` aggregation; this design does not alter config.
- **Deleting `routes/lab-detail.tsx` is irreversible without git history.** Keep deletion within the single atomic PR/commit and call it out in the PR description.
- **The TS field has no UI consumer in this slice.** This is deliberate and listed under Non-Goals; reviewers who want the field deferred should raise that before apply begins.
- **Point longitude key differs across current code boundaries.** Go's `internal/gpx/types.go` serializes `Point.Lon` as JSON `lon`; the TS comparator's `isFlatPoint` and the specified converter input use `lng`. The required integration behavior assumes `{lat,lng,ele}`. The tasks phase must define and test a bounded adapter/contract for this mismatch before apply; otherwise valid Go responses may be rejected by `isFlatPoint` or produce no map points. Do not resolve by silently altering the authoritative delta specs or weakening the defensive filter.
- **Schema generation drift is not expected.** Migration `00010_gpx_tracks_elevation_coverage.sql` and SQLC column mappings already exist; no migration or sqlc regen is needed, and any absent column would be a newly observed blocker rather than work authorized by this slice.

## Rollback

- One PR; one revert undoes the complete slice.
- No migrations are added or removed, so rollback never touches the schema.
- `ElevationCoverage` propagation is additive: the field already exists in the Go JSON type and rollback only stops populating it. Clients that tolerate `null`/absence remain compatible.
- The `TrackDetailPage` wire change rolls back by restoring the previous `RouteDetailContainer.tsx` ready-branch render.
- `rawPointsToTrackPoints.ts` is new; rollback deletes it and reverts the two profile call sites.
- The `GpxAnalysis` TS field addition can be reverted independently with its server-side addition still backward-compatible; the deleted placeholder can be restored from git if needed.

## Out of Scope (Design-Level)

- New `/api/v1/gpx/{id}/points` endpoint.
- Wider TODO-comment cleanup in `web/src/features/lab/*`.
- Visible UI for `elevation_coverage` (badge / warning / partial label).
- Chain-strategy selection (deferred to `/sdd-tasks`).
- No new migrations, no sqlc regen.
- Sharing the Madrid → Cercedilla fixture as a Go-side test (cross-language fixture sharing would expand the budget and is deferred).
