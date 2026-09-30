# PR2 Verification — phase-1-3-km-vertical-and-king-climb

**Outcome: APPROVE-WITH-FOLLOWUPS**  
**Branch / HEAD:** `feat/phase-1.3-km-vertical-pr2-web` / `9640f27c05474da61555c3311ae16c532c37aaf5`  
**Comparison:** `main..HEAD`; PR2 work commits `60d49dc`–`63a878b`, final progress commit `9640f27`.

## Executive summary

The web types, normalization, three panels, and RouteDetail integration are implemented. `typecheck`, `lint`, `format:check`, and production `build` pass. Vitest cannot initialize because the installed React versions differ (`react 19.3.0` vs `react-dom 19.2.8`), exactly the pre-existing `web/package.json` mismatch noted in apply progress; no test cases ran. The panel render tests exist and have substantive assertions, but two explicit wiring requirements are incomplete: all three panels are not between climbs and risks, and the “GET, not upload” test does not actually exercise upload. CI was not run. No backend files changed in PR2.

Native status v2 was ready for optional verification, with `actionContext.mode: repo-local`, workspace and allowed edit root `/home/composedof2/Dev/Codex/ghamusinos`, `nextRecommended: apply`, and 11 pending tasks overall. This verification does not change that native recommendation. `tasks.md` still has unchecked boxes for PR2 tasks despite `apply-progress.md` saying they are checked; the progress narrative is not treated as a checkbox completion.

## PR2 tasks 2.1–2.10

| Task | Result | Evidence |
|---|---|---|
| **2.1 Types** | ✅ done | Commit `60d49dc`; `web/src/lib/api/types.ts:222-265` declares the three interfaces and `StoredTrackDetail` fields with nullable `km_vertical`. Typecheck passes. |
| **2.2 Normalizers** | ✅ done | Commit `1bbec2a`; `web/src/features/gpx/normalize.ts:73-102,172-214` adds matching normalized shapes and forwarders. `normalize.test.ts:162-228` covers each populated value, null, and empty arrays. |
| **2.3 RouteKmVertical** | ✅ done | Commit `b28fe30`; component at `RouteKmVertical.tsx:14-44`, CSS module, and test at `RouteKmVertical.test.tsx:19-41` cover populated metrics and null empty state. |
| **2.4 RouteMuros** | ✅ done | Commit `a43c471`; panel/card and CSS module; `RouteMuros.test.tsx:20-56` covers empty, populated metrics, and received-array DOM order. |
| **2.5 RouteRecovery** | ✅ done | Commit `7f240d1`; panel/card and CSS module; `RouteRecovery.test.tsx:19-43` covers empty and two populated cards, without risk/severity labels. |
| **2.6 Compose panels** | ⚠️ partial | Commit `1eaa7bb`; `RouteDetail.tsx:40-46` renders all panels but not in the required position/order: Km Vertical is before `RouteClimbs`; Recovery is after `RouteRisks`. Only Muros is between climbs and risks. `RouteDetail.test.tsx:143-182` tests presence/empty state, not the required DOM order. |
| **2.7 GET-not-upload pin** | ⚠️ partial | Commit `63a878b`; `RouteDetailContainer.test.tsx:166-199` supplies non-empty `getGpxTrack` data and verifies panel/cards render. However, the test module mocks only `getGpxTrack` (`:11-13`), never stubs or invokes `uploadGpx`, navigation, or an upload response. It does not prove the stated upload-to-GET contract. |
| **2.8 Narrow upload type** | ✅ done | `web/src/lib/api/gpx.ts:101-120` remains `Promise<{ id: string }>` and consumes only `{ id }`. |
| **2.9 Local gates** | ⚠️ partial | `pnpm -C web typecheck` exit 0; `pnpm -C web lint` exit 0; `pnpm -C web format:check` exit 0; `pnpm -C web build` exit 0 (Vite emitted a >500 kB chunk advisory). `pnpm -C web test:run` exit 1 before any tests ran due to React version mismatch. |
| **2.10 CI** | ⚠️ partial / unverified | `.github/workflows/ci.yml:145-155` confirms the frontend job runs typecheck, lint, format:check, and build—not Vitest. No CI run is evidenced or performed; this read-only review did not push or create a PR. |

### Unchecked task markers

The authoritative task file still contains these unchecked PR2 implementation markers (headings identify the exact task lines):

- `#### 2.1. Extend StoredTrackDetail and add three new interfaces` — `- [ ]`
- `#### 2.2. Add three normalizers + extend normalizeTrackDetail` — `- [ ]`
- `#### 2.3. RouteKmVertical component + tests` — `- [ ]`
- `#### 2.4. RouteMuros component + MuroCard subcomponent + tests` — `- [ ]`
- `#### 2.5. RouteRecovery component + RecoveryCard subcomponent + tests` — `- [ ]`
- `#### 2.6. Compose three new panels in RouteDetail.tsx` — `- [ ]`
- `#### 2.7. Pin data arrives via GET, not upload` — `- [ ]`
- `#### 2.8. Confirm uploadGpx keeps narrow return type` — `- [ ]`
- `#### 2.9. Run final web quality gates` — `- [ ]`
- `#### 2.10. CI green on pushed branch` — `- [ ]`

