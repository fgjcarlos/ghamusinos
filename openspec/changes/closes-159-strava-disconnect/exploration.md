# Exploration: closes-159-strava-disconnect

> Phase 1.1 (preferences completion) + Phase 1.2 (Strava management).
> Investigates the gaps left in #159 by PR #225 and the Strava
> connection lifecycle beyond OAuth handshake. This exploration is
> the evidence base for the proposal/design/spec chain.

## Problem statement

Issue #159 (preferences + /perfil) is **partially closed**. The
squash commit `081d318` (PR #225) merged the endpoints, validation,
and the SPA skeleton, but **two acceptance criteria remain unmet**:

1. **"Los errores 422 del backend se pintan junto al campo que los
   provoca, no en un banner genérico."**
   `web/src/features/profile/PreferencesContainer.tsx:74-82` routes
   every `ApiError(422)` to a single `errors.timezone` field plus a
   global "No se pudieron guardar las preferencias" banner. The
   backend's `preferencesPatch.validate()` (handlers/me_preferences.go:122-146)
   returns only one string per request, so the container has nothing
   more granular to map. The result is that out-of-range `hr_max` and
   invalid `level` both surface under the timezone input.

2. **Strava disconnection is unsupported.** The `ConnectionCard.tsx`
   top-of-file comment (lines 1-8) documents that
   `DELETE /api/v1/strava/connection` is **intentionally not
   rendered** because the endpoint does not exist. `internal/strava/oauth.go:30`
   confirms: *"No expone endpoints de 'desconectar' (se hace con
   DeleteStravaTokensByUserID)"*. The query lives in
   `internal/db/sqlc/strava_tokens.sql.go:14` but no HTTP handler
   wraps it.

The Connect button is also broken in a related way:
`ConnectionCard.tsx:73-77` renders a `<Link to="/api/v1/strava/connect">`
anchor. The handler at `internal/strava/oauth.go:189-217` returns
JSON `{"authorize_url": "..."}` — it does not redirect. Following the
link in a browser hits the JSON response, not Strava. The intended
flow is documented in `web/src/lib/api/strava.ts:90-105`
(`getStravaAuthorizeURL` + `window.location.assign(url)`) and that
client function exists but is unused by the card.

The profile page renders `connected: false` hardcoded
(`web/src/routes/profile.tsx:11`). Without a
`GET /api/v1/strava/connection`, the SPA cannot know the real state
and `athlete_id` / `scopes` / `last_sync` shown after a connect are
fiction.

## Files and call-sites that prove the gap

| File | Line(s) | Evidence |
|---|---|---|
| `web/src/features/profile/PreferencesContainer.tsx` | 74-82 | `setErrors({ timezone: detail })` + `setStatus('error')` — single-field, global banner |
| `web/src/features/profile/PreferencesForm.tsx` | 33-67 | Per-field `errors` already wired and rendered under each `<Field>` |
| `web/src/features/profile/ConnectionCard.tsx` | 1-8 | Comment admits DELETE endpoint absent |
| `web/src/features/profile/ConnectionCard.tsx` | 73-77 | `<Link to="/api/v1/strava/connect">` — wrong mechanism (handler returns JSON) |
| `web/src/routes/profile.tsx` | 9-14 | `connected: false` hardcoded; comment names the missing GET |
| `internal/http/handlers/me_preferences.go` | 122-146 | `validate() string` returns first error only |
| `internal/http/handlers/me_preferences.go` | 73-77 | `NewUnprocessableEntity(msg, ...)` flattens to `ProblemDetail.Detail` |
| `internal/http/handlers/errors.go` | 9-15 | `ProblemDetail` has no extension members; only scalar fields |
| `internal/strava/oauth.go` | 30 | "No expone endpoints de 'desconectar'" |
| `internal/strava/oauth.go` | 189-217 | `ConnectHandler` returns `{"authorize_url": "..."}` JSON |
| `internal/db/sqlc/strava_tokens.sql.go` | 14, 20 | `DeleteStravaTokensByUserID` query exists, exported |
| `internal/db/queries/strava_tokens.sql` | 25 | Source SQL for the delete |
| `internal/auth/resolver_test.go` | 238 | Mock already implements the delete (returns nil) |
| `internal/http/router.go` | 198, 224 | `/me` and `/strava/connect` are mounted; no `/strava/connection` |
| `web/src/lib/api/strava.ts` | 80-105 | `getStravaAuthorizeURL` exists; no `getStravaConnection`/`disconnectStrava` |
| `web/src/lib/api/types.ts` | 89-100 | `ApiError` has `status`+`message`; no field-level detail |

## What the existing code already supports

