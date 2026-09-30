# Tasks: phase-1-3-km-vertical-and-king-climb

> `sdd-tasks` artefact for `phase-1-3-km-vertical-and-king-climb` (issue #15
> Fase 1.3 close-out slice). Persists and rehydrates the three climb-derived
> detection routines (`FindKmVertical`, `FindMuros`, `FindRecoveryZones`)
> that today are computed at upload and silently discarded; renders them on
> the SPA track-detail page. User-confirmed delivery strategy: **2-PR chain**
> (PR1 backend + wire shape; PR2 web UI), `chain_strategy: stacked-to-main`,
> review budget **400 changed lines per PR**.

## References

- Proposal: `openspec/changes/phase-1-3-km-vertical-and-king-climb/proposal.md`
  (818 lines, user-locked 2-PR chain).
- Exploration: `openspec/changes/phase-1-3-km-vertical-and-king-climb/exploration.md`
  (615 lines).
- Spec (backend):
  `openspec/changes/phase-1-3-km-vertical-and-king-climb/specs/gpx-backend/spec.md`
  (8 ADDED requirements, 34 scenarios).
- Spec (web):
  `openspec/changes/phase-1-3-km-vertical-and-king-climb/specs/gpx-lab/spec.md`
  (7 ADDED requirements, 17 scenarios).
- Feature document: `odd/tasks/phase-1.3-km-vertical-and-king-climb.md`
  (already committed; aligns 1:1 with this tasks list).
- Branch: `feat/phase-1.3-km-vertical` on `/home/composedof2/Dev/Codex/ghamusinos`.

## Scope

### In scope (both PRs)

- Backend (`.`): new migration `00011_gpx_muros_recovery_kmvertical.sql`; 3
  SQLC query files; `StoredTrackDetail` grows 3 fields; `SQLCStore.CreateDetail`
  signature change; `GetDetail` rehydration; `UploadGPXStore` /
  `GPXStore` mirrors; upload handler drops the orphaned-keys wrapper.
- Web (`web/`): `StoredTrackDetail` mirrors the Go struct; 3 new
  `Normalize*` helpers; 3 presentational components (`RouteKmVertical`,
  `RouteMuros`, `RouteRecovery`); `RouteDetail` composition; container test
  pins "data arrives via GET, not upload".

### Out of scope (whole change)

3D MapLibre + slope heatmap (Fase 1.6 per
`docs/architecture/feature-inventory.md:108`); comparator (`computeDiff`)
extension to muros/km_vertical/recovery (follow-up slice); backfill of
pre-existing tracks; visible UI for `elevation_coverage`; OAuth / Strava
lifecycle; UX redesign of the track-detail page. See
`proposal.md § "Out of scope"` for the canonical list.

## Strategy Summary

- **Delivery**: 2-PR chain, `chain_strategy: stacked-to-main`. PR1 lands
  first on `main`; PR2 rebases on `main` after PR1 merges. PR1 has no UI;
  PR2 depends on PR1's stable wire shape (see `gpx-lab/spec.md` requirement
  "SPA reads from the persisted GET path, not the transient upload response").
- **Review budget**: 400 changed lines per PR (per `openspec/config.yaml`).
- **TDD posture (project convention, despite `strict_tdd: false`)**:
  RED → GREEN → REFACTOR for every new testable surface. Every task below
  that adds behaviour carries an explicit "TDD" line so the apply agent
  writes the failing test first, then the implementation, then refactors.
  Mechanical changes (interface signature updates) and verification-only
  tasks do not need a separate RED phase; they are gated by an existing
  test that the production change re-greens.
- **Test command pinning**: `openspec/config.yaml` declares
  `test_command: ""` (no aggregator). Per-project commands below.

## Per-Project Test Commands

| Project | Command (CI variant) | Purpose |
|---|---|---|
| Go (`.`) | `make test` (raw: `GOTOOLCHAIN=local go test -race ./...`) | Gate every Go task. |
| Go lint | `make lint` (raw: `GOTOOLCHAIN=local golangci-lint run ./...`) | Final PR1 gate. |
| Go vet / fmt | `make vet && make fmt` | Final PR1 gate. |
| Web (`web/`) | `pnpm -C web test:run` (alt: `pnpm -C web test` for watch) | Gate every web task. |
| Web typecheck | `pnpm -C web typecheck` (raw: `tsc --noEmit`) | Gate every TS-typed task. |
| Web lint | `pnpm -C web lint` (raw: `eslint .`) | Final PR2 gate. |
| Web format | `pnpm -C web format:check` (raw: `prettier --check .`) | Final PR2 gate. |

Integration tests (`*_integration_test.go` with `//go:build integration`)
are SKIPPED by `make test` per existing project convention; this slice is
pure in-memory propagation plus SQLC mocks, so no DB tier is required.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | PR1 ≈ 300 authored lines; PR2 ≈ 250 authored lines; combined ≈ 550 LoC. |
| 400-line budget risk | **Low** — PR1 ≈ 100 LoC under; PR2 ≈ 150 LoC under. |
| Chained PRs recommended | **Yes** (already locked at proposal time). |
| Suggested split | PR1 (Go: migration + SQLC + store + handler + tests) → PR2 (web: types + normalize + 3 components + composition + tests). |
| Delivery strategy | `ask-on-risk` (parent-cached; user already accepted at `sdd-tasks` time). |
| Chain strategy | `stacked-to-main` (PR1 → main → PR2 rebases on main). |

```text
Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Low
```

### PR1 LoC Budget (per file)

| Change | Forecast LoC | Notes |
|---|---|---|
| Migration `00011_gpx_muros_recovery_kmvertical.sql` | ~50 | goose Up + Down, 3 tables, 3 indexes, 1 UNIQUE. |
| SQLC queries (`gpx_muros.sql`, `gpx_recovery_zones.sql`, `gpx_km_vertical.sql`) | ~30 | 6 queries total. |
| `internal/db/sqlc/*` regenerated | (mechanical, not authored) | Touches `querier.go`, `models.go`, and the three new `.sql.go` files. |
| `internal/gpx/store.go` (`gpxQuerier` extension + `CreateDetail` signature + `createDetail` body + `GetDetail` rehydration + 3 new accessors) | ~70 | |
| `internal/gpx/types.go` (`StoredTrackDetail` fields + `GPXStore.CreateDetail` signature) | ~10 | |
| `internal/http/handlers/gpx_upload.go` (`UploadGPXStore` interface + drop wrapper + `escribirJSON(w, detail)`) | ~10 | |
| `internal/http/handlers/gpx_upload_test.go` (mock extension + assertion move) | ~30 | |
| `internal/http/handlers/gpx_get_test.go` (new `TestGetGPXRehydratesMurosAndKmVerticalAndRecoveryZones`) | ~40 | |
| `internal/gpx/store_test.go` (mock extension + new persistence test + rehydration test + `databaseTrack` guard extension) | ~60 | |
| **PR1 total** | **~300** | ~100 LoC under the 400-line budget. |

