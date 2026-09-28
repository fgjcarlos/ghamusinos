# Archive Report: closes-159-strava-disconnect

## Summary

Closed the remaining acceptance criteria of issue #159 (preferences
profile + Strava lifecycle) with a chained 2-PR slice. The first PR
(#226, merged `59e6a24`) shipped the per-field 422 contract and SPA
mapping; the second PR (#227, merged `0b82f9c`) shipped the
`DELETE /api/v1/strava/connection` endpoint and router mount.

Together both PRs close #159's backend validation gaps and the
destructive Strava lifecycle. The remaining open work (UI wiring
of the disconnect button + the real Strava connect flow that
fixes the broken `<Link>` to `/api/v1/strava/connect`) is out of
scope of this change and tracked as a follow-up.

## Lineage

| Branch | PR | Merge commit | Work-unit commits |
|---|---|---|---|
| `feat/closes-159-strava-disconnect` | #226 | `59e6a24` | 5 (`7d6b214`, `13eac0e`, `41fb891`, `875fbfb`, `52eabae`) |
| `feat/closes-159-strava-disconnect-strava-delete` | #227 | `0b82f9c` | 4 (`1c2075c`, `01434b6`, `0bfc3ff`, `f881ac4`) |

Both branches were deleted after merge.

### PR1 #226 — work-unit commits

| Order | Commit | Message | Notes |
|---|---|---|---|
| 1 | `7d6b214` | `refactor(http): add ProblemDetail.Errors extension member for 422 field errors` | RFC 9457 §3.1 extension member. Keeps legacy `Detail` populated for backward compat. |
| 2 | `13eac0e` | `refactor(http): preferencesPatch.validate() returns per-field map` | TDD pair: tests re-asserted to assert `ProblemDetail.Errors[field]` rather than substring matching. |
| 3 | `41fb891` | `feat(web): ApiError carries field-level 422 errors from RFC 9457 body` | `fields` is optional; other ApiError consumers unchanged. |
| 4 | `875fbfb` | `fix(web): PreferencesContainer paints per-field 422 errors` | Adds `PreferencesContainer.test.tsx` (4 cases). |
| 5 | `52eabae` | `docs(sdd): record closes-159-strava-disconnect exploration and proposal (PR1)` | SDD artifacts. |

### PR2 #227 — work-unit commits

| Order | Commit | Message | Notes |
|---|---|---|---|
| 1 | `1c2075c` | `feat(http): DELETE /api/v1/strava/connection handler` | Thin wrapper around existing `DeleteStravaTokensByUserID`. Idempotent. |
| 2 | `01434b6` | `test(http): DELETE /api/v1/strava/connection contract` | 5 cases (success, unauthorized, invalid user id, DB error, idempotent). Adds `deleteStravaTokens` hook to existing `preferencesMockQuerier` to avoid duplicating the sqlc.Querier interface. |
| 3 | `0bfc3ff` | `feat(http): mount DELETE /api/v1/strava/connection` | Inside `/api/v1/strava` group, gated by `s.queries != nil`. |
| 4 | `f881ac4` | `style(http): apply gofmt to strava disconnect handler and tests` | CI gofmt step flagged three files; cosmetic fix. |

## Specs Promoted

The change did **not** introduce new delta specs. The capabilities
touched (`/api/v1/me/preferences` PATCH validation and
`/api/v1/strava/connection` DELETE) are pre-existing handlers in
`internal/http/handlers/`; the change refactors their contract and
adds a new handler in the same package. There is no
`openspec/specs/{preferences|strava}/spec.md` to copy deltas into
— these are infra endpoints without formal spec coverage.

If formal spec coverage is desired in the future, the
canonical specs for these capabilities should be created at:

- `openspec/specs/preferences/spec.md`
- `openspec/specs/strava/spec.md`

Tracking as follow-up, not blocking this archive.

## Per-Task Outcome

Source of truth: `tasks.md` and `verify-report.md`. Task state at
archive time: every work-unit commit landed via a green CI run;
no outstanding local TODOs; one post-CI gofmt follow-up commit on
PR2 resolved the only failure.

## Outstanding warnings

1. **Other 422-emitting handlers still use flat `Detail`.**
   `internal/http/handlers/gpx_upload.go` (4 sites) keeps the
   legacy contract. No regression; future UX improvement should
   migrate them to `NewUnprocessableEntityFields`.

2. **Frontend wiring for the disconnect endpoint is not in this
   change.** A follow-up should add `GET /api/v1/strava/connection`,
   the `disconnectStrava` TS client, the connect flow fix
   (`window.location.assign(authorizeURL)`), and the Disconnect
   button on `ConnectionCard.tsx`.

3. **gofmt drift caught only by CI.** Add a pre-push
   `gofmt -l .` hook to catch this locally next time.

## Issue closure

Both PRs close issue #159 (preferences + Strava disconnect).
Suggested manual action: add a closing comment linking both PRs
and the verification report from `verify-report.md`.
