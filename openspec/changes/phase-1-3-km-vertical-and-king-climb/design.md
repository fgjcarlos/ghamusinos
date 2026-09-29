# Design: phase-1-3-km-vertical-and-king-climb

## Source
- Proposal: `openspec/changes/phase-1-3-km-vertical-and-king-climb/proposal.md`
- Specs: `openspec/changes/phase-1-3-km-vertical-and-king-climb/specs/gpx-backend/spec.md`, `openspec/changes/phase-1-3-km-vertical-and-king-climb/specs/gpx-lab/spec.md`
- Tasks: `openspec/changes/phase-1-3-km-vertical-and-king-climb/tasks.md`
- Context: `openspec/changes/phase-1-3-km-vertical-and-king-climb/exploration.md`

## Decisions locked by proposal (no new decisions in this design)

### D1: Three relational tables, not JSONB columns

**Rationale** (proposal §“Column placement decision”):
- Shape consistency with the existing `gpx_climbs` and `gpx_risk_zones` tables.
- Muros is a list and may produce multiple rows per track.
- Recovery zones are semantically distinct from risk zones; they represent post-climb flat sections, not risk severity/type.

**Trade-off:** more tables than JSONB columns, in exchange for consistency with the existing relational model and ordered list reads.

### D2: Singleton table for `km_vertical`

The detector returns at most one result per track. `gpx_km_vertical` enforces this with `UNIQUE(track_id)`; `UpsertGPXKmVertical` is a `:one` upsert and `GetGPXKmVerticalByTrack` rehydrates the optional result.

**Trade-off:** the constraint prevents accidental multi-row inserts; re-running analysis for a track replaces its existing result.

### D3: Two-PR chain delivery, stacked-to-main

PR1 delivers backend persistence and the stable wire shape. PR2 delivers the web types, normalization, panels, and composition on top of PR1's merged contract. Each PR is forecast below the 400 changed-line review budget.

**Rationale:** the boundary keeps backend and UI reviewable independently and lets PR2 consume a settled wire shape. **Trade-off:** more PR coordination in exchange for smaller reviews.

### D4: Upload emits `StoredTrackDetail` directly

The upload handler drops its bespoke wrapper and writes `escribirJSON(w, detail)`. `StoredTrackDetail` gains the three fields, so the keys remain at the same top-level paths and the upload response matches the GET shape. This is a refactor of the source of those values, not a removal or relocation of the keys; the SPA continues to consume only `{ id: string }` from upload and reads detail through GET.

**Rationale:** one cohesive response shape, sourced from the persisted detail rather than transient detector locals. **Compatibility note:** proposal and backend spec describe this as byte-equivalent in shape (modulo serialization details); they do not identify a breaking removal of those top-level keys.

### D5: No backfill of existing tracks

The migration creates empty, net-new tables and does not re-run deterministic detectors over existing tracks. Older tracks therefore retain empty muros/recovery lists and a nil `km_vertical` result.

**Trade-off:** existing data remains unchanged; backfill is deferred to a separate change.

## Data flow

### Upload (`POST /api/v1/gpx/upload`)

1. Parse multipart GPX, validate it, and run analysis.
2. Run `FindAllClimbs` → `FindKingClimb` → `markKingClimb` → `FindMuros` → `FindRecoveryZones` → `FindKmVertical` → `FindRiskZones` → `DetectType`.
3. Pass the results to `CreateDetail`, including `muros`, `recoveryZones`, and nullable `kmVertical`; construct `StoredTrackDetail` with these values.
4. `SQLCStore.CreateDetail` persists inside its existing transaction: the `gpx_tracks` row, climb and risk-zone rows, one row per muro/recovery zone, and the optional `km_vertical` upsert.
5. Return `escribirJSON(w, detail)`. The response has the same top-level `StoredTrackDetail` shape as GET, including empty arrays and `km_vertical: null` when absent.

### Read (`GET /api/v1/gpx/{id}`)

1. The handler calls `SQLCStore.GetDetail(ctx, userID, trackID, resolution)`.
2. The store fetches the authorized track and its existing climbs/risk zones.
3. It additionally selects muros ordered by `start_idx`, recovery zones ordered by `start_idx`, and the single `km_vertical` row (or nil when absent).
4. The store populates `StoredTrackDetail.{Muros, RecoveryZones,KmVertical}`; GET serializes that detail directly.

### Web (`TrackDetailPage`)

1. The SPA's `useQuery` path calls `getGpxTrack`; JSON is parsed as the TS `StoredTrackDetail` and normalized to `NormalizedTrackDetail`.
2. Normalization forwards the two arrays and nullable singleton without changing their values.
3. `RouteDetail` renders `RouteKmVertical`, `RouteMuros`, and `RouteRecovery` between the existing `RouteClimbs` and `RouteRisks` panels.
4. Empty arrays and null km vertical render the specified empty states. Pre-existing tracks therefore render safely without backfilled data.

## File-by-file changes

### PR1 — Backend persistence + wire shape (proposal §PR1)

**Created**
- `internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql` — create three tables; `Down` drops them in reverse order.
- `internal/db/queries/gpx_muros.sql` — create and start-index-ordered list queries.
- `internal/db/queries/gpx_recovery_zones.sql` — create and start-index-ordered list queries.
- `internal/db/queries/gpx_km_vertical.sql` — singleton upsert and lookup queries.
- `internal/db/sqlc/gpx_muros.sql.go`, `internal/db/sqlc/gpx_recovery_zones.sql.go`, `internal/db/sqlc/gpx_km_vertical.sql.go` — generated SQLC code.

