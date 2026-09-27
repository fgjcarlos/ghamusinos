# Delta for gpx-lab (web)

## Source

- Proposal: `openspec/changes/wire-elevation-and-map-to-track-detail/proposal.md`
- Exploration: `openspec/changes/wire-elevation-and-map-to-track-detail/exploration.md`

## Domain Metadata

| Project root | `web/` |
| --- | --- |
| Stack | React 19 + TypeScript 6 + Vite 8 + Vitest 5 + @testing-library/react + jsdom |
| Gating test command | `pnpm -C web test` (alt: `pnpm -C web test:run`) |
| Quality gates | `pnpm -C web typecheck` (`tsc --noEmit`), `pnpm -C web lint` |

This delta covers the front-end slice of the lab track-detail wiring:
the already-built `TrackDetailPage` + `MapView` is mounted behind
`/rutas/:id` and `/lab/:id`; the single-track elevation profile
renders real points instead of an empty array; the comparator
elevation profile stops producing `NaN` axis labels; the backend's
`elevation_coverage` field becomes observable from TypeScript so
propagation is type-safe end-to-end; and the dead-code placeholder
under `web/src/routes/` is removed.

No canonical `openspec/specs/gpx-lab/spec.md` exists yet. Archive
will copy this file into the canonical location.

## ADDED Requirements

### Requirement: Track detail page wires the MapLibre polyline above the route detail

When `RouteDetailContainer` reaches the `ready` state, the rendered
page MUST be `TrackDetailPage` (which composes `MapView` on top of
`RouteDetail`), not the bare `RouteDetail`. The other state-machine
branches (`loading`, `no-token`, `not-found`, `error`) MUST remain
visually and semantically unchanged.

**Project root**: `web`
**Gating test command**: `pnpm -C web test`
**Pin point**: `web/src/features/gpx/RouteDetailContainer.tsx`

#### Scenario: ready status with enough points renders the map above the route detail

- GIVEN `RouteDetailContainer` is in the `ready` state with a track
  whose `points` array contains at least two entries with valid
  numeric `lat` and `lng`
- WHEN the component is rendered
- THEN the `MapView` polyline element (testid `map-view`) is in the
  document
- AND the `RouteDetail` article (testid `route-detail`) is also in
  the document
- AND the `map-view` element appears before the `route-detail`
  element in DOM order

#### Scenario: ready status with fewer than two usable points falls back to the empty map region

- GIVEN `RouteDetailContainer` is in the `ready` state with a track
  whose `points` array contains zero or one entry, or only
  malformed entries (no numeric `lat`/`lng`)
- WHEN the component is rendered
- THEN the empty-map region (testid `map-empty`) is in the document
- AND the `RouteDetail` article (testid `route-detail`) is still in
  the document, composed below the empty-map region

#### Scenario: non-ready states do not mount the map or the route detail article

- GIVEN `RouteDetailContainer` is in any of `loading`, `no-token`,
  `not-found`, or `error`
- WHEN the component is rendered
- THEN neither the `map-view` testid nor the `route-detail` testid
  appear in the document
- AND the existing state-specific testid (e.g.
  `route-detail-status-loading`, `route-detail-status-no-token`,
  `route-detail-status-not-found`, `route-detail-status-error`)
  appears instead

### Requirement: Single-track elevation profile renders real track points

`RouteDetail` MUST feed `projectTrack` the result of converting
`data.track.points` via `rawPointsToTrackPoints`. Hardcoding an
empty array of points into `projectTrack` is forbidden after this
slice lands.

**Project root**: `web`
**Gating test command**: `pnpm -C web test`
**Pin point**: `web/src/features/gpx/RouteDetail/RouteDetail.tsx`

#### Scenario: a track with several points renders a non-degenerate profile

- GIVEN a `NormalizedTrackDetail` whose `track.points` array
  contains five `{ lat, lng, ele }` entries that resolve to non-zero
  cumulative distance, plus the existing climbs and risk-zones
  fixture
- WHEN `<RouteDetail>` is rendered
- THEN the elevation-profile SVG (testid `elevation-profile`) is in
  the document
- AND the projected `path` attribute contains at least four `L`
  commands (one per segment between consecutive points)
- AND the path string does NOT contain the substring `NaN`

#### Scenario: a track with zero points still renders the empty-profile state

- GIVEN a `NormalizedTrackDetail` whose `track.points` array is
  empty
