# Proposal: closes-159-strava-disconnect (PR1 — contracts + DELETE)

> Phase 1.1 (preferences completion) + Phase 1.2 (Strava
> disconnect). This is the **first of two chained PRs** that close
> the remaining acceptance criteria of #159. PR1 ships the contract
> change and the destructive HTTP endpoint; PR2 ships the read
> endpoint, the React wiring, and the disconnect button.

## Why

PR #225 (squashed to `081d318`) closed most of #159 but left two
real gaps documented in the exploration:

1. **422 errors do not surface per field.** The SPA renders them
   under the timezone input and a global red banner. The backend
   only ships one string per request, so even a clean fix on the
   container side has nothing to map. The contract needs an
   extension member so the SPA can paint the right field.

2. **Strava cannot be disconnected.** The DELETE handler does not
   exist; the Connect button is also wired wrong (uses `<Link>` to
   an endpoint that returns JSON, not a redirect). PR1 only adds
   the DELETE handler — the wrong-connect-flow fix is part of PR2
   where the React side changes anyway.

Both gaps are blocking criteria in the #159 acceptance list and
block Phase 1.2's Strava management story (you cannot connect,
then realise you cannot disconnect, and ship a product).

## What Changes

### Backend (`internal/http/handlers/`)

- **`errors.go`**: add `Errors map[string]string \`json:"errors,omitempty"\``
  to `ProblemDetail`. This is an RFC 9457 §3.1 extension member.
  Tests that string-match the body MUST be updated to JSON-decode
  and look up `errors.{field}`. Add a constructor
  `NewUnprocessableEntityFields(detail, instance string, fields map[string]string)`
  that sets both `Detail` (the legacy scalar summary, kept for
  backward compatibility with any external consumer) and `Errors`.

- **`me_preferences.go`**: change `validate()` from
  `func (b *preferencesPatch) validate() string` to
  `func (b *preferencesPatch) validate() map[string]string`.
  Each field check becomes one entry in the map. Empty map =
  valid. The handler in `PatchPreferences` switches from
  `if msg := body.validate(); msg != ""` to
  `if errs := body.validate(); len(errs) > 0` and calls
  `NewUnprocessableEntityFields(...)`. The cross-field warning
  (`lthr > hr_max`) is unchanged.

- **New file `strava_connection.go`**: handler factory
  `DeleteStravaConnection(q sqlc.Querier) http.Handler`. Reads
  `auth.AuthUser(r.Context())`. On nil → 401 problem. On valid →
  `q.DeleteStravaTokensByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})`.
  On DB error → 500 problem. On success → `204 No Content`. Same
  shape as the existing `/me/preferences` handlers; lives next to
  the other `me_*` handlers for discoverability rather than in
  the `strava` package (the OAuth flow lives there; this is a
  normal CRUD endpoint).

- **`me_preferences_test.go`**: rewrite the `TestPatchPreferences_RejectsOutOfRange`
  cases so each subtest asserts `ProblemDetail.Errors[field]`
  contains the expected substring. The flat-message substring
  match stays for `Detail` (legacy compatibility).

### Router (`internal/http/router.go`)

- Inside the `/api/v1` group, add a new sub-route
  `r.Route("/strava", func(r chi.Router) { r.Delete("/connection", handlers.DeleteStravaConnection(s.queries).ServeHTTP) })`.
  Position next to the existing `/strava/connect` mount.
  `router_test.go` extended to assert the route resolves and that
  an unauthenticated request returns 401.

### Frontend (`web/src/features/profile/PreferencesContainer.tsx`)

- The 422 handler currently does
  `setErrors({ timezone: detail })`. Change to
  `setErrors(err.fields)` where `err.fields` is a new optional
  property on `ApiError` carrying the parsed `errors` map.
- The global `status === 'error'` banner stays — it remains the
  honest signal when the server fails in a non-field way (500,
  network).
- `ApiError` in `web/src/lib/api/types.ts` gains an optional
  `fields?: Record<string, string>` populated when the 422 body
  has an `errors` extension member.

### Out of scope (PR1)

- The Connect button mechanism (`<Link>` vs `window.location.assign`).
  It still misroutes today but PR1 does not touch the React side
  beyond `PreferencesContainer` + `ApiError`.
- The GET endpoint `GET /api/v1/strava/connection`. PR2.
- Adding a Disconnect button to `ConnectionCard.tsx`. PR2.

## Wire-shape diff

### Before (`ProblemDetail` 422 body)

```json
{
  "type": "about:blank",
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "hr_max must be between 1 and 260 (or null)",
  "instance": "<reqId>"
}
```

### After (`ProblemDetail` 422 body, multiple field errors)

```json
{
  "type": "about:blank",
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "request validation failed",
  "instance": "<reqId>",
  "errors": {
    "hr_max": "hr_max must be between 1 and 260 (or null)",
    "level": "level must be one of beginner | intermediate | advanced (or null)"
  }
}
```

Single-field errors keep the legacy `Detail` populated verbatim so
existing consumers (none in this repo, but possible external ones
in the future) keep working without JSON field lookup.

### DELETE response

- Success: `204 No Content`, empty body.
- 401: standard `ProblemDetail`.
- 500: standard `ProblemDetail`.

## Risks

- **Test rewrite cost**: seven existing `TestPatchPreferences_RejectsOutOfRange`
  subtests change from substring match to JSON-decode + map lookup.
  Low risk (test code only), but the diff is broader than the
  production change.
- **`Detail` redundancy**: keeping the legacy scalar summary while
  adding `errors` means a future consumer could read either. This
  is intentional (RFC 9457 allows extension members; keeping the
  scalar summary preserves any tooling that already parses it),
  but the proposal reviewer should confirm.
- **PR1 leaves /perfil still partially broken**: the wrong
  Connect-link is not fixed until PR2. PR1's contract change is
  inert until PR2 wires it through, but the existing container
  behaviour is already buggy, so PR1 does not regress anything.

## Chain strategy

This proposal is for PR1 only. PR2's exploration + proposal + spec
get their own directory once PR1 merges. The chain is
**stacked-to-main**: PR1 lands first, rebases on `main`, then PR2
rebases on top of `main` after PR1 merges (so PR2 sees the new
`/strava/connection` DELETE route in CI without race conditions).
The CI review budget of 400 LoC comfortably accommodates PR1.

## Verification plan

- `make test` (covers new DELETE handler + refactored preferences
  validation).
- `pnpm -C web test:run` covers the `PreferencesContainer` field
  mapping change with a new test case that mocks the 422 + `errors`
  body.
- Manual `curl` smoke: `PATCH /api/v1/me/preferences` with two
  invalid fields, assert the JSON body has both entries under
  `errors`; `DELETE /api/v1/strava/connection` with a valid bearer
  token, assert 204.

## Acceptance criteria (PR1-specific)

- `validate()` returns `map[string]string`; the `errors` extension
  member appears in 422 bodies; the `Detail` scalar stays.
- `DELETE /api/v1/strava/connection` returns 204 on success, 401
  unauthenticated, 500 on DB error.
- `PreferencesContainer` renders each error from
  `ApiError.fields` under its `<Field>` and removes the legacy
  always-timezone behaviour.
- `golangci-lint run`, `make test`, `pnpm -C web test:run`,
  `pnpm -C web typecheck`, `pnpm -C web lint`, and
  `pnpm -C web format:check` all pass.

## Out-of-scope (whole chain)

- OAuth handshake, refresh, webhooks (already shipped).
- Deleting imported activities on disconnect.
- Server-side Strava grant revocation.