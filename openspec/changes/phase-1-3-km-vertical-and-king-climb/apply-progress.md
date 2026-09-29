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