- WHEN `<RouteDetail>` is rendered
- THEN the elevation-profile SVG (testid `elevation-profile`) is
  still in the document
- AND the rest of the page (header, metrics, climbs, risks) still
  composes without exception
- AND no `NaN` substring appears anywhere in the rendered DOM

### Requirement: Comparator elevation profile renders correct ticks (no NaN)

`ComparisonElevationProfile` MUST convert raw `{ lat, lng, ele }`
points to `TrackPoint[]` via `rawPointsToTrackPoints` before passing
them to `projectTrack`. The existing
`t.points as unknown as Parameters<typeof projectTrack>[0]` cast
MUST NOT exist after this slice lands.

**Project root**: `web`
**Gating test command**: `pnpm -C web test`
**Pin point**: `web/src/features/lab/ComparisonElevationProfile/ComparisonElevationProfile.tsx`

#### Scenario: three tracks with real points render distinct, NaN-free SVGs

- GIVEN three tracks, each with at least ten `{ lat, lng, ele }`
  points whose cumulative distance is non-zero
- WHEN `<ComparisonElevationProfile>` is rendered
- THEN three SVG elements appear inside the
  `comparison-elevation-profile` testid container
- AND none of the axis tick labels (profile label text content)
  contain the substring `NaN`
- AND none of the path `d` attributes contain the substring `NaN`

#### Scenario: a single track with empty points still renders without NaN

- GIVEN one track whose `points` array is empty
- WHEN `<ComparisonElevationProfile>` is rendered
- THEN the empty-profile state is rendered for that track
- AND no `NaN` substring appears anywhere in the rendered DOM
- AND no exception is raised during render

### Requirement: `rawPointsToTrackPoints` haversine helper

A pure module at `web/src/features/gpx/rawPointsToTrackPoints.ts`
MUST export a function `rawPointsToTrackPoints(input)` that:

- accepts `Array<{ lat: number; lng?: number; lon?: number; ele?: number | null }>`
  as input. The longitude key MAY be `lng` OR `lon` to match the
  Go wire shape emitted by `internal/gpx/types.go` `Point.Lon`
  (JSON tag `json:"lon"`); when BOTH `lng` and `lon` are present
  on the same entry, `lng` wins. If neither key carries a finite
  number, the entry is treated as having no longitude (see the
  drop rule below).
- returns `Array<{ distance_m: number; elevation_m: number | null }>`
  where each element's `distance_m` is the cumulative haversine
  distance from the first input point, computed with
  `earthRadiusM = 6371e3` and the atan2-based haversine formula
  (mirroring `internal/gpx/analysis.go`);
- emits the first output element with `distance_m === 0`;
- preserves `null` `ele` values as `elevation_m: null` (does NOT
  coerce `null` to `0`);
- drops entries whose `lat` is non-finite OR whose effective
  longitude (resolving `lng` first, then `lon` as a fallback) is
  non-finite, without poisoning the cumulative-distance
  accumulator for the survivors;
- contains no imports of `react`, `react-dom`, `react-router-dom`,
  or any DOM-only API.

**Project root**: `web`
**Gating test command**: `pnpm -C web test`
**Pin point**: `web/src/features/gpx/rawPointsToTrackPoints.ts` (new file)
**TDD posture**: RED-GREEN-REFACTOR. The test file
`web/src/features/gpx/rawPointsToTrackPoints.test.ts` MUST be
written and committed with the failing assertions below before the
helper is implemented.

#### Scenario: empty input

- GIVEN `input = []`
- WHEN `rawPointsToTrackPoints(input)` is called
- THEN it returns `[]`

#### Scenario: single point

- GIVEN `input = [{ lat: 40.4, lng: -3.7, ele: 1000 }]`
- WHEN `rawPointsToTrackPoints(input)` is called
- THEN the result has exactly one element
- AND its `distance_m === 0`
- AND its `elevation_m === 1000`

#### Scenario: two-point zero-distance input

- GIVEN two points with identical `lat` and identical `lng`
- WHEN `rawPointsToTrackPoints(input)` is called
- THEN the second element's `distance_m === 0`

#### Scenario: one kilometre on the equator returns ≈1000 m within ±0.5 m

- GIVEN `input = [{ lat: 0, lng: 0, ele: 0 }, { lat: 0, lng: 0.008983, ele: 0 }]`
  (approximately one kilometre east along the equator)
- WHEN `rawPointsToTrackPoints(input)` is called
- THEN the second element's `distance_m` is within `±0.5 m` of
  `1000`

