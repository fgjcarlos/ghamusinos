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

## Known warnings and limitations

- **W1 — The list window is mandatory and browser-local.** `GET
  /api/v1/workouts` requires `from` and `to` (a missing one is a `400`),
  and the SPA computes them from the browser clock: from today to the
  same month and day of the following year. `users.timezone` is not
  consulted server-side, unlike `design.md` of slice 1, which reserved
  that resolution for the API edge. The stored value stays a bare
  `DATE`, so the day never shifts for a later reader.
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
