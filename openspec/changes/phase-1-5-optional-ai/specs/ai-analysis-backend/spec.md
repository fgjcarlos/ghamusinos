# Spec Delta: phase-1-5-optional-ai

> Domain: ai-analysis-backend
> Source: openspec/changes/phase-1-5-optional-ai/{proposal,design}.md
> Cross-project: `.` (Go)
> Stack: Go 1.26.0 + chi/v5 + pgx/v5 + goose/v3 + riverqueue + sqlc + testify + httptest
> Gating test command: `make test`
> Quality gates: `make lint`, `make vet`, `make fmt`

This delta covers the whole backend of the optional AI feature: the
provider-agnostic domain contract (`internal/ai`), the three provider
adapters, the global configuration that never blocks the boot, the
explicit per-user consent, the `ai_analysis` table, and the River job
that is the only egress towards an AI provider. No canonical
`openspec/specs/ai-analysis-backend/spec.md` exists yet; the archive
step of the epic will copy this file into the canonical location.

The capability is **optional by construction**: with the global flag
off, with a broken configuration, or with a provider down, the
deterministic product keeps working exactly as before (S7).

## ADDED Requirements

### Requirement: the domain talks to providers through one provider-agnostic interface (`AB-001`)

`internal/ai/analyzer.go` SHALL expose the single interface the domain
sees: `Analyzer { Provider() Provider; Analyze(ctx context.Context,
payload Payload) (Output, error) }`, together with `ProviderConfig`
(`Provider`, `Model`, `APIKey`, optional `HTTPClient`) and
`NewAnalyzer(cfg ProviderConfig) (Analyzer, error)`. `NewAnalyzer`
SHALL return the adapter of the configured provider or an error, and
SHALL NEVER fall back to another provider. `SupportedProviders()` SHALL
return the providers in the ADR 0002 order — `openai`, `claude`,
`openrouter` — and `ParseProvider(name)` SHALL accept exactly those
three names (the configuration lowercases and trims the configured value
before calling it). A provider adapter SHALL perform exactly
one HTTP attempt per `Analyze` call, SHALL NOT follow redirects, SHALL
cap the response body at `MaxResponseBytes` (1 MiB), SHALL accept only
`application/json` responses, and SHALL honour the caller's
`context.Context` deadline. The credential SHALL travel only in the
provider's standard authentication header.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/ai/analyzer.go`, `internal/ai/openai.go`, `internal/ai/claude.go`, `internal/ai/openrouter.go`

#### Scenario: the configured provider is the only one that is called

- GIVEN a configuration with `Provider = "openai"` and a fake OpenAI
  transport
- WHEN `NewAnalyzer` is called and `Analyze` is invoked once
- THEN the returned analyzer reports `Provider() == "openai"`
- AND exactly one HTTP request reached the OpenAI endpoint
- AND no request reached the Claude or the OpenRouter endpoint

#### Scenario: a provider failure never switches to another provider

- GIVEN a configuration with `Provider = "openai"` whose fake transport
  answers `503`
- WHEN `Analyze` is invoked once
- THEN the call returns an `*ai.Error` with `Kind == ErrorKindRetryable`
  and `Reason == "provider_error"`
- AND no request reached any other provider endpoint
- AND the error message contains neither the credential nor the payload

#### Scenario: an unknown provider name is rejected

- GIVEN a configuration with `Provider = "gemini"`
- WHEN `ParseProvider("gemini")` is called
- THEN it returns an error
- AND `NewAnalyzer` with that provider returns an error instead of an
  adapter

#### Scenario: the response body is capped and redirects are not followed

- GIVEN a fake provider that answers with a body larger than 1 MiB
- WHEN `Analyze` is invoked
- THEN the call fails with a permanent `*ai.Error` whose `Reason` is
  `"oversized_body"`
- GIVEN a fake provider that answers `302` with a `Location` header
- WHEN `Analyze` is invoked
- THEN the call fails with a permanent `*ai.Error` whose `Reason` is
  `"redirect"`
