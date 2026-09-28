# Tasks: closes-159-strava-disconnect

The change was delivered as a chained 2-PR slice because the
combined LoC (999 across both PRs) exceeded the 400-line review
budget from `openspec/config.yaml`. PR1 covered the contract
refactor + SPA mapping; PR2 added the destructive Strava endpoint.

Each PR kept the work-unit commit discipline (one commit per
concern, conventional messages, tests + behaviour paired).

## PR1 (merged as `59e6a24`) — feat(http,web): per-field 422 errors

Five work-unit commits:

1. `refactor(http): add ProblemDetail.Errors extension member for 422 field errors`
   - `internal/http/handlers/errors.go`: add optional `Errors map[string]string`
     JSON extension member (RFC 9457 §3.1) and constructor
     `NewUnprocessableEntityFields`.
   - Keep `NewUnprocessableEntity` intact for legacy handlers
     (`gpx_upload.go`) that still emit a flat `Detail` string.

2. `refactor(http): preferencesPatch.validate() returns per-field map`
   - `internal/http/handlers/me_preferences.go`: change `validate()`
     return type to `map[string]string`; evaluate every field rule.
   - `PatchPreferences` handler emits 422 with `Errors` populated
     alongside a stable scalar `Detail`.
   - `me_preferences_test.go`: re-assert the seven
     `TestPatchPreferences_RejectsOutOfRange` cases against
     `ProblemDetail.Errors[field]` instead of substring matching.
   - Add `TestPatchPreferences_ReportsMultipleFieldErrors` to lock
     in the multi-field contract the SPA depends on.

3. `feat(web): ApiError carries field-level 422 errors from RFC 9457 body`
   - `web/src/lib/api/types.ts`: add `fields?: Record<string, string>`
     to `ApiError`.
   - `web/src/lib/api/profile.ts`: parse `application/problem+json`
     bodies in `jsonResponse` and populate `ApiError.fields` from
     the `errors` member.

4. `fix(web): PreferencesContainer paints per-field 422 errors`
   - `web/src/features/profile/PreferencesContainer.tsx`: replace
     `setErrors({ timezone: detail })` with `setErrors(err.fields)`.
     The global red banner stays only for non-field failures (5xx,
     network).
   - `web/src/features/profile/PreferencesContainer.test.tsx`:
     four cases (empty form, two field errors without banner,
     5xx banner, saved status).

5. `docs(sdd): record closes-159-strava-disconnect exploration and proposal (PR1)`
   - `openspec/changes/closes-159-strava-disconnect/exploration.md`
     and `proposal.md`.

## PR2 (merged as `0b82f9c`) — feat(http): DELETE /api/v1/strava/connection

Three work-unit commits (after a `style(http)` gofmt fix):

1. `feat(http): DELETE /api/v1/strava/connection handler`
   - `internal/http/handlers/strava_connection.go`: thin HTTP
     wrapper around `DeleteStravaTokensByUserID`. Returns 204 on
     success, 401 unauthenticated, 500 on DB error or malformed
     user id. Idempotent at the user level (sqlc's `:exec` returns
     nil for zero rows affected).

2. `test(http): DELETE /api/v1/strava/connection contract`
   - `internal/http/handlers/strava_connection_test.go`: five
     cases (success, unauthorized, invalid user id, DB error,
     idempotent).
   - `me_preferences_test.go`: add `deleteStravaTokens` hook on
     `preferencesMockQuerier` so the same mock satisfies the new
     test file without duplicating the entire sqlc.Querier
     interface.

3. `feat(http): mount DELETE /api/v1/strava/connection`
   - `internal/http/router.go`: add
     `r.Delete("/strava/connection", handlers.DeleteStravaConnection(s.queries).ServeHTTP)`
     inside the `/api/v1/strava` group, gated by `s.queries != nil`.

4. (post-CI) `style(http): apply gofmt to strava disconnect handler and tests`
   - Cosmetic fix for the CI gofmt step that flagged
     misaligned comments and missing trailing newlines.
