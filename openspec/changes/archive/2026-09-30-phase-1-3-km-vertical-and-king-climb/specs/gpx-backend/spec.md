# Delta for gpx-backend (Go)

## Source

- Proposal: `openspec/changes/phase-1-3-km-vertical-and-king-climb/proposal.md`
- Exploration: `openspec/changes/phase-1-3-km-vertical-and-king-climb/exploration.md`

## Domain Metadata

| Project root | `.` |
| --- | --- |
| Stack | Go 1.26.0 + chi/v5 + pgx/v5 + goose/v3 + sqlc + testify |
| Gating test command | `make test` (raw: `GOTOOLCHAIN=local go test -race ./...`) |
| Quality gates | `make lint` (`golangci-lint run ./...`), `make vet`, `make fmt` |

This delta covers the back-end slice of issue #15's Fase 1.3
close-out: the three orphaned climb-derived detection
routines (`FindKmVertical`, `FindMuros`,
`FindRecoveryZones`) become persistent, observable from
`GET /api/v1/gpx/{id}`, and the upload response shape stops
emitting them under orphaned top-level keys. The
`StoredTrackDetail` Go struct grows three top-level fields
and the upload handler emits `StoredTrackDetail` directly
(the same shape `GET /api/v1/gpx/{id}` already returns, now
byte-equivalent). No `openspec/specs/gpx-routes/spec.md`
exists, so this delta carries the public API contract for
both `GET /api/v1/gpx/{id}` and `POST /api/v1/gpx/upload`.

## ADDED Requirements

### Requirement: Migration creates three new climb-derived tables

`internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql`
MUST introduce three new tables for the climb-derived
detection routines that today are computed and discarded:

- `gpx_muros(track_id UUID NOT NULL REFERENCES
  gpx_tracks(id) ON DELETE CASCADE, start_idx INT NOT NULL,
  end_idx INT NOT NULL, gain_m NUMERIC NOT NULL,
  distance_m NUMERIC NOT NULL, avg_slope_pct NUMERIC NOT
  NULL)` with `idx_gpx_muros_track ON (track_id)`. Multiple
  rows per track are allowed (a list of walls).
- `gpx_recovery_zones(track_id UUID NOT NULL REFERENCES
  gpx_tracks(id) ON DELETE CASCADE, start_idx INT NOT NULL,
  end_idx INT NOT NULL, distance_m NUMERIC NOT NULL)` with
  `idx_gpx_recovery_zones_track ON (track_id)`. Recovery
  zones are NOT risk zones: no `severity`, no `risk_type`.
  Multiple rows per track are allowed.
- `gpx_km_vertical(track_id UUID NOT NULL UNIQUE REFERENCES
  gpx_tracks(id) ON DELETE CASCADE, start_idx INT NOT NULL,
  end_idx INT NOT NULL, gain_m NUMERIC NOT NULL,
  distance_m NUMERIC NOT NULL)` with
  `idx_gpx_km_vertical_track ON (track_id)`. The
  `UNIQUE(track_id)` constraint enforces the singleton
  invariant the detector encodes
  (`FindKmVertical` returns at most one row per track).

The migration MUST be net-new (no `ALTER TABLE` against
`gpx_tracks` or any pre-existing table); pre-existing rows
MUST NOT be modified. `goose Down` MUST drop the three
tables in reverse order (`gpx_km_vertical`, then
`gpx_recovery_zones`, then `gpx_muros`) without touching
other data. No backfill of existing tracks is performed in
this slice.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**:
`internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql`
(new file)

#### Scenario: forward migration creates the three tables with the expected schema

- GIVEN a fresh database or a database at the pre-`00011`
  goose version
- WHEN `goose Up` runs `00011`
- THEN the database contains three tables `gpx_muros`,
  `gpx_recovery_zones`, `gpx_km_vertical`
- AND `gpx_muros` exposes `track_id`, `start_idx`,
  `end_idx`, `gain_m`, `distance_m`, `avg_slope_pct`
- AND `gpx_recovery_zones` exposes `track_id`,
  `start_idx`, `end_idx`, `distance_m` (and does NOT
  expose `severity` or `risk_type`)
