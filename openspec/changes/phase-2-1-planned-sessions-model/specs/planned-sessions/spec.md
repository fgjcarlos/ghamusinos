# Spec Delta: phase-2-1-planned-sessions-model

> Domain: planned-sessions
> Source: openspec/changes/phase-2-1-planned-sessions-model/{proposal,design}.md
> Cross-project: `.` (Go)
> Stack: Go 1.26.0 + chi/v5 + pgx/v5 + goose/v3 + sqlc + testify
> Gating test command: `make test`
> Quality gates: `make lint`, `make vet`, `make fmt`

## ADDED Requirements

### Requirement: a planned session is stored in its owner's calendar day (`PS-001`)

`planned_sessions` SHALL store one planned training session per row, with
`user_id` referencing `users(id)` ON DELETE CASCADE, and `scheduled_on` as
a `DATE` carrying the calendar day the session belongs to. The column
SHALL NOT carry a time or a timezone: the user's local day MUST be
resolved before the value is written, so that the stored day is
independent of the session timezone of any later reader.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/migrations/00016_planned_sessions.sql`

#### Scenario: the stored day does not depend on the session timezone

- GIVEN a session stored with `scheduled_on = '2026-11-20'`
- WHEN it is read under `SET TIME ZONE 'Europe/Madrid'` and again under `SET TIME ZONE 'UTC'`
- THEN both reads return `2026-11-20`
- AND the day never shifts because of who is reading it

#### Scenario: deleting the owner removes the sessions

- GIVEN a user with planned sessions
- WHEN the user row is deleted
- THEN the sessions are removed by the cascade
- AND no session of that user remains

### Requirement: every planned-session operation is scoped to its owner (`PS-002`)

Every query over `planned_sessions` SHALL filter by `user_id`. `Get`,
`Update` and `Delete` SHALL scope by the session id **and** the owning
`user_id`; `List` SHALL scope by `user_id`; `Create` SHALL write the
owning `user_id` supplied by the caller. A session belonging to another
user SHALL be indistinguishable from a session that does not exist.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/queries/planned_sessions.sql`,
`internal/workouts/store.go`

#### Scenario: another user cannot read, edit or delete the session

- GIVEN user A has a planned session
- WHEN user B reads, updates or deletes that session
- THEN the read returns no row
- AND the update affects no row
- AND the delete affects no row
- AND the session of user A is unchanged

#### Scenario: the list of another user is empty

- GIVEN user A has a planned session inside the queried window
- WHEN user B lists that window
- THEN the result contains no session of user A

### Requirement: the database rejects invalid sessions (`PS-003`)

The database SHALL reject a row whose `workout_type` is outside the
closed set (`rodaje`, `tirada_larga`, `series`, `cuestas`, `tempo`,
`fartlek`, `fuerza`, `recuperacion`, `otro`), whose `target_intensity` is
outside `z1`..`z5`, or whose `target_duration_sec`,
`target_distance_m` or `target_elevation_gain_m` is negative. The
constraints SHALL apply to every writer, including one that does not go
through the domain package.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/migrations/00016_planned_sessions.sql`

#### Scenario: a negative objective is refused

- GIVEN a user and a valid session
- WHEN the session is inserted with a negative `target_duration_sec`
- THEN the insert fails with a constraint violation

#### Scenario: an unknown workout type is refused

- GIVEN a user and a valid session
- WHEN the session is inserted with `workout_type = 'nope'`
- THEN the insert fails with a constraint violation

#### Scenario: an unknown intensity is refused

- GIVEN a user and a valid session
- WHEN the session is inserted with `target_intensity = 'z9'`
- THEN the insert fails with a constraint violation

### Requirement: the domain layer owns the vocabulary and the invariants (`PS-004`)

`internal/workouts` SHALL expose the planned session as plain Go types
(`uuid.UUID`, `time.Time`, `string`, `*int`, `*Intensity`) and SHALL
validate a session before it is written: the title SHALL be present and
at most 200 characters, the notes at most 2000 characters, the workout
type and the intensity SHALL belong to the vocabularies of `PS-003`, the
objectives SHALL NOT be negative, and the date SHALL be set. A validation
failure SHALL name the offending field.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/workouts/types.go`,
`internal/workouts/validation.go`

#### Scenario: an invalid session never reaches the database

- GIVEN a session with an empty title
- WHEN it is submitted to the store
- THEN the store returns a validation error naming the title
- AND no query is issued

#### Scenario: an unplanned objective is not the same as a zero one

- GIVEN a session with no target distance
- WHEN it is stored and read back
- THEN the target distance is absent, not zero

### Requirement: the migration is reversible and the generated code is committed (`PS-005`)

The migration SHALL declare a goose `Down` section that removes the table.
The SQLC schema SHALL mirror the migration, the generated code SHALL be
committed, and re-running the generator SHALL produce no diff.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/migrations/00016_planned_sessions.sql`,
`internal/db/schema.sql`, `internal/db/sqlc/`

#### Scenario: the migration is reverted and re-applied

- GIVEN a database with the migration applied
- WHEN the migration is reverted and applied again
- THEN both operations succeed
- AND the table exists with the same shape afterwards

#### Scenario: the static contract holds

- GIVEN the migration, the schema and the query file on disk
- WHEN the contract test runs
- THEN it asserts that the table, the columns, the cascade, the
  vocabularies, the non-negative constraints, the index and the five
  query names are present in every place they should be
