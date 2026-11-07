# Spec Delta: phase-1-5-optional-ai

> Domain: ai-analysis-api
> Source: openspec/changes/phase-1-5-optional-ai/{proposal,design}.md
> Cross-project: `.` (Go) and `web/` (TypeScript client)
> Stack: Go 1.26.0 + chi/v5 + pgx/v5 + riverqueue + sqlc + testify + httptest; React 19 + TypeScript 6 + Vitest 5
> Gating test command: `make test` (Go) and `pnpm -C web test:run` (web)
> Quality gates: `make lint`, `make vet`, `make fmt`, `pnpm -C web typecheck`, `pnpm -C web lint`

This delta covers the HTTP surface of the optional AI feature and its
TypeScript client: the status of the feature for the authenticated user,
the on-demand request of one analysis, the read of the user's own
analyses, the isolation between users and the wire contract between the
Go handlers and `web/src/lib/api/ai.ts`. No canonical
`openspec/specs/ai-analysis-api/spec.md` exists yet; the archive step
of the epic will copy this file into the canonical location.

The API never talks to a provider: it creates the row and enqueues the
job, which is the only egress (see `ai-analysis-backend`).

## ADDED Requirements

### Requirement: the three AI routes live inside the authenticated `/api/v1` group (`AA-001`)

`internal/http/router.go` SHALL mount three routes under the
authenticated group `/api/v1/ai`: `GET /api/v1/ai/status`,
`POST /api/v1/ai/analyses` and `GET /api/v1/ai/analyses`. All three
SHALL require an authenticated user (the same JWT middleware as the rest
of `/api/v1`) and SHALL answer `401` with the RFC 9457 problem document
when there is none. The routes SHALL NOT be mounted outside the
authenticated group, and their presence SHALL NOT change the behaviour
of any other route.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/router.go`, `internal/http/handlers/ai_analysis.go`

#### Scenario: an anonymous request to the status route is rejected

- GIVEN a request with no `Authorization` header
- WHEN `GET /api/v1/ai/status` is invoked
- THEN the response status is `401`
- AND the body is an `application/problem+json` document with a `type`,
  a `title`, a `status` of `401` and a `detail`

#### Scenario: an anonymous request to create an analysis is rejected

- GIVEN a request with no `Authorization` header
- WHEN `POST /api/v1/ai/analyses` is invoked
- THEN the response status is `401`
- AND no row was created in `ai_analysis`

### Requirement: `GET /api/v1/ai/status` reports the feature state and the consent of the caller (`AA-002`)

`handlers.GetAIStatus(q AIConsentStore, cfg config.AIConfig)` SHALL
answer `200` with the body `{ enabled: boolean, available: boolean,
status: string, provider: string | null, consented: boolean }`, where
`status` is `disabled`, `unavailable` or `ready` (the same vocabulary as
`config.AIConfig.Status()`), `provider` is the name of the configured
provider and `consented` is whether **the authenticated user** accepted
the transfer notice. The response SHALL NEVER carry the credential nor
the configured model. While the AI is off, the endpoint SHALL answer
`200` with `enabled = false`, `available = false`,
`status = "disabled"` and `provider = null`, so that a client can paint
the state without treating it as an error. The endpoint SHALL answer
`500` when the preferences of the user cannot be read.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/ai_analysis.go` (`GetAIStatus`)

#### Scenario: the status answers 200 while the AI is disabled

- GIVEN a configuration with `Enabled = false` and `Available = false`
- WHEN `GET /api/v1/ai/status` is invoked by an authenticated user
- THEN the response status is `200`
- AND the body has `enabled = false`, `available = false`,
  `status = "disabled"` and `provider = null`
- AND the body has no credential and no model

#### Scenario: the status reports the consent of the caller and the provider

- GIVEN a configuration with `Enabled = true`, `Available = true` and
  `Provider = "openai"`
- AND a user whose stored `ai_enabled` is `true`
- WHEN `GET /api/v1/ai/status` is invoked by that user
- THEN the response status is `200`
- AND the body has `status = "ready"`, `provider = "openai"` and
  `consented = true`

#### Scenario: the consent of another user is never reported

- GIVEN a user whose stored `ai_enabled` is `false`
- WHEN `GET /api/v1/ai/status` is invoked by that user
- THEN the body has `consented = false`

