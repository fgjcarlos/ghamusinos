# Canonical spec: planned-sessions-api (Go)

## Source

- Slice 2 of GitHub epic #301 (Fase 2.1 — Creacion de entrenos), issue
  #299, delivered in commit `7da655e` (`feat(api): add authenticated
  planned workout CRUD (#325)`).
- Contract documentation: `docs/api/workouts.md`.
- Backend persistence: `openspec/specs/planned-sessions-backend/spec.md`.

## Domain Metadata

| Project root | `.` |
| --- | --- |
| Stack | Go 1.26.0 + chi/v5 + pgx/v5 + testify |
| Gating test command | `make test` (raw: `go test -race ./...`) |
| Quality gates | `make lint` (`golangci-lint run ./...`), `make vet`, `make fmt` |

Canonical spec for the HTTP contract of the planned sessions. The
persistence rules are in `openspec/specs/planned-sessions-backend/spec.md`;
the SPA screen is in `openspec/specs/planned-sessions-web/spec.md`.

## ADDED Requirements

### Requirement: the planned-sessions routes require authentication (`PA-001`)

The API SHALL expose the planned-session routes under `/api/v1/workouts`
and every one of them SHALL require an authenticated caller. A request
without an authenticated user SHALL be answered with `401` and an RFC
9457 problem document. The routes SHALL be registered only when a
workout store is configured.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/router.go`, `internal/http/handlers/workouts.go`

#### Scenario: an anonymous caller is refused

- GIVEN a request to `GET /api/v1/workouts?from=2026-11-01&to=2026-11-30` without a session
- WHEN the request reaches the router
- THEN the response status is `401`
- AND the body is a problem document with the `application/problem+json` content type

#### Scenario: the routes exist when the store is wired

- GIVEN a server built with `WithWorkouts`
- WHEN the route table is inspected
- THEN `POST /api/v1/workouts`, `GET /api/v1/workouts`,
  `GET /api/v1/workouts/{id}`, `PATCH /api/v1/workouts/{id}` and
  `DELETE /api/v1/workouts/{id}` are present

### Requirement: a session is created with its required fields (`PA-002`)

`POST /api/v1/workouts` SHALL accept `scheduled_on`, `title` and
`workout_type` as required fields, and `notes`,
`target_duration_sec`, `target_distance_m`, `target_elevation_gain_m`
and `target_intensity` as optional ones. `scheduled_on` SHALL be a
strict `YYYY-MM-DD` civil date. An omitted target SHALL be stored as
unplanned (`null` in the response) and SHALL NOT be stored as zero;
omitted `notes` SHALL default to an empty string. A target SHALL fit a
signed 32-bit integer. The response SHALL be `201` with the created
session.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/workouts.go` (`CreateWorkout`)

#### Scenario: a complete session is created

- GIVEN an authenticated caller
- WHEN it posts a body with `scheduled_on` `2026-11-20`, `title` `Tirada larga` and `workout_type` `tirada_larga`, plus a `target_distance_m` of `20000` and a `target_intensity` of `z2`
- THEN the response status is `201`
- AND the body carries the session with `target_duration_sec` and `target_elevation_gain_m` as `null`
- AND the persisted session belongs to the caller

#### Scenario: a missing required field is refused

- GIVEN an authenticated caller
- WHEN it posts a body with `title` and `workout_type` but no `scheduled_on`
- THEN the response status is `400`
- AND no session is created

#### Scenario: a malformed date is refused

- GIVEN an authenticated caller
- WHEN it posts a body with `scheduled_on` set to `20-11-2026`
- THEN the response status is `400`
- AND no session is created

#### Scenario: a target that does not fit the database is refused

- GIVEN an authenticated caller
- WHEN it posts a session with a `target_distance_m` above the 32-bit range
- THEN the response status is `422`
- AND no session is created

#### Scenario: an unknown field is refused

- GIVEN an authenticated caller
- WHEN it posts a body carrying a field that is not part of the session
- THEN the response status is `400`
- AND no session is created

### Requirement: the sessions of a window are listed with pagination (`PA-003`)

`GET /api/v1/workouts` SHALL require the query parameters `from` and
`to` as strict `YYYY-MM-DD` civil dates, SHALL reject a range whose `to`
precedes `from`, and SHALL list the sessions inside the inclusive range
ordered by `scheduled_on` ascending and then by `id` ascending. The
parameters `limit` (default `20`, allowed `1`..`100`) and `offset`
(default `0`, not negative) SHALL paginate the result. The response
SHALL be an object with `data`, `total`, `limit`, `offset` and `has_next`,
where `data` is always an array — an empty one when the page is past the
end — and `has_next` tells whether more rows exist after this page.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/workouts.go` (`ListWorkouts`)

#### Scenario: a window is listed

- GIVEN an authenticated caller with three sessions in November 2026
- WHEN it lists the range `from=2026-11-01` and `to=2026-11-30`
- THEN the response status is `200`
- AND `data` holds the three sessions in date order
- AND `total` is `3`
- AND `has_next` is `false`

#### Scenario: the window is mandatory

- GIVEN an authenticated caller
- WHEN it lists without `from` or without `to`
- THEN the response status is `400`

#### Scenario: an inverted range is refused

- GIVEN an authenticated caller
- WHEN it lists `from=2026-11-30` and `to=2026-11-01`
- THEN the response status is `400`

#### Scenario: the page size is bounded

- GIVEN an authenticated caller
- WHEN it lists with `limit=0`, with `limit=101` or with a non-numeric `limit`
- THEN the response status is `400`

#### Scenario: a page past the end is empty, not absent

- GIVEN an authenticated caller with three sessions
- WHEN it lists with `offset=10`
- THEN the response status is `200`
- AND `data` is an empty array
- AND `has_next` is `false`

### Requirement: a session is read, patched and deleted by id (`PA-004`)

`GET /api/v1/workouts/{id}` SHALL return the session with `200`.
`PATCH /api/v1/workouts/{id}` SHALL update only the supplied fields,
preserving the omitted ones, SHALL allow the nullable target fields to be
set to `null` to clear them, and SHALL NOT accept `null` for
`scheduled_on`, `title`, `workout_type` or `notes`; it SHALL answer `200`
with the updated session. `DELETE /api/v1/workouts/{id}` SHALL remove
the session and answer `204` with no body. An id that is not a UUID, a
session that does not exist and a session of another user SHALL all be
answered with `404`.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/workouts.go` (`GetWorkout`,
`PatchWorkout`, `DeleteWorkout`)

#### Scenario: a session is read

- GIVEN an authenticated owner with a session
- WHEN it reads that session id
- THEN the response status is `200`
- AND the body is the session

#### Scenario: a patch changes only the supplied fields

- GIVEN an authenticated owner with a session whose title is `Tirada larga` and whose distance target is `20000`
- WHEN it patches a body with a new `title` of `Tirada larga 30k`
- THEN the response status is `200`
- AND the title is the new one
- AND the distance target is still `20000`

#### Scenario: a target is cleared

- GIVEN an authenticated owner with a session that has a distance target
- WHEN it patches a body with `target_distance_m` set to `null`
- THEN the response status is `200`
- AND the distance target is `null`

#### Scenario: an empty or unknown patch is refused

- GIVEN an authenticated owner
- WHEN it patches with an empty body, with an unknown field or with a `null` title
- THEN the response status is `400`
- AND the stored session is unchanged

#### Scenario: a session is deleted

- GIVEN an authenticated owner with a session
- WHEN it deletes that session id
- THEN the response status is `204`
- AND the body is empty
- AND the session is gone afterwards

### Requirement: another user's session is invisible (`PA-005`)

Every route SHALL operate on the sessions of the authenticated caller
only. A session of another user SHALL be answered exactly like a session
that does not exist (`404` on the by-id routes) and SHALL NOT appear in
the list of the caller. No response SHALL disclose whether the session
of another user exists.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/workouts.go`,
`internal/db/queries/planned_sessions.sql`

#### Scenario: another user's session cannot be read, patched or deleted

- GIVEN user A with a session
- WHEN user B reads, patches or deletes that session id
- THEN every response status is `404`
- AND the session of user A is unchanged

#### Scenario: another user's session is not listed

- GIVEN user A with a session inside the queried window
- WHEN user B lists that window
- THEN `data` contains no session of user A
- AND `total` counts only the sessions of user B

### Requirement: the errors are RFC 9457 problem documents (`PA-006`)

Every error response SHALL use the `application/problem+json` media type
and the RFC 9457 shape. A malformed body, an invalid date, an invalid
range, an invalid pagination parameter and an unknown field SHALL be
answered with `400`. A body over the size limit SHALL be answered with
`413`. A domain validation failure — an empty or oversized title, an
oversized note, an unknown workout type or intensity, a negative target
— SHALL be answered with `422`. A storage failure SHALL be answered with
`500`. The response SHALL carry the request id.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/workouts.go`,
`internal/http/handlers/errors.go`

#### Scenario: a domain validation failure is unprocessable

- GIVEN an authenticated caller
- WHEN it posts a session with an empty title
- THEN the response status is `422`
- AND the content type is `application/problem+json`
- AND the problem document names the offending field

#### Scenario: an oversized body is refused

- GIVEN an authenticated caller
- WHEN it posts a body over the size limit
- THEN the response status is `413`

### Requirement: the session is serialized as plain JSON (`PA-007`)

The session JSON SHALL carry `id`, `scheduled_on`, `title`, `notes`,
`workout_type`, `target_duration_sec`, `target_distance_m`,
`target_elevation_gain_m` and `target_intensity`. `scheduled_on` SHALL
be a `YYYY-MM-DD` civil date. The numeric targets SHALL serialize as
plain JSON integers or `null`, never as database driver values, and
`target_intensity` SHALL be `null` or one of `z1`..`z5`.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/workouts.go` (`workoutDTO`)

#### Scenario: an unplanned target is null, not zero

- GIVEN a session created without a distance target
- WHEN it is read back
- THEN `target_distance_m` is `null`
- AND the value is not `0`

#### Scenario: the date is a civil date

- GIVEN a session scheduled on the twentieth of November 2026
- WHEN it is read back
- THEN `scheduled_on` is `2026-11-20`
