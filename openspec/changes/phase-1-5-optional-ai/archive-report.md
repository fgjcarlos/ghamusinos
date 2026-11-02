# Archive Report: phase-1-5-optional-ai

> **Status: not archived yet.** This report describes the delivered
> change while the deltas still live under
> `openspec/changes/phase-1-5-optional-ai/`. The promotion of the three
> deltas into `openspec/specs/` is the final step of the epic and is
> performed by the parent; this unit deliberately does **not** touch
> `openspec/specs/`.

## Summary

Closed the epic **#17 — Fase 1.5 (IA opcional)**. The slice delivers
the *optional* interpretation of the training data:

- the provider-agnostic domain contract (`internal/ai`): an aggregate-only
  payload allowlist and one strict output schema shared by every
  provider;
- the three provider adapters in the ADR 0002 order (OpenAI → Claude →
  OpenRouter), without implicit failover;
- the global configuration (`AI_ENABLED`, `AI_PROVIDER`, `AI_MODEL`, one
  credential per provider, `AI_TIMEOUT`, `AI_MAX_RETRIES`) that never
  blocks the boot;
- the explicit per-user consent: migration `00014` resets every
  inherited `ai_enabled = true`, and the transfer notice is shown in the
  profile;
- the `ai_analysis` table (migration `00015`) with safe error columns;
- the River job `ai_analysis`, the only egress towards a provider, with
  a bounded retry budget and safe error codes;
- the HTTP surface (`GET /api/v1/ai/status`, `POST`/`GET
  /api/v1/ai/analyses`) and its TypeScript client;
- the web controls: a shared control, a per-activity trigger and the
  weekly and load analyses on the performance dashboard.

The criterion of the issue — *"Con IA desactivada o caída, el producto
funciona igual"* — holds end to end: with the flag off no credential is
read, no provider is contacted, the status endpoint answers `200`, the
request endpoint answers `503` without creating a row, and the web
screens behave exactly as before.

## Lineage

Branch (local only; nothing was pushed, no PR was opened, nothing was
merged):

- `feat/phase-1-5-optional-ai` — the integrating branch of the epic,
  based on `main@a6adff0`.

Base and tip:

| | Commit |
|---|---|
| Base | `main@a6adff0` (`docs(openspec): archive phase-1-4-performance-dashboard and promote its specs (#271)`) |
| Tip | `8265703` (`feat(web): on-demand AI analysis controls in the existing screens`) |

The nine work units were committed on that single branch, in dependency
order (no rebase, no amend, no force-push):

| # | Commit | Unit | Subject |
|---|---|---|---|
| 1 | `99db0a9` | T1 | `feat(ai): payload allowlist and shared strict output contract` |
| 2 | `49f8585` | T2 | `feat(ai): require fresh explicit consent for AI transfer` |
| 3 | `fc6c54f` | T4 | `feat(ai): global AI configuration and ai_analysis storage` |
| 4 | `8d823f9` | T3 | `feat(ai): provider adapters for OpenAI, Claude and OpenRouter` |
| 5 | `5cdd64c` | T5 | `feat(ai): River job for the on-demand AI analysis` |
| 6 | `b8d9360` | T6 | `feat(ai): wire the AI worker so River runs without Strava` |
| 7 | `67be97a` | T10 | `fix(ai): scale the payload ceilings with the analysed window` |
| 8 | `4de56a5` | T7 | `feat(api): endpoints to request and read the optional AI analyses` |
| 9 | `8265703` | T8 | `feat(web): on-demand AI analysis controls in the existing screens` |

## Changed lines per work unit

`git show --numstat` over each work-unit commit (added / deleted lines;
the *test* column counts the lines of the `*_test.go`, `*.test.ts` and
`*.test.tsx` files of the same commit).