- AND `gpx_km_vertical` exposes `track_id`, `start_idx`,
  `end_idx`, `gain_m`, `distance_m`
- AND each table has a `track_id` index
- AND `gpx_km_vertical` has a `UNIQUE(track_id)` constraint
- AND each table has `ON DELETE CASCADE` on the FK to
  `gpx_tracks(id)`

#### Scenario: backward migration drops the three tables

- GIVEN the database is at goose version `00011`
- WHEN `goose Down` runs `00011`
- THEN the three tables `gpx_muros`,
  `gpx_recovery_zones`, `gpx_km_vertical` are removed
- AND no other table is touched
- AND a subsequent `goose Up` reapplies the migration
  cleanly

#### Scenario: no backfill of existing tracks

- GIVEN the database has existing rows in `gpx_tracks`
  before `00011` runs
- WHEN `goose Up` runs `00011`
- THEN every existing `gpx_tracks` row count is unchanged
- AND the three new tables are empty (zero rows)
- AND no `UPDATE` against pre-existing tables is performed
- AND no row in `gpx_tracks` is rewritten

### Requirement: SQLC queries for the three new tables

`internal/db/queries/` MUST add three new SQLC query files:

- `gpx_muros.sql` with `CreateGPXMuro` (`:one`, parameter
  set: `track_id`, `start_idx`, `end_idx`, `gain_m`,
  `distance_m`, `avg_slope_pct`) and
  `ListGPXMurosByTrack` (`:many`, parameter: `track_id`,
  `ORDER BY start_idx`).
- `gpx_recovery_zones.sql` with `CreateGPXRecoveryZone`
  (`:one`, parameter set: `track_id`, `start_idx`,
  `end_idx`, `distance_m`) and
  `ListGPXRecoveryZonesByTrack` (`:many`, parameter:
  `track_id`, `ORDER BY start_idx`).
- `gpx_km_vertical.sql` with `UpsertGPXKmVertical`
  (`:one`, parameter set: `track_id`, `start_idx`,
  `end_idx`, `gain_m`, `distance_m`; `ON CONFLICT
  (track_id) DO UPDATE`) and
  `GetGPXKmVerticalByTrack` (`:one`, parameter:
  `track_id`).