### Requirement: `POST /api/v1/ai/analyses` requests one analysis on demand (`AA-003`)

`handlers.PostAIAnalysis(q AIAnalysisStore, inserter
jobs.RiverJobInserter, cfg config.AIConfig)` SHALL accept a body with
`kind` (`activity`, `weekly` or `load`), `activity_id` (only with
`kind = activity`), `window_start` (`YYYY-MM-DD`, only with `weekly` or
`load`, defaulting to today in UTC) and `window_days` (only with
`weekly` or `load`). The handler SHALL check, **in this order**: the
authenticated user (`401`), the global flag and the availability of the
AI (`503`), the presence of a job queue (`503`), the consent of the
user (`403`), the validity of the body (`422`) and the ownership of the
target (`404`). None of those rejections SHALL create a row. When every
check passes, the handler SHALL create the row in status `pending`,
SHALL enqueue the job `AIAnalysisArgs` with
`river.InsertOpts{MaxAttempts: jobs.AIAnalysisMaxAttempts(cfg)}` and
SHALL answer `202` with the created analysis. The weekly analysis SHALL
use a fixed window of 7 days: a different `window_days` SHALL be a
`422`, not a silent analysis of another window. The load analysis SHALL
accept 1..`ai.MaxWindowDays` (90) days and SHALL default to 28. A body
that mixes `activity_id` with `window_start` or `window_days` SHALL be
rejected with `422`.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/ai_analysis.go` (`PostAIAnalysis`, `aiWindowDays`)

#### Scenario: a valid request answers 202 with a pending analysis

- GIVEN a ready configuration, a job queue and a consented user
- WHEN `POST /api/v1/ai/analyses` is invoked with
  `{ "kind": "activity", "activity_id": "<uuid of the user>" }`
- THEN the response status is `202`
- AND the body has `kind = "activity"`,
  `scope = "activity:<uuid>"` and `status = "pending"`
- AND the stored row is `pending`
- AND the job queue recorded one insert whose options carry
  `MaxAttempts = 1 + AI_MAX_RETRIES`

#### Scenario: a request without consent is rejected with 403 and creates nothing

- GIVEN a ready configuration and a user whose stored `ai_enabled` is
  `false`
- WHEN `POST /api/v1/ai/analyses` is invoked
- THEN the response status is `403`
- AND the body is an `application/problem+json` document whose `detail`
  mentions the consent
- AND no row was created
- AND the job queue recorded zero inserts

#### Scenario: a request while the AI is off is rejected with 503 and creates nothing

- GIVEN a configuration with `Enabled = false`
- WHEN `POST /api/v1/ai/analyses` is invoked by a consented user
- THEN the response status is `503`
- AND no row was created
- AND the job queue recorded zero inserts

#### Scenario: a request without a job queue is rejected with 503 and creates nothing

- GIVEN a ready configuration and no job queue
- WHEN `POST /api/v1/ai/analyses` is invoked by a consented user
- THEN the response status is `503`
- AND no row was created

#### Scenario: an invalid body is rejected with 422 and creates nothing

- GIVEN a ready configuration, a job queue and a consented user
- WHEN `POST /api/v1/ai/analyses` is invoked with `{ "kind": "monthly" }`
- THEN the response status is `422`
- AND the problem document carries an `errors` member with the field
  `kind`
- WHEN the same endpoint is invoked with `{ "kind": "activity",
  "activity_id": "<uuid>", "window_days": 7 }`
- THEN the response status is `422`
- WHEN the same endpoint is invoked with `{ "kind": "weekly",
  "window_days": 14 }`
- THEN the response status is `422` (the weekly window is fixed to 7
  days)
- AND no row was created in any of those cases

#### Scenario: the activity of another user is a 404 and creates nothing

- GIVEN a ready configuration, a job queue and a consented user
- WHEN `POST /api/v1/ai/analyses` is invoked with an `activity_id` that
  belongs to another user
- THEN the response status is `404`
- AND no row was created
- AND the job queue recorded zero inserts

#### Scenario: the load window is bounded by 90 days

- GIVEN a ready configuration, a job queue and a consented user
- WHEN `POST /api/v1/ai/analyses` is invoked with
  `{ "kind": "load", "window_days": 91 }`
- THEN the response status is `422`
- WHEN the same endpoint is invoked with `{ "kind": "load" }`
- THEN the response status is `202`
- AND the stored row has the scope `load:<start>:<days>` with the
  default window of 28 days

### Requirement: `GET /api/v1/ai/analyses` lists the analyses of the caller or returns one (`AA-004`)

`handlers.ListAIAnalyses(q AIAnalysisStore)` SHALL answer `200` with
`{ data: [...], limit: number }`, ordered from the most recent to the
oldest, when it is called without parameters; the limit SHALL default
to 20 and SHALL be capped at 100. With the pair `kind` and `scope` it
SHALL answer `200` with that single analysis, or `404` when it does not
exist. `kind` and `scope` SHALL be provided together: only one of them
SHALL be a `422`, and a `kind` outside `activity | weekly | load` SHALL
be a `422` too. A `limit` that is not an integer SHALL be a `422`. The
listing SHALL only ever contain the analyses of the authenticated user.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/ai_analysis.go` (`ListAIAnalyses`)

