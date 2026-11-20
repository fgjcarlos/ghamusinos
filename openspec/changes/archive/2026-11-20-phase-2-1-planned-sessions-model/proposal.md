# Proposal: phase-2-1-planned-sessions-model

> Slice 1 of 3 for epic #301 (Fase 2.1 — Creacion de entrenos).
> Closes issue #298 ("modelo de sesiones planificadas — migracion y
> queries SQLC"). Sibling slices: #299 (CRUD HTTP), #300 (pantalla web).
>
> Base: `main@02e65d8`. Branch: `feat/phase-2.1-planned-sessions`.
> Project in scope: `.` (Go) only. Test command: `make test`.
> Integration command: `go test -tags integration -race -count=1
> ./internal/db/sqlc/...`.

## Intent

V2 has no planning model. `activities` stores what the athlete *did*;
nothing stores what they *planned*. Phase 2.1 is the first step of V2 and
every later phase (2.2 calendar, 2.3 race goals, 2.4 reports, 2.5 planning
AI) hangs off the same table.

This slice ships only the model: the table, the SQLC queries, and a domain
package. No HTTP, no UI. The deliverable is a persistence layer that later
slices can build on without re-deciding the schema.

## Scope

### In scope

- Migration `00016_planned_sessions.sql` (up/down) with the table
  `planned_sessions`, its CHECK constraints and its index.
- The same table mirrored into `internal/db/schema.sql` (the SQLC source of
  truth), plus `internal/db/queries/planned_sessions.sql` with five
  ownership-scoped queries, and the regenerated `internal/db/sqlc/` code.
- `internal/workouts`: domain types, domain validation, and a `Store` that
  is the only place `pgtype` appears. The HTTP handler of #299 will talk to
  `workouts.Session`, never to `pgtype.*`.
- Tests: a static contract test (migration / schema / queries / generated
  code must agree), an integration test against a live PostgreSQL
  (ownership isolation + constraint enforcement), and table tests for the
  domain validation.

### Out of scope

- HTTP endpoints, routing, RFC 9457 mapping: issue #299.
- The "Entrenos" screen, chips, form: issue #300.
- Session status (completed / skipped / modified) and the link to a real
  activity: phase 2.2.
- Race-goal association (`race_id`): phase 2.3, when `races` exists.
- `planned_sessions` in the API contract documentation: it ships with #299,
  which is where the contract becomes public.

## Capabilities

### New Capabilities
- `planned-sessions`: persistence of planned training sessions, with
  ownership isolation and domain validation.

### Modified Capabilities
- None.

## Approach

One table, five queries, one domain package. The table owns the invariants
that must hold regardless of who writes: ownership, the closed vocabulary
of workout types and intensities, and non-negative objectives. The domain
package owns the invariants that are nicer to explain in Spanish than in a
CHECK constraint: length bounds and required fields. Both layers are
needed: the CHECK constraints reject bad data even if a future caller
bypasses the domain package.

Ownership is enforced in SQL, not in Go. Every query filters by `user_id`,
so a session of another user simply does not exist for that caller. The
domain `Store` takes the caller's `user_id` and passes it to the query
unconditionally; it never decides *whether* to filter.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/db/migrations/00016_planned_sessions.sql` | New | The table, its constraints and its index. |
| `internal/db/schema.sql` | Modified | Mirror of the new table for SQLC. |
| `internal/db/queries/planned_sessions.sql` | New | Five ownership-scoped queries. |
| `internal/db/sqlc/` | Modified | Regenerated code + new `PlannedSession` model. |
| `internal/workouts/` | New | Domain types, validation, `Store`. |
| `internal/http/**`, `internal/auth/` | Modified | Test mocks only: the `sqlc.Querier` interface grows by five methods. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| The `sqlc.Querier` interface grows and breaks every full mock | High (certain) | Known and enumerated: five mock types in five files need five new stubs each. PR1 carries the fix so no commit is left red. |
| A `sqlc.yaml` type override on `workout_type` creates an import cycle | High | Verified: `internal/db/sqlc` -> `internal/workouts` -> `internal/db/sqlc`. The override is deliberately NOT added; the domain package converts the string itself. |
| Domain vocabulary drifts from the CHECK constraint | Medium | The static contract test reads both the migration and the Go constants and fails if they disagree. |
| `scheduled_on` lands on the wrong calendar day | Medium | The column is a `DATE` (no time, no offset): the user's local day is resolved at the API edge, and the DB stores a bare calendar date. Verified timezone-invariant. |

## Rollback Plan

`go run ./cmd/migrate --allow-destructive down` drops the table. The
sessions are planning data the user can re-enter, so losing them is
acceptable; there is no external system to reconcile. The migration is
additive: no existing table, column or query changes.

## Dependencies

- None. The slice touches only the Go project.
- `users.timezone` already exists (migration 00003) and is what #299 will
  read to resolve "today" in the user's local time.

## Success Criteria

- [ ] `make migrate` applies `00016` and `--allow-destructive down` reverts
      it, both verified against a real PostgreSQL.
- [ ] `make generate` produces no diff on a second run.
- [ ] The integration test proves user B cannot read, edit or delete a
      session of user A, and that the database rejects negative objectives
      and unknown types.
- [ ] The domain validation has table-driven tests.
- [ ] `make test`, `make lint` and `gofmt` are clean.
