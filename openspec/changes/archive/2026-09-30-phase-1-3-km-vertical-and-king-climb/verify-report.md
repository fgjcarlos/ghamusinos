# Verify Report: phase-1-3-km-vertical-and-king-climb

Consolidated verify report for the 2-PR chain. Both rounds
returned APPROVE-WITH-FOLLOWUPS; all actionable follow-ups were
closed before the corresponding PR was merged.

---

# PR1 — Backend persistence + wire shape

## Branch
- Branch: `feat/phase-1.3-km-vertical-pr1-backend`
- Base: `feat/phase-1.3-km-vertical`
- Final commit SHA: `1dc325cc49fb5993cf2a0adbabe80020bec1cc81`
- Authored LoC (Go, excluding regen + test mocks): **120 production Go added lines** (gross additions; 18 production Go lines deleted).
- Generated SQLC LoC: **247 added lines** (three new `.sql.go` files, `models.go`, `querier.go`).
- Mechanical mock stubs LoC: **90 added lines** (five unrelated Querier mock files, 18 each).
- PR1 backend footprint: no `web/**` changes. The generated SQLC changes are limited to the three new query files plus `models.go` and `querier.go`.

## Outcome
**APPROVE-WITH-FOLLOWUPS** (verify round 3 of 3)

## Requirements coverage (specs/gpx-backend/spec.md ADDED)

| Req ID | Title | Implementation file | Test file | Scenarios passing | Notes |
|---|---|---|---|---|---|
| 1 | Migration creates three new climb-derived tables | `internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql`; `internal/db/schema.sql` | None for live migration behavior | Static schema inspection only; **0 migration scenarios executed against PostgreSQL** | Up/Down and schema definitions match; no backfill SQL. No live up/down/re-up smoke, despite apply-progress noting this remains unrun. |
| 2 | SQLC queries for the three new tables | `internal/db/queries/gpx_{muros,recovery_zones,km_vertical}.sql`; generated SQLC files | `internal/gpx/store_test.go` | Store accessor/order mock tests pass; no database-level order/upsert/unique-constraint test | Query text has both `ORDER BY start_idx` clauses and the per-track upsert. Actual SQL execution and uniqueness failure scenario remain unverified. |
| 3 | `StoredTrackDetail` carries the three top-level fields | `internal/gpx/types.go` | `internal/gpx/store_test.go`; `internal/http/handlers/gpx_get_test.go` | Empty-array and nil singleton JSON shape passes; GET populated fields pass | Struct tags and position are correct. Serialization test does not separately assert populated field-for-field serialization. |
| 4 | `CreateDetail` persists the new fields transactionally | `internal/gpx/store.go`; handler/store interfaces | `internal/gpx/store_test.go` | Nonempty call counts/selected parameters and empty/nil no-op tests pass | Writes execute inside the pre-existing transaction callback. No test injects a failure in a new-table write to prove rollback of the whole transaction. |
| 5 | `GetDetail` rehydrates all three fields | `internal/gpx/store.go` | `internal/gpx/store_test.go` | Populated rehydration and nonnil empty lists/nil km vertical tests pass | Lists map SQLC rows; accessors normalize no-row km vertical to nil. Full package race tests pass. |
| 6 | `SQLCStore` exposes the three accessors | `internal/gpx/store.go`, `internal/gpx/types.go` | `internal/gpx/store_test.go` | Muros/recovery order passthrough and missing km vertical nil result pass | Implementation delegates to SQLC methods. |
| 7 | Upload handler serializes `StoredTrackDetail` directly | `internal/http/handlers/gpx_upload.go` | `internal/http/handlers/gpx_upload_test.go` | Upload test passes and confirms top-level keys are present | Handler now calls `escribirJSON(w, detail)`. Current test's new captured-field assertions only exercise empty/nil values, not nonempty detector outputs. Wire keys remain top-level as specified by proposal/design. |
| 8 | Tests pin wire shape, persistence, and rehydration | `internal/gpx/store_test.go`; handler test files | `internal/gpx/store_test.go`; `internal/http/handlers/gpx_get_test.go`; `internal/http/handlers/gpx_upload_test.go` | New GET test and store tests pass | **Required `databaseTrack` regression guard is absent**: `databaseTrack` remains unchanged and no adjacent assertions pin nonempty `Muros`, `RecoveryZones`, or `KmVertical`. Apply-progress claims this task complete, contrary to the diff. |

## Task completion (tasks.md 1.1–1.16)

All 14 PR1 implementation tasks completed in tasks.md before PR1
verify round 3. The verify report surfaced the gap in task 1.14
(databaseTrack regression pin) — closed in commits `c617939`,
`b7b04fb`, `7379d80` before PR1 was opened.

## Follow-ups closed before merge

| ID | Severity | Resolution |
|---|---|---|
| V1.R1 | HIGH | Hardened `databaseTrack` regression pin (`c617939`): explicit assertions on `Muros`/`RecoveryZones`/`KmVertical` against the populated fixture |
| V1.R2 | MEDIUM | Hardened read-side rehydration (`b7b04fb`): explicit `databaseTrack` rehydration assertions |
| V1.R3 | LOW | Hardened upload-body shape pin (`7379d80`): upload test asserts `detail` shape equals GET shape |

