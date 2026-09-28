# Verification Report: wire-elevation-and-map-to-track-detail

**Status: READY for archive, with recorded non-blocking environment limitations and two unchecked quality-gate tasks.** Verification was requested explicitly although native status recommends `apply`; native v2 was left unchanged and authorizes optional verification. The native projection identifies this change, `artifactStore: openspec`, `dependencies.verify: ready`, `actionContext.mode: repo-local`, workspace and allowed edit root `/home/composedof2/Dev/Codex/ghamusinos`, no blockers, and `nextRecommended: apply`. Its task projection reports 12/14 complete. This report does not rederive or override that recommendation.

## Executive summary

- **gpx-lab requirements:** map/detail ready-state composition and unchanged non-ready branches are implemented; the container tests cover map presence and empty-map behavior. Single-track and comparison profiles convert raw points to cumulative-distance points and assert non-degenerate/NaN-free output. Converter scenarios cover empty/single/duplicate points, equatorial distance, Madrid reference, null elevation, malformed coordinates, and Go `lon`/TS `lng` interoperability. Coverage typing/normalization, stale TODO removal, and dead placeholder deletion are present.
- **gpx-backend requirement:** `storedTrack` copies `row.ElevationCoverage` with `numericPointer`; the store regression test asserts non-null `0.87` and NULL propagation. Shared conversion is used by the store read paths.
- **Task state:** 12/14 checkboxes are complete. The exact remaining tasks are `- [ ] **2.10 Quality gates (lint, format)**` and `- [ ] **3.2 Run `make lint`, `make vet`, `make fmt`**`. Vet/format succeeded and web lint succeeded; full web formatting and Go lint are limited by documented baseline/environment issues. No checkbox was changed.
- **TDD evidence:** `apply-progress.md` records RED→GREEN narratives for backend task 1.1 and frontend task 2.1, plus red/green regression evidence for web wiring tasks. Current focused/full tests confirm GREEN. Workspace strict TDD is false; the slice-specific RED/GREEN posture is documented and observed in the apply narrative.
- **Go gates:** `make test` passes with the cached Go 1.25.13 toolchain. The default PATH invocation initially failed because `/usr/local/go/bin/go` is 1.22.0; this was resolved using the cached toolchain. `make vet && make fmt` passes with that toolchain. `make lint` cannot start because `golangci-lint` is absent; this is the documented environment limitation (CI installs v2).
- **Web gates:** `pnpm -C web test:run` passed (36 files, 204 tests); `pnpm -C web typecheck` and `pnpm -C web lint` passed. `pnpm -C web format:check` reported seven existing unrelated files in `web/src/features/profile/*`; a Prettier check over the slice's changed/added TS/TSX files passed.
- **Size and boundary:** `git show 02e6c41 --stat | tail -1` reports 21 files, 366 insertions, 69 deletions (435 total changed lines). `tasks.md` explicitly records the parent-approved `size:exception`; one single-PR slice was implemented, consistent with `Chain strategy: single-pr`. No scope creep found. Commit `a5ad7a2` is a parent corrective auth/http mock stub, not part of the slice diff under review.
- **Spec drift / archive:** the spec coordinate `0.008983°` computes to `998.864 m`, outside ±0.5 m of 1000 m; the test's `0.0089932°` computes to `999.998 m`, within tolerance. The test value is valid; reconcile the spec coordinate in a later authorized change as recorded in tasks. Both delta specs exist and have well-formed OpenSpec `ADDED Requirements` sections; the supplied convention confirms archive promotes domain deltas to `openspec/specs/{domain}/spec.md`. No archive readiness override is made.

## Requirement-to-implementation mapping