- AND the redirect target was never requested

### Requirement: the outbound payload carries aggregate metrics only (`AB-002`)

`internal/ai/payload.go` SHALL build the outbound payload from a
`Builder` whose JSON form contains exactly `kind`, `window_days` and
five aggregate metrics: `distance_meters`, `duration_seconds`,
`elevation_gain_meters`, `avg_heart_rate` and `training_load`. The
payload MUST NOT contain any field for identity, name, e-mail address,
GPS coordinates, raw streams, timestamps or free text; no such field
SHALL exist in the type. `Build()` SHALL reject a metric that is not
finite (NaN, ±Inf), is negative, or exceeds the limit of its kind. An
aggregate that was not measured SHALL be **omitted** from the JSON
SHALL NOT be sent as an invented `0`; an explicit `0` SHALL be sent,
because a rest day has a real load of `0`. An `avg_heart_rate` of `0`
SHALL be rejected as an invalid metric, because the mean of a non-empty
sample is never `0`.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/ai/payload.go`

#### Scenario: an unmeasured aggregate is omitted and an explicit zero is sent

- GIVEN a builder for `kind = "activity"` with `DistanceMeters(10000)`
  and `DurationSeconds(3600)` and no `TrainingLoad`
- WHEN `Build` is called
- THEN the payload marshals to JSON with `kind`, `window_days`,
  `distance_meters` and `duration_seconds`
- AND the JSON has no `training_load` key
- GIVEN the same builder plus `TrainingLoad(0)`
- WHEN `Build` is called and the payload is marshalled
- THEN the JSON carries `training_load` equal to `0`

#### Scenario: an impossible value is rejected

- GIVEN a builder for `kind = "activity"` with `DistanceMeters(-1)`
- WHEN `Build` is called
- THEN it returns an error
- GIVEN a builder with `AvgHeartRate(0)`
- WHEN `Build` is called
- THEN it returns an error
- GIVEN a builder with `TrainingLoad(1001)` (above `MaxTrainingLoad`)
- WHEN `Build` is called
- THEN it returns an error

#### Scenario: the payload carries no identity, no GPS and no raw stream

- GIVEN a fully populated builder for `kind = "load"` with a window of
  28 days
- WHEN the payload is marshalled to JSON
- THEN the JSON object has exactly the keys `kind`, `window_days`,
  `distance_meters`, `duration_seconds`, `elevation_gain_meters`,
  `avg_heart_rate` and `training_load`
- AND no key contains a name, an e-mail address, a coordinate or a
  stream

### Requirement: the payload ceilings scale with the analysed window (`AB-003`)

`internal/ai/payload.go` SHALL expose `LimitsFor(kind Kind,
windowDays int) Limits` and SHALL apply it as the ceiling of every
metric. For the four **summed** aggregates (`distance_meters`,
`duration_seconds`, `elevation_gain_meters`, `training_load`) the
ceiling SHALL be the per-day ceiling multiplied by the number of days of
the window, where the per-day ceiling is `MaxDistanceMeters`
(1 000 000 m), `MaxDurationSeconds` (604 800 s), `MaxElevationGainMeters`
(50 000 m) and `MaxTrainingLoad` (1 000). A payload of kind `activity`
SHALL NEVER scale: it keeps the per-activity ceilings. `avg_heart_rate`
SHALL NEVER scale in any window, because it is a mean and not a sum.
`windowDays` SHALL be clamped to `1..MaxWindowDays` (90), and a window
longer than `MaxWindowDays` SHALL be rejected rather than silently
extended.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/ai/payload.go` (`LimitsFor`)

#### Scenario: a 28-day window admits the totals of 28 days

- GIVEN a builder for `kind = "load"` with a window of 28 days and a
  `training_load` of 28 000 (28 × the per-day ceiling of 1 000)
- WHEN `Build` is called
- THEN it returns no error
- GIVEN the same builder with a `training_load` of 28 001
- WHEN `Build` is called
- THEN it returns an error

#### Scenario: an activity keeps its per-activity ceilings