| Unit | Commit | Files | Added | Deleted | of which test | Budget (400) |
|---|---|---|---|---|---|---|
| T1 — contract | `99db0a9` | 5 | 1 396 | 0 | 635 | ⚠ over (one cohesive unit: contract + validator + tests) |
| T2 — consent | `49f8585` | 11 | 746 | 2 | 529 | ⚠ over (migration + notice + Go tests + web tests) |
| T4 — config + storage | `fc6c54f` | 14 | 2 201 | 0 | 1 199 | ⚠ over (config + migration + sqlc + contract tests) |
| T3 — adapters | `8d823f9` | 10 | 2 466 | 12 | 1 391 | ⚠ over (interface + three adapters + wire tests) |
| T5 — job | `5cdd64c` | 11 | 2 362 | 14 | 1 115 | ⚠ over (job + scope keys + store queries + tests) |
| T6 — wiring | `b8d9360` | 3 | 149 | 15 | 72 | ✔ within |
| T10 — window ceilings | `67be97a` | 6 | 400 | 27 | 246 | ✔ within |
| T7 — API + TS client | `4de56a5` | 8 | 2 213 | 0 | 1 107 | ⚠ over (three handlers + router + client + tests) |
| T8 — web controls | `8265703` | 20 | 1 475 | 16 | 627 | ⚠ over (shared control + two mounts + tests) |
| **Total** | **9 commits** | **88 file changes** | **13 408** | **86** | **6 921** | — |

Reading of the budget: the forecast of the feature document was
2 500–4 000 authored lines and explicitly allowed that *"coherent slices
may exceed the advisory ~400-line budget"*. Five of the nine units are
above the advisory 400 lines; each of them is a single reviewable
coherent slice (one package, one migration, one endpoint family, one
screen family) and none mixes an unrelated concern. The two units that
fit the budget are the smallest ones (the wiring and the contract fix).

## Canonical specs

The three deltas of this change are ready to be promoted; the promotion
itself is the final step of the epic and belongs to the parent.

| Delta | Target canonical spec | Requirements |
|---|---|---|
| `specs/ai-analysis-backend/spec.md` | `openspec/specs/ai-analysis-backend/spec.md` | AB-001..AB-014 |
| `specs/ai-analysis-api/spec.md` | `openspec/specs/ai-analysis-api/spec.md` | AA-001..AA-008 |
| `specs/ai-analysis-web/spec.md` | `openspec/specs/ai-analysis-web/spec.md` | AW-001..AW-006 |

Coverage of the feature document (`odd/tasks/phase-1-5-optional-ai.md`,
S1..S8):

| Requirement of the feature document | Covered by |
|---|---|
| S1 — "Vamos con la issue #17": enrich the interpretation without making it mandatory | AB-012, AB-002, AA-003, AW-001 |
| S2 — multi-provider interface OpenAI → Claude → OpenRouter (ADR 0002) | AB-001, AB-004 |
| S3 — global flag + per-user opt-in, consent checked at enqueue and before sending | AB-005, AB-006, AB-008, AA-002, AA-003, AW-001 |
| S4 — analysis per activity / weekly / load, on demand only | AB-008, AB-011, AA-003, AW-002, AW-003 |
| S5 — payload builder + validated output schema identical across providers, aggregate metrics only | AB-002, AB-003, AB-004, AA-005, AW-005 |
| S6 — retries with backoff, execution in jobs, persistence in `ai_analysis` | AB-007, AB-009, AB-010, AA-004 |
| S7 — with the AI disabled or down the product works the same | AB-005, AB-012, AB-013, AW-001, AW-006 |
| S8 — chain over the branch of the epic, small units, progressive review | AB-014 (and the lineage table above) |

## Verification evidence

Recorded per work unit (the commands the parent ran when verifying each
unit, as logged in `odd/tasks/phase-1-5-optional-ai.md`):

| Unit | Command | Observed result |
|---|---|---|
| T1 | `TZ=UTC go test -count=1 ./internal/ai`, `go vet ./internal/ai` | ok, vet clean |
| T2 | `TZ=UTC go test ./internal/http/handlers ./internal/auth`; `pnpm -C web --config.verify-deps-before-run=false test:run src/features/profile src/lib/api/profile.test.ts` | 34 web tests pass, Go ok |
| T4 | `TZ=UTC go test -race -count=1 ./...` (full seam check after the `sqlc.Querier` ripple) | green |
| T5 | `TZ=UTC go test ./internal/jobs ./internal/ai ./internal/db/sqlc`, `go vet` | ok, vet clean |
| T6 | `go test ./internal/app ./internal/jobs`, `go vet` | ok (wiring covered by tests) |
| T10 | `go test ./internal/ai ./internal/jobs`, `go vet` | ok, vet clean |
| T7 | `TZ=UTC go test ./internal/http/... ./internal/app`, `go vet`; `pnpm -C web test:run src/lib/api/ai.test.ts` | Go ok, vet clean, web client tests pass |
| T8 | `pnpm -C web test:run src/features/ai src/features/activities src/features/profile`; `eslint` | 75 web tests pass, eslint clean |