| Delta requirement | Evidence in `02e6c41` / validation | Result |
|---|---|---|
| gpx-lab: MapLibre above route detail, ready state only | `RouteDetailContainer.tsx` renders `TrackDetailPage`; `TrackDetailPage` composes map then `RouteDetail`. Container tests assert ready map and empty map for one point; full web suite passes. Non-ready state tests remain. | Satisfied |
| gpx-lab: single-track profile from real points | `RouteDetail.tsx` calls `rawPointsToTrackPoints(data.track.points)` before `projectTrack`; component test asserts multi-segment path without NaN; empty data is handled. | Satisfied |
| gpx-lab: comparison profiles without NaN | Comparison component converts points instead of unsafe `as unknown as`; regression test checks populated paths and NaN absence. | Satisfied |
| gpx-lab: haversine converter contract | Pure helper uses radius `6371e3`, cumulative atan2 haversine, millimetre rounding, preserves null elevation, drops invalid coordinates, supports `lon` fallback and `lng` precedence. Nine tests cover cases; full suite passes. | Satisfied |
| gpx-lab: `GpxAnalysis.elevation_coverage` | API type declares `number | null`; normalization preserves values and maps absent to null; tests and typecheck pass. | Satisfied |
| gpx-lab: remove stale missing-points TODOs | Exact source searches produced no matching stale comments. | Satisfied |
| gpx-lab: delete placeholder and retain live route | `web/src/routes/lab-detail.tsx` is absent; source reference grep has no output; typecheck passes. Router/container implementation remains wired. | Satisfied |
| gpx-backend: propagate nullable `ElevationCoverage` | `storedTrack` uses `numericPointer(row.ElevationCoverage)`; store test checks `0.87` and nil; Go test suite passes. | Satisfied |

The shared `storedTrack` conversion is the implementation point for `GetByID`, `GetDetail`, `FindByHash`, and `List`; no separate path-specific mapping was introduced. No out-of-scope endpoint, migration, SQLC generation, or Go wire-key change appears in the slice commit.

## Task-to-implementation mapping

- **1.1** complete: store propagation and valid/NULL regression test are in the slice.
- **1.2** complete: apply record confirms default tests; integration-tagged DB tests are not part of this invocation and were not claimed as run.
- **2.1** complete: helper and nine converter scenarios exist; apply-progress records initial missing-module RED and GREEN.
- **2.2–2.3** complete: `RouteComparator` and `TrackDetailPage` tolerate Go `lon`; tests cover both boundaries.
- **2.4–2.6** complete: real points feed the route/comparison projections; ready page renders `TrackDetailPage`; targeted regression tests exist.
- **2.7** complete: type and normalization expose `elevation_coverage`; normalization tests included.
- **2.8** complete: placeholder deleted; grep and typecheck confirm no dependency remains.
- **2.9** complete: final typecheck and web unit suite passed in this verification.
- **2.10** unchecked: web lint passes, but full format check fails on the seven pre-existing profile files listed below. Changed slice TS/TSX files pass focused Prettier check.
- **3.1** complete: Go `make test` passed with Go 1.25.13.
- **3.2** unchecked: `make vet` and `make fmt` pass with Go 1.25.13; `make lint` is unavailable because `golangci-lint` is not installed.

## TDD, assertion quality, and workload

`openspec/config.yaml` sets workspace `strict_tdd: false`. Slice TDD is nevertheless explicitly required by its proposal/design/tasks and evidenced in `apply-progress.md`: backend coverage test before implementation and converter test before the helper, each with RED then GREEN noted. Additional wiring regressions have corresponding tests. Assertions check computed values, null handling, rendered paths/DOM, or finite values; no tautological or type-only-only test was identified. Current tests pass. The Madrid reference fixture is hard-coded and compares each cumulative result within 1 m.

The tasks forecast single PR / `single-pr`. Although implementation exceeded 400 changed lines, this was explicitly accepted by the parent/user as `size:exception` and recorded in `tasks.md`; measured stats match 21 files / +366 / -69. The slice is bounded to the planned backend and web surface, with no identified scope creep.

## Commands and observed results