#### Scenario: Madrid → Cercedilla reference track is within ±1 m of ground truth

- GIVEN a precomputed reference fixture with the lat/lng coordinates
  of a Madrid → Cercedilla route (~42 km) and a precomputed
  ground-truth array of cumulative distances (one per input point)
- WHEN `rawPointsToTrackPoints(input)` is called
- THEN for every output element at index `i`, `result[i].distance_m`
  is within `±1 m` of `groundTruth[i]`

#### Scenario: null `ele` mid-track is propagated as null

- GIVEN a three-point track where the middle point has `ele: null`
- WHEN `rawPointsToTrackPoints(input)` is called
- THEN the second element's `elevation_m === null`
- AND the second element's `distance_m` is still computed from the
  haversine formula (no skip)

#### Scenario: NaN coordinate is dropped without poisoning the accumulator

- GIVEN a track where one entry has `lat: NaN` (the others are
  valid)
- WHEN `rawPointsToTrackPoints(input)` is called
- THEN the entry with `NaN` `lat` is omitted from the result
- AND no element in the result has `distance_m === NaN`
- AND the remaining cumulative distances are non-decreasing for the
  survivors

#### Scenario: input with `lon` (Go wire shape) yields the same output as the equivalent `lng` input

- GIVEN `inputLon = [{ lat: 40.4, lon: -3.7, ele: 1000 }, { lat: 40.41, lon: -3.69, ele: 1010 }]`
  (Go wire shape — no `lng` key, only `lon`)
- AND `inputLng = [{ lat: 40.4, lng: -3.7, ele: 1000 }, { lat: 40.41, lng: -3.69, ele: 1010 }]`
  (TS-idiomatic shape — no `lon` key, only `lng`)
- AND a third mixed-shape input `inputMixed = [{ lat: 40.4, lng: -3.7, ele: 1000 }, { lat: 40.41, lon: -3.69, ele: 1010 }]`
- WHEN `rawPointsToTrackPoints` is called once per input
- THEN `result(inputLon)` and `result(inputLng)` are element-wise
  equal within `±1e-9 m` (haversine is deterministic; the only
  difference is the source key name)
- AND `result(inputMixed)[1].distance_m` matches
  `result(inputLng)[1].distance_m` within `±1e-9 m`
  (the `lng` key wins when both are present on the same entry)

#### Scenario: entry with neither `lng` nor `lon` is dropped without poisoning the accumulator

- GIVEN an input of two entries: the first has
  `{ lat: 40.4, lng: -3.7, ele: 1000 }` and the second has
  `{ lat: 40.41, ele: 1010 }` (no longitude key at all)
- WHEN `rawPointsToTrackPoints(input)` is called
- THEN the second entry is omitted from the result
- AND the first entry's `distance_m === 0`
- AND no element in the result has `distance_m === NaN`

### Requirement: `GpxAnalysis` declares `elevation_coverage`

`web/src/lib/api/types.ts` MUST declare
`elevation_coverage: number | null` on the `GpxAnalysis` interface
so the field that the backend already serialises (when non-null) is
reachable from TypeScript without `as unknown` casts. The field is
optional on the wire only because the Go JSON tag is `omitempty`;
the TypeScript view MUST treat absent and explicit-null as the
same shape (`number | null`).

**Project root**: `web`
**Gating test command**: `pnpm -C web typecheck` (`tsc --noEmit`)
**Pin point**: `web/src/lib/api/types.ts`

#### Scenario: typecheck passes when reading the field

- GIVEN a TypeScript consumer that reads
  `analysis.elevation_coverage` on a `GpxAnalysis` value
- WHEN `pnpm -C web typecheck` is run
- THEN no `TS2339` ("Property … does not exist on type …") error is
  emitted against the consumer
- AND no `TS2741` ("Property … is missing in type …") error is
  emitted against existing fixtures that build `GpxAnalysis`
  literals without the new field

#### Scenario: the field is typed `number | null`

- GIVEN the `GpxAnalysis` interface
- WHEN it is inspected via `pnpm -C web typecheck`
- THEN the declaration includes `elevation_coverage: number | null`
  using the same shape already used for `max_elevation_m` and
  `min_elevation_m`

### Requirement: Stale TODO comments removed from the lab detail surface