Known environmental failures (not caused by this change):

- `pnpm -C web typecheck` **cannot run in this sandbox**: the
  `web/node_modules` tree lacks `echarts`, which phase 1.4 (`283faf6`)
  already added to `web/package.json`. The same command fails
  identically on the base commit, so the failure is pre-existing and
  unrelated to this slice. The rest of the web suite (vitest) runs.
- The provider adapters are exercised with `httptest` and a transport
  that rewrites the provider host to a local server: **no real provider
  request, no real credential and no cost** were ever issued. The wire
  details of the three providers were therefore **not** verified against
  the live official documentation in this environment; that remains a
  proof gap for the adapters (recorded in the feature log as L18).

## Outstanding warnings

- **W1 — RESOLVED: the web project type-checks and its whole suite
  passes once the declared dependencies are installed.** The gap was
  environmental, not a defect: `echarts` is declared in
  `web/package.json` but was absent from `web/node_modules` in this
  sandbox, so `pnpm -C web typecheck` and 5 dashboard test files failed
  identically on the base commit. After installing the locked
  dependencies (no change to `go.mod`, `go.sum`, `web/package.json` or
  `web/pnpm-lock.yaml`), the parent verified `pnpm -C web test:run`
  (59 files, 327 tests green), `pnpm -C web typecheck`, `pnpm -C web
  lint` and `pnpm -C web format:check`, all clean. **Severity: none
  (closed).**
- **W2 — the weekly window is pinned to 7 days.** The scope key of a
  weekly analysis (`weekly:<YYYY-MM-DD>`) does **not** encode the
  length of the window, so a weekly analysis with another length would
  collide with the same row. The API therefore rejects any
  `window_days` other than 7 for `kind = "weekly"` with a `422` instead
  of analysing a different window silently, and the job hardcodes the
  same 7 days. **Severity: low** (a deliberate constraint, documented
  in `design.md` D14), but it is a product limitation: the weekly
  analysis is not configurable in length.
- **W3 — the default window start is resolved in UTC.** When
  `window_start` is omitted, the API resolves it to *today in UTC*
  (`aiWindowStart`), while the job counts the days of the window in the
  timezone stored for the user (`users.timezone`). For a user whose
  local date is not the UTC date, the default window can start one day
  off; an explicit `window_start` is always interpreted in the user's
  local timezone. The web always sends an explicit `window_start`. The
  two conventions meet only for users whose local date equals the UTC
  date. **Severity: low**, but it is a real inconsistency worth closing
  in a follow-up.
- **W4 — a failed enqueue leaves a row in `pending`.** The API creates
  the `ai_analysis` row **before** it inserts the job in River. If that
  insert fails, the endpoint answers `503` and the row stays `pending`
  forever (it is not marked `failed`), and it stays visible in the
  listing as a pending analysis until the same request is repeated —
  the `ON CONFLICT` then resets it to `pending` and enqueues it again.
  **Severity: low** (no data is lost and no analysis is analysed
  twice), but the user can see a "pending" analysis that no job will
  ever pick up. A follow-up could mark the row `failed` with a safe
  code when the insert fails.
- **W5 — the task row of T3 in the feature document is stale.** The
  row of T3 in `odd/tasks/phase-1-5-optional-ai.md` still says
  `pending` / `commit: none`, but the provider adapters **were**
  delivered, in `8d823f9`. The parent should update that row so the
  feature document matches the delivered state.
- **W6 — the wire details of the providers are not verified against the
  live documentation.** The adapters were written against the contract
  recorded in `docs/architecture/ai-contract.md` (endpoints, headers,
  `json_schema` for OpenAI/OpenRouter, forced tool for Claude) and are
  covered by contract tests with fake transports, but no live call was
  made. Before the first real deployment the three adapters SHALL be
  checked against the current official documentation of each provider.

## Outcome

Every deliverable of issue #17 shipped: the multi-provider interface,
the global flag plus the per-user consent, the three on-demand
analyses, the payload builder with a validated output schema identical
across providers, and the retries with backoff executed in jobs with
persistence in `ai_analysis`. The criterion of the issue — the product
works the same with the AI disabled or down — is enforced by
construction at the configuration, the API, the job and the web layer.

The three deltas of this change describe the **delivered** behaviour
and are ready to be promoted to `openspec/specs/` in the final archive
step of the epic.