### PR2 LoC Budget (per file)

| Change | Forecast LoC | Notes |
|---|---|---|
| `web/src/lib/api/types.ts` (3 new fields + 3 new interfaces) | ~25 | |
| `web/src/features/gpx/normalize.ts` (3 new normalizer functions + 3 new `Normalized*` interfaces + `normalizeTrackDetail` extension) | ~40 | |
| `web/src/features/gpx/normalize.test.ts` (fixture + assertions) | ~20 | |
| `web/src/features/gpx/RouteKmVertical/RouteKmVertical.tsx` + `.module.css` + `.test.tsx` | ~40 | |
| `web/src/features/gpx/RouteMuros/RouteMuros.tsx` + `MuroCard.tsx` + `.module.css` + `.test.tsx` | ~50 | |
| `web/src/features/gpx/RouteRecovery/RouteRecovery.tsx` + `RecoveryCard.tsx` + `.module.css` + `.test.tsx` | ~45 | |
| `web/src/features/gpx/RouteDetail/RouteDetail.tsx` (compose 3 panels between `RouteClimbs` and `RouteRisks`) | ~15 | |
| `web/src/lib/api/gpx.ts` (no signature change; assertion pin only) | ~5 | |
| `web/src/features/gpx/RouteDetailContainer.test.tsx` (extra assertion: GET not upload carries new fields) | ~10 | |
| **PR2 total** | **~250** | ~150 LoC under the 400-line budget. |

---

## PR1 — Backend persistence + wire shape

**Branch**: `feat/phase-1.3-km-vertical-pr1-backend` (cut from `main` at
`feat/phase-1.3-km-vertical` head). **Project root**: `.` (Go).
**Quality gate**: `make fmt && make vet && make lint && make test` all
green. TDD posture applies to every behaviour task below; mechanical
tasks carry an explicit "Mechanical, no separate RED phase" line.

### 1. Backend — DB schema + SQLC + store + handler + tests

#### 1.1. Migration `00011_gpx_muros_recovery_kmvertical.sql` (Up + Down)

- [x]