- **HTTP plumbing**: `router.go` already groups `/api/v1/*` under the
  auth middleware and mounts `/me`, `/me/preferences`, `/strava/connect`
  — adding `/strava/connection` (GET, DELETE) is a one-line
  `r.Route("/strava", ...)` extension.
- **DB layer**: `DeleteStravaTokensByUserID` is sqlc-generated and
  every test mock already implements it. Zero SQL changes.
- **Validation predicates**: the field-level validators in
  `validate()` are already factored — they just need a return type
  change from `string` to `map[string]string`.
- **UI scaffolding**: `PreferencesForm` already accepts `errors`
  per-field and renders them with `role="alert"` under each input;
  `ConnectionCard` already has a `connected` branch and tests
  cover it; only the Disconnect button + correct connect flow are
  missing.
- **TS API client**: `getStravaAuthorizeURL` already exists and
  documents the `window.location.assign(url)` invariant.
- **Tests pattern**: `handlers/me_preferences_test.go` builds a
  `preferencesMockQuerier` for the Querier interface. Adding two
  HTTP handlers that need only `DeleteStravaTokensByUserID` and
  `GetStravaTokensByUserID` extends the existing test cleanly.

## Constraints / risks

- **Wire-shape compatibility**: the 422 body changes from a flat
  string in `ProblemDetail.Detail` to a structured `errors` map
  alongside `Detail`. Existing consumers (only the `/perfil` form)
  must be updated in the same PR; the curl shape used by tests
  changes from substring-match on `Body.String()` to JSON-parse +
  per-field lookup.
- **RFC 9457 §3.1**: extension members are allowed in problem
  details as long as they do not clash with standard fields. We add
  `errors` (plural, lowercase, snake_case-keyed map). No conflict.
- **Strava DELETE is destructive**: removes encrypted tokens but
  not the imported activities. The frontend should confirm before
  the call. Out of scope for this slice — the disconnect button is
  rendered unconditionally and the user clicks it knowingly. The
  button is honest: it does not claim to delete historical
  activities, only the OAuth link.
- **Auth context**: `auth.AuthUser(r.Context())` returns nil for
  unauthenticated requests; the handler MUST 401 in that case (the
  existing `/me/preferences` pattern).
- **Strava callbacks re-fire**: if the user disconnects and
  reconnects, the OAuth callback fires `UpsertStravaTokens` again.
  The existing `upsert` query handles this; no schema change.
- **Last-sync**: not stored in `strava_tokens`. The card renders
  `last_sync` from `StravaToken.UpdatedAt` (already populated by
  the `UpsertStravaTokens` query), so the GET handler returns it
  without a schema change.
- **Profile page route**: `/perfil` is already mounted. No router
  change in PR1 (frontend-only in PR2).

## Out-of-scope (explicit non-goals)

- **OAuth handshake / refresh / webhooks** — already shipped under
  issues #14 and ADR-0001.
- **Deleting imported Strava activities** — destructive, no user
  signal in this slice. Future separate change.
- **Revoking the Strava grant server-side** — Strava offers
  `DELETE /oauth/deauthorize` but it requires a separate API call
  with the access token; the OAuth handshake design (ADR-0001)
  deliberately keeps the cipher key on our side and treats the
  server-side deauthorize as a follow-up.
- **A "Connect" button on `/perfil` that goes through the connect
  flow** — yes, this slice fixes that flow. PR1 only adds the
  DELETE handler; PR2 wires the actual `getStravaAuthorizeURL` call
  and the disconnect button.
- **Web coverage / chained PR strategy** — covered by
  `openspec/config.yaml` `rules.proposal` and the 400-line budget
  we picked at the planning step.

## Drift discovered

None at the architecture level. The drift is at the **wire-shape**
level: the SPA was written assuming per-field 422 errors, but the
backend only sends one flat message. This is a contract gap, not a
code drift — both sides need to move together. PR1 contains both.

## Decision summary

A two-PR chain, decided with the user up front:

- **PR1 — Contracts + DELETE**: refactor `validate()` to return
  per-field errors, extend `ProblemDetail` with `errors` extension
  member, fix `PreferencesContainer` to map errors by field, add
  `DELETE /api/v1/strava/connection`, mount in router, Go tests.
- **PR2 — GET + UI**: add `GET /api/v1/strava/connection`, TS API
  client (`getStravaConnection`, `disconnectStrava`), refactor
  `ConnectionCard` to use `window.location.assign(authorizeURL)`
  and add a Disconnect button, replace mock state in
  `routes/profile.tsx` with a real fetch, React tests.

PR1 is self-contained: the new DELETE handler is reachable through
`curl` with a bearer token, the new 422 shape is exercised by Go
tests, and `PreferencesContainer` is fixed against the new
backend. PR2 has a clean UI-only diff on top.

PR1 is the slice this exploration feeds; PR2 gets its own
exploration and proposal when PR1 merges.