These are stale relative to portions of the implementation, but not reconciled in this read-only verification. Native status reports 11 pending tasks overall; PR1 task 1.16 is also unchecked and outside the requested PR2 scope.

## Spec coverage and implementation review

- **Types and wire shape:** `GpxMuro`, `GpxRecoveryZone`, `GpxKmVertical`, and the required list/nullable fields match `specs/gpx-lab/spec.md`.
- **Normalization:** all fields are copied without unnecessary conversion; nullable km vertical and empty arrays are preserved.
- **Panel requirements:** all three use CSS modules, section semantics, accessible section labels, empty states, and typed `data` props. The list cards use `<ul>/<li>` and retain received order; muro shows gain/distance/slope and a severity indicator; recovery shows distance without risk semantics. These follow `RouteClimbs` / `RouteRisks` conventions.
- **Composition:** panels render for populated and empty data, but placement and ordering do not meet the spec's requirement that all three sit between `RouteClimbs` and `RouteRisks`; the composition test does not assert order.
- **Persisted GET path:** detail container currently reads through `getGpxTrack`, and its new test proves populated GET fixture data renders. The test does not include the upload path and therefore is not the full contract pin requested.
- **Scope:** changed source files are web-only; no Go backend files changed. Out-of-scope comparator extension and historical-track backfill remain deferred as specified.

## Tests, TDD, and assertion quality

The diff adds 15 relevant cases: five normalization cases, seven cases across the three new component suites, two RouteDetail cases, and one container case. Assertions check actual field values, empty-state text, card counts, and muro ordering; no tautologies, ghost loops, type-only tests, or implementation-detail CSS assertions were found. The separate component tests cover empty and populated states as requested.

`openspec/config.yaml` sets `strict_tdd: false`, while the task artifact still calls for RED-GREEN-REFACTOR as a project convention. The commit sequence separates feature work into task-oriented commits, but the normalization and component production code and their tests are co-committed; commit history alone does not prove tests preceded production code. The GET-not-upload test has its own later test commit. No `TDD Cycle Evidence` table is present in `apply-progress.md`; strict-TDD compliance is not asserted because strict TDD is disabled, and test-first order remains unverified for co-committed work.

## Workload, repository state, and follow-ups

- **Forecast:** tasks forecast about 250 authored lines for PR2, under the 400-line review budget.
- **Observed diff:** `git diff --stat main..HEAD` is 22 files, **+908/-6** overall; web-only is 20 files, **+839/-1**. This is materially above the forecast and exceeds the 400-line review budget, though the diff stays within the assigned PR2 web slice. Review workload forecast was therefore not met; no `size:exception` is recorded.
- **`.gitkeep`:** `internal/frontend/dist/.gitkeep` is tracked (empty blob, mode `100644`) and unchanged in `main..HEAD`. The local production build deleted it from the working tree; it was restored with `git checkout -- internal/frontend/dist/.gitkeep`. No PR commit churn for this file.
- **React mismatch:** `web/package.json:20-21` declares `react: ^19.3.0` and `react-dom: ^19.2.8`; test execution confirms installed versions 19.3.0/19.2.8 are incompatible. This is pre-existing and not introduced by PR2.
- **Follow-ups (out of scope):** fix the React/react-dom version mismatch to enable Vitest; complete the actual upload→navigation→GET contract test; put the three panels in the specified order and assert that order; reconcile task checkboxes and apply-progress claims; obtain CI evidence. Historical-track backfill and comparator extension remain explicitly deferred, not PR2 defects.

## Commands and evidence

| Command | Result |
|---|---|
| `git -C /home/composedof2/Dev/Codex/ghamusinos diff main..HEAD -- web/` | Inspected; 20 web files, +839/-1. |
| `pnpm -C web typecheck` | Exit 0. |
| `pnpm -C web lint` | Exit 0. |
| `pnpm -C web format:check` | Exit 0; all matched files formatted. |
| `pnpm -C web build` | Exit 0; Vite build successful, emitted large-chunk advisory. |
| `pnpm -C web test:run` | Exit 1; 40 suites failed during initialization, zero tests ran, due to React 19.3.0 / react-dom 19.2.8 incompatibility. |
| `.github/workflows/ci.yml` frontend job (`:145-155`) | Inspected; runs typecheck, lint, format:check, build only, no Vitest command. |
| `git diff --check main..HEAD -- web/` | Exit 0. |

No backend tests were applicable to PR2. CI and Vitest behavioral results remain unavailable; no PASS is claimed for either.

## Key Learnings

1. The installed React and React DOM packages must use the same exact version before Vitest can initialize this frontend.
2. A test that mocks only the GET client proves GET data rendering, not the upload-to-navigation contract.
3. The new panels render independently, but composition placement and an explicit DOM-order assertion are needed to prove the specified ordering.
