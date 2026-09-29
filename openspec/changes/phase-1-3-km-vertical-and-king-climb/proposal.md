# Proposal: phase-1-3-km-vertical-and-king-climb

> Phase 1.3 close-out slice. Closes the only remaining gap in
> issue #15's surface by persisting, serving, and rendering the
> three climb-derived detection routines (`FindKmVertical`,
> `FindMuros`, `FindRecoveryZones`) that today are computed at
> upload and then silently discarded. User-confirmed delivery
> strategy: **2-PR chain** — backend first (persistence + wire
> shape), web second (UI on the now-stable wire). Forecast
> ~550 LoC total, ~300 in PR1 and ~250 in PR2, each under the
> 400-line review budget defined in `openspec/config.yaml`.

## Intent

Issue #15's "Fase 1.3 — Laboratorio GPX (base)" surface is
feature-complete end-to-end except for one persistent gap
documented in `openspec/changes/phase-1-3-km-vertical-and-king-climb/exploration.md`:

`internal/gpx/climbs.go` ships three detection routines —
`FindKmVertical` (line 183), `FindMuros` (line 108), and
`FindRecoveryZones` (line 142) — that are reachable only via
`POST /api/v1/gpx/upload`. The handler
(`internal/http/handlers/gpx_upload.go:94-105`) computes them,
returns them in the upload response body under orphaned
top-level keys (`gpx_upload.go:143-146`), and **never persists
them**. `GET /api/v1/gpx/{id}` therefore does not return them,
the `StoredTrackDetail` Go struct (`internal/gpx/types.go:151`)
has no fields for them, and the SPA's `uploadGpx`
(`web/src/lib/api/gpx.ts:101`) parses only `{ id: string }` from
the upload body. After a page refresh, the user's Km Vertical,
muros and recovery zones are gone from view even though the
detectors still emit the correct values. This proposal closes
that gap by persisting the three routines, exposing them on the
existing GET endpoints, and rendering them on
`TrackDetailPage`. The chain strategy is locked to two PRs so
each one stays inside the 400-line review budget.

## Scope

### In scope

- New persistence for `FindKmVertical`, `FindMuros`, and
  `FindRecoveryZones` (one new migration, three new SQLC
  queries, store-side re-hydration on every read path that
  goes through `CreateDetail` / `GetDetail`).
- `StoredTrackDetail` Go struct grows three new fields
  (`Muros`, `RecoveryZones`, `KmVertical`).
- `internal/http/handlers/gpx_upload.go` stops emitting the
  orphaned top-level `muros` / `recovery_zones` / `km_vertical`
  JSON keys; the upload response becomes the same shape as the
  GET response (one cohesive `StoredTrackDetail` body).
- SPA `StoredTrackDetail` type mirrors the Go struct.
- Three new presentational components on `TrackDetailPage`
  (`RouteKmVertical`, `RouteMuros`, `RouteRecovery`) following
  the existing `RouteClimbs.tsx` / `RouteRisks.tsx` template.
- Tests at every boundary the slice touches: store unit tests,
  handler unit tests, component tests, normalize tests.

### Out of scope

