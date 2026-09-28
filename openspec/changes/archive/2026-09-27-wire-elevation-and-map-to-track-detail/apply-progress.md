# Apply Progress: Wire elevation and map to track detail

## Structured status consumed

Consumed the parent-provided native `gentle-ai.sdd-status` v2 projection for `wire-elevation-and-map-to-track-detail`: `applyState: ready`, `dependencies.apply: ready`, `nextRecommended: apply`, `actionContext.mode: repo-local`, workspace `/home/composedof2/Dev/Codex/ghamusinos`, and allowed edit root equal to that workspace. No native blockers or action-context warnings were reported. The status projection initially reflected 0/14 checked tasks; the saved checkbox state below is the state after this corrective resume. No local readiness inference replaced native status.

## Workload / PR boundary

The task forecast is `Decision needed before apply: No`, `Chained PRs recommended: No`, `Chain strategy: single-pr`, and `400-line budget risk: Low`. This is one cohesive PR slice; no exception or chaining decision was needed.

## Completed tasks and checkbox updates

The following persisted checkboxes in `tasks.md` are `[x]` after their gates passed:

- [x] **1.1 Propagate `Analysis.ElevationCoverage` in `storedTrack`** — resumed implementation from the previous dispatch; the assertion pins non-null `0.87` and NULL. Earlier focused RED→GREEN evidence is preserved from prior progress. Full Go gate passed in this dispatch.
- [x] **1.2 Verify integration tests are not silently broken by the change** — full `make test` passed; integration-tagged DB tests were not run, as the task states they are excluded from the default target.
- [x] **2.1 Implement `rawPointsToTrackPoints` haversine helper (with lon/lng tolerance)** — nine scenarios added; focused test first failed because the helper module was missing, then passed after implementation.
- [x] **2.2 Update `RouteComparator.isFlatPoint` to accept `lon` as an alias for `lng`** — test first failed to produce comparator map coordinates for Go `lon`; normalized accepted points and passed the web suite.
- [x] **2.3 Update `TrackDetailPage` defensive filter to accept `lon` as an alias for `lng`** — lon-key map regression test first failed, then passed the web suite.
- [x] **2.4 Consume real points in `RouteDetail.tsx` and remove stale TODO** — real profile path regression test first failed, then passed. The converter skips malformed null point entries defensively.
- [x] **2.5 Replace `as unknown as` cast in `ComparisonElevationProfile.tsx` and remove stale TODO** — regression test first failed on an empty projected path, then passed with real converted points.
- [x] **2.6 Wire `TrackDetailPage` into `RouteDetailContainer` ready branch** — ready map/detail and insufficient-points empty-map tests passed.
- [x] **2.7 Add `elevation_coverage` to `GpxAnalysis` interface** — added nullable type and propagated it through normalization, including absent-to-null normalization. Test and typecheck passed.
- [x] **2.8 Delete `web/src/routes/lab-detail.tsx`** — file deleted, reference grep returned no matches, typecheck passed.
- [x] **2.9 Run `pnpm -C web typecheck` and `pnpm -C web test:run` final gate** — both passed.
- [x] **3.1 Run `make test` final gate** — passed.

The remaining persisted unchecked tasks are exactly:

- [ ] **2.10 Quality gates (lint, format)**
- [ ] **3.2 Run `make lint`, `make vet`, `make fmt`**

These remain unchecked because the repository-wide web format check detects seven pre-existing formatting violations in unrelated `web/src/features/profile/*` files and Go lint could not start (`golangci-lint: not found`). Web lint, Go vet, and Go format passed. The full formatter was run once as required, then only its unrelated profile-file changes were restored to avoid scope drift; all changed slice files independently pass Prettier check.

## Files changed for this slice

- `internal/gpx/store.go` — propagate `ElevationCoverage` from SQLC row.
- `internal/gpx/store_test.go` — test valid and NULL coverage propagation.
- `web/src/features/gpx/rawPointsToTrackPoints.ts` — new pure Haversine converter with `lon` fallback and `lng` precedence.
- `web/src/features/gpx/rawPointsToTrackPoints.test.ts` — nine conversion scenarios.
- `web/src/features/lab/RouteComparator/RouteComparator.tsx` — accept/normalize Go `lon` coordinates.
- `web/src/features/lab/RouteComparator/RouteComparator.test.tsx` — comparator wire-shape regression.
- `web/src/features/lab/TrackDetailPage/TrackDetailPage.tsx` — accept either longitude key for map coordinates.
- `web/src/features/lab/TrackDetailPage/TrackDetailPage.test.tsx` — lon-key map regression.
- `web/src/features/gpx/RouteDetail/RouteDetail.tsx` — feed converted real points into profile.
- `web/src/features/gpx/RouteDetail/RouteDetail.test.tsx` — non-degenerate profile regression.
- `web/src/features/lab/ComparisonElevationProfile/ComparisonElevationProfile.tsx` — convert raw points and remove unsafe cast/stale TODO.
- `web/src/features/lab/ComparisonElevationProfile/ComparisonElevationProfile.test.tsx` — populated comparator profiles regression.
- `web/src/features/gpx/RouteDetailContainer.tsx` — render `TrackDetailPage` when ready.
- `web/src/features/gpx/RouteDetailContainer.test.tsx` — ready map and empty-map regressions.
- `web/src/lib/api/types.ts` — declare nullable `elevation_coverage`.
- `web/src/features/gpx/normalize.ts` — normalize coverage to nullable stable shape.
- `web/src/features/gpx/normalize.test.ts` — coverage and null regression.
- `web/src/features/gpx/RouteHeader/RouteHeader.test.tsx`, `web/src/features/gpx/RouteMetrics/RouteMetrics.test.tsx`, `web/src/features/lab/ComparisonMode/ComparisonMode.test.tsx` — update fixtures for the required analysis field.
- `web/src/routes/lab-detail.tsx` — deleted dead placeholder.
- `openspec/changes/wire-elevation-and-map-to-track-detail/tasks.md` — mark only completed task checkboxes.
- `openspec/changes/wire-elevation-and-map-to-track-detail/apply-progress.md` — cumulative apply record.

