# Delta for gpx-backend (Go)

## Source

- Proposal: `openspec/changes/wire-elevation-and-map-to-track-detail/proposal.md`
- Exploration: `openspec/changes/wire-elevation-and-map-to-track-detail/exploration.md`

## Domain Metadata

| Project root | `.` |
| --- | --- |
| Stack | Go 1.25.13 + chi/v5 + pgx/v5 + goose/v3 + sqlc + testify |
| Gating test command | `make test` (raw: `GOTOOLCHAIN=local go test -race ./...`) |
| Quality gates | `make lint` (`golangci-lint run ./...`), `make vet`, `make fmt` |

This delta covers the back-end slice: the
`Analysis.ElevationCoverage` field is propagated from the
sqlc-generated row into the value that `storedTrack` returns.
Migration 00010 already persists the column and SQLC already
selects it; no migration or SQLC regen is required for this slice.

No canonical `openspec/specs/gpx-backend/spec.md` exists yet.
Archive will copy this file into the canonical location.

## ADDED Requirements

### Requirement: `storedTrack` propagates `Analysis.ElevationCoverage`

`internal/gpx/store.go` `storedTrack` MUST set
`ElevationCoverage: numericPointer(row.ElevationCoverage)` on the
returned `Analysis` literal. After this change:

- a track that the database persists with a non-null
  `elevation_coverage` column MUST be returned to API consumers
  with a non-nil `Analysis.ElevationCoverage` pointer holding the
  same numeric value (within float64 round-trip tolerance); and
- a track persisted with `NULL` MUST be returned with
  `Analysis.ElevationCoverage == nil`.

The propagation MUST flow through every read path that goes through
`storedTrack` (`GetByID`, `GetDetail` via `GetByID`, `FindByHash`,
and `List`), because `storedTrack` is the single conversion point
shared by those paths.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/gpx/store.go` `storedTrack` function
**TDD posture**: RED-GREEN-REFACTOR. The propagation test MUST be
extended (or a new test added) with the fixture and assertion below
before the production line is added.

#### Scenario: non-null coverage value is propagated as a non-nil pointer

- GIVEN a `databaseTrack` fixture (or an equivalent new fixture)
  that sets `ElevationCoverage` to a `pgtype.Numeric` scanned from
  `"0.87"` (i.e. `Valid: true`)
- WHEN `SQLCStore.GetByID` is invoked with that fixture's row
- THEN `stored.Track.Analysis.ElevationCoverage` is non-nil
- AND its dereferenced value is within float64 tolerance of `0.87`
- AND (regression pin) a future refactor that drops the
  `ElevationCoverage:` line from the `Analysis` literal would cause
  this assertion to fail, because the zero value of `*float64` is
  `nil`; that regression protection is the explicit purpose of this
  test per the proposal's risk section

#### Scenario: NULL coverage value propagates as a nil pointer

- GIVEN a `databaseTrack` fixture whose `ElevationCoverage` is the
  pgtype zero value (`Valid: false`, representing `NULL`)
- WHEN `SQLCStore.GetByID` is invoked with that fixture's row
- THEN `stored.Track.Analysis.ElevationCoverage == nil`

#### Scenario: propagation is consistent across every read path

- GIVEN a fixture track with a non-null `ElevationCoverage`
- WHEN `SQLCStore.GetByID`, `SQLCStore.GetDetail`,
  `SQLCStore.FindByHash`, and `SQLCStore.List` are each invoked
  (single-track reads via the first three, single-item sampling via
  `List`)
- THEN every returned `StoredTrack.Analysis.ElevationCoverage`
  matches the fixture value within float64 tolerance
- AND no read path requires additional plumbing beyond the
  `storedTrack` helper itself, because `storedTrack` is the single
  conversion point they share

## Out of Scope

The following items are intentionally excluded from this slice and
MUST NOT be touched by `sdd-apply`:

- Any new migrations. Migration
  `00010_gpx_tracks_elevation_coverage.sql` is already applied and
  is not touched by this slice.
- Any SQLC regen. The SQLC code already selects `elevation_coverage`;
  no `internal/db/queries/*.sql` change is required.
- Any modification of `internal/gpx/analysis.go` or the Go haversine
  formula. The Go side is already correct; this slice only fixes
  the propagation gap in `storedTrack`.
- Visible UI for the elevation-coverage signal on the front end.
  The web-side delta adds the type only; rendering decisions are
  deferred.
- A new `GET /api/v1/gpx/{id}/points` endpoint (also excluded by
  the web-side delta).
- Chain-strategy selection. This belongs to `/sdd-tasks`.

## Coverage Matrix

| Requirement | Project root | Gating test command | Pin point |
| --- | --- | --- | --- |
| `storedTrack` propagates `Analysis.ElevationCoverage` | `.` | `make test` | `internal/gpx/store.go` `storedTrack` |