**Modified**
- `internal/db/sqlc/querier.go`, `internal/db/sqlc/models.go` (and any SQLC-generated query files touched by generation) — expected generated declarations/types only; review generated diff for unrelated changes.
- `internal/gpx/types.go` — add `StoredTrackDetail` fields and extend `GPXStore.CreateDetail`; keep `ClimbDetector` unchanged.
- `internal/gpx/store.go` — extend `gpxQuerier`; persist new values in `CreateDetail`'s transaction; rehydrate in `GetDetail`; add `ListMuros`, `ListRecoveryZones`, and `GetKmVertical` accessors.
- `internal/http/handlers/gpx_upload.go` — extend `UploadGPXStore.CreateDetail`, pass detector results through, and serialize `detail` directly.
- `internal/gpx/store_test.go` — extend mock and cover create persistence, read hydration, and the detail regression pin.
- `internal/http/handlers/gpx_upload_test.go` — extend mock/captured arguments and replace the lossy response assertions with forwarding assertions.
- `internal/http/handlers/gpx_get_test.go` — add `TestGetGPXRehydratesMurosAndKmVerticalAndRecoveryZones`.
- `internal/http/handlers/gpx_compare_test.go` — propagate the `CreateDetail` signature change to its mock; comparator behavior itself remains unchanged.

### PR2 — Web UI (proposal §PR2)

**Created**
- `web/src/features/gpx/RouteKmVertical/` — singleton panel and tests.
- `web/src/features/gpx/RouteMuros/` — list panel, `MuroCard`, and tests.
- `web/src/features/gpx/RouteRecovery/` — list panel, `RecoveryCard`, and tests.

**Modified**
- `web/src/lib/api/types.ts` — add `GpxMuro`, `GpxRecoveryZone`, `GpxKmVertical` and the corresponding `StoredTrackDetail` fields.
- `web/src/features/gpx/normalize.ts` — add three forwarder normalizers and extend `normalizeTrackDetail`.
- `web/src/features/gpx/normalize.test.ts` — pin populated and empty array handling and nullable km vertical round trips.
- `web/src/features/gpx/RouteDetail/RouteDetail.tsx` — compose the three panels between climbs and risks.
- `web/src/features/gpx/RouteDetailContainer.test.tsx` — pin that the feature data comes from GET, not the narrow upload response.
- `web/src/lib/api/gpx.ts` — retain `uploadGpx(): Promise<{ id: string }>`; no widening to consume the upload payload.

## Test strategy

Tasks §“Per-Slice TDD Posture” requires RED → GREEN → REFACTOR for behavior changes. Mechanical work (SQLC regeneration review, interface ripples, formatting/lint/CI gates) is labeled mechanical and does not need a separate RED phase. Workspace `strict_tdd` is `false` in `openspec/config.yaml`; project convention still applies per behavior task.

- **Backend:** persistence and rehydration are covered through store/handler tests, including empty slices, nil singleton, ordering, upload forwarding, GET serialization, and migration up/down behavior. Gate with `make test` (`GOTOOLCHAIN=local go test -race ./...`); final gates are `make lint`, `make vet`, and `make fmt`.
- **Web:** test normalizers, populated/empty panel states, ordering, composition, and GET-not-upload data flow. Gate with `pnpm -C web test` (CI variant `pnpm -C web test:run`), `pnpm -C web typecheck`, `pnpm -C web lint`, and `pnpm -C web format:check`.
- The migration adds only new tables; task guidance notes ordinary in-memory tests and SQLC mocks do not require a database tier. CI migration smoke remains a separate verification gate.

## Rollback plan

### PR1 rollback
- Run `goose Down` for migration `00011`; it drops only the three new tables and loses no pre-existing data.
- Revert the PR1 commit to restore generated SQLC files, store/interfaces, tests, and the prior upload serialization implementation. No backfill or other-table restoration is needed.

### PR2 rollback
- Revert the web commit to remove the three panels, normalizers, types, composition, and tests. Backend persistence and wire fields remain available; there is no backend impact.

### Combined rollback
- Revert PR2 first, then PR1, so the UI is removed before the wire contract it consumes. Reverting both in reverse merge order returns to the pre-slice behavior.

## Out of scope (acknowledged, not addressed here)

- 3D MapLibre and slope heatmap (deferred to Fase 1.6).
- Extending comparator `computeDiff` for muros, km vertical, or recovery zones.
- Backfill/re-analysis of pre-existing tracks.
- Track-detail UX redesign or visible `elevation_coverage` UI.
- OAuth/Strava lifecycle changes.

## Risks

- `CreateDetail` signature changes ripple across handler/store mocks, including upload, GET/router, and compare test fixtures.
- SQLC regeneration touches generated files; reviewers should confirm additions are limited to the expected query/model/interface output.
- Existing tracks are not backfilled and will show empty muros/recovery panels and no km vertical result.
- A discrepancy exists between the requested outline's “breaking change” label and the proposal/spec contract: the proposal explicitly says direct detail serialization retains the same top-level keys and is byte-equivalent in shape. This design follows the proposal/spec; it does not treat those consumers as broken.
- Deferred 3D MapLibre, comparator extension, and backfill remain separate follow-up risks and are not solved by this slice.
