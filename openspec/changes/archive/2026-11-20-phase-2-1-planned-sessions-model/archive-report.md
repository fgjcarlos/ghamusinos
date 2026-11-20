# Archive Report: phase-2-1-planned-sessions-model

> **Status: archived on 2026-11-20.** The `PS-001`..`PS-005` requirements
> were promoted to `openspec/specs/planned-sessions-backend/spec.md`; the
> HTTP contract of slice 2 (#299) and the SPA screen of slice 3 (#300)
> were written directly as `openspec/specs/planned-sessions-api/spec.md`
> and `openspec/specs/planned-sessions-web/spec.md`. This change now lives
> at `openspec/changes/archive/2026-11-20-phase-2-1-planned-sessions-model/`.

## Delivered

Phase 2.1 closes GitHub epic #301 ("Fase 2.1 — Creacion de entrenos").
The epic is the first phase of V2: V1 analyses what the athlete did, V2
plans what the athlete will do. The slice shipped the model, the
authenticated CRUD and the "Entrenos" screen of the SPA.

| Slice | Issue | Unit | Commit |
| --- | --- | --- | --- |
| 1 | #298 | Migration `00016_planned_sessions`, SQLC queries, `internal/workouts` domain | `d82080f` |
| 2 | #299 | Authenticated CRUD `/api/v1/workouts` with RFC 9457 errors and pagination | `7da655e` |
| 3 | #300 | `Entrenos` screen in the AppShell navigation | `80b25b9` |

Contract documentation: `docs/api/workouts.md`.

## Spec promotion

| Delta | Canonical spec | Requirements |
| --- | --- | --- |
| `specs/planned-sessions/spec.md` | `openspec/specs/planned-sessions-backend/spec.md` | PS-001..PS-005 |
| (no delta: slice 2) | `openspec/specs/planned-sessions-api/spec.md` | PA-001..PA-007 |
| (no delta: slice 3) | `openspec/specs/planned-sessions-web/spec.md` | PW-001..PW-005 |

The `planned-sessions-backend` canonical file is the promotion of the
archived delta, unchanged in its requirements. The API and web canonical
specs are new: slices #299 and #300 shipped without an openspec change
folder of their own, so their requirements were written from the
delivered code and the published contract.

## Verification

Run on 2026-11-20 against `main@108adef`:

- `go test -race ./...` → green, every package ok.
- `go vet ./...` → clean.
- `pnpm -C web test:run` → 76 files / 469 tests passed.
- `golangci-lint` → **unavailable** in this environment (the binary is
  not installed); no lint verdict is claimed.
- No live third-party request in any test: the Go side uses `httptest`
  and sqlc mocks, and the web client stubs `fetch`.

## Review rounds

The native review (readability lens) ran over the branch and approved it in
every round. Every finding was advisory: none opened a correction, and
none reopens a review.

| Round | Findings | Applied |
| --- | --- | --- |
| 1 | leap-day rule undefined; slice metric unexplained; the placeholder token called the user token; the title and note limits in bytes on the web and in characters on the backend; contradictory owner of the calendar day | leap-day clamp in `PW-002`; the metric named in the ODD document; the token described as the placeholder it is; the limits in UTF-8 bytes in both specs; the day resolved by the caller |
| 2 | the slice metric was still hard to audit | the tool that counts the slice, the comparison range and the metric are named |
| 3 | the elevation target described as positive but validated as non-negative | the elevation target is the climb, never negative, and zero is valid |
| 4 | the slice count is net growth, not added lines; the owner of the signed 32-bit range check unclear | the metric counts additions minus deletions; the store truncates to `int4` and the caller rejects out-of-range targets |
| 5 | contradictory owner of the day resolution; the leap-day rule promised here but not stated; the range check owner still unclear; the form window undefined | the API stores the civil date verbatim; the clamp rule is stated in `W1`; the store truncates and the caller rejects; the form uses the window of `PW-002` |

## Known warnings and limitations

- **W1 — The list window is mandatory and browser-local.** `GET
  /api/v1/workouts` requires `from` and `to` (a missing one is a `400`),
  and the SPA computes them from the browser clock: from today to the
  same month and day of the following year, clamped to the last day of
  that month when the day does not exist in it (29 February becomes
  28 February). The API stores the submitted civil date verbatim and
  never reads `users.timezone`, so the resolution of the day belongs to
  the caller; server-side resolution from `users.timezone` is deferred to
  phase 2.2. The stored value stays a bare `DATE`, so the day never
  shifts for a later reader.
- **W2 — The association to an objective is a set of columns.** The
  session carries the duration, distance, elevation and intensity
  targets; the race relation (`race_id`) is deferred to phase 2.3,
  when `races` exists.
- **W3 — No session status and no link to a real activity.** Both belong
  to phase 2.2.
- **W4 — The SPA authenticates with a placeholder token**
  (`VITE_AUTH_TOKEN`). Real Clerk authentication in the frontend is
  issue #306, a recommended predecessor of the epic.
- **W5 — Only slice 1 had an openspec change folder.** Slices #299 and
  #300 shipped without one; their canonical specs were written from
  the delivered code and the published contract.
- **W6 — The lint gate did not run.** `golangci-lint` is not installed
  in this environment.

## Follow-ups

1. Resolve the user's calendar day server-side from `users.timezone`,
   replacing the browser-local window (phase 2.2).
2. Add `race_id` and the association to a race objective (phase 2.3).
3. Add the session status and the link to a real activity (phase 2.2).
4. Replace the placeholder token with the real Clerk session (#306).
5. Fix the wording of the domain error message: it says `title exceeds
   200 characters` while the limit counts UTF-8 bytes.