- **TDD**: RED. Write a goose migration test in
  `internal/db/migrations/00011_gpx_muros_recovery_kmvertical_test.go`
  (or co-located `00011_*_test.go` if the project has a pattern; if not,
  create one) that asserts:
  1. After `goose Up`, `\d gpx_muros`, `\d gpx_recovery_zones`, and
     `\d gpx_km_vertical` expose the expected columns (see
     `gpx-backend/spec.md` requirement "Migration creates three new
     climb-derived tables", scenarios 1–3).
  2. `gpx_km_vertical` has a `UNIQUE(track_id)` constraint.
  3. Each table has `ON DELETE CASCADE` on the FK to `gpx_tracks(id)`.
  4. After `goose Down`, all three tables are removed and no other table
     is touched.
  5. A subsequent `goose Up` re-applies the migration cleanly.
  The test FAILS today because no `00011` migration exists.
  GREEN. Write the migration file with three `CREATE TABLE` statements
  (in the Up half) and three `DROP TABLE` statements in reverse order
  (`gpx_km_vertical`, `gpx_recovery_zones`, `gpx_muros`) in the Down
  half. Follow the `-- +goose Up` / `-- +goose StatementBegin` pattern
  used by `00010_gpx_tracks_elevation_coverage.sql`. **No `ALTER TABLE`
  against `gpx_tracks`; pre-existing rows MUST NOT be rewritten.**
- **Pin point**: `internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql` (new).
- **Gate**: `make test` (with the new migration test running in-process).
- **LoC estimate**: ~50 SQL + ~60 test = ~110.
- **Depends on**: nothing (entry point).

#### 1.2. SQLC queries — `gpx_muros.sql`

- [x]

- **TDD**: RED. Write a store test in `internal/gpx/store_test.go`
  (`TestSQLCStoreListGPXMurosByTrackReturnsRowsInStartIdxOrder`) that
  injects a mock `gpxQuerier` whose `ListGPXMurosByTrack` returns two
  rows in `start_idx` order `10, 50`; the test asserts the returned
  `StoredTrackDetail.Muros` matches in that order. The test FAILS
  because `gpxQuerier` has no `ListGPXMurosByTrack` method yet
  (`mockGPXQuerier` embeds `sqlc.Querier` so the call site won't compile).
  GREEN. Write `internal/db/queries/gpx_muros.sql` with
  `-- name: CreateGPXMuro :one` and `-- name: ListGPXMurosByTrack :many
  SELECT * FROM gpx_muros WHERE track_id = $1 ORDER BY start_idx`. Then
  `make generate` to regen `internal/db/sqlc/`.
- **Pin point**: `internal/db/queries/gpx_muros.sql` (new);
  `internal/db/sqlc/gpx_muros.sql.go` (regenerated).
- **Gate**: `make test`.
- **LoC estimate**: ~10 SQL + ~30 test = ~40.
- **Depends on**: 1.1 (the migration must exist before the SQLC regen
  succeeds).

#### 1.3. SQLC queries — `gpx_recovery_zones.sql`

- [x]

- **TDD**: RED. Same posture as 1.2: write
  `TestSQLCStoreListGPXRecoveryZonesByTrackReturnsRowsInStartIdxOrder`
  against a mock that returns two rows ordered `20, 100`. The test FAILS
  because the method does not exist.
  GREEN. Write `internal/db/queries/gpx_recovery_zones.sql` with
  `CreateGPXRecoveryZone :one` and
  `ListGPXRecoveryZonesByTrack :many … ORDER BY start_idx`. Regenerate.
- **Pin point**: `internal/db/queries/gpx_recovery_zones.sql` (new);
  `internal/db/sqlc/gpx_recovery_zones.sql.go` (regenerated).
- **Gate**: `make test`.
- **LoC estimate**: ~10 SQL + ~20 test = ~30.
- **Depends on**: 1.1.

#### 1.4. SQLC queries — `gpx_km_vertical.sql` (Upsert + Get)

- [x]

- **TDD**: RED. Write `TestSQLCStoreUpsertGPXKmVerticalReplacesSingleton`
  asserting that two `UpsertGPXKmVertical` calls with the same `track_id`
  result in exactly one row whose `gain_m` matches the second call's
  value, AND a second `track_id` does NOT update the first row
  (per-track isolation, matching `gpx-backend/spec.md` requirement
  "SQLC queries … km_vertical upsert replaces the singleton row").
  Also write `TestSQLCStoreGetGPXKmVerticalByTrackReturnsNilWhenMissing`
  asserting `(nil, nil)` (not `(nil, error)`) when the row is absent.
  Both tests FAIL today because the methods do not exist.
  GREEN. Write `internal/db/queries/gpx_km_vertical.sql` with
  `UpsertGPXKmVertical :one INSERT … ON CONFLICT (track_id) DO UPDATE`
  and `GetGPXKmVerticalByTrack :one SELECT … WHERE track_id = $1`.
  Regenerate.
- **Pin point**: `internal/db/queries/gpx_km_vertical.sql` (new);
  `internal/db/sqlc/gpx_km_vertical.sql.go` (regenerated).
- **Gate**: `make test`.
- **LoC estimate**: ~10 SQL + ~40 test = ~50.
- **Depends on**: 1.1.

#### 1.5. Review `make generate` diff

- [x]

- **Mechanical, no separate RED phase.** After 1.2–1.4 land, run
  `make generate` and inspect the diff in `internal/db/sqlc/`:
  - Expected: three new `.sql.go` files; new `QueryOptions`, `Params`,
    `Row` types added to `querier.go` and `models.go`.
  - Unexpected (would fail review): column changes to
    `gpx_tracks.sql.go` / `gpx_climbs.sql.go` / `gpx_risk_zones.sql.go`;
    `models.go` losing an existing type; any change to the existing
    `gpxQuerier` methods' signatures.
- **Pin point**: `internal/db/sqlc/*` (regenerated, expected additions
  only).
- **Gate**: `make test` (regen + tests still green).
- **LoC estimate**: (mechanical, not authored).
- **Depends on**: 1.2, 1.3, 1.4.

#### 1.6. Extend `gpxQuerier` interface in `internal/gpx/store.go`

- [x]

- **TDD**: RED. The mock `mockGPXQuerier` (lines 1–80 of
  `internal/gpx/store_test.go`) embeds `sqlc.Querier`; once the
  regenerated SQLC adds the new methods, `mockGPXQuerier` already
  satisfies them transitively. But the `gpxQuerier` interface
  (`internal/gpx/store.go:99-108`) does NOT expose them; until the
  interface is extended, `CreateDetail`'s write loop and `GetDetail`'s
  rehydration cannot compile against the regenerated querier. Compile
  failure IS the RED phase. GREEN. Add the six new methods to the
  `gpxQuerier` interface, declaring them against the regenerated SQLC
  types (`CreateGPXMuro`, `ListGPXMurosByTrack`,
  `CreateGPXRecoveryZone`, `ListGPXRecoveryZonesByTrack`,
  `UpsertGPXKmVertical`, `GetGPXKmVerticalByTrack`).
- **Pin point**: `internal/gpx/store.go:99-108` (`gpxQuerier`).
- **LoC estimate**: ~8 lines (one per method declaration).
- **Depends on**: 1.5.

#### 1.7. Extend `StoredTrackDetail` Go struct

- [x]

- **TDD**: RED. Write a serialisation test in
  `internal/gpx/types_test.go` (new file if absent) that marshals a
  `StoredTrackDetail{ Muros: [m1], RecoveryZones: [r1], KmVertical: &k1 }`
  and asserts the JSON contains the top-level keys `"muros"`,
  `"recovery_zones"`, and `"km_vertical"` with the expected shapes
  (matching `gpx-backend/spec.md` scenarios "GET response carries the
  three new fields at the top level" and "km_vertical emits the literal
  `null` when the detector returned nil"). The test FAILS because the
  fields do not exist.
  GREEN. Add the three fields to `StoredTrackDetail`
  (`internal/gpx/types.go:151-156`):
  ```go
  Muros         []Muro             `json:"muros"`
  RecoveryZones []RecoveryZone     `json:"recovery_zones"`
  KmVertical    *KmVerticalResult  `json:"km_vertical"`
  ```
  No `omitempty` on any of them so the wire shape always carries the
  three keys (`muros: []`, `recovery_zones: []`, `km_vertical: null` in
  the empty case).
- **Pin point**: `internal/gpx/types.go:151-156` `StoredTrackDetail`.
- **Gate**: `make test`.
- **LoC estimate**: ~5 struct + ~30 test = ~35.
- **Depends on**: 1.5.

#### 1.8. Extend `CreateDetail` signature + body in `internal/gpx/store.go`

- [x]

- **TDD**: RED. Write
  `TestSQLCStoreCreateDetailPersistsMurosAndRecoveryZonesAndKmVertical`
  in `internal/gpx/store_test.go` that constructs a `mockGPXQuerier`
  (extended per 1.6) and asserts:
  1. Two `CreateGPXMuro` calls with parameters matching the input
     muros (field-for-field).
  2. Two `CreateGPXRecoveryZone` calls with parameters matching the
     input recovery zones.
  3. Exactly one `UpsertGPXKmVertical` call with parameters matching
     the non-nil `kmVertical`.
  4. Empty muros → zero `CreateGPXMuro` calls; empty recovery zones →
     zero `CreateGPXRecoveryZone` calls; nil `kmVertical` → zero
     `UpsertGPXKmVertical` calls.
  The test FAILS because `CreateDetail` does not accept the new
  parameters yet (`method has wrong number of arguments`).
  GREEN. Change the `SQLCStore.CreateDetail` signature to
  `CreateDetail(ctx, track, analysis, climbs, riskZones, kingClimb,
  muros, recoveryZones, kmVertical) (*StoredTrackDetail, error)`. Pass
  the new params into `createDetail`; add the three new loops inside
  `createDetail` (`internal/gpx/store.go:224-262`), inside the same
  `BEGIN`/`COMMIT` envelope (the existing transactional path). No-op
  when the slice / pointer is empty / nil.
- **Pin point**: `internal/gpx/store.go:224-262` `createDetail`.
- **Gate**: `make test`.
- **LoC estimate**: ~30 production + ~60 test = ~90.
- **Depends on**: 1.6, 1.7.

#### 1.9. Extend `GPXStore.CreateDetail` and `UploadGPXStore.CreateDetail`

- [x]

- **TDD**: RED. Compile failure IS the RED phase: the
  `GPXStore.CreateDetail` interface in `internal/gpx/types.go:223` and
  the `UploadGPXStore.CreateDetail` interface in
  `internal/http/handlers/gpx_upload.go:18-22` do not declare the new
  params. Once 1.8 lands, the production code no longer satisfies the
  interfaces, so `make build` fails.
  GREEN. Extend both interfaces with the three new params (in the same
  order as 1.8) and update the mock implementations
  (`uploadGPXStore.CreateDetail` in `gpx_upload_test.go:35`,
  `detailGPXStore.GetDetail` is read-only but the package-internal
  mock in `gpx_get_test.go` may need a shim — see 1.13,
  `compareGPXStore` in `gpx_compare_test.go`, plus the mock
  `gpxQuerier` in `store_test.go` which is unaffected because it
  implements `gpxQuerier`, not `GPXStore`).
- **Pin point**: `internal/gpx/types.go:223`; `internal/http/handlers/gpx_upload.go:18-22`.
- **Gate**: `make build && make test`.
- **LoC estimate**: ~6 lines (interface signatures) + ~15 lines
  (mock shims).
- **Depends on**: 1.8.

#### 1.10. Extend `GetDetail` rehydration in `internal/gpx/store.go`

- [x]

- **TDD**: RED. Extend `TestSQLCStoreGetDetailHydratesChildren` (the
  existing test that pins read-side rehydration for climbs / risk
  zones) with three new sub-cases asserting that `GetDetail` populates
  `Muros`, `RecoveryZones`, and `KmVertical` from the corresponding
  mock methods. Add three new mock methods on `mockGPXQuerier` that
  record the call and return canned rows (e.g.
  `mockGPXQuerier.ListGPXMurosByTrack` returns two rows). The test
  FAILS because `GetDetail` does not call the new methods.
  GREEN. Inside `GetDetail` (`internal/gpx/store.go:276-289`) add three
  calls (`ListGPXMurosByTrack`, `ListGPXRecoveryZonesByTrack`,
  `GetGPXKmVerticalByTrack`), translate each row set into the Go types
  (`Muro`, `RecoveryZone`, `*KmVerticalResult`), and populate the
  fields on the returned `StoredTrackDetail`. The propagation MUST
  flow through the same helper chain that `GetByID` / `FindByHash` /
  `List` share — extend the shared helper rather than per-call plumbing.
- **Pin point**: `internal/gpx/store.go:276-289` `GetDetail`.
- **Gate**: `make test`.
- **LoC estimate**: ~25 production + ~30 test = ~55.
- **Depends on**: 1.6, 1.7, 1.8.

#### 1.11. Add `ListMuros`, `ListRecoveryZones`, `GetKmVertical` accessors

- [x]

- **TDD**: RED. Three short tests pinning each accessor:
  `TestSQLCStoreListMurosReturnsRowsInOrder`,
  `TestSQLCStoreListRecoveryZonesReturnsRowsInOrder`,
  `TestSQLCStoreGetKmVerticalReturnsNilWhenMissing` (the last asserts
  `(nil, nil)`, not `(nil, error)`, mirroring `gpx-backend/spec.md`
  requirement "`SQLCStore` exposes `ListMuros`, `ListRecoveryZones`,
  `GetKmVertical` accessor methods", scenario "GetKmVertical returns
  nil, nil when the row does not exist"). FAIL because the methods do
  not exist.
  GREEN. Add the three accessor methods to `SQLCStore`, each a thin
  delegate to the corresponding SQLC method (parity with the existing
  `ListClimbs` / `ListRiskZones` accessors at lines 291-318).
- **Pin point**: `internal/gpx/store.go` (alongside `ListClimbs`).
- **Gate**: `make test`.
- **LoC estimate**: ~25 production + ~30 test = ~55.
- **Depends on**: 1.6, 1.10.

#### 1.12. Drop orphaned-keys wrapper in `gpx_upload.go`

- [x]

- **TDD**: RED. Replace the lossy pins at `gpx_upload_test.go:132-134`
  (`require.Contains(t, recorder.Body.String(), "muros":[])` etc.)
  with assertions on the captured `*StoredTrackDetail`:
  1. Extend the mock `uploadGPXStore` (`gpx_upload_test.go:21-37`)
     with three new captured fields (`createdMuros`,
     `createdRecoveryZones`, `createdKmVertical`) and update its
     `CreateDetail` signature.
  2. Extend the test fixture to populate `Muros = [m1]`,
     `RecoveryZones = [r1]`, `KmVertical = &k1` on the returned
     `detail`.
  3. Add assertions `store.createdMuros` deep-equals `[m1]`,
     `store.createdRecoveryZones` deep-equals `[r1]`,
     `store.createdKmVertical` deep-equals `&k1`.
  4. REMOVE the three lossy body-substring assertions
     (`"muros":[]` etc.).
  5. The test FAILS today because the upload body still carries the
     orphaned top-level keys AND because the mock signature is wrong.
  GREEN. Replace the `escribirJSON(w, struct{…}{StoredTrackDetail:
  detail, Muros: muros, RecoveryZones: recoveryZones, KmVertical:
  kmVertical})` block at `gpx_upload.go:142-146` with
  `escribirJSON(w, detail)`. The response body becomes byte-equivalent
  to `json.Marshal(detail)`. The three keys are still present at the
  top level because `detail` now carries them natively. `markKingClimb`
  and `analyzeUploadedTrack` are unchanged.
- **Pin point**: `internal/http/handlers/gpx_upload.go:142-146`.
- **Gate**: `make test`.
- **LoC estimate**: ~5 production + ~30 test = ~35.
- **Depends on**: 1.9, 1.10.

#### 1.13. New `TestGetGPXRehydratesMurosAndKmVerticalAndRecoveryZones`

- [x]

- **TDD**: RED → GREEN in one task (test-first, then a read-only
  assertion trip that the existing GET handler already produces the
  right body once the store returns a populated `StoredTrackDetail`).
  RED. Write the test in `internal/http/handlers/gpx_get_test.go`: a
  `detailGPXStore` whose `GetDetail` returns a `StoredTrackDetail`
  with `Muros = [m1]`, `RecoveryZones = [r1]`, `KmVertical = &k1`.
  The handler must be invoked and the recorder's body must contain
  `"muros":[…m1…]`, `"recovery_zones":[…r1…]`,
  `"km_vertical":{…k1…}` at the same JSON level as `climbs` and
  `risk_zones`. The test FAILS today because the GET handler's
  response shape (a JSON of the returned `*StoredTrackDetail`) lacks
  the three keys.
  GREEN. No production change is needed — the existing GET handler
  marshals the returned `StoredTrackDetail` directly. The test passes
  once tasks 1.7 + 1.10 + 1.12 are landed because `StoredTrackDetail`
  carries the fields.
- **Pin point**: `internal/http/handlers/gpx_get_test.go` (new test).
- **Gate**: `make test`.
- **LoC estimate**: ~40 test (no production change).
- **Depends on**: 1.10, 1.12.

#### 1.14. Extend `databaseTrack` regression guard in `store_test.go`

- [x]

- **TDD**: RED. Extend the existing `databaseTrack` fixture (referenced
  by the `wire-elevation-and-map-to-track-detail` regression pin) so
  that every non-empty fixture's `StoredTrackDetail` carries a
  non-empty `Muros`, `RecoveryZones`, and `KmVertical`. The fixture's
  adjacent assertions (the regression pin) MUST verify these three
  fields are non-empty. The test FAILS today because the fields don't
  exist on `StoredTrackDetail` (compile error), which is the RED.
  GREEN. Add the three fields with non-empty fixtures
  (`Muros: []Muro{ { StartIdx: 95, … } }`, etc.) and three assertions
  on `databaseTrack.stored.Muros` etc.
- **Pin point**: `internal/gpx/store_test.go` (`databaseTrack` fixture
  + adjacent regression assertions).
- **Gate**: `make test`.
- **LoC estimate**: ~15 production (fixture) + ~10 test = ~25.
- **Depends on**: 1.7, 1.10.

#### 1.15. Run `make fmt vet lint test` final gate (PR1)

- [x]

- **Mechanical, no separate RED phase.** Final quality gate.
  - `make fmt` (may auto-format; idempotent on a clean checkout).
  - `make vet`.
  - `make lint` (`golangci-lint run ./...`).
  - `make test` (full Go test run).
- **Gate**: all four green.
- **LoC estimate**: 0 LoC; ~30 seconds wall time.
- **Depends on**: 1.1–1.14.

#### 1.16. CI green on pushed branch (verification only, do not actually push)

- [ ]

- **Mechanical, no separate RED phase.** Document in the PR1 body
  which CI jobs to expect green: `backend` (gofmt + golangci-lint v2 +
  govulncheck + `go test -race -coverprofile` + build + migration
  smoke). Migration smoke runs `make db-up && make migrate` and must
  succeed (the `00011` migration applies cleanly forward + backward).
- **LoC estimate**: 0 LoC; PR description text only.
- **Depends on**: 1.15.

### PR1 Acceptance Criteria

- ✅ `make fmt && make vet && make lint && make test` all green.
- ✅ `goose Up` applies `00011` cleanly; `goose Down` reverses it
  cleanly; a second `goose Up` re-applies it cleanly.
- ✅ A new upload's `StoredTrackDetail` carries non-null
  `Muros` / `RecoveryZones` / `KmVertical` when the detectors emit.
- ✅ `GET /api/v1/gpx/{id}` rehydrates the three new fields on the
  existing detail endpoint.
- ✅ Upload response body no longer carries the three orphaned
  top-level keys (the wrapper struct is gone); the body is
  byte-equivalent to `json.Marshal(detail)`.
- ✅ `make db-up && make migrate` (CI migration smoke) green.
- ✅ Every existing test that previously pinned the lossy body
  (`muros:[]`, `recovery_zones:[]`, `km_vertical:null` as substrings)
  is updated to pin the captured `*StoredTrackDetail` fields instead.

---

## PR2 — Web UI

**Branch**: `feat/phase-1.3-km-vertical-pr2-web` (cut from `main` AFTER
PR1 merges; `stacked-to-main` chain). **Project root**: `web/`.
**Quality gate**: `pnpm -C web typecheck && pnpm -C web lint &&
pnpm -C web format:check && pnpm -C web test:run` all green. TDD
posture applies to every behaviour task below; mechanical tasks carry
"Mechanical, no separate RED phase".

### 2. Web — types + normalize + components + composition + tests

#### 2.1. Extend `StoredTrackDetail` and add three new interfaces

- [x]

- **TDD**: RED. Run `pnpm -C web typecheck` BEFORE the edit; existing
  consumers that read `detail.muros` / `detail.recovery_zones` /
  `detail.km_vertical` produce `TS2339` ("Property … does not exist
  on type …"). The compile failure IS the RED.
  GREEN. In `web/src/lib/api/types.ts` (lines 219-224), extend
  `StoredTrackDetail` with three fields:
  ```ts
  muros: GpxMuro[];
  recovery_zones: GpxRecoveryZone[];
  km_vertical: GpxKmVertical | null;
  ```
  Add three new interfaces above `StoredTrackDetail`:
  ```ts
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
- **Pin point**: `web/src/lib/api/types.ts:219-224`.
- **Gate**: `pnpm -C web typecheck`.
- **LoC estimate**: ~25 lines.
- **Depends on**: PR1 merged (the wire shape must carry the three
  fields; this is `chain_strategy: stacked-to-main` enforced).

#### 2.2. Add three normalizers + extend `normalizeTrackDetail`

- [x]

- **TDD**: RED. In `web/src/features/gpx/normalize.test.ts`, add
  fixture data with one muro, one recovery zone, and a non-null
  `km_vertical`; add three assertions: the result's `muros[0]` matches
  the input field-for-field; the result's `recovery_zones[0]` matches;
  the result's `km_vertical` is non-null and matches. Also assert that
  a fixture with `km_vertical: null` round-trips as `null`. Also assert
  that `muros: []` and `recovery_zones: []` round-trip as `[]` (no
  `undefined` elements). The test FAILS today because `normalizeTrackDetail`
  drops the three new fields (they are not on `StoredTrackDetail` per
  pre-PR1 state, so even the input shape would be `TS2741`).
  GREEN. In `web/src/features/gpx/normalize.ts`, add three new
  normalizer functions (`normalizeMuro`, `normalizeRecoveryZone`,
  `normalizeKmVertical`), three new `Normalized*` interfaces
  (`NormalizedMuro`, `NormalizedRecoveryZone`, `NormalizedKmVertical`),
  and extend `normalizeTrackDetail` (lines 150-156) to forward
  `detail.muros`, `detail.recovery_zones`, `detail.km_vertical` into
  the new fields on `NormalizedTrackDetail`. Singleton handling:
  `km_vertical === null` → `null` after normalization.
- **Pin point**: `web/src/features/gpx/normalize.ts:150-156`; new tests
  in `web/src/features/gpx/normalize.test.ts`.
- **Gate**: `pnpm -C web test:run`.
- **LoC estimate**: ~40 normalizer + ~20 test = ~60.
- **Depends on**: 2.1.

#### 2.3. `RouteKmVertical` component + tests

- [x]

- **TDD**: RED. Create `web/src/features/gpx/RouteKmVertical/RouteKmVertical.test.tsx`
  with two cases:
  1. Non-null `km_vertical` (gain_m=850, distance_m=10000) renders
     the singleton card with `data-testid="route-km-vertical"`; text
     contains "850" (gain) and the singleton's `distance_m`. The
     empty-state message is NOT in the document.
  2. `km_vertical === null` renders the empty-state message ("este
     track no tiene un tramo de subida sostenida ≥ 50 m continuo") and
     no numerical value.
  The test FAILS because the file does not exist (Vitest reports
  "Cannot find module").
  GREEN. Create the component folder with `RouteKmVertical.tsx`,
  `RouteKmVertical.module.css`, and `RouteKmVertical.test.tsx`. Follow
  the `RouteClimbs.tsx` / `RouteRisks.tsx` template: a `<section>` with
  a header reading "Km Vertical", a body when `data.km_vertical !==
  null`, and an empty-state section otherwise. Singleton card — no
  list.
- **Pin point**: new `web/src/features/gpx/RouteKmVertical/`.
- **Gate**: `pnpm -C web test:run`.
- **LoC estimate**: ~30 component + ~40 test = ~70.
- **Depends on**: 2.2.

#### 2.4. `RouteMuros` component + `MuroCard` subcomponent + tests

- [x]

- **TDD**: RED. Three test cases in
  `web/src/features/gpx/RouteMuros/RouteMuros.test.tsx`:
  1. Empty `muros: []` → empty-state message in the document, no
     `MuroCard` rendered.
  2. Single muro `[m1]` → exactly one `data-testid="muro-card"`; its
     text contains `m1.gain_m`, `m1.distance_m`, `m1.avg_slope_pct`.
  3. Three muros `[m1, m2, m3]` with `start_idx` values `10, 30, 50`
     → three `muro-card` elements in DOM order `10, 30, 50`.
  Tests FAIL because the file does not exist.
  GREEN. Create `RouteMuros/` folder with `RouteMuros.tsx`,
  `MuroCard.tsx`, `RouteMuros.module.css`, `RouteMuros.test.tsx`.
  Header "Muros"; empty state when `data.muros.length === 0`;
  otherwise list of `MuroCard` subcomponents each rendering
  `gain_m` / `distance_m` / `avg_slope_pct` with a small severity
  icon (reuse `SeverityPill` from `RouteRisks.tsx` or an inline icon).
- **Pin point**: new `web/src/features/gpx/RouteMuros/`.
- **Gate**: `pnpm -C web test:run`.
- **LoC estimate**: ~40 component + ~50 test = ~90.
- **Depends on**: 2.2.

#### 2.5. `RouteRecovery` component + `RecoveryCard` subcomponent + tests

- [x]

- **TDD**: RED. Two test cases in
  `web/src/features/gpx/RouteRecovery/RouteRecovery.test.tsx`:
  1. Empty `recovery_zones: []` → empty-state message in the
     document, no `RecoveryCard` rendered.
  2. Two recovery zones `[r1, r2]` with `distance_m` `200, 400` →
     two `data-testid="recovery-card"` elements; text contains `200`
     and `400` respectively. No `severity` / `risk_type` text is
     rendered.
  Tests FAIL because the file does not exist.
  GREEN. Create `RouteRecovery/` folder with `RouteRecovery.tsx`,
  `RecoveryCard.tsx`, `RouteRecovery.module.css`,
  `RouteRecovery.test.tsx`. Header "Recovery zones"; empty state
  when `data.recovery_zones.length === 0`; otherwise list of
  `RecoveryCard` subcomponents rendering `distance_m` only (recovery
  has no `gain_m` / `avg_slope_pct`).
- **Pin point**: new `web/src/features/gpx/RouteRecovery/`.
- **Gate**: `pnpm -C web test:run`.
- **LoC estimate**: ~35 component + ~45 test = ~80.
- **Depends on**: 2.2.

#### 2.6. Compose three new panels in `RouteDetail.tsx`

- [x]

- **TDD**: RED. Extend the existing
  `web/src/features/gpx/RouteDetail/RouteDetail.test.tsx` with two
  scenarios:
  1. `ready` state with a populated `NormalizedTrackDetail` →
     `data-testid="route-km-vertical"`, `data-testid="route-muros"`,
     `data-testid="route-recovery"` are all in the document, AND
     they appear between `data-testid="route-climbs"` and
     `data-testid="route-risks"` in DOM order.
  2. Empty-state data (`muros: []`, `recovery_zones: []`,
     `km_vertical: null`) → three testids still in the document,
     each shows its empty-state message, no exception raised.
  The test FAILS because `RouteDetail.tsx` does not import or render
  the three new components.
  GREEN. In `web/src/features/gpx/RouteDetail/RouteDetail.tsx`, add
  imports for `RouteKmVertical`, `RouteMuros`, `RouteRecovery`. Add
  three `<RouteKmVertical data={data} />`,
  `<RouteMuros data={data} />`, `<RouteRecovery data={data} />` slots
  between `<RouteClimbs>` and `<RouteRisks>`. No new state-machine
  branches; the container still owns loading / no-token / not-found /
  error.
- **Pin point**: `web/src/features/gpx/RouteDetail/RouteDetail.tsx`.
- **Gate**: `pnpm -C web test:run`.
- **LoC estimate**: ~15 production + ~25 test = ~40.
- **Depends on**: 2.3, 2.4, 2.5.

#### 2.7. Pin "data arrives via GET, not upload" in `RouteDetailContainer.test.tsx`

- [x]

- **TDD**: RED. Extend
  `web/src/features/gpx/RouteDetailContainer.test.tsx` (the
  upload → navigation → detail-fetch happy-path test) so that
  `uploadGpx` is stubbed to return `{ id: 'abc' }` only (no muros /
  recovery / km), AND `getGpxTrack('abc')` is stubbed to return a
  `StoredTrackDetail` with non-empty `muros`, `recovery_zones`, and
  `km_vertical`. The test asserts that the rendered `RouteDetail` shows
  `route-muros` with the muro cards from the GET fixture (NOT from the
  upload response), `route-recovery` from the GET fixture, and
  `route-km-vertical` with the gain from the GET fixture. The test
  FAILS today because the components don't exist (2.3–2.6 not yet
  landed) and the wire shape doesn't carry the three fields (PR1 not
  yet merged).
  GREEN. No production change — this is an assertion trip on the
  already-stubbed happy-path test. After 2.6 lands, the test passes.
- **Pin point**: `web/src/features/gpx/RouteDetailContainer.test.tsx`.
- **Gate**: `pnpm -C web test:run`.
- **LoC estimate**: ~10 test lines.
- **Depends on**: 2.6.

#### 2.8. Confirm `uploadGpx` keeps narrow return type

- [x]

- **Mechanical, no separate RED phase.** Read
  `web/src/lib/api/gpx.ts:95-115`; verify it still returns
  `Promise<{ id: string }>`. If any prior slice widened it, narrow it
  back. No tests are required (the existing `RouteDetailContainer`
  test already exercises the narrow shape).
- **Pin point**: `web/src/lib/api/gpx.ts:95-115`.
- **Gate**: `pnpm -C web typecheck`.
- **LoC estimate**: ~5 lines (verification comment + guard test if
  needed).
- **Depends on**: nothing.

#### 2.9. Run `pnpm -C web typecheck lint format:check test:run` final gate (PR2)

- [x]

- **Mechanical, no separate RED phase.** Final quality gate.
  - `pnpm -C web typecheck` (`tsc --noEmit`).
  - `pnpm -C web lint` (`eslint .`).
  - `pnpm -C web format:check` (`prettier --check .`); auto-fix with
    `pnpm -C web format` if drift is reported.
  - `pnpm -C web test:run` (`vitest run`).
- **Gate**: all four green.
- **LoC estimate**: 0 LoC; ~2 minutes wall time.
- **Depends on**: 2.1–2.8.

#### 2.10. CI green on pushed branch (verification only, do not actually push)

- [x]

- **Mechanical, no separate RED phase.** Document in the PR2 body
  which CI jobs to expect green: `frontend` (`pnpm typecheck` +
  `vite build`). Note in the PR description that
  `pnpm -C web format:check` over the full workspace may flag
  pre-existing unrelated files (mirroring the `wire-elevation-and-map-to-track-detail`
  precedent — only the files this slice created or edited must be clean).

- **LoC estimate**: 0 LoC; PR description text only.
- **Depends on**: 2.9.

### PR2 Acceptance Criteria

- ✅ `pnpm -C web typecheck && pnpm -C web lint && pnpm -C web format:check
  && pnpm -C web test:run` all green.
- ✅ `/rutas/:id` shows the three new panels (`RouteKmVertical`,
  `RouteMuros`, `RouteRecovery`) when the persisted data is populated.
- ✅ Empty state for pre-existing tracks (muros=[],
  recovery_zones=[], km_vertical=null) does NOT crash; the three
  panels render with their empty-state messages.
- ✅ The SPA reads muros / recovery / km from the persisted GET
  response, not the transient upload response (pinned by
  `RouteDetailContainer.test.tsx`).
- ✅ No regression in the existing `RouteClimbs` / `RouteRisks` /
  `RouteMetrics` / `ElevationProfile` rendering.
- ✅ `pnpm -C web format:check` is clean for the files this slice
  created or edited.

---

## PR Boundary Forecast (sanity check)

| Field | Value |
|---|---|
| PR1 changed lines | ~300 LoC (forecast); under the 400-line budget by ~100. |
| PR2 changed lines | ~250 LoC (forecast); under the 400-line budget by ~150. |
| Combined | ~550 LoC; the 2-PR split is the smallest valid decomposition. |
| Over-budget rows | None — both PRs are inside budget. |

If the apply agent's actual diff exceeds either forecast by >20%, the
parent should pause and either (a) extend the per-PR budget by the
amount of the overrun (requires explicit user `size:exception`) or
(b) split the PR further. The proposal locks the chain shape, so
`auto-chain` is not in play.

## Dependencies and Order

1. **PR1 must finish and merge to `main` before PR2 starts**
   (`chain_strategy: stacked-to-main`).
2. **Within PR1**, the order is non-negotiable:
   `1.1 migration → 1.2/1.3/1.4 SQLC queries → 1.5 regen review →
    1.6 gpxQuerier extension → 1.7 StoredTrackDetail fields →
    1.8 CreateDetail signature + body → 1.9 interface mirrors →
    1.10 GetDetail rehydration → 1.11 accessors →
    1.12 drop orphaned-keys wrapper → 1.13 GET rehydration test →
    1.14 databaseTrack regression guard → 1.15 quality gate →
    1.16 CI verification`. Skipping ahead causes compile breaks.
3. **Within PR2**, the order is non-negotiable:
   `2.1 types → 2.2 normalize → 2.3/2.4/2.5 components (parallelizable) →
    2.6 composition → 2.7 GET-not-upload pin → 2.8 uploadGpx shape check
    → 2.9 quality gate → 2.10 CI verification`.
4. **No PR depends on something not in this change.** PR2 depends
   only on PR1's wire shape; both PRs are self-contained end-to-end.
5. **No `git revert` chain** is needed in normal operation; each PR
   is independently revertible per `proposal.md § "Rollback"`.

## Risks (Task-Level)

- **Mechanical interface signature ripples** (tasks 1.6, 1.9): every
  test mock implementing `UploadGPXStore.CreateDetail` /
  `GPXStore.CreateDetail` / `gpxQuerier` needs the three new params.
  Identified call-sites: `uploadGPXStore` in
  `gpx_upload_test.go:35`, `detailGPXStore` in `gpx_get_test.go:25`,
  `compareGPXStore` in `gpx_compare_test.go`, plus any router test.
  Mitigation — the apply agent updates each mock signature inline; CI
  `go build` catches any drift.
- **`make generate` diff surprises** (task 1.5): the regen touches
  `querier.go`, `models.go`, and three new `.sql.go` files. Reviewers
  MUST confirm only the expected additions appear (no surprise column
  or schema changes in `gpx_tracks.sql.go` / `gpx_climbs.sql.go` /
  `gpx_risk_zones.sql.go`).
- **`internal/db/sqlc/*` regenerated files inflate the PR diff**: SQLC
  regen adds ~3 new `.sql.go` files which contribute to the PR diff
  even though they are mechanical. They are not in the authored-line
  budget; the budget above counts authored lines only.
- **Per-track isolation of `km_vertical` UPSERT** (task 1.4): the
  scenario "km_vertical upsert replaces the singleton row" includes a
  per-track isolation case. If a future reviewer asks why the schema
  has `UNIQUE(track_id)` instead of a composite key, the answer is
  documented in `proposal.md § "Column placement decision"` and the
  singleton invariant is enforced by the DB.
- **Web `uploadGpx` shape stays narrow** (task 2.8): no test
  regression risk, but a future slice could accidentally widen
  `uploadGpx`. The `gpx-lab/spec.md` requirement "SPA reads from the
  persisted GET path, not the transient upload response" pins the
  contract; `task 2.7` pins it in tests.
- **Pre-existing `format:check` drift** (tasks 2.9, 2.10):
  `pnpm -C web format:check` over the full workspace may flag
  unrelated pre-existing files (this happened in the previous
  `wire-elevation-and-map-to-track-detail` slice). The PR2 acceptance
  criterion is that the files this slice created or edited are clean;
  pre-existing workspace drift is recorded as a known environmental
  failure, not a slice failure.

## Per-Slice TDD Posture (binding for `sdd-apply`)

For every behaviour task above, the apply agent MUST write the failing
test FIRST (RED), then add the minimum production change to make it
pass (GREEN), then refactor without regressing the assertion (REFACTOR).
The apply prompt for PR1 and PR2 MUST include:

> TDD per project convention: write the failing test first, then the
> implementation, then refactor. Do NOT implement first.

Mechanical changes (interface signature updates, regen reviews,
quality-gate runs, CI verification) are explicitly excluded from the
per-task RED requirement but are still in-scope for the apply agent.

## Out of Scope (Task-Level)

Repeats the proposal § "Out of scope" and adds:

- **Cross-language fixture sharing** between Go and TS (the
  Madrid → Cercedilla reference is owned by each side as a literal in
  its own test file).
- **A new `GET /api/v1/gpx/{id}/points` endpoint** that decouples
  points from the rest of the detail payload (Approach 3 in the
  exploration; rejected for budget reasons).
- **Renaming any existing field** on `StoredTrackDetail` /
  `GpxAnalysis` / `GpxClimb` / `GpxRiskZone`. The wire shape is
  additive on the GET side and refactored-but-byte-equivalent on the
  upload side.
- **Modifying `internal/gpx/climbs.go`** (the detectors are correct;
  this slice only persists their outputs).
- **`Migration 00012` or later**. `00011` is the next number.
- **Comparator (`computeDiff`) extension to muros/km_vertical/
  recovery_zones**. Follow-up slice; the `gpx-backend/spec.md`
  Out-of-Scope list documents this.
- **Visible UI for `elevation_coverage`**. Already shipped as a typed
  field; no badge / warning / partial-estimate label is in scope.
- **OAuth / Strava lifecycle**. Separate change.
- **Lowering the integration-test entry barrier** for `make test`.
  `*_integration_test.go` files require live PostgreSQL via
  `docker compose`; this slice is pure in-memory propagation plus SQLC
  mocks, so no DB tier is required to apply.

## Rollback Plan (per PR)

### PR1 rollback

A `git revert` of the PR1 merge commit removes every PR1 delta in one
operation. Per-file paths (in execution order):

1. `internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql` —
   revert. `goose Down` drops the three new tables.
2. `internal/db/queries/gpx_muros.sql`,
   `internal/db/queries/gpx_recovery_zones.sql`,
   `internal/db/queries/gpx_km_vertical.sql` — revert (delete the three
   new query files). The reverted commit carries the regenerated
   `internal/db/sqlc/*` in their pre-PR1 state.
3. `internal/gpx/store.go` — revert the `CreateDetail` signature
   change, the `gpxQuerier` extension, the `ListMuros` /
   `ListRecoveryZones` / `GetKmVertical` accessors, and the
   `GetDetail` / `createDetail` hydration calls. `StoredTrackDetail` in
   `internal/gpx/types.go` reverts to its three-field shape.
4. `internal/http/handlers/gpx_upload.go` — revert the
   `escribirJSON(w, detail)` simplification back to the bespoke
   `*StoredTrackDetail`-plus-three-keys wrapper. The SPA's `uploadGpx`
   continues to parse only `{ id: string }`, so client-side behaviour
   is unchanged.
5. Test files — `gpx_upload_test.go`, `gpx_get_test.go`,
   `gpx_compare_test.go`, `store_test.go` revert to their pre-PR1
   signatures and assertions (including the lossy pins on
   `"muros":[]` / `"recovery_zones":[]` / `"km_vertical":null`).
6. **No router changes**; **no auth changes**; **no `web/` changes in
   PR1**.

After revert: the codebase is byte-equivalent to the pre-PR1 state.
The slice is fully reversible.

### PR2 rollback

A `git revert` of the PR2 merge commit removes every PR2 delta in one
operation. Per-file paths:

1. `web/src/lib/api/types.ts` — drop the three new fields on
   `StoredTrackDetail` and the three new `GpxMuro` /
   `GpxRecoveryZone` / `GpxKmVertical` interfaces. PR1's wire still
   serializes the three fields (typed `any` in TS once removed);
   `tsc --noEmit` continues to compile because the field types are not
   referenced. Mitigate by reverting PR2 BEFORE PR1 if the user wants
   the codebase back to a pre-slice state.
2. `web/src/features/gpx/normalize.ts` — drop the three new normalizer
   functions and the three new `Normalized*` interfaces.
3. `web/src/features/gpx/RouteKmVertical/`, `RouteMuros/`,
   `RouteRecovery/` — delete the three new feature folders. Their
   `.test.tsx` files disappear with them.
4. `web/src/features/gpx/RouteDetail/RouteDetail.tsx` — drop the three
   new component imports and JSX slots.
5. `web/src/features/gpx/normalize.test.ts` — drop the new fixture
   data and assertions.
6. `web/src/lib/api/gpx.ts` — `uploadGpx` reverts to its existing
   shape (no body change in this PR).

After revert: the SPA renders the same as it does today pre-slice —
muros, recovery zones, and km vertical are not shown, but the backend
(PR1) still persists and returns them. The user can either (a) accept
that the data is persisted but not rendered (recoverable by re-applying
PR2), or (b) revert PR1 too to return to a fully pre-slice state.

### Worst-case combined rollback

If both PRs need to come out, revert PR2 first (UI changes that
depend on PR1's wire shape), then revert PR1 (data layer). The PR
order matters: PR2's types compile against PR1's wire, so reverting
PR1 first would leave PR2 unable to compile.

## Open Questions

None. The wire shape is locked by the proposal and the specs. The 2-PR
chain strategy is locked by `delivery_strategy: ask-on-risk` resolved
at `sdd-tasks` time. The 3D map / comparator / backfill / OAuth items
are explicitly out of scope per the proposal.

## Apply Prompt Boilerplate (for `sdd-apply`)

When the parent invokes `sdd-apply` for PR1, the parent MUST inject
this discipline line into the apply prompt:

> TDD per project convention: write the failing test first, then the
> implementation, then refactor. Do NOT implement first.

When the parent invokes `sdd-apply` for PR2, the same line MUST be
injected. The PR2 prompt MUST also remind the apply agent that PR1
has already merged to `main` and that the SPA reads the new fields
from the persisted GET response, not the transient upload response.

## Ready for Apply

**Yes** — `nextRecommended: apply`. The parent should run `sdd-apply`
for PR1 first, gate on `make fmt && make vet && make lint &&
make test`, then `sdd-verify` on PR1, then `sdd-archive` for PR1,
then rebase `feat/phase-1.3-km-vertical` onto the updated `main`,
then `sdd-tasks` again for PR2 (lightweight — confirm wire shape
matches the spec), then `sdd-apply` for PR2, then `sdd-verify`, then
`sdd-archive` for PR2.