---

# PR2 — Web UI: muros / recovery_zones / km_vertical

## Branch
- Branch: `feat/phase-1.3-km-vertical-pr2-web`
- Base: `main`
- Final commit SHA: `03e10ec7a...` (verify round 1 follow-up commit)
- Comparison: `main..HEAD`; PR2 work commits `60d49dc`–`63a878b`, progress commit `9640f27`, follow-up `03e10ec`.

## Outcome
**APPROVE-WITH-FOLLOWUPS** (verify round 1 of 1)

## Executive summary

The web types, normalization, three panels, and RouteDetail
integration are implemented. `typecheck`, `lint`, `format:check`,
and production `build` pass. The panel render tests exist and
have substantive assertions; two wiring requirements were flagged
in verify round 1 and were closed in `03e10ec` before the PR was
opened:

- Panel ordering: `RouteKmVertical` moved inside the `.columns`
  block between `RouteClimbs` and `RouteRisks` (final layout:
  `RouteClimbs → RouteKmVertical → RouteMuros → RouteRisks →
  RouteRecovery`).
- GET-not-upload pin: `RouteDetailContainer.test.tsx` now mocks
  `uploadGpx` as well as `getGpxTrack`, asserts `uploadGpx` is
  NOT called during the read flow, and asserts `getGpxTrack` is
  called with the trackId.
- Tasks checkboxes: `tasks.md` rows 2.1–2.9 flipped to `[x]`,
  row 2.10 left `[ ]` until CI evidence confirmed.

Vitest cannot run locally because of a pre-existing
`react@^19.3.0` / `react-dom@^19.2.8` version mismatch in
`web/package.json`. CI's frontend job does not run tests; only
typecheck, lint, format:check, and build.

## PR2 tasks 2.1–2.10

| Task | Result | Evidence |
|---|---|---|
| **2.1 Types** | ✅ done | Commit `60d49dc`; `web/src/lib/api/types.ts:222-265` declares the three interfaces and `StoredTrackDetail` fields with nullable `km_vertical`. Typecheck passes. |
| **2.2 Normalizers** | ✅ done | Commit `1bbec2a`; `web/src/features/gpx/normalize.ts:73-102,172-214` adds matching normalized shapes and forwarders. `normalize.test.ts:162-228` covers each populated value, null, and empty arrays. |
| **2.3 RouteKmVertical** | ✅ done | Commit `b28fe30`; component at `RouteKmVertical.tsx:14-44`, CSS module, and test at `RouteKmVertical.test.tsx:19-41` cover populated metrics and null empty state. |
| **2.4 RouteMuros** | ✅ done | Commit `a43c471`; panel/card and CSS module; `RouteMuros.test.tsx:20-56` covers empty, populated metrics, and received-array DOM order. |
| **2.5 RouteRecovery** | ✅ done | Commit `7f240d1`; panel/card and CSS module; `RouteRecovery.test.tsx:19-43` covers empty and two populated cards, without risk/severity labels. |
| **2.6 Compose panels** | ⚠️ → ✅ after `03e10ec` | Commit `1eaa7bb`; `RouteDetail.tsx:40-46` renders all panels but not in the required position/order. **Closed in `03e10ec`**: `RouteKmVertical` moved inside `.columns` between `RouteClimbs` and `RouteMuros`; final layout `RouteClimbs → RouteKmVertical → RouteMuros → RouteRisks → RouteRecovery`. |
| **2.7 GET-not-upload pin** | ⚠️ → ✅ after `03e10ec` | Commit `63a878b`; `RouteDetailContainer.test.tsx:166-199` supplied non-empty `getGpxTrack` data and verified panel/cards render. **Closed in `03e10ec`**: the test now also mocks `uploadGpx`, asserts `mockedUploadGpx.not.toHaveBeenCalled()`, and asserts `mockedGetGpxTrack.toHaveBeenCalledWith('tok', 'track-1', expect.anything())`. |
| **2.8 Narrow upload type** | ✅ done | `web/src/lib/api/gpx.ts:101-120` remains `Promise<{ id: string }>` and consumes only `{ id }`. |
| **2.9 Local gates** | ✅ done | `pnpm -C web typecheck` exit 0; `pnpm -C web lint` exit 0; `pnpm -C web format:check` exit 0; `pnpm -C web build` exit 0 (Vite emitted a >500 kB chunk advisory). `pnpm -C web test:run` blocked locally by React version mismatch — out of slice scope. |
| **2.10 Document CI green expectation** | ✅ done | PR body documented in `/tmp/pr2-body.md`; merged into PR #238 description. CI confirmed green: Backend, Frontend, Release, GitGuardian. |

## CI evidence (post-merge)

| Check | Result |
|---|---|
| Backend (lint + test + build + migrate) | ✅ PASS |
| Frontend (build) | ✅ PASS |
| Release (binario + smoke con SPA embebida) | ✅ PASS |
| GitGuardian Security Checks | ✅ PASS |

PR #238 merged to `main` as commit `366f485` via
`--squash --delete-branch`.