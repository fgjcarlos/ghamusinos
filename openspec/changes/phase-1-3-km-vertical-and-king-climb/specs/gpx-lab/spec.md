# Delta for gpx-lab (web)

## Source

- Proposal: `openspec/changes/phase-1-3-km-vertical-and-king-climb/proposal.md`
- Exploration: `openspec/changes/phase-1-3-km-vertical-and-king-climb/exploration.md`

## Domain Metadata

| Project root | `web/` |
| --- | --- |
| Stack | React 19 + TypeScript 6 + Vite 8 + Vitest 5 + @testing-library/react + jsdom |
| Gating test command | `pnpm -C web test` (alt: `pnpm -C web test:run`) |
| Quality gates | `pnpm -C web typecheck` (`tsc --noEmit`), `pnpm -C web lint` |

This delta covers the front-end slice of issue #15's Fase 1.3
close-out: the three new persisted climb-derived features
(muros, recovery zones, km_vertical) become observable on
the existing `/rutas/:id` track detail page through three
new presentational components, mirroring the existing
`RouteClimbs` / `RouteRisks` template. The TS-side
`StoredTrackDetail` type and `normalizeTrackDetail`
extension make the new fields round-trippable; the SPA
reads the new data from the persisted GET response, not
the transient upload response.

## ADDED Requirements

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

The following items are intentionally excluded from this
slice and MUST NOT be touched by `sdd-apply`:

- **3D MapLibre + slope heatmap**. Explicit Fase 1.6 per
  `docs/architecture/feature-inventory.md:108`.
- **Comparator diff extensions** for muros /
  km_vertical / recovery_zones. The
  `ComparisonElevationProfile` /
  `RouteComparator` / `MetricsDiffTable` keep their
  current 6-metric surface; a future slice can extend
  `computeDiff` against the now-stable persisted fields.
- **Backfill of pre-existing tracks** on the front end.
  The web only displays the new fields when the backend
  persisted them; pre-existing tracks show the empty
  states.
- **Track-detail UX redesign**. The three new panels
  compose into the existing `RouteDetail` shell with the
  same visual idiom as `RouteClimbs` / `RouteRisks`.
- **Visible UI for `elevation_coverage`** (already
  shipped as a typed field on `GpxAnalysis` by the
  previous slice; no badge / warning / partial-estimate
  label is in scope).
- **OAuth / Strava lifecycle** (separate change).
- **A new `GET /api/v1/gpx/{id}/points` endpoint** that
  decouples points from the rest of the detail payload
  (Approach 3 in the exploration).
- **Renaming an existing field**. No `RENAMED` sections
  in this delta. The wire shape is additive on the GET
  side.
- **Backend changes** (`gpx-backend` domain). This delta
  is web-only; the back-end persistence, store plumbing,
  handler shape, and migration live in the
  `gpx-backend` delta for this change.

## Coverage Matrix

| Requirement | Project root | Gating test command | Pin point |
| --- | --- | --- | --- |
| `StoredTrackDetail` declares muros, recovery_zones, km_vertical | `web` | `pnpm -C web typecheck` | `web/src/lib/api/types.ts:219-224` |
| Normalizers handle muros, recovery_zones, km_vertical | `web` | `pnpm -C web test` | `web/src/features/gpx/normalize.ts:150-156` |
| `RouteKmVertical` singleton card | `web` | `pnpm -C web test` | new `RouteKmVertical/` folder |
| `RouteMuros` list card | `web` | `pnpm -C web test` | new `RouteMuros/` folder |
| `RouteRecovery` list card | `web` | `pnpm -C web test` | new `RouteRecovery/` folder |
| `RouteDetail` composes the three new panels | `web` | `pnpm -C web test` | `RouteDetail.tsx` |
| SPA reads from persisted GET path, not upload response | `web` | `pnpm -C web test` | `gpx.ts:95-115` `uploadGpx`; `RouteDetailContainer.test.tsx` |