- `git show 02e6c41 --stat` — success; diff lists the implementation files and reports 21 files, 366 insertions, 69 deletions.
- `git show 02e6c41 --stat | tail -1` — `21 files changed, 366 insertions(+), 69 deletions(-)`.
- `make test` — **initial invocation failed (exit 2)** because PATH selected Go 1.22.0 while `go.mod` requires Go >=1.25.13.
- `PATH="/home/composedof2/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.13.linux-amd64/bin:$PATH" make test` — **PASS (exit 0)**, all Go packages passed `go test -race ./...`.
- `pnpm -C web test:run` — **PASS (exit 0)**, 36 test files and 204 tests.
- `pnpm -C web typecheck` — **PASS (exit 0)**.
- `pnpm -C web lint` — **PASS (exit 0)**.
- `pnpm -C web format:check` — **FAIL (exit 1), informational baseline**: seven existing files (`ConnectionCard.test.tsx`, `ConnectionCard.tsx`, `HRZonePreview.test.tsx`, `HRZonePreview.tsx`, `PreferencesContainer.tsx`, `PreferencesForm.test.tsx`, `PreferencesForm.tsx`) under `web/src/features/profile/`.
- `pnpm -C web exec prettier --check src/features/gpx/RouteDetail/RouteDetail.tsx src/features/gpx/RouteDetail/RouteDetail.test.tsx src/features/gpx/RouteDetailContainer.tsx src/features/gpx/RouteDetailContainer.test.tsx src/features/gpx/normalize.ts src/features/gpx/normalize.test.ts src/features/gpx/rawPointsToTrackPoints.ts src/features/gpx/rawPointsToTrackPoints.test.ts src/features/lab/ComparisonElevationProfile/ComparisonElevationProfile.tsx src/features/lab/ComparisonElevationProfile/ComparisonElevationProfile.test.tsx src/features/lab/RouteComparator/RouteComparator.tsx src/features/lab/RouteComparator/RouteComparator.test.tsx src/features/lab/TrackDetailPage/TrackDetailPage.tsx src/features/lab/TrackDetailPage/TrackDetailPage.test.tsx src/lib/api/types.ts` — **PASS (exit 0)**.
- `make lint` — **FAIL to start (exit 2), informational environment limitation**: `/bin/sh: 1: golangci-lint: not found`. The recorded CI gate installs golangci-lint v2 independently.
- `make vet && make fmt` — **initial invocation failed (exit 2)** because PATH selected Go 1.22.0.
- `PATH="/home/composedof2/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.13.linux-amd64/bin:$PATH" make vet && PATH="/home/composedof2/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.13.linux-amd64/bin:$PATH" make fmt` — **PASS (exit 0)**; both `go vet ./...` and `go fmt ./...` completed.
- `grep -RIn 'backend doesn.t yet return the raw points\|the backend currently doesn.t return' web/src/` — no matching stale TODOs.
- `grep -RIn 'lab-detail' web/src/ --exclude-dir=node_modules` — no references; `test ! -e web/src/routes/lab-detail.tsx` passed.
- `test -f .../specs/gpx-lab/spec.md && test -f .../specs/gpx-backend/spec.md` — both delta specs exist.
- `git status --short` after formatter commands shows the pre-existing `D internal/frontend/dist/.gitkeep`, untracked `.pi/`, `odd/`, and `openspec/`; no source-code modifications from verification. Only `verify-report.md` is authored by this phase.

## Findings and risks

1. The two unchecked task markers remain verbatim and reflect incomplete repository-wide quality gates, not unfinished feature implementation. The failed format gate is attributed to unrelated pre-existing profile files; the missing Go linter is an environment limitation. These are the exact known environmental exceptions authorized for this verification.
2. Spec coordinate follow-up: `6371000 * radians(0.008983) = 998.864026 m`, while the test's `0.0089932°` yields `999.998214 m`. Thus the spec coordinate is inconsistent with ±0.5 m, but the test coordinate is within tolerance. `apply-progress.md` incorrectly says the test coordinate is ~1009.998 m/outside tolerance; this report corrects that arithmetic without modifying the historical apply artifact. Reconcile the spec coordinate in a future authorized spec change; not a blocker to this implementation.
3. Integration-tagged database tests and runtime browser E2E are not part of the requested commands and were not run; the default Go test gate passed and web component tests exercise the changed states.
4. Archive promotion is supported by the OpenSpec file convention: the two domain delta specs can be promoted to their matching canonical paths. Archive remains governed by fresh native status and permissions.

**Exact blockers:** none under the explicitly accepted environment-gate policy. Unchecked tasks and informational failures are preserved above; no report/task/spec readiness has been fabricated.

## Key Learnings

1. Go wire longitude uses `lon`, so browser point boundaries must normalize it before map and profile rendering.
2. The equatorial longitude delta `0.008983°` is about 998.864 metres with a 6371-kilometre Earth radius.
3. The corrected test delta `0.0089932°` computes to approximately 999.998 metres and meets the half-metre tolerance.
4. The repository formatter baseline currently flags seven unrelated profile files while changed slice files pass focused formatting.
