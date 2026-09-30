# Apply Progress: phase-1-3-km-vertical-and-king-climb

## Status

PR1 backend implementation is complete on branch `feat/phase-1.3-km-vertical-pr1-backend`. The parent-provided native `gentle-ai.sdd-status` v2 was consumed before implementation: `applyState: ready`, `nextRecommended: apply`, `artifactStore: openspec`, `actionContext.mode: repo-local`, workspace and allowed edit root `/home/composedof2/Dev/Codex/ghamusinos`. Only PR1 backend files were changed; no `web/**` files were edited. The prior attempt's blocker was resolved by adding mechanical stubs to every discovered Querier mock.

Previous progress recorded an initial partial implementation blocked by newly-added shared Querier methods. That blocker is resolved; earlier writes were retained and completed. The pre-existing unrelated untracked `.pi/` and `odd/tasks/*.md` files were left untouched and are not included in the commits.

## Completed PR1 tasks

Tasks 1.1–1.15 are checked in `tasks.md`:

- 1.1: added reversible migration and schema; **no co-located goose migration test pattern exists** (`internal/db/migrations/*_test.go` search returned none), so the optional test was skipped as explicitly permitted. Migration up/down was not exercised against a live database.
- 1.2–1.5: added all six queries, regenerated SQLC with `make generate`, and reviewed the diff. Changes to generated files are limited to `models.go`, `querier.go`, and the three new query output files; no existing generated query files changed.
- 1.6–1.9: extended the GPX interfaces, detail shape, transactional writes, and upload forwarding.
- 1.10–1.14: added GET rehydration, empty-list/nil-singleton handling, accessors, upload/GET tests, and regression coverage for the three new fields.
- 1.15: final local Go quality gates passed.

Task 1.16 remains unchecked: remote CI was not run because the branch was not pushed, as instructed. No push or PR was created.

## Verification evidence

- `make generate` — passed (SQLC 1.31.1 container).
- `GOTOOLCHAIN=local go build ./...` — passed after each compatibility-mock update.
- `GOTOOLCHAIN=local go test ./internal/gpx ./internal/http/handlers ./internal/db/...` — passed.
- `make fmt && make vet && make lint && make test` — passed with Go 1.26.4 on `PATH` and golangci-lint 2.12.2 installed per repository CI version. `make test` ran `go test -race ./...`; all packages passed. Lint reported `0 issues`.
- `git diff --check` — passed.
- No live migration up/down smoke or remote CI was run; integration/migration verification remains for the normal CI/verify path.

## Files changed

Created:
- `internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql`
- `internal/db/queries/gpx_km_vertical.sql`
- `internal/db/queries/gpx_muros.sql`
- `internal/db/queries/gpx_recovery_zones.sql`
- `internal/db/sqlc/gpx_km_vertical.sql.go`
- `internal/db/sqlc/gpx_muros.sql.go`
- `internal/db/sqlc/gpx_recovery_zones.sql.go`

Modified:
- `internal/db/schema.sql`
- `internal/db/sqlc/models.go`
- `internal/db/sqlc/querier.go`
- `internal/gpx/store.go`
- `internal/gpx/store_test.go`
- `internal/gpx/types.go`
- `internal/http/handlers/activities_test.go`
- `internal/http/handlers/gpx_get_test.go`
- `internal/http/handlers/gpx_upload.go`
- `internal/http/handlers/gpx_upload_test.go`
- `internal/http/handlers/me_preferences_test.go`
- `internal/http/handlers/me_test.go`
- `internal/http/router_test.go`
- `internal/auth/resolver_test.go`
- `openspec/changes/phase-1-3-km-vertical-and-king-climb/tasks.md`
- `openspec/changes/phase-1-3-km-vertical-and-king-climb/apply-progress.md`

## LoC and PR boundary

PR1 stays within the 400-line limit: 386 authored added lines (generated SQLC files and mechanical Querier stubs in unrelated tests excluded; 47 deleted lines are not counted as authored additions). No size exception is required. The PR1 implementation tip before this progress-file commit is `4b30e0ff2b84165c9b476b920fbe01754b4c3675`. No deviation from the backend specs/design was made. Migration test omission and missing remote migration/CI smoke are the only verification caveats.

## Commits

