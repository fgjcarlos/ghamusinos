# Design: phase-2-1-planned-sessions-model

## Decisions

### D1 — `scheduled_on` is a `DATE`, and the day is resolved before writing

- **Context:** the issue asks for "the day in the user's timezone
  (`users.timezone`)". A `DATE` column carries no time and no offset, so
  whatever we store is a calendar day, never an instant.
- **Decision:** `scheduled_on DATE NOT NULL`. The conversion from "now" to
  "the user's local calendar day" happens at the API edge (#299), where
  `users.timezone` is already read. The DB never shifts a day.
- **Rationale:** a `TIMESTAMPTZ` would make "the day the user planned"
  depend on the session timezone of whatever process reads it later, and
  the same row would fall on two different calendar days depending on who
  asked. Verified against a live PostgreSQL: the stored date reads back
  identically under `SET TIME ZONE 'Europe/Madrid'` and `SET TIME ZONE
  'UTC'`. The column is timezone-invariant by construction.
- **Alternatives:** `TIMESTAMPTZ` (rejected: reintroduces the ambiguity we
  are removing); storing a `TEXT` date (rejected: no date semantics, no
  range index).
- **Tradeoffs:** the DB cannot answer "what is the user's day right now".
  That question belongs to #299 anyway, which owns the request context.

### D2 — The workout-type and intensity vocabularies live in the domain, and the CHECK constraints repeat them

- **Context:** the issue proposes `workout_type` with a `CHECK` and asks
  to validate the list in the change. The legacy module
  `ghamusinos_/__` (the reference implementation) is not reachable from
  this repo, so the list is taken from the issue itself.
- **Decision:** the closed set is
  `rodaje, tirada_larga, series, cuestas, tempo, fartlek, fuerza,
  recuperacion, otro` for `workout_type`, and `z1..z5` for
  `target_intensity`. The Go constants live in `internal/workouts`; the
  `CHECK` constraints repeat the same literals in the migration. The
  static contract test asserts both sides contain every member of the
  set, so the two copies cannot drift silently.
- **Rationale:** `target_intensity` reuses the `z1..z5` vocabulary the
  product already uses for heart-rate zones (`hr_zones.z1_seconds` ..
  `z5_seconds`, and `web/src/lib/api/dashboard.ts` types zones as
  `'z1' | 'z2' | 'z3' | 'z4' | 'z5'`). One vocabulary for zones, not two.
- **Alternatives:** a Postgres `ENUM` type (rejected: `ALTER TYPE` to add a
  value is heavier than `ALTER TABLE ... DROP/ADD CONSTRAINT`, and the repo
  uses `TEXT` + `CHECK` everywhere else); storing the intensity as an
  `INTEGER` 1..5 (rejected: the rest of the product speaks `z1`..`z5`).
- **Tradeoffs:** adding a workout type later needs a migration, not just
  a Go constant. Accepted: the vocabulary is intentionally closed, and
  `otro` is the escape hatch.

### D3 — `internal/workouts` owns the `pgtype` boundary

- **Context:** the issue requires that "the HTTP handler does not know
  about `pgtype`". SQLC generates `pgtype.UUID`, `pgtype.Date`,
  `pgtype.Text` and `pgtype.Int4` for the columns of this table, so
  something has to convert them.
- **Decision:** `internal/workouts` exposes plain Go types (`uuid.UUID`,
  `time.Time`, `string`, `*int`, `*Intensity`) and a `Store` that converts
  in both directions. The conversion helpers live in `store.go` next to
  the queries that produce them. #299's handler will import
  `internal/workouts` and never `internal/db/sqlc` for this feature.
- **Rationale:** one boundary, in one place, with one direction of
  conversion per field. The alternative -- letting the handler marshal
  `sqlc.PlannedSession` directly -- is what the issue explicitly rules
  out, and it would also leak `pgtype` JSON shapes into the API contract.
- **Alternatives:** a `sqlc.yaml` type override mapping
  `planned_sessions.workout_type` to `workouts.WorkoutType` (rejected:
  **verified to be an import cycle** -- `internal/db/sqlc` would import
  `internal/workouts`, which imports `internal/db/sqlc`. The cycle was
  reproduced before rejecting it.); converting in the handler (rejected by
  the issue).
- **Tradeoffs:** the domain types are hand-written rather than generated,
  so a new column means updating `fromRow` by hand. The contract test
  fails if a column is added to the migration without being reflected in
  the domain.

### D4 — Ownership is enforced in SQL; the domain layer never decides

- **Context:** the acceptance criterion is that "user B cannot read, edit
  or delete user A's sessions".
- **Decision:** every one of the five queries filters by `user_id`:
  `Get` and `Update` and `Delete` all carry `AND user_id = $n`, and
  `List` filters `WHERE user_id = $1`. The `Store` receives the caller's
  `user_id` and forwards it unconditionally.
- **Rationale:** the filter lives in the only place that cannot be
  bypassed. A missing filter is a row leak that no amount of Go-side
  care would prevent; a redundant filter in Go would be a second place to
  keep in sync.
- **Alternatives:** filtering in the handler only (rejected: the next
  caller would forget it).
- **Tradeoffs:** `Delete` returns the affected row count (`:execrows`)
  rather than `:exec`, because the store has to distinguish "deleted"
  from "not yours" to answer 404. `Get` and `Update` return
  `pgx.ErrNoRows`, which the store maps to `ErrNotFound`.

### D5 — The `Store` depends on a five-method interface, not on `sqlc.Querier`

- **Context:** `sqlc.Querier` has 69 methods after this change and every
  new query widens it, breaking every mock that implements it in full.
- **Decision:** `internal/workouts` declares its own five-method
  `Querier` interface (the same pattern as `internal/gpx`'s
  `gpxQuerier`) and the `Store` takes that. The `sqlc.Queries` type
  satisfies it implicitly.
- **Rationale:** the domain package depends on the five operations it
  actually performs. The unit test needs a mock of five methods, not 69,
  and adding a sixth unrelated query to the project does not touch this
  package's tests.
- **Alternatives:** depending on `sqlc.Querier` (rejected: the mock would
  have to implement the whole interface; `internal/gpx/store_test.go` has
  to embed `sqlc.Querier` to cope with exactly this).
- **Tradeoffs:** the interface has to be updated when the store grows a
  method. That is the point: it is a deliberate, small step.

### D6 — The domain validation mirrors the DB constraints instead of replacing them

- **Context:** two layers could enforce the same rules (Go validation and
  `CHECK` constraints). That looks like duplication.
- **Decision:** both exist, and they do different jobs. The `CHECK`
  constraints are the invariant ("an objective is never negative", "the
  type is one of these nine") and are enforced for every writer,
  including a future one that does not go through the domain package. The
  Go validation owns what a `CHECK` expresses badly: that a title is
  required and that titles and notes have length bounds, with a message
  naming the offending field.
- **Rationale:** the database is the last line of defence, and the issue
  asks for it explicitly ("the DB rejects negative objectives and unknown
  types (tests)"). The Go layer exists so the user gets a useful error
  instead of a constraint violation.
- **Alternatives:** Go validation only (rejected: a bug in the handler
  would write a negative objective; the issue requires the DB to reject
  it); `CHECK` constraints only (rejected: the error would not say which
  field was wrong, and the issue asks for table tests of the domain
  validation).
- **Tradeoffs:** the valid workout-type list exists in two places (Go map
  and SQL `CHECK`). The static contract test is what keeps them honest.

### D7 — `notes` is `NULL` when empty; objectives are `NULL` when unplanned

- **Context:** the API of #299 will `PATCH` a session and needs to tell
  "not planned" from "planned as zero".
- **Decision:** optional columns are nullable and the domain uses
  pointers (`*int`, `*Intensity`); the empty string is converted to
  `NULL` for `notes`.
- **Rationale:** a planned objective of zero and an absent objective are
  different facts, and the difference has to survive the round trip.
  Collapsing them to zero would make the calendar lie about what the user
  planned.
- **Alternatives:** zero-means-absent (rejected: loses the distinction);
  a sentinel like `-1` (rejected: the `CHECK` would have to permit it).
- **Tradeoffs:** the JSON of the API will need `omitempty` on the
  optional fields, and #299 must not emit a zero for an unset objective.

## Data model

```sql
CREATE TABLE planned_sessions (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scheduled_on            DATE        NOT NULL,
    title                   TEXT        NOT NULL CONSTRAINT planned_sessions_title_valido
                                      CHECK (title <> '' AND length(title) <= 200),
    notes                   TEXT        CONSTRAINT planned_sessions_notes_valido
                                      CHECK (notes IS NULL OR length(notes) <= 2000),
    workout_type            TEXT        NOT NULL CONSTRAINT planned_sessions_workout_type_valido
                                      CHECK (workout_type IN ('rodaje', 'tirada_larga', 'series', 'cuestas',
                                                             'tempo', 'fartlek', 'fuerza', 'recuperacion', 'otro')),
    target_duration_sec     INTEGER     CONSTRAINT planned_sessions_target_duration_no_negativo
                                      CHECK (target_duration_sec IS NULL OR target_duration_sec >= 0),
    target_distance_m       INTEGER     CONSTRAINT planned_sessions_target_distance_no_negativo
                                      CHECK (target_distance_m IS NULL OR target_distance_m >= 0),
    target_elevation_gain_m INTEGER     CONSTRAINT planned_sessions_target_elevation_no_negativo
                                      CHECK (target_elevation_gain_m IS NULL OR target_elevation_gain_m >= 0),
    target_intensity        TEXT        CONSTRAINT planned_sessions_target_intensity_valido
                                      CHECK (target_intensity IS NULL OR target_intensity IN ('z1', 'z2', 'z3', 'z4', 'z5')),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_planned_sessions_user_scheduled ON planned_sessions (user_id, scheduled_on);
```

`users.timezone` is deliberately **not** denormalised into the table: the
stored value is a calendar day, which does not need a timezone to be
unambiguous.

## Query surface

| Query | Kind | Ownership filter | Notes |
|-------|------|------------------|-------|
| `CreatePlannedSession` | `:one` | `user_id` in the `INSERT` | `RETURNING *`. |
| `GetPlannedSession` | `:one` | `AND user_id = $2` | `pgx.ErrNoRows` when absent or not owned. |
| `ListPlannedSessionsByRange` | `:many` | `WHERE user_id = $1` | Inclusive date window, `COUNT(*) OVER()` for the real total, `LIMIT`/`OFFSET`. |
| `UpdatePlannedSession` | `:one` | `AND user_id = $2` | `RETURNING *`; `pgx.ErrNoRows` when absent or not owned. |
| `DeletePlannedSession` | `:execrows` | `AND user_id = $2` | Returns the affected count: 0 means "not yours or not there". |

`ListPlannedSessionsByRange` uses `sqlc.arg()` for its parameters. With
positional `$2`/`$3` on two `DATE` columns SQLC cannot infer the types
and the generation fails ("could not determine data type of parameter
$2"); with named arguments it generates `FromDate` / `ToDate`. This was
verified by generating both ways.

## Ripple: the `sqlc.Querier` interface grows by five methods

`sqlc.yaml` sets `emit_interface: true`, so the generated `Querier`
interface contains every query in the project. Adding five queries breaks
every mock that implements it in full. The build fails with:

```
*mockQuerier does not implement sqlc.Querier (missing method CreatePlannedSession)
```

Five mock types in five files need five stubs each (16 lines per file):

| File | Mock type |
|------|-----------|
| `internal/http/router_test.go` | `mockQuerier` |
| `internal/http/handlers/activities_test.go` | `activitiesMockQuerier` |
| `internal/http/handlers/me_test.go` | `mockMeQuerier` |
| `internal/http/handlers/me_preferences_test.go` | `preferencesMockQuerier` |
| `internal/auth/resolver_test.go` | `mockQuerier` |

`internal/gpx/store_test.go` embeds `sqlc.Querier` and therefore keeps
compiling unchanged. `dashboardMockQuerier` and `healthQuerier` embed
`activitiesMockQuerier` and inherit the new stubs.

The stubs go in **PR1**, together with the queries that cause the break:
no commit in the chain may be left red.