The misleading TODO comments that claim "the backend doesn't yet
return the raw points array" on
`web/src/features/gpx/RouteDetail/RouteDetail.tsx` (currently
around lines 22–28) and on
`web/src/features/lab/ComparisonElevationProfile/ComparisonElevationProfile.tsx`
(currently around lines 46–50) MUST be removed once the wiring is
complete. Broader TODO-comment cleanup across
`web/src/features/lab/*` is out of scope.

**Project root**: `web`
**Gating test command**: `pnpm -C web test` plus PR review
**Pin point**: comment removal in `RouteDetail.tsx` and
`ComparisonElevationProfile.tsx`

#### Scenario: no stale TODO mentions the missing-points assumption

- GIVEN a fresh checkout of the post-merge tree
- WHEN
  `grep -RIn "backend doesn't yet return the raw points" web/src/`
  and
  `grep -RIn "the backend currently doesn't return \`points\`" web/src/`
  are run
- THEN neither command produces any output

### Requirement: Dead-code placeholder `web/src/routes/lab-detail.tsx` is deleted

The placeholder file `web/src/routes/lab-detail.tsx` MUST NOT exist
after this slice lands. The file is not referenced by
`web/src/main.tsx` (which mounts `/lab/:id` and `/rutas/:id` to
`RouteDetailContainer`), by any test, or by any import.

**Project root**: `web`
**Gating test command**: `pnpm -C web typecheck` (regression
check — no module references the deleted file)
**Pin point**: deletion of `web/src/routes/lab-detail.tsx`

#### Scenario: the file no longer exists

- GIVEN a fresh checkout of the post-merge tree
- WHEN `ls web/src/routes/lab-detail.tsx` is run
- THEN the command exits non-zero

#### Scenario: no module references the deleted file

- GIVEN a fresh checkout of the post-merge tree
- WHEN
  `grep -RIn "lab-detail" web/src/ --exclude-dir=node_modules`
  is run
- THEN no output references the deleted file. References to the
  route path `/lab/:id` inside `main.tsx` are acceptable and remain

#### Scenario: `/lab/:id` route still resolves to `RouteDetailContainer`

- GIVEN the routes registered in `web/src/main.tsx`
- WHEN `/lab/:id` is visited in the SPA
- THEN the same `<RouteDetailContainer trackId={id}>` component is
  rendered as for `/rutas/:id`
- AND the existing `route-detail-status-loading`,
  `route-detail`, and `track-detail-page` testids apply unchanged

## Out of Scope

The following items are intentionally excluded from this slice and
MUST NOT be touched by `sdd-apply`:

- A new `GET /api/v1/gpx/{id}/points` endpoint that returns the
  points decoupled from the rest of the detail payload (Approach 3
  in the exploration; would push the slice past the 400-line review
  budget on its own).
- Wider TODO-comment cleanup across `web/src/features/lab/*` other
  than the two specific comments addressed by this slice.
- Visible UI for the elevation-coverage signal (badge, warning, or
  partial-estimate label). The slice only makes the value
  observable end-to-end so a future UI slice can read it without
  further plumbing. The product decision on what to render is
  deferred.
- Chain-strategy selection (`stacked-to-main` vs
  `feature-branch-chain`). This belongs to `/sdd-tasks`, not to the
  spec phase.
- Any new migrations, SQLC regen, or schema change in this slice.
  Migration `00010_gpx_tracks_elevation_coverage.sql` is already
  applied and SQLC already selects `elevation_coverage`. The web
  delta consumes the field as a type addition only.

## Coverage Matrix

| Requirement | Project root | Gating test command | Pin point |
| --- | --- | --- | --- |
| Track detail page wires MapLibre polyline | `web` | `pnpm -C web test` | `RouteDetailContainer.tsx` |
| Single-track elevation profile renders real points | `web` | `pnpm -C web test` | `RouteDetail.tsx` |
| Comparator elevation profile renders correct ticks (no NaN) | `web` | `pnpm -C web test` | `ComparisonElevationProfile.tsx` |
| `rawPointsToTrackPoints` haversine helper | `web` | `pnpm -C web test` | `rawPointsToTrackPoints.ts` (new) |
| `GpxAnalysis` declares `elevation_coverage` | `web` | `pnpm -C web typecheck` | `lib/api/types.ts` |
| Stale TODO comments removed from lab detail surface | `web` | `pnpm -C web test` + PR review | `RouteDetail.tsx`, `ComparisonElevationProfile.tsx` |
| Dead-code placeholder `lab-detail.tsx` deleted | `web` | `pnpm -C web typecheck` | `routes/lab-detail.tsx` (deletion) |