#### Scenario: the listing returns the analyses of the caller with the default limit

- GIVEN an authenticated user with three analyses
- WHEN `GET /api/v1/ai/analyses` is invoked
- THEN the response status is `200`
- AND the body has `limit = 20`
- AND `data` has the three analyses of that user, most recent first

#### Scenario: the limit is capped at 100

- GIVEN an authenticated user with analyses
- WHEN `GET /api/v1/ai/analyses?limit=500` is invoked
- THEN the body has `limit = 100`
- WHEN `GET /api/v1/ai/analyses?limit=abc` is invoked
- THEN the response status is `422`
- AND the problem document carries an `errors` member with the field
  `limit`

#### Scenario: one analysis is addressed with kind and scope together

- GIVEN an authenticated user with an analysis of scope
  `weekly:2026-01-05`
- WHEN `GET /api/v1/ai/analyses?kind=weekly&scope=weekly:2026-01-05` is
  invoked
- THEN the response status is `200`
- AND the body is that single analysis
- WHEN `GET /api/v1/ai/analyses?kind=weekly` is invoked
- THEN the response status is `422`
- WHEN `GET /api/v1/ai/analyses?kind=weekly&scope=weekly:1999-01-01` is
  invoked
- THEN the response status is `404`

### Requirement: the response never exposes the sent payload (`AA-005`)

The body of an analysis SHALL be
`{ id, kind, scope, status, provider, model, output, error_code,
error_message, created_at, updated_at }`. The `payload` that was sent to
the provider SHALL NOT appear in any response, not even to the user who
requested it. `output` SHALL be present only when the analysis is
`completed`, and SHALL carry the validated `{ summary, observations,
cautions }`. `error_code` and `error_message` SHALL be present only when
the analysis is `failed`, and SHALL carry the safe code and message
written by the application, never the body returned by the provider. The
timestamps SHALL be RFC 3339 in UTC.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/ai_analysis.go` (`aiAnalysisResponse`)

#### Scenario: a completed analysis exposes the validated output and not the payload

- GIVEN a completed analysis whose stored payload holds the aggregate
  metrics that were sent
- WHEN `GET /api/v1/ai/analyses?kind=activity&scope=...` is invoked
- THEN the response body has `status = "completed"`
- AND the body has `output` with the summary, the observations and the
  cautions
- AND the body has no `payload` key
- AND the body contains none of the numbers that were sent

#### Scenario: a failed analysis exposes only the safe code and message

- GIVEN a failed analysis with `error_code = "consent_revoked"`
- WHEN `GET /api/v1/ai/analyses?kind=activity&scope=...` is invoked
- THEN the body has `status = "failed"`
- AND the body has `error_code = "consent_revoked"`
- AND the body has no `output` key
- AND the body has no `payload` key

### Requirement: every read and write of an analysis is scoped to its owner (`AA-006`)

Every query used by the API SHALL filter by the `user_id` of the
authenticated user: `GetActivityForUser` filters by `(id, user_id)`, so
the activity of another user does not exist for the API;
`GetAIAnalysis` and `ListAIAnalyses` filter by `user_id`; and
`CreateAIAnalysis` writes with the `user_id` of the authenticated user.
A user SHALL NEVER read, list or overwrite the analysis of another
user, and the analysis of another user SHALL NEVER be enqueued.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/queries/ai_analysis.sql`, `internal/db/queries/activities_ext.sql`, `internal/http/handlers/ai_analysis.go`

