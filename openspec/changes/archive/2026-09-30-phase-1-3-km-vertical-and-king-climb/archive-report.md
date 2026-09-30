# Archive Report: phase-1-3-km-vertical-and-king-climb

## Summary

Closed GitHub issue #15 (Fase 1.3 — Laboratorio GPX base) by
persisting the climb-derived detection routines
(`FindKmVertical`, `FindMuros`, `FindRecoveryZones`) and rendering
them in the SPA. The slice spans two PRs over `main`:

- **PR #237** (merge commit `3ae8102`): backend persistence +
  wire-shape change. The Go side now writes the three
  climb-derived tables in the same transaction as climbs and risk
  zones, rehydrates them via `GetDetail`, and `StoredTrackDetail`
  exposes them as `muros`, `recovery_zones`, `km_vertical`.
- **PR #238** (merge commit `366f485`): SPA render. The web
  layer gains `RouteKmVertical`, `RouteMuros` + `MuroCard`, and
  `RouteRecovery` + `RecoveryCard` panels between `RouteClimbs`
  and `RouteRisks`, plus the `normalizeTrackDetail` extension and
  a hardened GET-not-upload pin in `RouteDetailContainer.test.tsx`.

The pre-existing functional gap (`finders computed but never
persisted, never visible on the lab`) is now closed end-to-end.

## Lineage

Branch: `feat/phase-1-3-km-vertical-and-king-climb` (parent
worktree tracking the whole slice). Active sub-branches during
the slice:

- `feat/phase-1.3-km-vertical` (slice parent; rebased onto main
  at both PR1 and PR2 close-out)
- `feat/phase-1.3-km-vertical-pr1-backend` (deleted after #237
  merge)