- **3D MapLibre + slope heatmap.** Explicit Fase 1.6 per
  `docs/architecture/feature-inventory.md:108` ("Mapa 3D
  interactivo (terreno, heatmap pendientes, KM, nutrición)")
  and the roadmap's own "Fase 1.6 — Laboratorio GPX avanzado"
  section (`docs/roadmap/roadmap.md:38` lists 1.3 entregables
  but the inventory moves the 3D/heatmap row to 1.6). Not
  touched by this slice even though #15's body still mentions
  3D/heatmap. Surfaced here so the user can confirm the
  deferral at `sdd-tasks` time.
- **Comparator (`computeDiff`) integration of muros/km_vertical/
  recovery_zones.** The existing comparator
  (`internal/http/handlers/gpx_compare.go:108` `computeDiff`)
  keeps its current 6-metric surface. Adding muros/km/recovery
  comparisons is a follow-up slice that consumes the now-stable
  persisted fields; this proposal does not extend it.
- **Re-running analysis on existing tracks.** The detectors are
  deterministic; pre-existing tracks keep their current
  (empty) muros/km/recovery state. Backfill is a separate
  change.
- **UX redesign of the track-detail page.** The three new
  panels compose into the existing `RouteDetail` shell with
  the same visual idiom as `RouteClimbs` and `RouteRisks`.
- **Visible UI for `elevation_coverage`.** Already shipped as
  `GpxAnalysis.elevation_coverage: number | null` by the
  previous slice (closed-out as `wire-elevation-and-map-to-track-detail`).
  No badge / warning / partial-estimate label in this slice.
- **OAuth/Strava lifecycle.** Separate change.
- **Chain-strategy selection.** Locked at the parent preflight
  (`delivery_strategy: ask-on-risk`, `chain strategy: deferred until
  chaining is selected`); the two-PR split proposed here is
  what `sdd-tasks` will confirm.

## Approach

### Column placement decision

**Three new tables (`gpx_muros`, `gpx_recovery_zones`,
`gpx_km_vertical`), not three new JSONB columns on
`gpx_tracks`.** Rationale:

1. **Shape consistency.** `gpx_climbs` and `gpx_risk_zones`
   are already separate tables
   (`internal/db/migrations/00006_gpx_tracks.sql:38-58`).
   Adding three JSONB columns for three new features would
   create a mixed (relational + JSONB) persistence model for
   the same domain.
2. **Muros is a list, not a singleton.** `FindMuros` may emit
   multiple rows per track (`internal/gpx/climbs.go:108-139`).
   A single JSONB column would force every consumer to
   re-parse the array on every read; a relational table
   orders by `start_idx` like `gpx_climbs` does today.
3. **Recovery zones are semantically distinct from risk
   zones.** `FindRecoveryZones` flags post-climb flat
   (`internal/gpx/climbs.go:142-179`); it has no severity or
   risk_type. Folding it into `gpx_risk_zones` would conflate
   "risky" with "flat recovery", which the product surface
   treats differently.

The migration is `00011_gpx_muros_recovery_kmvertical.sql`,
following the existing goose numbering. Each table gets a
`track_id UUID NOT NULL REFERENCES gpx_tracks(id) ON DELETE
CASCADE` index plus a `UNIQUE(track_id)` constraint on
`gpx_km_vertical` (the detector returns at most one row).

### PR1 — Backend persistence + wire shape

**Project root:** `.` (Go). **Gating test command:** `make test`
(raw: `GOTOOLCHAIN=local go test -race ./...`).

1. **New migration `internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql`:**
   - `gpx_muros(track_id UUID …, start_idx INT, end_idx INT,
     gain_m NUMERIC, distance_m NUMERIC, avg_slope_pct NUMERIC)`
     with `idx_gpx_muros_track ON (track_id)`.
   - `gpx_recovery_zones(track_id UUID …, start_idx INT, end_idx
     INT, distance_m NUMERIC)` with `idx_gpx_recovery_zones_track
     ON (track_id)`. No `severity` / `risk_type` — recovery is
     not a risk.
   - `gpx_km_vertical(track_id UUID … UNIQUE, start_idx INT,
     end_idx INT, gain_m NUMERIC, distance_m NUMERIC)` with
     `idx_gpx_km_vertical_track ON (track_id)`. The UNIQUE
     constraint enforces the singleton invariant the detector
     encodes.
   - `goose Down` drops the three tables (no backfill needed;
     these tables are net-new).

2. **New SQLC queries** under `internal/db/queries/`:
   - `gpx_muros.sql`: `CreateGPXMuro` (`:one`), `ListGPXMurosByTrack` (`:many`, `ORDER BY start_idx`).
   - `gpx_recovery_zones.sql`: `CreateGPXRecoveryZone` (`:one`), `ListGPXRecoveryZonesByTrack` (`:many`, `ORDER BY start_idx`).
   - `gpx_km_vertical.sql`: `UpsertGPXKmVertical` (`:one`, `ON CONFLICT (track_id) DO UPDATE`) and `GetGPXKmVerticalByTrack` (`:one`). `Upsert` keeps the table singleton-friendly and matches the "exactly one row per track" detector contract; `Get` re-hydrates on read.
   - `make generate` regens `internal/db/sqlc/*`. Reviewers
     should expect diff in `gpx_tracks.sql.go`,
     `gpx_climbs.sql.go`, `gpx_risk_zones.sql.go`,
     `models.go`, `querier.go`, and the new files. Only the
     expected additions show up.

3. **`internal/gpx/store.go` changes:**
   - Extend `gpxQuerier` interface (lines 99-108) with the six
     new methods above.
   - Extend `SQLCStore.CreateDetail` signature to
     `CreateDetail(ctx, track, analysis, climbs, riskZones,
     muros, recoveryZones, kmVertical, kingClimb) →
     *StoredTrackDetail`. The `UploadGPXStore` interface
     (`internal/http/handlers/gpx_upload.go:18-22`) and the
     `GPXStore` interface (`internal/gpx/types.go:223`) mirror.
   - Extend `createDetail` (lines 224-262): after the existing
     climbs/risk zones loop, insert the new loops for muros
     and recovery zones, then a single `UpsertGPXKmVertical`
     call (no-op when `kmVertical == nil`).
   - Extend `GetDetail` (lines 276-289): call the three new
     `List*`/`Get*` queries and hydrate the new
     `StoredTrackDetail` fields.
   - Add `ListMuros`, `ListRecoveryZones`, `GetKmVertical`
     methods on `SQLCStore` for callers that want them
     individually (parity with `ListClimbs` / `ListRiskZones`
     at lines 291-318). Not all callers will use them; `GetDetail`
     is the hot path.

4. **`internal/gpx/types.go` changes:**
   - `StoredTrackDetail` (lines 151-156) gains three fields:
     `Muros []Muro`, `RecoveryZones []RecoveryZone`,
     `KmVertical *KmVerticalResult`. JSON tags mirror the Go
     field names (`json:"muros"` etc.) so the wire shape
     matches what `gpx_upload.go` currently emits at the
     orphaned top level.
   - `GPXStore.CreateDetail` interface (line 223) signature
     change propagates here.
   - `ClimbDetector` interface (lines 208-210) is **unchanged**
     — the detectors are callers, not callees, for this slice.

5. **`internal/http/handlers/gpx_upload.go` changes:**
   - Replace the `escribirJSON` wrapper at lines 142-146 (which
     emits the three orphaned top-level keys) with a direct
     `escribirJSON(w, detail)`. The upload response shape now
     matches `GET /api/v1/gpx/{id}` exactly. **Wire-shape
     change**: the upload body stops carrying the three
     duplicated top-level keys; they now live under
     `stored_track_detail.muros` / `.recovery_zones` /
     `.km_vertical`, identical to the GET response.
   - `UploadGPXStore` interface (lines 18-22) signature grows
     by three params to mirror `SQLCStore.CreateDetail`.
   - `markKingClimb`, `analyzeUploadedTrack`, and the rest of
     the upload pipeline are unchanged.

6. **Tests (TDD posture: extend the lossy pins, then add
   fresh pins for the new behavior):**
   - `internal/http/handlers/gpx_upload_test.go` lines
     132-134 currently pin the lossy body with
     `require.Contains(..., "muros":[])`,
     `require.Contains(..., "recovery_zones":[])`,
     `require.Contains(..., "km_vertical":null)`. These
     assertions MOVE: the new assertions verify that
     `detail.muros`, `detail.recovery_zones`, and
     `detail.km_vertical` are present on the returned
     `StoredTrackDetail` via `store.createdTrack` /
     `store.createdClimbs` etc. The `muros` / `recovery_zones`
     / `km_vertical` substrings disappear from the upload
     body because the orphaned keys disappear. The mock
     (`uploadGPXStore` at lines 21-37) gains three new
     captured fields so the test can assert that the slices
     were forwarded into `CreateDetail`.
   - `internal/http/handlers/gpx_get_test.go` (new
     `TestGetGPXRehydratesMurosAndKmVerticalAndRecoveryZones`):
     inject a `detailGPXStore` (lines 18-31) that returns a
     `StoredTrackDetail` with the three new fields populated;
     assert the JSON body carries them.
   - `internal/gpx/store_test.go` (new
     `TestSQLCStoreCreateDetailPersistsMurosAndRecoveryZonesAndKmVertical`
     + extension to `TestSQLCStoreGetDetailHydratesChildren`):
     pin the create-side persistence (mock `gpxQuerier`
     records the three new method calls) and the read-side
     rehydration (mock `gpxQuerier` returns non-empty rows for
     the three new queries). The existing
     `TestSQLCStoreCreateDetailPersistsClimbsAndRiskZones` is
     the template.
   - `internal/gpx/store_test.go`: also extend
     `databaseTrack` (referenced in `wire-elevation-and-map-to-track-detail`'s
     pin) with a guard that `StoredTrackDetail` carries the
     three new fields for any non-empty fixture, so future
     refactors cannot silently drop them again. (Same posture
     as the existing `ElevationCoverage` regression pin.)

#### PR1 upload→read flow (sequence diagram)

```text
Client                  UploadGPX                  SQLCStore                PostgreSQL
  │                        │                          │                          │
  │ POST /api/v1/gpx/upload│                          │                          │
  │ (multipart GPX file)   │                          │                          │
  │───────────────────────>│                          │                          │
  │                        │ analyze + detect         │                          │
  │                        │  climbs, muros,          │                          │
  │                        │  recovery, kmVertical    │                          │
  │                        │  risks, type             │                          │
  │                        │ CreateDetail(...)        │                          │
  │                        │─────────────────────────>│                          │
  │                        │                          │ BEGIN                    │
  │                        │                          │─────────────────────────>│
  │                        │                          │ INSERT gpx_tracks        │
  │                        │                          │─────────────────────────>│
  │                        │                          │ INSERT gpx_climbs (loop) │
  │                        │                          │─────────────────────────>│
  │                        │                          │ INSERT gpx_risk_zones    │
  │                        │                          │─────────────────────────>│
  │                        │                          │ INSERT gpx_muros (loop)  │  ← NEW
  │                        │                          │─────────────────────────>│
  │                        │                          │ INSERT gpx_recovery (loop)│  ← NEW
  │                        │                          │─────────────────────────>│
  │                        │                          │ UPSERT gpx_km_vertical    │  ← NEW
  │                        │                          │─────────────────────────>│
  │                        │                          │ COMMIT                   │
  │                        │                          │─────────────────────────>│
  │                        │   *StoredTrackDetail     │                          │
  │                        │<─────────────────────────│  (muros/recovery/km       │
  │                        │   {Track, Climbs,        │   embedded directly)     │
  │                        │    RiskZones, Muros,     │                          │
  │                        │    RecoveryZones,        │                          │
  │                        │    KmVertical}           │                          │
  │   201 Created          │                          │                          │
  │<───────────────────────│                          │                          │
  │  body = detail (no     │                          │                          │
  │   orphaned keys)       │                          │                          │

  ...later, page refresh or direct GET...

  │ GET /api/v1/gpx/{id}   │                          │                          │
  │───────────────────────>│                          │                          │
  │                        │ GetDetail(...)           │                          │
  │                        │─────────────────────────>│                          │
  │                        │                          │ SELECT gpx_tracks        │
  │                        │                          │─────────────────────────>│
  │                        │                          │ SELECT gpx_climbs        │
  │                        │                          │─────────────────────────>│
  │                        │                          │ SELECT gpx_risk_zones    │
  │                        │                          │─────────────────────────>│
  │                        │                          │ SELECT gpx_muros         │  ← NEW
  │                        │                          │─────────────────────────>│
  │                        │                          │ SELECT gpx_recovery      │  ← NEW
  │                        │                          │ SELECT gpx_km_vertical   │  ← NEW
  │                        │                          │<─────────────────────────│
  │                        │   *StoredTrackDetail     │                          │
  │                        │<─────────────────────────│                          │
  │   200 OK               │                          │                          │
  │<───────────────────────│                          │                          │
  │  body = same shape as  │                          │                          │
  │  the upload response   │                          │                          │
```

The non-trivial change is the disappearance of the
orphaned-key wrapper and the symmetric read path. PR1 closes
the wire gap end-to-end; PR2 closes the visual gap.

### PR2 — Web UI

**Project root:** `web/` (frontend). **Gating test command:**
`pnpm -C web test` (alt: `pnpm -C web test:run`).
**Quality gates:** `pnpm -C web typecheck` (`tsc --noEmit`),
`pnpm -C web lint`.

1. **`web/src/lib/api/types.ts`:** `StoredTrackDetail`
   (lines 219-224) gains three fields: `muros: GpxMuro[]`,
   `recovery_zones: GpxRecoveryZone[]`, `km_vertical:
   GpxKmVertical | null`. New `GpxMuro`, `GpxRecoveryZone`,
   `GpxKmVertical` interfaces mirror the Go types in
   `internal/gpx/types.go:112-129` (start_idx/end_idx integers;
   gain_m/distance_m/avg_slope_pct numbers; recovery's
   `distance_m` matches `Muro`'s shape minus gain/slope).

2. **`web/src/features/gpx/normalize.ts`:** add
   `NormalizedMuro`, `NormalizedRecoveryZone`,
   `NormalizedKmVertical` interfaces; add `normalizeMuro`,
   `normalizeRecoveryZone`, `normalizeKmVertical` functions;
   extend `normalizeTrackDetail` (lines 150-156) to forward
   `muros`, `recovery_zones`, `km_vertical` into the new
   fields on `NormalizedTrackDetail`. Singleton handling:
   `km_vertical === null` on the wire → `null` after
   normalization (parity with `elevation_coverage`,
   `max_elevation_m`, `min_elevation_m`).

3. **Three new presentational components**, each following the
   `RouteClimbs.tsx` / `RouteRisks.tsx` template (a `<section>`
   with a header, an empty state, and an `<ul>` of items):
   - `web/src/features/gpx/RouteKmVertical/RouteKmVertical.tsx` —
     singleton card. Header: "Km Vertical". Body: gain_m,
     distance_m, avg_slope_pct. Empty state when
     `data.km_vertical === null` ("este track no tiene un
     tramo de subida sostenida ≥ 50 m continuo"). One Vitest
     test (`RouteKmVertical.test.tsx`) with two cases:
     non-null renders all three values; null renders the
     empty state.
   - `web/src/features/gpx/RouteMuros/RouteMuros.tsx` —
     list card. Header: "Muros". Body: list of `MuroCard`
     subcomponents (each rendering gain_m / distance_m /
     avg_slope_pct with a small severity icon). Empty state
     when `data.muros.length === 0`. One Vitest test with
     three cases: empty list, single muro, three muros
     (assertion on order).
   - `web/src/features/gpx/RouteRecovery/RouteRecovery.tsx` —
     list card. Header: "Recovery zones". Body: list of
     `RecoveryCard` subcomponents rendering distance_m.
     Empty state when `data.recovery_zones.length === 0`.
     One Vitest test with two cases: empty list, two
     recovery zones.

4. **`web/src/features/gpx/RouteDetail/RouteDetail.tsx`:**
   compose the three new panels alongside the existing
   `RouteClimbs` / `RouteRisks`. Position: between
   `RouteClimbs` and `RouteRisks` (so muros/recovery sit next
   to the climbs they relate to). No new state machine
   branches; the container still owns loading/no-token/
   not-found/error.

5. **`web/src/features/gpx/normalize.test.ts`:** add fixture
   data for the three new fields; add 3-4 cases asserting
   the new normalizers round-trip and that `null`
   `km_vertical` survives normalization.

6. **`web/src/lib/api/gpx.ts` `uploadGpx` (lines 95-115):**
   keep the narrow return type `Promise<{ id: string }>` —
   the SPA navigates to `/lab/{id}` after upload and reads
   the full detail via `getGpxTrack`, so there is no value in
   widening the upload response shape on the client. The
   important contract change is that **`uploadGpx` MUST NOT
   rely on the upload body to read muros/recovery/km**. The
   narrow shape it consumes is the same in both "before" and
   "after": `{ id: string }`. The PR2 wiring contract is
   "use the persisted read path, not the transient response";
   the existing SPA already does this in the post-upload
   navigation flow (`web/src/routes/lab.tsx` triggers
   `getGpxTrack(id)` after upload), so no behavior change is
   required. The test that pins this is the existing
   `RouteDetailContainer.test.tsx` happy path; add one extra
   assertion that the `data.muros` / `data.recovery_zones` /
   `data.km_vertical` arrays arrive via the GET response,
   not the upload response, by mocking `getGpxTrack` to
   return a non-empty fixture while `uploadGpx` is stubbed
   to return only `{ id: 'abc' }`.

## Wire-shape delta

### GET `GET /api/v1/gpx/{id}` response — before

```json
{
  "track": {
    "track": { "id": "…", "user_id": "…", "name": "…", "file_hash": "…", "file_size_bytes": 12345, "points": [...], "track_type": "circular", "uploaded_at": "2026-09-27T…" },
    "analysis": { "distance_m": 42195, "moving_time_s": 14400, "d_plus_m": 850, "d_minus_m": 850, "elevation_coverage": 0.98, "max_elevation_m": 1450, "min_elevation_m": 600, "avg_slope_pct": 2.0, "max_slope_pct": 28.0, "effort_index": 245.0, "itra_points": 3.0, "leg_breaker_index": 1.2, "estimated_vam": 850.0, "difficulty_score": 60, "difficulty_label": "advanced", "runnability_pct": 78.5 }
  },
  "climbs": [ { "id": "…", "start_idx": 10, "end_idx": 80, "gain_m": 200, "distance_m": 2500, "avg_slope_pct": 8.0, "is_king_climb": true, "vam": 720.0 } ],
  "risk_zones": [ { "id": "…", "start_idx": 120, "end_idx": 130, "risk_type": "steep", "severity": "high" } ]
}
```

### GET `GET /api/v1/gpx/{id}` response — after (additive)

```json
{
  "track": { "…": "… same as before …" },
  "climbs": [ "…": "… same as before …" ],
  "risk_zones": [ "…": "… same as before …" ],
  "muros": [ { "start_idx": 95, "end_idx": 110, "gain_m": 45, "distance_m": 180, "avg_slope_pct": 25.0 } ],
  "recovery_zones": [ { "start_idx": 81, "end_idx": 95, "distance_m": 200 } ],
  "km_vertical": { "start_idx": 10, "end_idx": 80, "gain_m": 850, "distance_m": 10000 }
}
```

The `muros` / `recovery_zones` / `km_vertical` fields are
additive on the GET response. Clients that don't read them
are unaffected. When `km_vertical` is `null` (the detector
returns `nil` for tracks with no qualifying sustained climb),
the field is the literal `null` (Go JSON omits `omitempty`
on `*KmVerticalResult` per the design below).

### POST `POST /api/v1/gpx/upload` response — before

```json
{
  "track": { "…": "… same as GET …" },
  "climbs": [ "…": "… same as GET …" ],
  "risk_zones": [ "…": "… same as GET …" ],
  "muros": [ { "start_idx": 95, "end_idx": 110, "gain_m": 45, "distance_m": 180, "avg_slope_pct": 25.0 } ],
  "recovery_zones": [ { "start_idx": 81, "end_idx": 95, "distance_m": 200 } ],
  "km_vertical": { "start_idx": 10, "end_idx": 80, "gain_m": 850, "distance_m": 10000 }
}
```

The `muros` / `recovery_zones` / `km_vertical` keys are at the
top level (orphaned — not embedded under any parent). The
`StoredTrackDetail` returned by `CreateDetail` does NOT
contain them.

### POST `POST /api/v1/gpx/upload` response — after (breaking for any external consumer of the top-level keys)

```json
{
  "track": { "…": "… same as GET …" },
  "climbs": [ "…": "… same as GET …" ],
  "risk_zones": [ "…": "… same as GET …" ],
  "muros": [ { "start_idx": 95, "end_idx": 110, "gain_m": 45, "distance_m": 180, "avg_slope_pct": 25.0 } ],
  "recovery_zones": [ { "start_idx": 81, "end_idx": 95, "distance_m": 200 } ],
  "km_vertical": { "start_idx": 10, "end_idx": 80, "gain_m": 850, "distance_m": 10000 }
}
```

Identical at first glance — but **the upload response is now
just `escribirJSON(detail)`**, not the bespoke
`*StoredTrackDetail`-plus-three-keys wrapper. The shape is
the same because `detail` now carries the three fields
natively (PR1 step 3 + step 4). The only consumer in this
repo is the SPA, and the SPA's `uploadGpx` already parses
only `{ id: string }`, so the contract change is invisible
to the SPA. **External consumers** (CLI, mobile, future
integrations) that read the orphaned top-level keys continue
to work because the keys are still present, just under the
same name as before. The change is fully non-breaking on the
wire.

### Web types — before vs after

`web/src/lib/api/types.ts:219-224`:

```ts
// Before
export interface StoredTrackDetail {
  track: GpxTrackSummary;
  climbs: GpxClimb[];
  risk_zones: GpxRiskZone[];
}

// After (additive)
export interface StoredTrackDetail {
  track: GpxTrackSummary;
  climbs: GpxClimb[];
  risk_zones: GpxRiskZone[];
  muros: GpxMuro[];
  recovery_zones: GpxRecoveryZone[];
  km_vertical: GpxKmVertical | null;
}

export interface GpxMuro {
  start_idx: number;
  end_idx: number;
  gain_m: number;
  distance_m: number;
  avg_slope_pct: number;
}

export interface GpxRecoveryZone {
  start_idx: number;
  end_idx: number;
  distance_m: number;
}

export interface GpxKmVertical {
  start_idx: number;
  end_idx: number;
  gain_m: number;
  distance_m: number;
}
```

Additive on the wire (existing consumers unaffected).
`km_vertical` is `null` exactly when the Go side emits `null`
(detector returned `nil`); the TS type uses the
`number | null` parity that `elevation_coverage`,
`max_elevation_m`, and `min_elevation_m` already use.

### Verification commands per PR

- **PR1**: `make test` must pass; `make lint`, `make vet`,
  `make fmt` should be green. `golangci-lint run ./...` is
  the lint command. Manual smoke:
  `curl -H "Authorization: Bearer …" /api/v1/gpx/{id}` and
  verify the body has the three new keys with non-empty
  arrays on a known-good track.
- **PR2**: `pnpm -C web test:run` must pass (≈36 files /
  ~210+ tests after the slice lands); `pnpm -C web
  typecheck` (`tsc --noEmit`) and `pnpm -C web lint` must
  pass; `pnpm -C web format:check` over the changed files
  must pass (the workspace-wide format check is allowed to
  fail on pre-existing unrelated files, as it did for the
  previous slice — see `apply-progress.md` for `wire-elevation-and-map-to-track-detail`).

## Risk

- **Risk: `CreateDetail` signature change ripples through
  every mock.** The `UploadGPXStore` interface
  (`internal/http/handlers/gpx_upload.go:18-22`), the
  `gpxQuerier` interface (`internal/gpx/store.go:99-108`),
  the `GPXStore` interface (`internal/gpx/types.go:223`), and
  every test mock (`gpx_upload_test.go:35`, `gpx_get_test.go:25`,
  `gpx_compare_test.go:27`, plus router tests) need to add
  the three new params. **Severity: low** if PR1 carries a
  one-liner migration in each test mock. Mechanical change;
  the previous slice's `ElevationCoverage` propagation
  followed the same pattern and finished cleanly.

- **Risk: SQLC regen touches more files than expected.**
  Adding three tables + six queries means `make generate`
  rewrites `internal/db/sqlc/gpx_tracks.sql.go`,
  `gpx_climbs.sql.go`, `gpx_risk_zones.sql.go`, `models.go`,
  `querier.go`, plus the three new `.sql.go` files.
  **Severity: low**. The regen is mechanical and CI
  (`make generate` runs in CI per the existing pipeline)
  catches drift.

- **Risk: migration ordering with existing rows.** The new
  tables are net-new (`CREATE TABLE`); they don't touch
  `gpx_tracks`, so existing rows are unaffected. The `Up`
  migration succeeds with no rows; `Down` drops the tables.
  No `UPDATE … SET … = 0` backfill needed. **Severity:
  low**. The 00010 migration's precedent
  (`00010_gpx_tracks_elevation_coverage.sql`) handled
  nullable backfill explicitly; 00011 does not need that
  pattern.

- **Risk: breaking change for any external consumer of the
  upload response.** The wire-shape delta is the same
  shape; the orphaned top-level keys are not removed, just
  re-sourced from the same in-memory `*StoredTrackDetail`
  value. **Severity: very low** (effectively non-breaking).
  The only structural change in `gpx_upload.go` is replacing
  the bespoke wrapper struct with `escribirJSON(w, detail)`;
  the JSON output is byte-equivalent modulo whitespace.

- **Risk: 3D map + slope heatmap creep.** A reviewer asking
  "but the issue body says 3D map" is a real possibility;
  the answer is "the architecture decision moved this to
  Fase 1.6 per `feature-inventory.md:108`, and the roadmap's
  own 1.6 section confirms the deferral." **Severity: low**
  if surfaced in the proposal (done here) and confirmed at
  `sdd-tasks` time.

- **Risk: comparator (`computeDiff`) temptation.** A
  reviewer may ask "if muros/km/recovery are persisted, why
  not show them in the comparator diff too?" The
  exploration rejects this for the smallest-viable-slice
  reason: `computeDiff` keeps its 6-metric surface; a
  follow-up change can extend it. **Severity: low** if the
  decision is documented in the proposal (done here) and
  the user does not override at `sdd-tasks` time.

- **Risk: km_vertical semantics vs Recovery.** A reviewer
  may wonder why `km_vertical` is a singleton with `UPSERT`
  while muros and recovery are lists. The detector encodes
  "exactly one best sustained climb per track"; storing
  multiple rows for it would force the consumer to pick
  one. The `UNIQUE(track_id)` constraint plus
  `UpsertGPXKmVertical` makes the singleton invariant a DB
  constraint, not a convention. **Severity: very low**;
  flagged so the `sdd-spec` `Upsert` requirement lands as
  expected.

## Rollback

### PR1 rollback

PR1 is the slice that adds the new persistence and the
wire-shape contract change. A `git revert` of the merge
commit removes every PR1 delta in one operation:

1. **`internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql`**
   — revert. `goose Down` drops the three new tables. No
   existing data is touched because no other table references
   them. The migration is forward-only by design; the `Up`
   contains the `Down` for reversibility.
2. **`internal/db/queries/gpx_*.sql`** — revert (delete the
   three new query files). `make generate` would be needed to
   regenerate SQLC code without them; in practice the
   reverted commit already carries the regenerated
   `internal/db/sqlc/*` files in their pre-PR1 state.
3. **`internal/gpx/store.go`** — revert the
   `CreateDetail` signature change, the `gpxQuerier`
   extension, the `ListMuros` / `ListRecoveryZones` /
   `GetKmVertical` methods, and the `GetDetail` /
   `createDetail` hydration calls. `StoredTrackDetail` in
   `internal/gpx/types.go` reverts to its three-field shape.
4. **`internal/http/handlers/gpx_upload.go`** — revert the
   `escribirJSON(w, detail)` simplification back to the
   bespoke `*StoredTrackDetail`-plus-three-keys wrapper.
   Re-introduces the orphaned top-level keys; the SPA's
   `uploadGpx` continues to parse only `{ id: string }`, so
   client-side behavior is unchanged.
5. **Test files** — `gpx_upload_test.go`,
   `gpx_get_test.go`, `gpx_compare_test.go`, and
   `store_test.go` revert to their pre-PR1 signatures and
   assertions (including the lossy pins on
   `"muros":[]` / `"recovery_zones":[]` /
   `"km_vertical":null`).
6. **No router changes**; no auth changes; no
   `internal/frontend/dist` or `web/` changes in PR1.

After revert: the codebase is byte-equivalent to the
pre-PR1 state. The slice is fully reversible.

### PR2 rollback

PR2 is the UI slice. It compiles only on top of PR1's wire
shape. A `git revert` of the PR2 merge commit removes every
PR2 delta in one operation:

1. **`web/src/lib/api/types.ts`** — drop the three new
   fields on `StoredTrackDetail` and the three new
   `GpxMuro` / `GpxRecoveryZone` / `GpxKmVertical`
   interfaces. `tsc --noEmit` may fail (PR1's
   `StoredTrackDetail` now serializes the three new fields;
   removing the TS declaration is a regression but compiles
   because the fields are typed `any` once removed). Mitigate
   by reverting PR2 BEFORE PR1 if the user wants the
   codebase back to a pre-slice state.
2. **`web/src/features/gpx/normalize.ts`** — drop the three
   new normalizer functions and the three new
   `Normalized*` interfaces. Reverts to the three-field
   `NormalizedTrackDetail`.
3. **`web/src/features/gpx/RouteKmVertical/`,
   `RouteMuros/`, `RouteRecovery/`** — delete the three
   new feature folders. Their `.test.tsx` files disappear
   with them.
4. **`web/src/features/gpx/RouteDetail/RouteDetail.tsx`** —
   drop the three new component imports and JSX slots.
   Returns to a page that renders only `RouteHeader`,
   `RouteMetrics`, `RouteClimbs`, `RouteRisks`,
   `ElevationProfile`.
5. **`web/src/features/gpx/normalize.test.ts`** — drop the
   new fixture data and assertions.
6. **`web/src/lib/api/gpx.ts`** — `uploadGpx` reverts to
   its existing shape; no change to its body because PR2
   intentionally keeps the narrow `{ id: string }` return.

After revert: the SPA renders the same as it does today
pre-slice — muros, recovery zones, and km vertical are not
shown, but the backend (PR1) still persists and returns
them. The user can either (a) accept that the data is
persisted but not rendered (low-grade state, but
recoverable by re-applying PR2), or (b) revert PR1 too to
return to a fully pre-slice state.

### Worst-case combined rollback

If both PRs need to come out, revert PR2 first (UI changes
that depend on PR1's wire shape), then revert PR1 (data
layer). The PR order matters here: PR2's types compile
against PR1's wire, so reverting PR1 first would leave PR2
unable to compile.

## PR boundary forecast

Per the user-confirmed 2-PR chain, the LoC budget per PR is
the 400-line review budget defined in
`openspec/config.yaml`. Forecasts below are authored lines
(added or changed), not "lines of diff" — the project
standard from the previous slice's apply-progress.

### PR1 — backend persistence + wire shape (~300 LoC)

| Change | LoC |
|---|---|
| Migration `00011_gpx_muros_recovery_kmvertical.sql` | ~50 |
| SQLC queries (`gpx_muros.sql`, `gpx_recovery_zones.sql`, `gpx_km_vertical.sql`) | ~30 |
| `internal/db/sqlc/*` regenerated | (mechanical, not authored) |
| `internal/gpx/store.go` (`CreateDetail` signature + create loop + `GetDetail` hydration + new `List*`/`Get*` methods + `gpxQuerier` extension) | ~70 |
| `internal/gpx/types.go` (`StoredTrackDetail` fields + `GPXStore.CreateDetail` signature) | ~10 |
| `internal/http/handlers/gpx_upload.go` (`UploadGPXStore` interface + drop wrapper struct + call `escribirJSON(detail)`) | ~10 |
| `internal/http/handlers/gpx_upload_test.go` (mock extension + assertion move) | ~30 |
| `internal/http/handlers/gpx_get_test.go` (new rehydration test) | ~40 |
| `internal/gpx/store_test.go` (mock extension + new persistence + rehydration tests) | ~60 |
| **PR1 total** | **~300** |

Under the 400-line budget by ~100 LoC.

### PR2 — web UI (~250 LoC)

| Change | LoC |
|---|---|
| `web/src/lib/api/types.ts` (3 new fields on `StoredTrackDetail` + 3 new interfaces) | ~25 |
| `web/src/features/gpx/normalize.ts` (3 new normalizer functions + 3 new `Normalized*` interfaces + `normalizeTrackDetail` extension) | ~40 |
| `web/src/features/gpx/normalize.test.ts` (fixture + assertions) | ~20 |
| `web/src/features/gpx/RouteKmVertical/RouteKmVertical.tsx` + `RouteKmVertical.test.tsx` | ~40 |
| `web/src/features/gpx/RouteMuros/RouteMuros.tsx` + `MuroCard.tsx` + `RouteMuros.test.tsx` | ~50 |
| `web/src/features/gpx/RouteRecovery/RouteRecovery.tsx` + `RecoveryCard.tsx` + `RouteRecovery.test.tsx` | ~45 |
| `web/src/features/gpx/RouteDetail/RouteDetail.tsx` (compose three panels) | ~15 |
| `web/src/lib/api/gpx.ts` (no signature change; assertion pin only) | ~5 |
| `web/src/features/gpx/RouteDetailContainer.test.tsx` (extra assertion: GET not upload carries the new fields) | ~10 |
| **PR2 total** | **~250** |

Under the 400-line budget by ~150 LoC.

### Combined forecast

~550 LoC total across two PRs. Each PR is independently
revertible (see Rollback). Each PR fits the 400-line budget.
No `size:exception` is required. The chain strategy is the
**stacked-to-main** shape used by `closes-159-strava-disconnect`:
PR1 lands first on `main`, PR2 rebases on `main` after PR1
merges.

## Exploration caveats

The exploration is otherwise rigorous. Three minor items to
flag for `sdd-tasks` and beyond:

1. **Path-prefix drift.** The exploration cites
   `internal/http/handlers/types.go:151` for `StoredTrackDetail`;
   the file is actually `internal/gpx/types.go:151`. The
   proposal uses the correct path. No impact on the
   exploration's claims; surfaced so the next reviewer who
   greps the exploration's path finds nothing and isn't
   surprised.

2. **Column placement choice matches the exploration's
   intent but not its enumeration.** The exploration's
   "Affected areas" section enumerates "three new tables"
   (lines ~190-210) but the proposal must pick one and
   justify. The proposal picks separate tables with the
   3-line rationale above; this matches the exploration's
   intent and the existing `gpx_climbs` / `gpx_risk_zones`
   pattern.

3. **`uploadGpx` narrowing.** The exploration says
   `uploadGpx` "may stop discarding the full response body
   (or keep the `{ id: string }` narrowing — both are
   acceptable)." The proposal picks the second option (keep
   the narrow shape) for two reasons: (a) the SPA navigates
   to `/lab/{id}` after upload and reads from
   `getGpxTrack`, so widening has no client benefit; (b) the
   narrow shape is byte-identical to what the SPA consumes
   today, eliminating any front-end regression risk. The
   PR2 wiring contract is documented as "use the persisted
   read path, not the transient response", which the
   existing SPA already satisfies.

## Out of scope (whole change)

The following are intentionally excluded from this slice
chain and MUST NOT be touched by `sdd-apply`:

- A new `GET /api/v1/gpx/{id}/points` endpoint.
- A new `GET /api/v1/gpx/{id}/summary` endpoint that returns
  only the climb-derived features (Approach 2 in the
  exploration; rejected as adding cost without benefit since
  the existing detail endpoint already carries climbs and
  risk zones).
- Comparator (`computeDiff`) extensions to include
  muros/km_vertical/recovery (Approach 3; deferred to a
  follow-up change).
- Backfilling pre-existing tracks with muros/km/recovery.
  Future change.
- Visible UI for `elevation_coverage` (separate concern;
  previous slice already shipped the type).
- 3D MapLibre + slope heatmap (explicit Fase 1.6 per
  `feature-inventory.md:108`).
- OAuth/Strava lifecycle (separate change).
- Any modification of `internal/gpx/climbs.go` (the
  detectors are correct; this slice only persists their
  outputs).
- Migration `00012` or later — `00011` is the next number;
  no skips.

## Ready for Spec

**Yes** — `nextRecommended: spec`. The next phase should be
`sdd-spec` for both the backend (`openspec/specs/gpx-backend/spec.md`)
and the web (`openspec/specs/gpx-lab/spec.md`) deltas, then
`sdd-design`, then `sdd-tasks` (which will confirm the
2-PR chain), then `sdd-apply` gated by `make test` (PR1) and
`pnpm -C web test` (PR2), then `sdd-verify`, then archive.
This proposal does NOT write the spec deltas; that is
`sdd-spec`'s job.