- GIVEN a builder for `kind = "activity"` with a `training_load` of
  1 000
- WHEN `Build` is called
- THEN it returns no error
- GIVEN a builder for `kind = "activity"` with a `training_load` of
  1 001
- WHEN `Build` is called
- THEN it returns an error

#### Scenario: the average heart rate never scales

- GIVEN a builder for `kind = "load"` with a window of 90 days and an
  `avg_heart_rate` of 240
- WHEN `Build` is called
- THEN it returns no error
- GIVEN the same builder with an `avg_heart_rate` of 241
- WHEN `Build` is called
- THEN it returns an error

### Requirement: every provider shares one strict output schema and one validator (`AB-004`)

`internal/ai/output.go` SHALL expose `OutputSchema() Schema` — the JSON
schema `training_analysis` with `strict: true`,
`additionalProperties: false`, a required `summary` (1..1200
characters), `observations` (at most 10 items of 1..500 characters) and
`cautions` (at most 10 items of 1..500 characters) — and SHALL deliver
that same schema to every provider. `ValidateOutput(raw []byte, sent
Payload) error` and `ParseOutput(raw []byte, sent Payload) (Output,
error)` SHALL apply the contract of that schema and SHALL reject: a
missing required field, an unknown field, a wrong type, a `null` value
(including at the root), more than one JSON document, a length out of
range, more than 10 observations or cautions, a diagnostic vocabulary
(cardiopatía, arritmia, diagnóstico, enfermedad, lesión, medicación) and
**any number that was not part of the payload that was sent** (the
sent metrics plus `window_days`). A rejected response SHALL be
discarded exactly as if the provider had failed.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/ai/output.go`

#### Scenario: a well-formed response is accepted

- GIVEN a payload with `distance_meters = 10000` and
  `window_days = 7`
- WHEN a response `{"summary":"Sesión sólida","observations":["Subida
  constante"],"cautions":[]}` is validated
- THEN the validation returns no error
- AND the parsed output carries the summary, the observation and the
  empty list of cautions

#### Scenario: an unknown field, a null and a second document are rejected

- GIVEN a response that adds a `metrics` field to the valid output
- WHEN it is validated
- THEN the validation returns an error
- GIVEN a response whose `summary` is `null`
- WHEN it is validated
- THEN the validation returns an error
- GIVEN a response made of two JSON documents
- WHEN it is validated
- THEN the validation returns an error

#### Scenario: an invented metric is rejected

- GIVEN a payload with `distance_meters = 10000`
- WHEN a response whose summary contains the number `48` (a duration in
  hours that was never sent) is validated
- THEN the validation returns an error
- AND the failure reason is a schema violation

#### Scenario: a diagnostic statement is rejected

- GIVEN a response whose cautions contain the word "diagnóstico"
- WHEN it is validated
- THEN the validation returns an error

### Requirement: the global AI configuration never blocks the boot (`AB-005`)

`internal/config/config.go` SHALL expose `Config.AI AIConfig` with the
fields `Enabled`, `Available`, `Provider`, `Model`, `APIKey`, `Timeout`,
`MaxRetries`, `UnavailableReason` and `Warnings`, and
`AIConfig.Status()` SHALL return `disabled` when `AI_ENABLED` is off,
`unavailable` when the flag is on but the configuration cannot be used,
and `ready` otherwise. `config.Load()` SHALL NEVER fail because of the
AI: `loadAIConfig()` SHALL NOT return an error, and every misconfiguration
SHALL leave a warning that names environment variables only (never a
credential) and leave the AI in state `unavailable`. `AI_ENABLED` SHALL
default to `false`, and when it is off the loader SHALL NOT read any
credential. Only the credential of the selected provider SHALL be read
into memory (`AI_OPENAI_API_KEY`, `AI_CLAUDE_API_KEY` or
`AI_OPENROUTER_API_KEY`); the other two SHALL NOT be loaded. The model
(`AI_MODEL`) SHALL be configured by environment in every case: an empty
`AI_MODEL` SHALL leave the AI `unavailable` with a warning, and no
default model SHALL ever be assumed. `AI_TIMEOUT` (default `30s`) and
`AI_MAX_RETRIES` (default `3`, range `0..10`) SHALL be validated even
while the AI is off, so a typo is visible before the flag is turned on.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/config/config.go` (`loadAIConfig`, `AIConfig.Status`)

#### Scenario: with the flag off no credential is read and the status is disabled

- GIVEN the environment variable `AI_ENABLED` is unset and
  `AI_OPENAI_API_KEY` is set
- WHEN the configuration is loaded
- THEN `cfg.AI.Enabled` is `false`
- AND `cfg.AI.Status()` is `"disabled"`
- AND `cfg.AI.APIKey` is empty
- AND `cfg.AI.Available` is `false`

#### Scenario: a missing model leaves the AI unavailable without failing the boot

- GIVEN `AI_ENABLED=true`, `AI_PROVIDER=openai`,
  `AI_OPENAI_API_KEY=secret` and no `AI_MODEL`
- WHEN the configuration is loaded
- THEN `cfg.AI.Status()` is `"unavailable"`
- AND `cfg.AI.Warnings` contains a warning that names `AI_MODEL`
- AND no warning contains the value `secret`
- AND the configuration load itself returns no error

#### Scenario: an unsupported provider leaves the AI unavailable

- GIVEN `AI_ENABLED=true`, `AI_PROVIDER=gemini` and a valid
  `AI_MODEL`
- WHEN the configuration is loaded
- THEN `cfg.AI.Status()` is `"unavailable"`
- AND `cfg.AI.Warnings` names `AI_PROVIDER`
- AND the configuration load itself returns no error

#### Scenario: only the credential of the selected provider is loaded

- GIVEN `AI_ENABLED=true`, `AI_PROVIDER=claude`,
  `AI_CLAUDE_API_KEY=claude-key`, `AI_OPENAI_API_KEY=openai-key` and a
  valid `AI_MODEL`
- WHEN the configuration is loaded
- THEN `cfg.AI.APIKey` is `claude-key`
- AND `cfg.AI.Status()` is `"ready"`
- AND the openai key appears nowhere in the configuration

### Requirement: the per-user consent is explicit and fresh (`AB-006`)

`internal/db/migrations/00014_ai_explicit_opt_in.sql` SHALL set the
default of `users.ai_enabled` to `false` and SHALL reset every stored
`true` to `false`, because an inherited `true` cannot prove that the
user ever accepted the transfer notice. The `goose Down` section SHALL
NOT restore `DEFAULT true` and SHALL NOT rewrite any stored row, so a
rollback can never consent an account. The `PATCH /api/v1/me/preferences`
contract SHALL be extended with an optional `ai_enabled` field whose
omission is decoded as `false`: only an explicit `true` SHALL grant the
consent, and only an explicit `false` SHALL revoke it. The profile form
(`web/src/features/profile/PreferencesForm.tsx`) SHALL state, next to the
checkbox, which aggregates may leave the system (distance, duration,
elevation gain, heart rate, calculated load) and which never do
(identity, name, e-mail address, GPS, raw streams).

**Project root**: `.` and `web/`
**Gating test command**: `make test` (Go) and `pnpm -C web test:run`
(web)
**Pin point**: `internal/db/migrations/00014_ai_explicit_opt_in.sql`, `internal/http/handlers/me_preferences.go`, `web/src/features/profile/PreferencesForm.tsx`

#### Scenario: the migration resets the inherited consents and never consents on rollback

- GIVEN a database in which one user has `ai_enabled = true`
- WHEN the migration `00014` is applied
- THEN the default of `users.ai_enabled` is `false`
- AND that user has `ai_enabled = false`
- WHEN the migration `00014` is rolled back
- THEN no stored value is rewritten
- AND the default of `users.ai_enabled` is still `false`

#### Scenario: omitting the field is never consent

- GIVEN an authenticated user without consent
- WHEN `PATCH /api/v1/me/preferences` is called with a body that does
  not mention `ai_enabled`
- THEN the stored `ai_enabled` is `false`
- WHEN the same call is made with `ai_enabled: true`
- THEN the stored `ai_enabled` is `true`

#### Scenario: the profile states what may and may not leave the system

- GIVEN the profile preferences form
- WHEN the user opens it
- THEN the transfer notice names the allowed aggregates (distance,
  duration, elevation gain, heart rate, calculated load)
- AND the transfer notice names the exclusions (identity, name, e-mail
  address, GPS, raw streams)
- AND the consent checkbox is unchecked by default

### Requirement: the analyses are persisted in `ai_analysis`, scoped to their owner (`AB-007`)

`internal/db/migrations/00015_ai_analysis.sql` SHALL create the table
`ai_analysis (id UUID PRIMARY KEY, user_id UUID NOT NULL REFERENCES
users(id) ON DELETE CASCADE, kind TEXT NOT NULL, scope TEXT NOT NULL,
status TEXT NOT NULL DEFAULT 'pending', provider TEXT NULL, model TEXT
NULL, payload JSONB NOT NULL DEFAULT '{}'::jsonb, output JSONB NULL,
error_code TEXT NULL, error_message TEXT NULL, created_at TIMESTAMPTZ
NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
UNIQUE (user_id, kind, scope))` with `CHECK` constraints that restrict
`kind` to `activity | weekly | load`, `status` to
`pending | completed | failed`, `scope` to a non-empty string of at
most 128 characters, `error_code` to `^[a-z0-9_]{1,64}$` and
`error_message` to at most 500 characters, so that a provider body or a
credential cannot fit in the error columns. The `payload` column SHALL
hold the aggregate payload that was actually sent and the `output`
column the validated interpretation. Every query that reads or writes a
row SHALL filter by `user_id`; a row of another user SHALL behave as if
it did not exist (`pgx.ErrNoRows`). A repeated request for the same
`(user_id, kind, scope)` SHALL reset the same row to `pending` and
clear its previous result instead of creating a duplicate.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/db/migrations/00015_ai_analysis.sql`, `internal/db/queries/ai_analysis.sql`

#### Scenario: a repeated request reuses the same row

- GIVEN a user with an `activity` analysis for the scope
  `activity:<uuid>`
- WHEN the request is repeated for the same activity
- THEN the `ai_analysis` table contains exactly one row for that
  `(user_id, kind, scope)`
- AND its status is `pending` again
- AND its previous `output` and `error_code` are cleared

#### Scenario: the error columns cannot hold a provider body

- GIVEN a row in `ai_analysis`
- WHEN an `error_code` of `Provider body with details` is attempted
- THEN the write is rejected by the `CHECK` constraint
- WHEN an `error_message` longer than 500 characters is attempted
- THEN the write is rejected by the `CHECK` constraint

#### Scenario: a row of another user does not exist for the reader

- GIVEN a row owned by user A for a given `(kind, scope)`
- WHEN `GetAIAnalysis` is executed for user B with the same kind and
  scope
- THEN it returns `pgx.ErrNoRows`
- AND `ListAIAnalyses` for user B returns no row of user A

### Requirement: the River job is the only egress and rechecks the consent before sending (`AB-008`)

`internal/jobs/ai_analysis.go` SHALL implement the River worker
`ai_analysis` (`AIAnalysisArgs{UserID, AnalysisKind, Scope,
ActivityID, WindowStart, WindowDays}`), and it SHALL be the only unit
of the product that contacts an AI provider. The worker SHALL execute
its checks in this order: (1) the consent (`users.ai_enabled`), (2) the
global configuration, (3) the scope and the window, (4) the aggregate
payload, (5) the provider call, (6) the persistence, (7) the retry
decision. A revoked consent SHALL mark the row `failed` with
`consent_revoked`, SHALL NOT call the provider and SHALL NOT return an
error to River. The payload SHALL be built only from already stored
deterministic metrics: for `activity`, the activity row loaded by
`(id, user_id)` (distance, moving time, elevation gain, average heart
rate and the stored load of the day); for `weekly` and `load`, the
activities and the stored daily load of the window, with the average
heart rate weighted by duration. The provider SHALL be called exactly
once per attempt with a context bounded by the configured timeout. The
row SHALL be closed as `completed` (with the exact payload that was
sent and the validated output) or as `failed` (with a safe code and
message).

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/jobs/ai_analysis.go`

#### Scenario: a revoked consent stops the analysis before the provider is called

- GIVEN a user whose `ai_enabled` is `false` and a job for one of their
  activities
- WHEN the worker runs
- THEN the analysis row is `failed` with `error_code = "consent_revoked"`
- AND the analyzer recorded zero calls
- AND the worker returned no error

#### Scenario: an activity of another user is never analysed

- GIVEN a job that points at an activity owned by another user
- WHEN the worker runs
- THEN the analysis row is `failed` with `error_code = "not_found"`
- AND the analyzer recorded zero calls

#### Scenario: a successful analysis stores the sent payload and the validated output

- GIVEN a consented user with an activity whose metrics fit the payload
  contract
- WHEN the worker runs and the provider answers a valid output
- THEN the analysis row is `completed`
- AND its `payload` holds exactly the aggregate payload that was sent
- AND its `output` holds the validated summary, observations and cautions
- AND its `provider` and `model` are the ones used
- AND the analyzer recorded exactly one call

### Requirement: the retry budget is bounded and the failure classification decides the backoff (`AB-009`)

`internal/jobs/ai_analysis.go` SHALL expose
`AIAnalysisMaxAttempts(cfg config.AIConfig) int`, which SHALL return
`1 + AI_MAX_RETRIES` (one initial attempt plus the configured retries)
and SHALL never exceed `1 + config.MaxAIMaxRetries` (11). The API
SHALL enqueue the job with `river.InsertOpts{MaxAttempts:
AIAnalysisMaxAttempts(cfg)}`, and River SHALL apply its default
exponential backoff. The failure classification SHALL be the one of
`internal/ai`: a retryable failure (`rate_limited`, `provider_error`,
`network`, `timeout`) SHALL make the worker return the error so that
River retries it, and a permanent failure (`unauthorized`, `forbidden`,
`bad_request`, `invalid_model`, `refusal`, `truncated`, `invalid_json`,
`non_json_response`, `oversized_body`, `redirect`, `unexpected_response`,
`schema_violation`, `canceled`) SHALL make the worker finish without an
error. Between attempts the row SHALL stay `failed`; a successful retry
SHALL close it as `completed`.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/jobs/ai_analysis.go` (`AIAnalysisMaxAttempts`, `run`)

#### Scenario: the attempt budget follows the configuration

- GIVEN a configuration with `MaxRetries = 3`
- WHEN `AIAnalysisMaxAttempts` is called
- THEN it returns `4`
- GIVEN a configuration with `MaxRetries = 50`
- WHEN `AIAnalysisMaxAttempts` is called
- THEN it returns `11`

#### Scenario: a retryable provider failure is returned to River and a permanent one is not

- GIVEN a provider that answers `429`
- WHEN the worker runs
- THEN the worker returns an error
- AND the analysis row is `failed` with `error_code = "rate_limited"`
- GIVEN a provider that answers `401`
- WHEN the worker runs
- THEN the worker returns no error
- AND the analysis row is `failed` with `error_code = "unauthorized"`

### Requirement: the stored failures are safe tokens that carry no provider data (`AB-010`)

`internal/jobs/ai_analysis.go` SHALL write `ai_analysis.error_code`
and `ai_analysis.error_message` using a bounded vocabulary whose codes
match `^[a-z0-9_]{1,64}$` and whose messages are short phrases written
by the application. The product's own codes SHALL be
`consent_revoked`, `ai_disabled`, `ai_unavailable`, `not_found`,
`invalid_scope`, `invalid_window`, `window_too_large`, `invalid_metrics`,
`provider_mismatch` and `analysis_failed`; a provider failure SHALL store
the `Reason` of the classified `*ai.Error` (for example `rate_limited`,
`timeout`, `network`, `provider_error`, `unauthorized`). Neither column
SHALL ever carry the provider's response body, the sent payload, the
credential or the identity of the user, and the message SHALL be
truncated to the 500 characters allowed by the schema.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/jobs/ai_analysis.go` (`failAnalysis`, `sanitizeAICode`, `truncateAIMessage`)

#### Scenario: the stored code is one of the safe tokens

- GIVEN a job whose scope does not match its explicit arguments
- WHEN the worker runs
- THEN the analysis row is `failed` with `error_code = "invalid_scope"`
- AND the stored `error_message` is shorter than 500 characters
- AND neither column contains the identifier of the provider response

#### Scenario: a window outside the range fails with a code instead of being extended

- GIVEN a job for kind `load` with a window of 120 days
- WHEN the worker runs
- THEN the analysis row is `failed` with `error_code = "window_too_large"`
- AND the analyzer recorded zero calls

### Requirement: the scope keys identify the target deterministically (`AB-011`)

`internal/ai/scope.go` SHALL build the scope key of every analysis and
SHALL be the single source of truth shared by the API and the job:
`ai.ActivityScope(id)` returns `activity:<uuid>`,
`ai.WeeklyScope(start)` returns `weekly:<YYYY-MM-DD>` (the first day of
the window) and `ai.LoadScope(start, days)` returns
`load:<YYYY-MM-DD>:<days>`. `ai.ParseScope(scope)` SHALL invert those
keys and SHALL return an error for an unknown or malformed key. The
weekly analysis SHALL have a **fixed** window of 7 days
(`jobs.AIAnalysisWindowDays`), because the weekly key does not encode
the length of the window; the load analysis SHALL accept 1..90 days
(`ai.MaxWindowDays`). The days of a window SHALL be counted in the
timezone of the user (`users.timezone`), and the first day of the window
SHALL NOT be moved by a timezone offset.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/ai/scope.go`, `internal/jobs/ai_analysis.go` (`resolveAITarget`)

#### Scenario: the API and the job derive the same key

- GIVEN an activity with the id `33333333-3333-3333-3333-333333333333`
- WHEN `ai.ActivityScope` is called with that id
- THEN it returns `activity:33333333-3333-3333-3333-333333333333`
- AND `ai.ParseScope` of that key returns the same id and the kind
  `activity`

#### Scenario: the weekly key encodes the start date and the load key also the length

- GIVEN a window that starts on `2026-01-05`
- WHEN `ai.WeeklyScope` is called
- THEN it returns `weekly:2026-01-05`
- WHEN `ai.LoadScope` is called with the same start and 28 days
- THEN it returns `load:2026-01-05:28`
- AND `ai.ParseScope` of that key returns the kind `load`, the date
  `2026-01-05` and 28 days

#### Scenario: a malformed scope is rejected

- GIVEN the scope `weekly:not-a-date`
- WHEN `ai.ParseScope` is called
- THEN it returns an error
- GIVEN the scope `load:2026-01-05`
- WHEN `ai.ParseScope` is called
- THEN it returns an error

### Requirement: with the AI off or failing, the deterministic product keeps working (`AB-012`)

The product SHALL behave exactly as before the AI feature whenever the
AI is disabled, unavailable or failing: no credential SHALL be read, no
provider SHALL be contacted, and no endpoint outside `/api/v1/ai` SHALL
change its behaviour. `internal/app/app.go` SHALL build the analyzer
only from a valid global configuration (`buildAIAnalyzer`), and SHALL
return no analyzer — and therefore no AI worker — when the AI is
disabled, unavailable or misconfigured, instead of failing the boot. The
`GET /api/v1/ai/status` endpoint SHALL answer `200` with
`status = "disabled"` while the AI is off. `POST /api/v1/ai/analyses`
SHALL answer `503` without creating any row while the AI is off or
unavailable, or while no job queue is configured. The worker SHALL
finish **without an error** when the AI is off or unavailable, marking
the row `failed` with `ai_disabled` or `ai_unavailable`, so that the
queue is not consumed by retries of a decision that will not change.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/app/app.go` (`buildAIAnalyzer`), `internal/jobs/ai_analysis.go`

#### Scenario: a broken AI configuration does not stop the boot

- GIVEN a configuration with `AI_ENABLED=true` and an invalid provider
- WHEN the application is wired
- THEN the analyzer is not built
- AND no error is returned
- AND the deterministic routes of the API are mounted as usual

#### Scenario: the worker finishes without an error while the AI is off

- GIVEN a job for a consented user and a configuration with
  `Enabled = false`
- WHEN the worker runs
- THEN the analysis row is `failed` with `error_code = "ai_disabled"`
- AND the analyzer recorded zero calls
- AND the worker returned no error

#### Scenario: the API answers 503 without creating a row while the AI is off

- GIVEN the global flag `AI_ENABLED` is off
- WHEN `POST /api/v1/ai/analyses` is invoked by an authenticated,
  consented user
- THEN the response status is `503`
- AND no row was created in `ai_analysis`

### Requirement: the job queue starts with Strava or with an active AI (`AB-013`)

`internal/app/app.go` SHALL start the River client and its workers when
`cfg.Strava != nil` **or** when an AI analyzer was built, so an
instance without Strava credentials can still process the AI analyses.
`internal/jobs/workers.go` SHALL register the `ai_analysis` worker
whenever an analyzer exists, and SHALL NOT register it when the AI is
off. When neither Strava nor the AI is configured, the application SHALL
keep running in API-only mode without a job queue.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/app/app.go`, `internal/jobs/workers.go`

#### Scenario: the queue starts without Strava when the AI is active

- GIVEN a configuration without Strava credentials and with a valid AI
  configuration
- WHEN the application is wired
- THEN the job queue is started
- AND the `ai_analysis` worker is registered

#### Scenario: without Strava and without an active AI there is no queue

- GIVEN a configuration without Strava credentials and with the AI
  disabled
- WHEN the application is wired
- THEN the job queue is not started
- AND the `ai_analysis` worker is not registered
- AND the API is still mounted

### Requirement: the epic is delivered as an ordered chain of work units on one branch (`AB-014`)

The capability SHALL be delivered as a chain of small work units on the
branch `feat/phase-1-5-optional-ai`, based on `main@a6adff0`, so that
each unit can be reviewed and reverted on its own before the epic is
merged into `main`. The units SHALL be delivered in this order:
the domain contract (T1), the consent unit (T2), the configuration and
persistence unit (T4), the provider adapters (T3), the River job (T5),
the application wiring (T6), the window-aware payload ceilings (T10),
the HTTP API and its TypeScript client (T7) and the web controls (T8).
Each unit SHALL be green on the test command of its own project before
the next unit starts, and each unit SHALL be revertible without leaving
the branch in a state that does not compile.

**Project root**: `.` and `web/`
**Gating test command**: `make test` (Go) and `pnpm -C web test:run`
(web)
**Pin point**: the branch history of `feat/phase-1-5-optional-ai`

#### Scenario: the work units appear in dependency order

- GIVEN the branch `feat/phase-1-5-optional-ai` based on `main@a6adff0`
- WHEN the commits of the branch are listed from the base
- THEN they appear in this order: `99db0a9` (contract), `49f8585`
  (consent), `fc6c54f` (configuration and persistence), `8d823f9`
  (adapters), `5cdd64c` (job), `b8d9360` (wiring), `67be97a` (window
  ceilings), `4de56a5` (API), `8265703` (web)
- AND each of them is a self-contained commit with its own subject
  line

#### Scenario: the chain covers both projects and adds no dependency

- GIVEN the difference between `main@a6adff0` and the tip of the branch
- WHEN the changed manifests are inspected
- THEN no new dependency was added to `go.mod`, `go.sum`,
  `web/package.json` or `web/pnpm-lock.yaml`