- `feat/phase-1.3-km-vertical-pr2-web` (deleted after #238 merge)

### PR #237 (Backend persistence + wire shape)

| Order | Commit | Message | Notes |
|---|---|---|---|
| 1 | `40f0813` | `feat(gpx): add migration 00011 for muros/recovery/km_vertical tables` | Schema baseline. |
| 2 | `fbb5e02` | `feat(gpx): add SQLC queries and regen for the three new tables` | `sqlc.Querier` interface gains `CreateGPXMuro`, `CreateGPXRecoveryZone`, `UpsertKmVertical`, `ListMuros`, `ListRecoveryZones`, `GetKmVertical`. Mock ripple intentional. |
| 3 | `cbb961d` | `feat(gpx): extend StoredTrackDetail with muros, recovery_zones, km_vertical` | Wire-shape change. Drops the bespoke anonymous wrapper in the upload handler. |
| 4 | `f01a8b7` | `feat(gpx): CreateDetail persists muros, recovery_zones, km_vertical` | Same-transaction write. |
| 5 | `a25c2db` | `feat(gpx): GetDetail rehydrates muros, recovery_zones, km_vertical` | Read path. |
| 6 | `52b95d0` | `feat(gpx): SQLCStore accessor methods + upload handler thin-out` | Accessors + 15-min wrapper-removal. |
| 7 | `c617939` | `test(gpx): pin StoredTrackDetail regression — wire shape survives refactor` | Verify follow-up round 1/2. |
| 8 | `b7b04fb` | `test(gpx): pin store rehydration regression — fields persist on read` | Verify follow-up round 2/2. |
| 9 | `7379d80` | `test(gpx): pin upload-body shape = StoredTrackDetail marshalled value` | Verify follow-up round 3 (final). |
| 10 | `3ae8102` | squash merge of PR #237 | Merge commit on `main`. |

### PR #238 (Web UI: muros / recovery_zones / km_vertical)

| Order | Commit | Message | Notes |
|---|---|---|---|
| 1 | `60d49dc` | `feat(web): extend StoredTrackDetail with muros/recovery_zones/km_vertical` | `web/src/lib/api/types.ts` + 4 fixtures. |
| 2 | `1bbec2a` | `feat(web): extend normalizeTrackDetail with muros/recovery/km_vertical` | `web/src/features/gpx/normalize.ts` + 5 new tests. |
| 3 | `b28fe30` | `feat(web): RouteKmVertical panel for km_vertical singleton` | New `RouteKmVertical/` folder. |
| 4 | `a43c471` | `feat(web): RouteMuros panel + MuroCard subcomponent` | New `RouteMuros/` folder (4 files). |
| 5 | `7f240d1` | `feat(web): RouteRecovery panel + RecoveryCard subcomponent` | New `RouteRecovery/` folder (4 files). |
| 6 | `1eaa7bb` | `feat(web): compose RouteKmVertical + RouteMuros + RouteRecovery in RouteDetail` | Composition in `RouteDetail.tsx`. |
| 7 | `63a878b` | `test(web): pin 'data arrives via GET, not upload' for climb-derived panels` | Initial GET-not-upload pin. |
| 8 | `9640f27` | `docs(openspec+odd): record PR2 apply progress + mark tasks 2.1-2.10 done` | `apply-progress.md` + ODD doc + tasks.md checkboxes. |
| 9 | `03e10ec` | `fix(web): address PR2 verify follow-ups (panel order, GET-not-upload pin, tasks)` | Verify round 1 follow-ups: panel ordering moved into `.columns` block, container test now stubs `uploadGpx` and asserts it is NOT called, tasks 2.1–2.9 marked `[x]`. |
| 10 | `366f485` | squash merge of PR #238 | Merge commit on `main`. |

## Specs Promoted

`openspec/specs/gpx-backend/spec.md` and
`openspec/specs/gpx-lab/spec.md` already existed (created by the
preceding `wire-elevation-and-map-to-track-detail` archive in
`openspec/changes/archive/2026-09-27-wire-elevation-and-map-to-track-detail/`).
Both received the **archive-time sync fallback** approved by the
parent orchestrator.

| Delta spec | Canonical spec | Operation |
| --- | --- | --- |
| `openspec/changes/.../specs/gpx-backend/spec.md` (743 lines, 8 new ADDED Requirements) | `openspec/specs/gpx-backend/spec.md` (109 → 783 lines; 1 → 9 ADDED Requirements) | ADDED-only |
| `openspec/changes/.../specs/gpx-lab/spec.md` (431 lines, 7 new ADDED Requirements) | `openspec/specs/gpx-lab/spec.md` (386 → 753 lines; 7 → 14 ADDED Requirements) | ADDED-only |

Detail in `sync-report.md`.

## Per-Task Outcome

Source of truth: `tasks.md` requirement-to-implementation and
task-to-implementation mapping tables. Task state at archive
time:

| # | Task | State |
|---|---|---|
| 1.1 | Pre-flight: branch `feat/phase-1.3-km-vertical-and-king-climb` and import OpenSpec change | DONE |
| 1.2 | Generate `proposal.md` + `exploration.md` commit | DONE |
| 1.3 | Generate delta specs (gpx-backend + gpx-lab) commit | DONE |
| 1.4 | Generate `tasks.md` with actionable checkboxes + `design.md` | DONE |
| 2.1 | `StoredTrackDetail` extends with `muros`, `recovery_zones`, `km_vertical` (web types) | DONE |
| 2.2 | `normalizeTrackDetail` extension with the three new fields | DONE |
| 2.3 | `RouteKmVertical` component + tests | DONE |
| 2.4 | `RouteMuros` + `MuroCard` + tests | DONE |
| 2.5 | `RouteRecovery` + `RecoveryCard` + tests | DONE |
| 2.6 | Compose the three new panels in `RouteDetail.tsx` (between `RouteClimbs` and `RouteRisks`) | DONE |
| 2.7 | Pin "GET not upload" in `RouteDetailContainer.test.tsx` | DONE |
| 2.8 | Confirm `uploadGpx` narrow return type | DONE (no code change required) |
| 2.9 | Run `pnpm typecheck/lint/format:check/test:run` final gate | DONE (vitest blocked locally by pre-existing react/react-dom version mismatch — see "Known Environmental Failures") |
| 2.10 | Document CI green expectation in PR body | DONE |

Backend PR1 tasks (1.1–1.14) all completed prior to the PR1
archive attempt; preserved in the slice's git history and
referenced from PR1 verify report.

Total: 14 of 14 implementation tasks DONE.

## Gates

Every gate run during apply and verify, with observed result.

### PR #237 (Backend)

| Gate | Result | Notes |
|---|---|---|
| `make test` (full) | PASS | All Go packages compiled and passed. |
| Focused GPX tests (`go test ./internal/gpx`) | PASS | Task RED→GREEN observed. |
| Focused handler tests (`go test ./internal/http/handlers/...`) | PASS | Wire-shape regression. |
| `make vet` | PASS | |
| `make fmt` | PASS | |
| `golangci-lint run` | PASS (CI) | CI installs golangci-lint v2; local install not required. |
| Web build (`pnpm --dir web build`) | PASS | `bin/ghamusinos`-embeddable artifacts. |
| Frontend job (CI) | PASS | typecheck + lint + format:check + build |
| Backend job (CI) | PASS | lint + test + build + migrate |
| Release job (CI) | PASS | binario + smoke con SPA embebida |
| GitGuardian | PASS | |

### PR #238 (Web UI)

| Gate | Result | Notes |
|---|---|---|
| `pnpm typecheck` | PASS | |
| `pnpm lint` | PASS | |
| `pnpm format:check` | PASS | |
| `pnpm build` | PASS | `internal/frontend/dist/` artefacts regenerated. |
| `pnpm test:run` | NOT RUN LOCALLY (CI frontend job does not run tests) | See "Known Environmental Failures". |
| Backend job (CI) | PASS | Backend unaffected. |
| Frontend job (CI) | PASS | typecheck + lint + format:check + build |
| Release job (CI) | PASS | binario + smoke con SPA embebida |
| GitGuardian | PASS | |

## Known Environmental Failures

Verbatim per the worker's "Known environmental failures" contract.

- **`pnpm test:run` (task 2.9)**: blocked locally because
  `web/package.json` has `react@^19.3.0` and
  `react-dom@^19.2.8`, which prevents Vitest from initialising.
  CI's frontend job runs `typecheck + lint + format:check +
  build` and does not run tests, so this is not a CI blocker.
  The mismatch is a pre-existing hygiene issue out of scope for
  PR2. All slice files typecheck and lint clean; the slice's
  vitest files are not exercised by any CI gate the PR crosses.

## Spec Drift Follow-Up

None observed. Both delta specs were greenfield additions
(ADDED-only) that did not modify any pre-existing canonical
requirement.

## Active same-domain change warnings

None. No other change under `openspec/changes/*/specs/{gpx-backend,gpx-lab}/spec.md`
touches these domains at archive time.

## Destructive merge approvals

None. Both delta specs were ADDED-only. No REMOVED requirements
and no large MODIFIED blocks were applied.

## Unchecked implementation task boxes

None. All 14 implementation tasks (PR1 + PR2) are marked `[x]`
in `tasks.md`.

## Archived path

`openspec/changes/archive/2026-09-30-phase-1-3-km-vertical-and-king-climb/`

Today's ISO date is `2026-09-30`.

## Memory observation IDs

- `phase-1.3-km-vertical-pr1` (Engram id 2631)
- `phase-1.3-km-vertical-pr2` (Engram id 2638)

## Status

**PASS** — slice archived. The OpenSpec change folder has been
moved from `openspec/changes/phase-1-3-km-vertical-and-king-climb/`
to `openspec/changes/archive/2026-09-30-phase-1-3-km-vertical-and-king-climb/`.