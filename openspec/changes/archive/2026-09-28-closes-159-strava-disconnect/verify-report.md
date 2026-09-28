# Verification Report: closes-159-strava-disconnect

**Status: READY for archive.** Both PRs in the chain landed via
the user's manual merge. All CI checks passed at merge time.

## Executive summary

- **PR1 (#226, merged `59e6a24`)** — `feat(http,web): per-field
  422 errors for /me/preferences`. Backend, frontend, and types
  changed. Full test suite green in CI.
- **PR2 (#227, merged `0b82f9c`)** — `feat(http): DELETE
  /api/v1/strava/connection`. Handler + router mount + tests.
  CI gofmt step required a small follow-up commit on the same
  branch; second run was green end-to-end.

## Per-PR outcomes

### PR1 #226 — Run 36470300385

| Check | Status | Duration |
|---|---|---|
| Backend (lint + test + build + migrate) | pass | 1m 17s |
| Frontend (build) | pass | 25s |
| Release (binario + smoke con SPA embebida) | pass | 57s |
| GitGuardian Security Checks | pass | 2s |

### PR2 #227 — Run 36476872836 (after gofmt fix)

| Check | Status | Duration |
|---|---|---|
| Backend (lint + test + build + migrate) | pass | 1m 27s |
| Frontend (build) | pass | 28s |
| Release (binario + smoke con SPA embebida) | pass | 55s |
| GitGuardian Security Checks | pass | 2s |

The first run of PR2 (#36476217964) failed at the gofmt step
because three files were flagged as misformatted (a struct
comment with an extra space column and two files missing
trailing newlines). The fix commit `f881ac4 style(http):
apply gofmt to strava disconnect handler and tests` resolved
the failure on the second run.

## Requirement-to-implementation mapping

### Backend contract (RFC 9457 extension member)

| Requirement | Implementation |
|---|---|
| `ProblemDetail` carries per-field validation errors | `internal/http/handlers/errors.go` — `Errors map[string]string` field with `omitempty`; `NewUnprocessableEntityFields(detail, instance, fields)` constructor |
| `validate()` reports every failing field, not just one | `internal/http/handlers/me_preferences.go:122-156` — `validate() map[string]string` |
| 422 body keeps legacy `Detail` populated | same handler — sets `Detail: "request validation failed"` when fields map is non-empty |
| Tests assert the new contract, not substring match | `internal/http/handlers/me_preferences_test.go` — `TestPatchPreferences_RejectsOutOfRange` rewrites and `TestPatchPreferences_ReportsMultipleFieldErrors` new |

### Frontend contract

| Requirement | Implementation |
|---|---|
| `ApiError` carries `fields` from problem+json | `web/src/lib/api/types.ts:96-115` — `fields?: Record<string, string>` |
| Profile client parses `application/problem+json` | `web/src/lib/api/profile.ts:18-46` — `jsonResponse` parses and populates `fields` |
| `/perfil` paints per-field errors next to input | `web/src/features/profile/PreferencesContainer.tsx:73-90` — replaces timezone catch-all with `setErrors(err.fields)` |
| Global banner only on non-field failures | same container — `setStatus('idle')` when `fields` has any key |
| Frontend tests lock the behaviour | `web/src/features/profile/PreferencesContainer.test.tsx` — four cases |

### Strava disconnect

| Requirement | Implementation |
|---|---|
| `DELETE /api/v1/strava/connection` returns 204 | `internal/http/handlers/strava_connection.go` |
| 401 unauthenticated | same — guard at top of handler |
| 500 on DB error | same — `WriteProblem(NewInternalError(...))` |
| Idempotent at the user level | sqlc `:exec` semantics + verified by `TestDeleteStravaConnection_Idempotent` |
| Mounted in router under `/api/v1` | `internal/http/router.go` inside the existing `/strava` group |
| All test cases pass | `internal/http/handlers/strava_connection_test.go` — five cases |

## Outstanding warnings

- **CI gofmt step is manual**: the first PR2 run flagged three
  files that pass `go test` and `go vet` but fail `gofmt -l`.
  Recommend adding a local pre-push hook that runs `gofmt -l .`
  before `git push`. Documented in `apply-progress.md` notes
  for the next reviewer.
- **Other handlers still emit flat `Detail`**: `gpx_upload.go`
  (4 sites) does not yet use `NewUnprocessableEntityFields`. No
  regression because the new field is `omitempty`, but a future
  UX improvement on those screens would migrate them to the
  per-field contract.
- **No frontend wiring for the disconnect endpoint yet**: the
  Disconnect button on `ConnectionCard.tsx` is still omitted.
  This slice only added the backend capability. A follow-up
  change (call it `closes-159-strava-disconnect-ui`) should
  add `GET /api/v1/strava/connection`, a `disconnectStrava`
  TS client function, the real `window.location.assign(authorizeURL)`
  connect flow (current `<Link>` is broken), and the button
  itself in `ConnectionCard.tsx`.
