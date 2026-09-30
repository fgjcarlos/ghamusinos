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


### Requirement: `StoredTrackDetail` declares muros, recovery_zones, km_vertical

`web/src/lib/api/types.ts` `StoredTrackDetail` MUST add
three top-level fields that mirror the Go wire shape:

- `muros: GpxMuro[]`.
- `recovery_zones: GpxRecoveryZone[]`.
- `km_vertical: GpxKmVertical | null` — `null` exactly
  when the Go side emits `null` (the detector returned
  `nil`).

The same file MUST declare three new interfaces:

- `GpxMuro` with `start_idx: number`, `end_idx: number`,
  `gain_m: number`, `distance_m: number`,
  `avg_slope_pct: number`.
- `GpxRecoveryZone` with `start_idx: number`,
  `end_idx: number`, `distance_m: number` (no `severity`,
  no `risk_type` — recovery is semantically distinct
  from risk).
- `GpxKmVertical` with `start_idx: number`,
  `end_idx: number`, `gain_m: number`,
  `distance_m: number`.

The new fields are additive on the wire; existing
consumers are unaffected. `km_vertical: null` parity
follows the existing `elevation_coverage: number | null`,
`max_elevation_m: number | null`, and
`min_elevation_m: number | null` shape on `GpxAnalysis`.

**Project root**: `web`
**Gating test command**: `pnpm -C web typecheck`
(`tsc --noEmit`)
**Pin point**: `web/src/lib/api/types.ts:219-224`
`StoredTrackDetail` and the three new interfaces

#### Scenario: typecheck accepts the new fields

- GIVEN a TypeScript consumer that reads `detail.muros`,
  `detail.recovery_zones`, and `detail.km_vertical` on a
  `StoredTrackDetail` value
- WHEN `pnpm -C web typecheck` is run
- THEN no `TS2339` ("Property … does not exist on type …")
  error is emitted against the consumer
- AND no `TS2741` ("Property … is missing in type …")
  error is emitted against existing fixtures that build
  `StoredTrackDetail` literals without the new fields

#### Scenario: km_vertical is typed `null`-able exactly when the Go side emits null

- GIVEN a `StoredTrackDetail` value with
  `km_vertical: null`
- WHEN it is assigned to a variable typed
  `StoredTrackDetail`
- THEN the assignment is accepted by `tsc --noEmit`
- AND reading `detail.km_vertical` after a non-null check
  narrows the type to `GpxKmVertical` (not `null`)

#### Scenario: existing fields are unchanged

- GIVEN the existing `StoredTrackDetail` declaration
- WHEN it is inspected via `pnpm -C web typecheck`
- THEN the existing `track`, `climbs`, `risk_zones`
  fields remain declared with their existing types
- AND no field is renamed or removed

### Requirement: Normalizers handle muros, recovery_zones, km_vertical

`web/src/features/gpx/normalize.ts` MUST add three new
normalizer functions and three new `Normalized*`
interfaces:

- `normalizeMuro(input: GpxMuro): NormalizedMuro`.
- `normalizeRecoveryZone(input: GpxRecoveryZone):
  NormalizedRecoveryZone`.
- `normalizeKmVertical(input: GpxKmVertical | null):
  NormalizedKmVertical | null`.

`normalizeTrackDetail` MUST forward the three new fields
into `NormalizedTrackDetail`. `km_vertical` MUST
round-trip as `null` when the wire value is `null`
(parity with `elevation_coverage`, `max_elevation_m`,
`min_elevation_m`). The Go side emits primitive
`int`/`float64` types for these fields, so the normalizers
MUST NOT add custom conversion logic beyond what already
exists for `pgtype` — they are forwarders that map the
wire shape to the normalized shape used by the UI.

**Project root**: `web`
**Gating test command**: `pnpm -C web test`
**Pin point**: `web/src/features/gpx/normalize.ts:150-156`
`normalizeTrackDetail` and the three new normalizers
**TDD posture**: RED-GREEN-REFACTOR. The test file
`web/src/features/gpx/normalize.test.ts` MUST be extended
with the fixture data and assertions below before any
normalizer is implemented.