1. `9073ae8` — `chore(db): add GPX climb persistence tables`
2. `dac11a4` — `feat(gpx): persist climb-derived detail`
3. `802a122` — `feat(http): include climb data in GPX upload detail`
4. `754d675` — `test(http): cover GPX climb detail hydration`
5. `e006ce7` — `test(gpx): cover climb persistence and accessors`
6. `4b30e0f` — `chore(test-mocks): satisfy expanded SQLC querier`

## Remaining task

- `[ ]` 1.16. CI green on pushed branch (verification only, do not actually push).
- PR2 tasks 2.1–2.10 remain untouched and unchecked; they are outside this assigned PR1 slice.

## PR2 progress (Web UI)

Branch `feat/phase-1.3-km-vertical-pr2-web` carries the PR2 work.
Tasks 2.1–2.10 are checked in `tasks.md`. No new migration files;
PR2 only touches `web/**` per the PR1/PR2 split in the proposal.

### What changed in PR2

- 2.1: `web/src/lib/api/types.ts` — three new interfaces
  (`GpxMuro`, `GpxRecoveryZone`, `GpxKmVertical`) and three new
  fields on `StoredTrackDetail` (`muros`, `recovery_zones`,
  `km_vertical`). Fixtures in `ComparisonMode.test.tsx`,
  `RouteComparator.test.tsx`, `TrackDetailPage.test.tsx`,
  `RouteDetailContainer.test.tsx`, and `normalize.test.ts` updated
  to include empty values so the typecheck stays green.
- 2.2: `web/src/features/gpx/normalize.ts` — three new
  Normalized* interfaces, three normalizer helpers, and the
  `normalizeTrackDetail` extended to forward the three new
  fields. `km_vertical: null` is preserved (not mapped to `[]`).
  Tests in `normalize.test.ts` cover populated and null/empty
  round-trips.
- 2.3 / 2.4 / 2.5: `web/src/features/gpx/RouteKmVertical/`,
  `RouteMuros/`, `RouteRecovery/`. Each folder has a `.tsx`
  production component, a `.module.css` (RouteMuros + RouteRecovery
  each have an extra `MuroCard.tsx` / `RecoveryCard.tsx`), and a
  `.test.tsx` with vitest + testing-library. The empty-state
  testids are `route-km-vertical-empty`, `route-muros-empty`,
  `route-recovery-empty`; the populated root testids are
  `route-km-vertical`, `route-muros`, `route-recovery`.
- 2.6: `RouteDetail.tsx` composes the three new panels. RouteKmVertical
  sits between RouteMetrics and the columns block (singleton, full
  width); RouteMuros and RouteRecovery join RouteClimbs and RouteRisks
  in the columns block. Two new root testids added.
- 2.7: `RouteDetailContainer.test.tsx` — new pin test asserts the
  climb-derived data is read from `getGpxTrack` (GET), not from
  `uploadGpx` (POST). This documents the PR1/PR2 split: the SPA
  contract for upload is still `Promise<{ id: string }>`.
- 2.8: confirmed `uploadGpx` in `web/src/lib/api/gpx.ts` still
  returns `Promise<{ id: string }>`. PR1's wrapper removal did not
  change the SPA contract.
- 2.9: local gates (`pnpm typecheck`, `pnpm lint`, `pnpm
  format:check`, `pnpm build`) all green. `pnpm test:run` cannot
  be executed locally because of a pre-existing react@19.3.0 /
  react-dom@19.2.8 version mismatch in `web/package.json`; CI's
  frontend job (`typecheck + lint + format:check + build`) does
  not depend on vitest and is unaffected.

### Work-unit commits on PR2 branch

- `feat(web): extend StoredTrackDetail with muros/recovery_zones/km_vertical`
- `feat(web): extend normalizeTrackDetail with muros/recovery/km_vertical`
- `feat(web): RouteKmVertical panel for km_vertical singleton`
- `feat(web): RouteMuros panel + MuroCard subcomponent`
- `feat(web): RouteRecovery panel + RecoveryCard subcomponent`
- `feat(web): compose RouteKmVertical + RouteMuros + RouteRecovery in RouteDetail`
- `test(web): pin 'data arrives via GET, not upload' for climb-derived panels`

### Out of scope for PR2

- Backfill of pre-existing tracks (muros/recovery/km_vertical stay
  empty for tracks uploaded before PR1 #237). This is explicit in
  the proposal; per-track re-derivation happens on next re-upload.
- The pre-existing react/react-dom version mismatch is **not**
  addressed by PR2 (separate hygiene PR).