`internal/frontend/dist/.gitkeep` remains deleted as pre-existing unrelated work and is excluded from this slice.

## Test commands and gate evidence

- Task 1.1 / task 1.2 and final Go unit gate: `PATH="/home/composedof2/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.13.linux-amd64/bin:$PATH" make test` — PASS; all packages compiled and passed under `go test -race ./...`. `PATH="/usr/local/go/bin:$PATH" make test` first failed because that path provides Go 1.22.0, below the module's Go 1.25.13 minimum; the cached Go 1.25.13 binary was used thereafter.
- Task 2.1 RED: `pnpm -C web test -- rawPointsToTrackPoints` — expected failure because the imported helper module did not exist (one suite failed; 33 suites passed). Initial GREEN attempt exposed that the spec's `0.008983°` equatorial longitude delta computes to approximately `998.864 m`, incompatible with the stated ±0.5 m around 1000 m. The test input was corrected to `0.0089932°`, which represents 1 km for `R=6371e3`; focused run then passed all 9 cases.
- Task 2.2 RED/GREEN: `pnpm -C web test:run` — failed before normalization (empty coordinates), then passed.
- Task 2.3 RED/GREEN: `pnpm -C web test:run` — failed for `lon`-key map input, then passed.
- Task 2.4 RED/GREEN: `pnpm -C web test:run` — real-profile assertion failed before wiring, then passed after conversion.
- Task 2.5 RED/GREEN: `pnpm -C web test:run` — projection path assertion failed before conversion, then passed.
- Task 2.6 RED/GREEN: `pnpm -C web test:run` — ready page assertions failed before container wiring, then passed.
- Tasks 2.7 and 2.9 final web tests: `pnpm -C web test:run` — PASS, 36 test files / 204 tests; `pnpm -C web typecheck` — PASS.
- Task 2.8: `grep -RIn "lab-detail" web/src/ --exclude-dir=node_modules` — no output; `pnpm -C web typecheck` — PASS.
- Tasks 2.10 / end-of-slice: `pnpm -C web lint` — PASS. `pnpm -C web format:check` — FAIL due seven existing unrelated profile files. `pnpm -C web exec prettier --check` over all changed slice TS/TSX files — PASS.
- Task 3.1 / end-of-slice Go tests: `PATH="/home/composedof2/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.13.linux-amd64/bin:$PATH" make test` — PASS.
- Task 3.2 / end-of-slice Go quality: `make vet` — PASS; `make fmt` — PASS; `make lint` — FAIL to start because `golangci-lint` is not installed.

## Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `pnpm -C web test -- rawPointsToTrackPoints` — RED on missing module, then PASS with all 9 scenarios. Full `pnpm -C web test:run` — PASS, 36 files / 204 tests. Go `make test` — PASS across all packages. |
| Runtime harness command/scenario | N/A — this slice changes the existing browser page composition and pure conversion logic; there is no separate runtime integration harness in the task matrix. Component tests exercise ready map, empty-map, and profile rendering paths. |
| Rollback boundary | Revert this slice's two Go store files, listed web implementation/type/normalization files and tests, dead placeholder deletion, and the two OpenSpec apply artifacts; do not include the pre-existing `internal/frontend/dist/.gitkeep` deletion. |

## Deviations from design / risks

- The exact equatorial coordinate in the spec (`0.008983°`) does not satisfy its stated 1 km ±0.5 m assertion using the mandated Earth radius. The test uses `0.0089932°` instead; the spec should be reconciled in a later authorized spec edit.
- Full web format and Go lint gates are not green for environment/baseline reasons described above. No unrelated profile formatting was retained, and Go lint configuration was not altered.
- No Go JSON longitude tag, SQLC output, migrations, config, Makefile, CI configuration, or spec/design/proposal/exploration files were modified. No Engram writes, pushes, or PRs were performed.