#### Scenario: normalizeTrackDetail forwards the three new fields

- GIVEN a `StoredTrackDetail` fixture with one muro, one
  recovery zone, and a non-null `km_vertical`
- WHEN `normalizeTrackDetail(detail)` is called
- THEN the result's `muros` has length one and matches
  the fixture's muro field-for-field
- AND the result's `recovery_zones` has length one and
  matches the fixture's recovery zone field-for-field
- AND the result's `km_vertical` is non-null and matches
  the fixture's `km_vertical` field-for-field

#### Scenario: km_vertical null survives normalization

- GIVEN a `StoredTrackDetail` fixture with
  `km_vertical: null`
- WHEN `normalizeTrackDetail(detail)` is called
- THEN the result's `km_vertical` is `null`
- AND the result is JSON-round-trippable to the same
  shape (`null` → `null`)

#### Scenario: empty muros / recovery_zones round-trip as empty arrays

- GIVEN a `StoredTrackDetail` fixture with `muros: []`
  and `recovery_zones: []`
- WHEN `normalizeTrackDetail(detail)` is called
- THEN the result's `muros` is an empty array
- AND the result's `recovery_zones` is an empty array
- AND no element is `undefined` (the existing
  `flatMapNoUndefined` discipline is applied)

### Requirement: `RouteKmVertical` component renders the km_vertical singleton card

`web/src/features/gpx/RouteKmVertical/RouteKmVertical.tsx`
MUST render a `<section>` (following the
`RouteClimbs.tsx` / `RouteRisks.tsx` template) with:

- A header reading "Km Vertical".
- A body that, when `data.km_vertical !== null`,
  displays the singleton's `gain_m` and `distance_m`
  values (and any additional fields the Go type carries).