#### Scenario: the listing of a user does not contain the analyses of another user

- GIVEN user A with two analyses and user B with one analysis
- WHEN `GET /api/v1/ai/analyses` is invoked by user B
- THEN the body has exactly one entry
- AND that entry belongs to user B

#### Scenario: reading the analysis of another user is a 404

- GIVEN an analysis of user A with a given kind and scope
- WHEN `GET /api/v1/ai/analyses?kind=<that kind>&scope=<that scope>` is
  invoked by user B
- THEN the response status is `404`

### Requirement: the errors follow the RFC 9457 contract of the repository (`AA-007`)

Every error of these three routes SHALL be an
`application/problem+json` document with the members `type`, `title`,
`status`, `detail` and `instance` (the request id), using the existing
helpers of `internal/http/handlers/errors.go`. A validation error SHALL
add the extension member `errors` with the per-field detail (`kind`,
`activity_id`, `window_start`, `window_days`, `limit`). The status codes
SHALL be `401` (not authenticated), `403` (consent required), `404`
(activity or analysis not found), `422` (invalid request), `503` (the
AI is disabled, unavailable or the queue is not configured) and `500`
(internal error). The `detail` of the `503` SHALL distinguish the
disabled flag, the unconfigured provider and the missing job queue.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/http/handlers/errors.go`, `internal/http/handlers/ai_analysis.go`

#### Scenario: the refusals carry the status and the detail of their cause

- GIVEN a ready configuration, a job queue and a user without consent
- WHEN `POST /api/v1/ai/analyses` is invoked
- THEN the response status is `403`
- AND the content type is `application/problem+json`
- AND the `detail` mentions that the data transfer notice has to be
  accepted
- GIVEN a configuration with `Enabled = false`
- WHEN the same endpoint is invoked
- THEN the response status is `503`
- AND the `detail` mentions that the global AI flag is disabled

### Requirement: the TypeScript client mirrors the Go contract (`AA-008`)

`web/src/lib/api/ai.ts` SHALL expose `fetchAIStatus(token)`,
`createAIAnalysis(token, request)`, `listAIAnalyses(token, { limit? })`
and `fetchAIAnalysis(token, kind, scope)`, and SHALL export the types
`AIAnalysisKind` (`activity | weekly | load`), `AIAnalysisStatus`
(`pending | completed | failed`), `AIStatus` (`disabled | unavailable |
ready`), `AIStatusResponse`, `AIAnalysisOutput`, `AIAnalysis`,
`AIAnalysisListResponse` and `AIAnalysisRequest`. The types SHALL be
the mirror of the Go structures of the previous requirements, field by
field. The client SHALL send the bearer token in the `Authorization`
header, SHALL send and read JSON, and SHALL convert a failure into an
`ApiError` carrying the HTTP `status`, the `detail` of the problem
document and the per-field `fields` map, following the convention of
`web/src/lib/api/profile.ts`. The client SHALL NOT send a body that
mixes `activity_id` with `window_start` or `window_days`, because the
backend rejects that combination.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/lib/api/ai.ts`

#### Scenario: the client asks for the status with the bearer token

- GIVEN a token `t`
- WHEN `fetchAIStatus(t)` is called
- THEN the request went to `/api/v1/ai/status` with the header
  `Authorization: Bearer t`
- AND the parsed body is an `AIStatusResponse`

#### Scenario: a failure becomes an ApiError with the status, the detail and the fields

- GIVEN a response with the status `422` and a problem document whose
  `detail` is `request validation failed` and whose `errors` member
  maps `kind` to a message
- WHEN `createAIAnalysis` is called
- THEN it rejects with an `ApiError`
- AND the error carries `status = 422`
- AND the error carries the `detail` of the problem document
- AND the error carries the per-field map

#### Scenario: the client does not mix the activity and window fields

- GIVEN a request of kind `activity` with an `activity_id`
- WHEN the body is built
- THEN it has no `window_start` and no `window_days`
- GIVEN a request of kind `load` with a `window_start` and a
  `window_days`
- WHEN the body is built
- THEN it has no `activity_id`
