# Tasks: phase-2-1-planned-sessions-model

> Slice 1 of 3 for epic #301. Closes issue #298.
> Base: `main@02e65d8`. Branch: `feat/phase-2.1-planned-sessions`.
> Project in scope: `.` (Go) only.
> Gating test command: `make test`.
> Integration command: `go test -tags integration -race -count=1
> ./internal/db/sqlc/...` (needs a PostgreSQL with the migrations applied:
> `make db-up && make migrate`).
> Quality gates: `make lint`, `make vet`, `gofmt -l .` (empty).
>
> Delivery: a **3-PR chain on one branch**, each PR green on its own.
> The chain is driven by the ~1 375 authored lines, which exceed the
> 400-line review budget in `openspec/config.yaml`. The split is by
> concern, not by convenience, and each PR has been verified in isolation
> to build, vet, lint and test clean.
>
> The five `sqlc.Querier` mock stubs ride in **PR1** (the PR that breaks
> them). No commit in the chain is allowed to be red.

## PR1 — Persistence layer (migration, schema, queries, generated code)

~640 lines (including the generated `planned_sessions.sql.go`).

- [ ] **1.1** — Migration `00016_planned_sessions.sql`
  - New file `internal/db/migrations/00016_planned_sessions.sql`.
  - `CREATE TABLE planned_sessions` with the columns, the `CHECK`
    constraints and the index of `design.md`; goose `Up` and `Down`
    sections, `Down` drops the table.
  - Comment style follows `00015_ai_analysis.sql` (Spanish prose, why
    each constraint exists).
  - Verify: `make migrate` applies it; `go run ./cmd/migrate
    --allow-destructive down` reverts it; `make migrate` re-applies it.
- [ ] **1.2** — Mirror the table into `internal/db/schema.sql`
  - The SQLC schema is the source of truth for the generated code, so the
    table has to exist in both places. Same DDL, same constraints.
- [ ] **1.3** — `internal/db/queries/planned_sessions.sql`
  - The five queries of `design.md`, each with the ownership filter and a
    comment that says so.
  - `ListPlannedSessionsByRange` uses `sqlc.arg('from_date')` /
    `sqlc.arg('to_date')` (positional `$2`/`$3` on two `DATE` columns
    fails SQLC type inference).
  - `DeletePlannedSession` is `:execrows`, not `:exec`.
- [ ] **1.4** — `make generate` and commit the generated code
  - `internal/db/sqlc/planned_sessions.sql.go` (new),
  - `internal/db/sqlc/models.go` (adds `PlannedSession`),
  - `internal/db/sqlc/querier.go` (the interface grows by five methods).
  - Verify: run `make generate` a second time, `git diff` must be empty.
- [ ] **1.5** — Fix the five `sqlc.Querier` mocks
  - Add the five stubs to `mockQuerier` (`internal/http/router_test.go`),
    `activitiesMockQuerier` (`internal/http/handlers/activities_test.go`),
    `mockMeQuerier` (`internal/http/handlers/me_test.go`),
    `preferencesMockQuerier`
    (`internal/http/handlers/me_preferences_test.go`) and `mockQuerier`
    (`internal/auth/resolver_test.go`).
  - The stubs return zero values, exactly like the existing ones.
  - Verify: `go vet ./...` is clean (this is what reports the missing
    methods).
- [ ] **1.6** — Static contract test
  - New file `internal/db/sqlc/planned_sessions_contract_test.go`,
    following `ai_analysis_contract_test.go`: reads the migration and
    `schema.sql` from disk, asserts the table, the columns, the cascade
    FK, the type and intensity vocabularies, the non-negative
    constraints, the index, the `Down` section, and that the query file
    declares the five expected query names.
  - No database needed: it runs in the normal `make test`.

**PR1 is done when:** `make test`, `make lint` and `gofmt -l .` are clean,
`make generate` is idempotent, and the migration applies, reverts and
re-applies against a real PostgreSQL.

## PR2 — Domain package (`internal/workouts`)

~707 lines.

- [ ] **2.1** — Domain types (`internal/workouts/types.go`)
  - `Session` with plain Go types: `uuid.UUID`, `time.Time`, `string`,
    `*int`, `*Intensity`. JSON tags, no `pgtype`.
  - `WorkoutType` and `Intensity` string types with their constants.
- [ ] **2.2** — Domain validation (`internal/workouts/validation.go`)
  - `ErrValidation` sentinel; `Validate(Session) error` returning an error
    that wraps it and names the offending field.
  - Rules: title required and <= 200 chars, notes <= 2000 chars, known
    workout type, known intensity, non-negative objectives, non-zero
    date.
- [ ] **2.3** — `Store` (`internal/workouts/store.go`)
  - The five-method `Querier` interface (design D5) and `NewStore`.
  - `Create`, `Get`, `ListRange`, `Update`, `Delete`. Each takes the
    caller's `user_id` and forwards it (design D4).
  - `Get`/`Update` map `pgx.ErrNoRows` to `ErrNotFound`; `Delete` maps a
    zero row count to `ErrNotFound`.
  - The `pgtype` <-> domain conversions live here and nowhere else
    (design D3).
- [ ] **2.4** — Tests
  - `internal/workouts/validation_test.go`: table-driven, one case per
    rule plus the happy paths (minimal session, fully populated
    session).
  - `internal/workouts/store_test.go`: a five-method mock; asserts every
    store call forwards the caller's `user_id`, that a missing row is
    `ErrNotFound`, and that an invalid session never reaches the query.

**PR2 is done when:** `make test` and `make lint` are clean, and the
production code of the package touches `pgtype` in exactly one file
(`store.go`; `store_test.go` uses it to build the mock rows, and
`types.go` only mentions it in a comment).

## PR3 — Integration test against a live PostgreSQL

~253 lines.

- [ ] **3.1** — `internal/db/sqlc/planned_sessions_integration_test.go`
  - Build tag `integration`, package `sqlc_test`, reusing the existing
    `openTestDB` helper from `queries_integration_test.go` (do not open a
    second pool).
  - `TestIntegration_PlannedSessions_RejectsInvalidValues`: negative
    duration, negative distance, negative elevation, unknown workout
    type, unknown intensity -> the insert must fail.
  - `TestIntegration_PlannedSessions_OtherUserCannotTouchSession`: user B
    cannot read, edit or delete a session of user A; A's row is
    unchanged afterwards.
  - `TestIntegration_PlannedSessions_ListRangeIsScopedToUser`: B's list
    does not contain A's sessions.
  - `TestIntegration_PlannedSessions_CascadeDelete`: deleting the user
    removes the sessions.
  - Follow the user-creation and cleanup pattern of
    `ai_analysis_integration_test.go`.

**PR3 is done when:** `go test -tags integration -race -count=1
./internal/db/sqlc/...` passes against a PostgreSQL with the migrations
applied, and the same command skips cleanly when no `DATABASE_URL` is
set.

## Out of scope (explicitly, for the next slices)

- HTTP endpoints and RFC 9457 mapping -> #299.
- The "Entrenos" screen -> #300.
- Session status and activity linking -> phase 2.2.
- `race_id` -> phase 2.3.
- A `sqlc.yaml` type override for `workout_type`: it creates an import
  cycle (verified). The domain package converts the string itself.