- An empty-state message ("este track no tiene un tramo
  de subida sostenida ≥ 50 m continuo") when
  `data.km_vertical === null`.

The component does NOT render a sub-card list — km_vertical
is a singleton, not a list.

**Project root**: `web`
**Gating test command**: `pnpm -C web test`
**Pin point**: new
`web/src/features/gpx/RouteKmVertical/` folder; sibling
`RouteKmVertical.test.tsx`

#### Scenario: non-null km_vertical renders the singleton card

- GIVEN a `NormalizedTrackDetail` with a non-null
  `km_vertical` whose `gain_m = 850` and
  `distance_m = 10000`
- WHEN `<RouteKmVertical data={detail} />` is rendered
- THEN the singleton card (testid `route-km-vertical`)
  is in the document
- AND the rendered text contains "850" (the gain) and
  the singleton's `distance_m` value
- AND the empty-state message is NOT in the document

#### Scenario: null km_vertical renders the empty state

- GIVEN a `NormalizedTrackDetail` with
  `km_vertical: null`
- WHEN `<RouteKmVertical data={detail} />` is rendered
- THEN the empty-state message ("este track no tiene un
  tramo de subida sostenida …") is in the document
- AND no numerical value from a singleton is rendered

### Requirement: `RouteMuros` component renders the muros list card

`web/src/features/gpx/RouteMuros/RouteMuros.tsx` MUST
render a `<section>` (following the `RouteClimbs.tsx` /
`RouteRisks.tsx` template) with:

- A header reading "Muros".
- A body that, when `data.muros.length > 0`, lists each
  muro via a `MuroCard` subcomponent rendering the
  muro's `gain_m`, `distance_m`, and `avg_slope_pct`
  with a small severity icon.
- An empty-state message when `data.muros.length === 0`.

The list MUST be rendered in the order received from the
store (already ordered by `start_idx` at the SQLC layer).

**Project root**: `web`
**Gating test command**: `pnpm -C web test`
**Pin point**: new `web/src/features/gpx/RouteMuros/`
folder; sibling `RouteMuros.test.tsx`

#### Scenario: empty muros list renders the empty state

- GIVEN a `NormalizedTrackDetail` with `muros: []`
- WHEN `<RouteMuros data={detail} />` is rendered
- THEN the empty-state message is in the document
- AND no `MuroCard` subcomponent is rendered

#### Scenario: single muro renders one MuroCard

- GIVEN a `NormalizedTrackDetail` with `muros: [m1]`
- WHEN `<RouteMuros data={detail} />` is rendered
- THEN exactly one `MuroCard` (testid `muro-card`) is in
  the document
- AND its rendered text contains `m1.gain_m`,
  `m1.distance_m`, and `m1.avg_slope_pct`

#### Scenario: three muros render in start_idx order

- GIVEN a `NormalizedTrackDetail` with
  `muros: [m1, m2, m3]` whose `start_idx` values are
  `10`, `30`, `50` in that order
- WHEN `<RouteMuros data={detail} />` is rendered
- THEN three `MuroCard` elements appear in the document
- AND their `start_idx` values appear in DOM order `10`,
  `30`, `50`

### Requirement: `RouteRecovery` component renders the recovery_zones list card

`web/src/features/gpx/RouteRecovery/RouteRecovery.tsx`
MUST render a `<section>` (following the
`RouteClimbs.tsx` / `RouteRisks.tsx` template) with:

- A header reading "Recovery zones".
- A body that, when `data.recovery_zones.length > 0`,
  lists each zone via a `RecoveryCard` subcomponent
  rendering the zone's `distance_m` (recovery zones have
  no `gain_m` / `avg_slope_pct`; they are post-climb
  flat).
- An empty-state message when
  `data.recovery_zones.length === 0`.

The list MUST be rendered in the order received from the
store (already ordered by `start_idx` at the SQLC layer).
No `severity` / `risk_type` text is rendered — recovery
is semantically distinct from risk.

**Project root**: `web`
**Gating test command**: `pnpm -C web test`
**Pin point**: new `web/src/features/gpx/RouteRecovery/`
folder; sibling `RouteRecovery.test.tsx`

#### Scenario: empty recovery_zones list renders the empty state

- GIVEN a `NormalizedTrackDetail` with
  `recovery_zones: []`
- WHEN `<RouteRecovery data={detail} />` is rendered
- THEN the empty-state message is in the document
- AND no `RecoveryCard` subcomponent is rendered

#### Scenario: two recovery_zones render two RecoveryCards

- GIVEN a `NormalizedTrackDetail` with
  `recovery_zones: [r1, r2]` whose `distance_m` values
  are `200`, `400`
- WHEN `<RouteRecovery data={detail} />` is rendered
- THEN two `RecoveryCard` elements (testid
  `recovery-card`) appear in the document
- AND their rendered text contains `200` and `400`
  respectively
- AND no `severity` or `risk_type` text is rendered
  (recovery is not a risk)

### Requirement: `RouteDetail` composes the three new panels

`web/src/features/gpx/RouteDetail/RouteDetail.tsx` MUST
compose `<RouteKmVertical>`, `<RouteMuros>`, and
`<RouteRecovery>` alongside the existing `RouteClimbs`
and `RouteRisks`. The three new panels MUST be placed
between `RouteClimbs` and `RouteRisks` (so muros and
recovery sit next to the climbs they relate to). No new
state-machine branches; the container still owns
`loading` / `no-token` / `not-found` / `error`.

**Project root**: `web`
**Gating test command**: `pnpm -C web test`
**Pin point**:
`web/src/features/gpx/RouteDetail/RouteDetail.tsx`

#### Scenario: ready status composes the three new panels between RouteClimbs and RouteRisks

- GIVEN `RouteDetailContainer` is in the `ready` state
  with a `NormalizedTrackDetail` whose `muros`,
  `recovery_zones`, and `km_vertical` are populated
- WHEN the component tree renders
- THEN the testid `route-km-vertical` is in the
  document
- AND the testid `route-muros` is in the document
- AND the testid `route-recovery` is in the document
- AND `route-km-vertical` and `route-muros` and
  `route-recovery` appear between `route-climbs` and
  `route-risks` in DOM order

#### Scenario: empty-state data composes without exception

- GIVEN a `NormalizedTrackDetail` with empty `muros`,
  empty `recovery_zones`, and `km_vertical: null`
- WHEN the component tree renders
- THEN the three testids (`route-km-vertical`,
  `route-muros`, `route-recovery`) are still in the
  document
- AND each one shows its empty-state message
- AND no exception is raised during render

### Requirement: SPA reads from the persisted GET path, not the transient upload response

`web/src/lib/api/gpx.ts` `uploadGpx` MUST keep its narrow
return type `Promise<{ id: string }>`. The wiring
contract between PR1 (backend) and PR2 (web) is that the
SPA MUST NOT rely on the upload body to read muros /
recovery_zones / km_vertical: the SPA navigates to
`/lab/{id}` after upload and reads the full detail via
`getGpxTrack`. The `RouteDetailContainer.test.tsx`
happy-path test MUST be extended with an extra assertion
that `data.muros` / `data.recovery_zones` /
`data.km_vertical` arrive via the GET response, not via
the upload response.

**Project root**: `web`
**Gating test command**: `pnpm -C web test`
**Pin point**: `web/src/lib/api/gpx.ts:95-115`
`uploadGpx` (signature unchanged);
`RouteDetailContainer.test.tsx` (new assertion)

#### Scenario: uploadGpx return type stays narrow

- GIVEN the existing `uploadGpx` declaration
- WHEN the file is inspected via `pnpm -C web typecheck`
- THEN the function still returns
  `Promise<{ id: string }>`
- AND the SPA does NOT parse `muros`, `recovery_zones`,
  or `km_vertical` from the upload body
- AND no consumer reads those keys off the upload
  response

#### Scenario: persisted data arrives via getGpxTrack, not via uploadGpx

- GIVEN a `RouteDetailContainer` test that stubs
  `uploadGpx` to return `{ id: 'abc' }` AND stubs
  `getGpxTrack('abc')` to return a `StoredTrackDetail`
  with non-empty `muros`, `recovery_zones`, and
  `km_vertical`
- WHEN the test runs through the upload → navigation →
  detail-fetch flow
- THEN the rendered `RouteDetail` shows `route-muros`
  with the muro cards from the GET fixture
- AND the rendered `RouteDetail` shows `route-recovery`
  with the recovery cards from the GET fixture
- AND the rendered `RouteDetail` shows
  `route-km-vertical` with the gain from the GET fixture
- AND if `uploadGpx` is stubbed to return `{ id: 'abc' }`
  only (no muros / recovery / km), the three testids are
  STILL present (proving they did NOT come from the
  upload body)


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
| `StoredTrackDetail` declares muros, recovery_zones, km_vertical | `web` | `pnpm -C web typecheck` | `web/src/lib/api/types.ts:219-224` |
| Normalizers handle muros, recovery_zones, km_vertical | `web` | `pnpm -C web test` | `web/src/features/gpx/normalize.ts:150-156` |
| `RouteKmVertical` singleton card | `web` | `pnpm -C web test` | new `RouteKmVertical/` folder |
| `RouteMuros` list card | `web` | `pnpm -C web test` | new `RouteMuros/` folder |
| `RouteRecovery` list card | `web` | `pnpm -C web test` | new `RouteRecovery/` folder |
| `RouteDetail` composes the three new panels | `web` | `pnpm -C web test` | `RouteDetail.tsx` |
| SPA reads from persisted GET path, not upload response | `web` | `pnpm -C web test` | `gpx.ts:95-115` `uploadGpx`; `RouteDetailContainer.test.tsx` |