`make generate` MUST regenerate `internal/db/sqlc/*` so
that the new queries are reachable through the regenerated
`gpxQuerier` interface. The regen MAY also rewrite
`gpx_tracks.sql.go`, `gpx_climbs.sql.go`,
`gpx_risk_zones.sql.go`, `models.go`, and `querier.go` —
reviewers MUST confirm that only the expected additions
appear in those files (no surprise column or schema
changes).

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/queries/gpx_muros.sql`,
`internal/db/queries/gpx_recovery_zones.sql`,
`internal/db/queries/gpx_km_vertical.sql` (new files);
regenerated `internal/db/sqlc/*`

#### Scenario: muros are written and listed ordered by start_idx

- GIVEN an empty `gpx_muros` table
- WHEN `CreateGPXMuro` is called three times with the same
  `track_id = T` and `start_idx` values `50`, `10`, `30`
  (insertion order: `50`, `10`, `30`)
- AND `ListGPXMurosByTrack(track_id = T)` is called
- THEN it returns exactly three rows
- AND the returned `start_idx` values are `10`, `30`, `50`
  in that order

#### Scenario: recovery zones are written and listed ordered by start_idx

- GIVEN an empty `gpx_recovery_zones` table
- WHEN `CreateGPXRecoveryZone` is called twice with the
  same `track_id = T` and `start_idx` values `100`, `20`
- AND `ListGPXRecoveryZonesByTrack(track_id = T)` is called
- THEN it returns exactly two rows
- AND the returned `start_idx` values are `20`, `100` in
  that order

#### Scenario: km_vertical upsert replaces the singleton row

- GIVEN `gpx_km_vertical` has zero rows
- WHEN `UpsertGPXKmVertical` is called twice with the same
  `track_id = T` and different `gain_m` values (`850`,
  then `900`)
- THEN `GetGPXKmVerticalByTrack(track_id = T)` returns
  exactly one row
- AND that row's `gain_m === 900`
- AND a second invocation with a different `track_id` does
  NOT update the first row (per-track isolation)

#### Scenario: km_vertical singleton invariant is enforced by UNIQUE

- GIVEN `gpx_km_vertical` has zero rows
- WHEN a raw `INSERT INTO gpx_km_vertical (track_id, ...)
  VALUES (T, ...)` is followed by another `INSERT` with
  the same `track_id = T` and different `gain_m`
- THEN the second `INSERT` fails with a unique-constraint
  violation
- AND only the upsert path (`ON CONFLICT (track_id) DO
  UPDATE`) can replace the row for the same `track_id`

### Requirement: `StoredTrackDetail` carries muros, recovery_zones, km_vertical as top-level fields

`internal/gpx/types.go` `StoredTrackDetail` MUST add three
top-level fields with JSON tags that mirror the Go field
names:

- `Muros []Muro` with JSON tag `json:"muros"`. Go's default
  empty-slice encoding (`[]`) is used; no `omitempty`.
- `RecoveryZones []RecoveryZone` with JSON tag
  `json:"recovery_zones"`. Go's default empty-slice encoding
  is used; no `omitempty`.
- `KmVertical *KmVerticalResult` with JSON tag
  `json:"km_vertical"` and no `omitempty` so the field is
  always emitted (the literal `null` when the detector
  returned `nil`).

The new fields MUST appear at the same JSON level as the
existing `Track`, `Climbs`, and `RiskZones` fields. The Go
struct order, JSON tag names, and shape MUST match what
`internal/http/handlers/gpx_upload.go` currently emits at
the orphaned top level, so that `GET /api/v1/gpx/{id}` is
additive and the upload body becomes byte-equivalent after
the wrapper struct is removed (see the upload-handler
requirement below).

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/gpx/types.go:151-156`
`StoredTrackDetail`

#### Scenario: GET response carries the three new fields at the top level

- GIVEN a `StoredTrackDetail` with `Muros = [m1]`,
  `RecoveryZones = [r1]`, `KmVertical = &k1`
- WHEN `json.Marshal(detail)` is invoked with the default
  encoder
- THEN the output contains the top-level keys `"muros"`,
  `"recovery_zones"`, `"km_vertical"`
- AND `"muros"` is an array whose single element matches
  `m1` field-for-field
- AND `"recovery_zones"` is an array whose single element
  matches `r1` field-for-field
- AND `"km_vertical"` is a non-null object matching `k1`
  field-for-field

#### Scenario: km_vertical emits the literal `null` when the detector returned nil

- GIVEN a `StoredTrackDetail` with `KmVertical == nil` and
  empty `Muros` / `RecoveryZones`
- WHEN `json.Marshal(detail)` is invoked
- THEN the output contains `"muros":[]`,
  `"recovery_zones":[]`, AND `"km_vertical":null`
- AND the `"km_vertical"` key is present (not omitted) so
  the wire shape matches `GET /api/v1/gpx/{id}` and the
  upload body is byte-equivalent pre/post-slice

#### Scenario: pre-existing fields are unchanged in shape and order

- GIVEN a `StoredTrackDetail` with the new fields populated
- WHEN `json.Marshal(detail)` is invoked
- THEN the existing `"track"`, `"climbs"`, `"risk_zones"`
  keys are unchanged in shape and order
- AND the new fields are additive (no key is removed or
  renamed)
- AND the `"track"` key still wraps the legacy
  `track.track` / `track.analysis` envelope

### Requirement: `CreateDetail` persists muros, recovery zones, and km_vertical

`SQLCStore.CreateDetail` MUST grow its signature to accept
`muros []Muro`, `recoveryZones []RecoveryZone`, and
`kmVertical *KmVerticalResult`, in addition to the existing
`climbs`, `riskZones`, and `kingClimb`. The implementation
MUST persist the new fields in the same database
transaction as the existing climbs and risk zones (so an
upload either succeeds end-to-end or rolls back
end-to-end):

- `muros`: for each entry, call `CreateGPXMuro`. Empty
  input is a no-op.
- `recoveryZones`: for each entry, call
  `CreateGPXRecoveryZone`. Empty input is a no-op.
- `kmVertical`: when non-nil, call
  `UpsertGPXKmVertical` once. When `nil`, skip the call
  entirely.

`UploadGPXStore`
(`internal/http/handlers/gpx_upload.go`), `GPXStore`
(`internal/gpx/types.go`), and `gpxQuerier`
(`internal/gpx/store.go`) MUST mirror the new signature so
that callers and mocks at every layer compile and pass
tests. The `ClimbDetector` interface (which declares the
three `Find*` methods) is unchanged — the detectors are
callers, not callees, for this slice.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/gpx/store.go`
`SQLCStore.CreateDetail` and `createDetail`,
`internal/gpx/types.go` `GPXStore.CreateDetail`,
`internal/http/handlers/gpx_upload.go` `UploadGPXStore`
**TDD posture**: RED-GREEN-REFACTOR. The mock
`uploadGPXStore` MUST be extended with three captured
fields (`createdMuros`, `createdRecoveryZones`,
`createdKmVertical`) and the new signature before any
production line is added, so the existing tests fail
(`method has wrong number of arguments`) until the
production line is updated.

#### Scenario: non-empty muros are persisted via CreateGPXMuro

- GIVEN a `CreateDetail` call with two muros `[m1, m2]`
- WHEN the store executes the write loop
- THEN `CreateGPXMuro` is called twice (one call per
  muro)
- AND the persisted rows match the inputs parameter-for-
  parameter (track_id, start_idx, end_idx, gain_m,
  distance_m, avg_slope_pct)

#### Scenario: empty muros are a no-op

- GIVEN a `CreateDetail` call with `muros = []Muro{}`
- WHEN the store executes the write loop
- THEN `CreateGPXMuro` is called zero times
- AND no row is inserted into `gpx_muros`

#### Scenario: non-empty recovery zones are persisted via CreateGPXRecoveryZone

- GIVEN a `CreateDetail` call with two recovery zones
  `[r1, r2]`
- WHEN the store executes the write loop
- THEN `CreateGPXRecoveryZone` is called twice (one call
  per zone)
- AND the persisted rows match the inputs parameter-for-
  parameter (track_id, start_idx, end_idx, distance_m)

#### Scenario: empty recovery zones are a no-op

- GIVEN a `CreateDetail` call with
  `recoveryZones = []RecoveryZone{}`
- WHEN the store executes the write loop
- THEN `CreateGPXRecoveryZone` is called zero times
- AND no row is inserted into `gpx_recovery_zones`

#### Scenario: non-nil km_vertical is upserted

- GIVEN a `CreateDetail` call with a non-nil `kmVertical`
- WHEN the store executes the write loop
- THEN `UpsertGPXKmVertical` is called exactly once
- AND the parameters match `kmVertical` field-for-field

#### Scenario: nil km_vertical is a no-op

- GIVEN a `CreateDetail` call with `kmVertical == nil`
- WHEN the store executes the write loop
- THEN `UpsertGPXKmVertical` is called zero times
- AND no row is inserted into `gpx_km_vertical`

#### Scenario: signature change ripples through UploadGPXStore, GPXStore, and gpxQuerier

- GIVEN the new `CreateDetail` signature
- WHEN the codebase is compiled (`make test`)
- THEN `UploadGPXStore.CreateDetail`,
  `GPXStore.CreateDetail`, and the `gpxQuerier` interface
  all declare the three new parameters
- AND every mock implementation (`uploadGPXStore` in
  `internal/http/handlers/gpx_upload_test.go`,
  `detailGPXStore` in
  `internal/http/handlers/gpx_get_test.go`,
  `compareGPXStore` in
  `internal/http/handlers/gpx_compare_test.go`, the
  `gpxQuerier` mock in `internal/gpx/store_test.go`, plus
  any router test) implements the new signature
- AND `make build` succeeds with no "method has wrong
  number of arguments" diagnostics

#### Scenario: new fields persist in the same transaction as climbs and risk zones

- GIVEN a `CreateDetail` call with two muros, two recovery
  zones, a non-nil `kmVertical`, plus the existing climbs
  and risk zones
- WHEN the store executes
- THEN ALL of `CreateGPXMuro`, `CreateGPXRecoveryZone`,
  `UpsertGPXKmVertical`, the existing climb inserts, and
  the existing risk-zone inserts run inside a single
  `BEGIN`/`COMMIT` envelope
- AND if any single insert fails, the entire transaction
  rolls back (no partial write leaves any of the new
  tables or the existing tables half-populated)

### Requirement: `GetDetail` rehydrates muros, recovery zones, and km_vertical

`SQLCStore.GetDetail` MUST populate the three new
`StoredTrackDetail` fields by calling:

- `ListGPXMurosByTrack(track_id)` for `Muros`.
- `ListGPXRecoveryZonesByTrack(track_id)` for
  `RecoveryZones`.
- `GetGPXKmVerticalByTrack(track_id)` for `KmVertical`
  (nil when the row does not exist).

The propagation MUST flow through the same helper chain
that `GetByID` / `FindByHash` / `List` share, so that
every read path that goes through `GetDetail` rehydrates
the new fields without per-call plumbing.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/gpx/store.go`
`SQLCStore.GetDetail` (and any shared helper it delegates
to)
**TDD posture**: RED-GREEN-REFACTOR. The
`TestSQLCStoreGetDetailHydratesChildren` template MUST be
extended (or a sibling test added) with the fixtures and
assertions below before any production line is added.

#### Scenario: populated muros are rehydrated as a non-empty slice

- GIVEN a `gpxQuerier` mock whose
  `ListGPXMurosByTrack(track_id)` returns two rows with
  `start_idx` values `10` and `50`
- WHEN `SQLCStore.GetDetail(ctx, track_id)` is called
- THEN the returned `StoredTrackDetail.Muros` has length
  two
- AND the `start_idx` values appear in order `10`, `50`

#### Scenario: empty muros are rehydrated as a non-nil empty slice (not nil)

- GIVEN a `gpxQuerier` mock whose
  `ListGPXMurosByTrack(track_id)` returns zero rows
- WHEN `SQLCStore.GetDetail(ctx, track_id)` is called
- THEN the returned `StoredTrackDetail.Muros` is a
  non-nil empty slice
- AND the JSON output for `muros` is `[]` (NOT `null`,
  because the slice is non-nil)

#### Scenario: populated recovery zones are rehydrated as a non-empty slice

- GIVEN a `gpxQuerier` mock whose
  `ListGPXRecoveryZonesByTrack(track_id)` returns two rows
- WHEN `SQLCStore.GetDetail(ctx, track_id)` is called
- THEN the returned `StoredTrackDetail.RecoveryZones` has
  length two
- AND the order matches the SQL `ORDER BY start_idx`

#### Scenario: empty recovery zones are rehydrated as a non-nil empty slice

- GIVEN a `gpxQuerier` mock whose
  `ListGPXRecoveryZonesByTrack(track_id)` returns zero rows
- WHEN `SQLCStore.GetDetail(ctx, track_id)` is called
- THEN the returned `StoredTrackDetail.RecoveryZones` is a
  non-nil empty slice
- AND the JSON output for `recovery_zones` is `[]`

#### Scenario: km_vertical is rehydrated as a non-nil pointer when the row exists

- GIVEN a `gpxQuerier` mock whose
  `GetGPXKmVerticalByTrack(track_id)` returns a single
  row with `gain_m = 850`
- WHEN `SQLCStore.GetDetail(ctx, track_id)` is called
- THEN the returned `StoredTrackDetail.KmVertical` is
  non-nil
- AND its `GainM` value is `850`

#### Scenario: km_vertical is rehydrated as nil when no row exists

- GIVEN a `gpxQuerier` mock whose
  `GetGPXKmVerticalByTrack(track_id)` returns
  `sql.ErrNoRows`
- WHEN `SQLCStore.GetDetail(ctx, track_id)` is called
- THEN the returned `StoredTrackDetail.KmVertical` is nil
- AND the JSON output for `km_vertical` is the literal
  `null`

### Requirement: `SQLCStore` exposes `ListMuros`, `ListRecoveryZones`, `GetKmVertical` accessor methods

`SQLCStore` MUST expose three accessor methods for callers
that want them individually, in parity with the existing
`ListClimbs` / `ListRiskZones` accessors:

- `ListMuros(ctx, track_id) ([]Muro, error)` — delegates to
  `ListGPXMurosByTrack`, returns rows ordered by
  `start_idx`.
- `ListRecoveryZones(ctx, track_id) ([]RecoveryZone,
  error)` — delegates to `ListGPXRecoveryZonesByTrack`,
  returns rows ordered by `start_idx`.
- `GetKmVertical(ctx, track_id) (*KmVerticalResult,
  error)` — delegates to
  `GetGPXKmVerticalByTrack`; returns `nil, nil` when the
  row does not exist (Go convention for `*T` "not
  present").

These methods are not on the hot read path used by
`GetDetail`; they exist so other callers (future slices,
debugging tools) can consume the same persisted rows.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/gpx/store.go`

#### Scenario: ListMuros returns rows in start_idx order

- GIVEN a `gpxQuerier` mock whose
  `ListGPXMurosByTrack` returns two rows with `start_idx`
  values `50`, `10`
- WHEN `SQLCStore.ListMuros(ctx, track_id)` is called
- THEN it returns the two rows in the order the mock
  returned them (which already matches the SQL `ORDER BY
  start_idx`)

#### Scenario: GetKmVertical returns nil, nil when the row does not exist

- GIVEN a `gpxQuerier` mock whose
  `GetGPXKmVerticalByTrack` returns `sql.ErrNoRows`
- WHEN `SQLCStore.GetKmVertical(ctx, track_id)` is called
- THEN it returns `(nil, nil)` — not `(nil, error)`
- AND the caller does not need to import `sql` to check
  for `ErrNoRows`

### Requirement: Upload handler emits `StoredTrackDetail` directly with no orphan top-level keys

`internal/http/handlers/gpx_upload.go` MUST replace the
`escribirJSON` wrapper at lines 142-146 (which emits the
three orphaned top-level keys) with a direct
`escribirJSON(w, detail)` call. The upload response body
MUST be byte-equivalent to what `detail` marshals, so the
upload response shape matches `GET /api/v1/gpx/{id}`
exactly. The wire-shape delta on the upload side is
refactored-but-byte-equivalent: the three keys are still
present at the same JSON path, but the values are now
sourced from `detail.muros` / `detail.recovery_zones` /
`detail.km_vertical` (persisted and rehydrated) instead of
from local vars captured at upload time.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**:
`internal/http/handlers/gpx_upload.go:142-146`

#### Scenario: upload body equals the StoredTrackDetail marshalled value

- GIVEN an upload handler with a `detail *StoredTrackDetail`
  populated for the new fields
- WHEN the handler writes the response body
- THEN the body bytes equal `json.Marshal(detail)`
- AND the body contains the three new keys at the top level
  (`muros`, `recovery_zones`, `km_vertical`)
- AND no extra wrapper struct, no `map[string]any` merge,
  and no second `json.Marshal` call is involved
- AND the `markKingClimb`, `analyzeUploadedTrack`, and
  detector calls in the rest of the upload pipeline remain
  unchanged

#### Scenario: upload body matches GET body shape for the same StoredTrackDetail

- GIVEN a `StoredTrackDetail` `D`
- WHEN the upload handler marshals `D` into the response
  body
- AND the GET handler marshals the same `D` (after a
  database round-trip that re-hydrates the same values)
  into the response body
- THEN the two response bodies have the same top-level
  keys (`track`, `climbs`, `risk_zones`, `muros`,
  `recovery_zones`, `km_vertical`)
- AND the values for each key match (modulo numeric
  formatting tolerance)

#### Scenario: GET response is additive on the wire

- GIVEN a client that consumes only the `track`, `climbs`,
  and `risk_zones` keys from `GET /api/v1/gpx/{id}`
- WHEN the client receives the post-slice response body
- THEN the existing fields are unchanged in shape
- AND the new keys (`muros`, `recovery_zones`,
  `km_vertical`) appear at the same JSON level as
  `climbs` / `risk_zones`
- AND the client is unaffected by the new fields (no
  read-error, no schema-validation failure)

### Requirement: Tests pin the new wire shape, persistence, and rehydration

The test suite MUST move the lossy pins in
`internal/http/handlers/gpx_upload_test.go:132-134` from
the upload response body to assertions on the captured
`*StoredTrackDetail` returned to the mock store. The mock
`uploadGPXStore` MUST grow three new captured fields
(`createdMuros`, `createdRecoveryZones`,
`createdKmVertical`) so the test can assert the slices
were forwarded into `CreateDetail`.

In addition, new tests MUST cover:

- A new
  `TestGetGPXRehydratesMurosAndKmVerticalAndRecoveryZones`
  in `internal/http/handlers/gpx_get_test.go` injecting a
  `detailGPXStore` that returns a `StoredTrackDetail`
  with the three new fields populated, asserting the JSON
  body carries them.
- A new
  `TestSQLCStoreCreateDetailPersistsMurosAndRecoveryZonesAndKmVertical`
  in `internal/gpx/store_test.go` extending the existing
  `gpxQuerier` mock to record the three new method calls,
  asserting each call's parameters match the inputs.
- An extension of
  `TestSQLCStoreGetDetailHydratesChildren` asserting the
  three new queries are invoked and the new
  `StoredTrackDetail` fields are populated.
- An extension of the existing `databaseTrack` guard so
  the new `StoredTrackDetail` fields are pinned for every
  non-empty fixture (regression pin against future
  refactors silently dropping the fields, mirroring the
  `ElevationCoverage` regression pin from
  `wire-elevation-and-map-to-track-detail`).

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**:
`internal/http/handlers/gpx_upload_test.go:132-134`,
`internal/http/handlers/gpx_get_test.go` (new test),
`internal/gpx/store_test.go` (new + extended tests)
**TDD posture**: RED-GREEN-REFACTOR. The mock
`uploadGPXStore` MUST grow its captured fields and
signature before the production line is changed, so the
test fails (`method has wrong number of arguments` /
captured fields undefined) until the production line is
updated.

#### Scenario: upload test no longer pins the lossy body substrings but pins StoredTrackDetail fields

- GIVEN a fresh `TestUploadGPXOK` (or equivalent) test
  with `Muros = [m1]`, `RecoveryZones = [r1]`, and a
  non-nil `KmVertical = &k1` populated by the mock
  `climbDetector`
- WHEN the test runs the upload handler
- THEN the assertion on the response body for the
  substring `"muros":[]` is REMOVED
- AND the assertion on the response body for the
  substring `"recovery_zones":[]` is REMOVED
- AND the assertion on the response body for the
  substring `"km_vertical":null` is REMOVED
- AND the assertion
  `store.createdMuros` deep-equals `[m1]` PASSES
- AND the assertion
  `store.createdRecoveryZones` deep-equals `[r1]` PASSES
- AND the assertion
  `store.createdKmVertical` deep-equals `&k1` PASSES

#### Scenario: get test rehydrates the three new fields

- GIVEN a `detailGPXStore` mock whose `GetDetail` returns
  a `StoredTrackDetail` with `Muros = [m1]`,
  `RecoveryZones = [r1]`, and `KmVertical = &k1`
- WHEN the GET handler runs
- THEN the response body contains `"muros":[<m1>]`,
  `"recovery_zones":[<r1>]`, and `"km_vertical":{<k1>}`
- AND each of the three new keys appears at the same
  JSON level as `climbs` and `risk_zones`

#### Scenario: store test pins the create-side persistence

- GIVEN a `gpxQuerier` mock that records every call to
  `CreateGPXMuro`, `CreateGPXRecoveryZone`, and
  `UpsertGPXKmVertical`
- WHEN `SQLCStore.CreateDetail` is invoked with two
  muros, two recovery zones, and a non-nil `kmVertical`
- THEN the recorded call count for `CreateGPXMuro`
  equals two
- AND the recorded call count for
  `CreateGPXRecoveryZone` equals two
- AND the recorded call count for
  `UpsertGPXKmVertical` equals one
- AND every recorded parameter matches the corresponding
  input element field-for-field

#### Scenario: store test pins the read-side rehydration

- GIVEN a `gpxQuerier` mock whose
  `ListGPXMurosByTrack` returns two rows,
  `ListGPXRecoveryZonesByTrack` returns two rows, and
  `GetGPXKmVerticalByTrack` returns one row
- WHEN `SQLCStore.GetDetail(ctx, track_id)` is invoked
- THEN the returned `StoredTrackDetail.Muros` length
  equals two
- AND the returned `StoredTrackDetail.RecoveryZones`
  length equals two
- AND the returned `StoredTrackDetail.KmVertical` is
  non-nil
- AND the three new `gpxQuerier` methods were each
  invoked exactly once

#### Scenario: regression pin on the StoredTrackDetail shape

- GIVEN the existing `databaseTrack` fixture (or
  equivalent) carries a non-empty `StoredTrackDetail`
- WHEN a future refactor removes one of the new fields
  (e.g. drops `Muros:` from the literal) or re-orders
  the fields such that the wire shape changes
- THEN at least one assertion fails, mirroring the
  existing `ElevationCoverage` regression posture from
  `wire-elevation-and-map-to-track-detail`
- AND the failure message names the dropped field so the
  regression is debuggable

## Out of Scope

The following items are intentionally excluded from this
slice and MUST NOT be touched by `sdd-apply`:

- **3D MapLibre + slope heatmap**. Explicit Fase 1.6 per
  `docs/architecture/feature-inventory.md:108`. The
  architecture decision moves this out of 1.3 even though
  issue #15's body still mentions it; the proposal defers
  it.
- **Comparator (`computeDiff`) integration of
  muros/km_vertical/recovery_zones**. `computeDiff` keeps
  its current 6-metric surface; adding the new
  comparisons is a follow-up slice that consumes the
  now-stable persisted fields.
- **Backfill of pre-existing tracks**. Pre-existing
  `gpx_tracks` rows keep their current (empty) muros/km/
  recovery state. Re-running analysis on existing tracks
  is a separate change.
- **Track-detail UX redesign**. The three new panels
  compose into the existing `RouteDetail` shell with the
  same visual idiom as `RouteClimbs` / `RouteRisks`.
- **Visible UI for `elevation_coverage`**. Already shipped
  as a typed field on `GpxAnalysis` by the previous slice;
  no badge / warning / partial-estimate label is in
  scope.
- **OAuth / Strava lifecycle**. Separate change.
- **Migration `00012` or later**. `00011` is the next
  number; no skips.
- **Renaming an existing field**. No `RENAMED` sections
  in this delta. The wire shape is additive on the GET
  side and refactored-but-byte-equivalent on the upload
  side.
- **A new `GET /api/v1/gpx/{id}/points` endpoint** that
  decouples points from the rest of the detail payload
  (Approach 3 in the exploration; would push the slice
  past the 400-line review budget on its own).

## Coverage Matrix

| Requirement | Project root | Gating test command | Pin point |
| --- | --- | --- | --- |
| Migration creates three new climb-derived tables | `.` | `make test` | `internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql` |
| SQLC queries for the three new tables | `.` | `make test` | `internal/db/queries/gpx_muros.sql`, `gpx_recovery_zones.sql`, `gpx_km_vertical.sql`; `internal/db/sqlc/*` regen |
| `StoredTrackDetail` carries muros, recovery_zones, km_vertical | `.` | `make test` | `internal/gpx/types.go:151-156` `StoredTrackDetail` |
| `CreateDetail` persists muros, recovery zones, and km_vertical | `.` | `make test` | `internal/gpx/store.go` `SQLCStore.CreateDetail`, `createDetail` |
| `GetDetail` rehydrates muros, recovery zones, and km_vertical | `.` | `make test` | `internal/gpx/store.go` `SQLCStore.GetDetail` |
| `SQLCStore` exposes `ListMuros`, `ListRecoveryZones`, `GetKmVertical` | `.` | `make test` | `internal/gpx/store.go` |
| Upload handler emits `StoredTrackDetail` directly with no orphan keys | `.` | `make test` | `internal/http/handlers/gpx_upload.go:142-146` |
| Tests pin the new wire shape, persistence, and rehydration | `.` | `make test` | `internal/http/handlers/gpx_upload_test.go`, `gpx_get_test.go`, `internal/gpx/store_test.go` |