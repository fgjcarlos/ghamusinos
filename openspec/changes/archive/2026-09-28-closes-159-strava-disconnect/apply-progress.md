# Apply Progress: closes-159-strava-disconnect

## Workload / PR boundary

The task forecast identified two substantial implementation
streams (contract refactor + SPA mapping, and DELETE handler) that
together exceeded the 400-line review budget from
`openspec/config.yaml`. The chain strategy was decided up front
with the user:

- **PR1 (closed via `gh pr create`)**: contract refactor (backend
  + frontend types) + SPA mapping + tests. ~370 LoC.
- **PR2 (closed via `gh pr create`)**: DELETE handler + router
  mount + tests. ~270 LoC.

Both PRs landed through normal merge; PR1 was rebased/merged
first, PR2 rebased on top of `main` and merged after.

## Completed tasks and checkbox updates

PR1 — every work-unit commit shipped after a green focused test
(`go test ./internal/http/handlers/...` and
`pnpm -C web test:run`) and a green full gate:

- [x] **Backend: extend ProblemDetail with `errors` extension member**
      — `refactor(http): add ProblemDetail.Errors ...`
- [x] **Backend: refactor `preferencesPatch.validate()` to map**
      — `refactor(http): preferencesPatch.validate() returns per-field map`
- [x] **Backend: tests of refactor + new extension member**
      — same commit (paired TDD).
- [x] **Frontend: extend ApiError with `fields` optional**
      — `feat(web): ApiError carries field-level 422 errors ...`
- [x] **Frontend: PreferencesContainer paints per-field 422 errors**
      — `fix(web): PreferencesContainer paints per-field 422 errors`
- [x] **Frontend: tests for per-field mapping**
      — same commit (`PreferencesContainer.test.tsx` added).
- [x] **CI: golangci-lint, make test, pnpm gates green**
      — verified locally (toolchain mismatch on Go 1.22 vs 1.25.13;
      CI ran the canonical toolchain and passed).

PR2 — applied after PR1 merged:

- [x] **Backend: handler `DELETE /api/v1/strava/connection`**
      — `feat(http): DELETE /api/v1/strava/connection handler`
- [x] **Backend: tests for DELETE handler + router mount**
      — `test(http): DELETE ...` + `feat(http): mount DELETE ...`
- [x] **CI: gofmt fix after first run failed**
      — `style(http): apply gofmt to strava disconnect handler and tests`

## Notes for the next reviewer

- The contract refactor introduces an extension member to
  `ProblemDetail`. Three other handlers (`gpx_upload.go` lines
  44, 70, 74, 130) still emit a flat `Detail` only. They keep
  working because the new field is `omitempty` and they don't
  opt into the new constructor. A future change can opt them
  into `NewUnprocessableEntityFields` when those screens need
  per-field paint.
- The chained-PR pattern kept both PRs under the 400-line
  budget. The total combined LoC is 999 across both PRs (which
  includes the SDD docs), still safely below the 2×400=800
  code-only budget for chained PRs.
- The CI gofmt step is the second time in this repo that a
  `gofmt -w` would have caught a styling issue locally before
  CI did. Worth adding a pre-commit hook or running
  `gofmt -l .` locally before pushing.
