# Verify Report: PR1 phase-1-3-km-vertical-and-king-climb

## Branch
- Branch: `feat/phase-1.3-km-vertical-pr1-backend`
- Base: `feat/phase-1.3-km-vertical`
- Final commit SHA: `1dc325cc49fb5993cf2a0adbabe80020bec1cc81`
- Authored LoC (Go, excluding regen + test mocks): **120 production Go added lines** (gross additions; 18 production Go lines deleted).
- Generated SQLC LoC: **247 added lines** (three new `.sql.go` files, `models.go`, `querier.go`).
- Mechanical mock stubs LoC: **90 added lines** (five unrelated Querier mock files, 18 each).
- PR1 backend footprint: no `web/**` changes. The generated SQLC changes are limited to the three new query files plus `models.go` and `querier.go`.

## Requirements coverage (specs/gpx-backend/spec.md ADDED)

| Req ID | Title | Implementation file | Test file | Scenarios passing | Notes |
|---|---|---|---|---|---|
| 1 | Migration creates three new climb-derived tables | `internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql`; `internal/db/schema.sql` | None for live migration behavior | Static schema inspection only; **0 migration scenarios executed against PostgreSQL** | Up/Down and schema definitions match; no backfill SQL. No live up/down/re-up smoke, despite apply-progress noting this remains unrun. |
| 2 | SQLC queries for the three new tables | `internal/db/queries/gpx_{muros,recovery_zones,km_vertical}.sql`; generated SQLC files | `internal/gpx/store_test.go` | Store accessor/order mock tests pass; no database-level order/upsert/unique-constraint test | Query text has both `ORDER BY start_idx` clauses and the per-track upsert. Actual SQL execution and uniqueness failure scenario remain unverified. |
| 3 | `StoredTrackDetail` carries the three top-level fields | `internal/gpx/types.go` | `internal/gpx/store_test.go`; `internal/http/handlers/gpx_get_test.go` | Empty-array and nil singleton JSON shape passes; GET populated fields pass | Struct tags and position are correct. Serialization test does not separately assert populated field-for-field serialization. |
| 4 | `CreateDetail` persists the new fields transactionally | `internal/gpx/store.go`; handler/store interfaces | `internal/gpx/store_test.go` | Nonempty call counts/selected parameters and empty/nil no-op tests pass | Writes execute inside the pre-existing transaction callback. No test injects a failure in a new-table write to prove rollback of the whole transaction. |
| 5 | `GetDetail` rehydrates all three fields | `internal/gpx/store.go` | `internal/gpx/store_test.go` | Populated rehydration and nonnil empty lists/nil km vertical tests pass | Lists map SQLC rows; accessors normalize no-row km vertical to nil. Full package race tests pass. |
| 6 | `SQLCStore` exposes the three accessors | `internal/gpx/store.go`, `internal/gpx/types.go` | `internal/gpx/store_test.go` | Muros/recovery order passthrough and missing km vertical nil result pass | Implementation delegates to SQLC methods. |
| 7 | Upload handler serializes `StoredTrackDetail` directly | `internal/http/handlers/gpx_upload.go` | `internal/http/handlers/gpx_upload_test.go` | Upload test passes and confirms top-level keys are present | Handler now calls `escribirJSON(w, detail)`. Current test's new captured-field assertions only exercise empty/nil values, not nonempty detector outputs. Wire keys remain top-level as specified by proposal/design. |
| 8 | Tests pin wire shape, persistence, and rehydration | `internal/gpx/store_test.go`; handler test files | `internal/gpx/store_test.go`; `internal/http/handlers/gpx_get_test.go`; `internal/http/handlers/gpx_upload_test.go` | New GET test and store tests pass | **Required `databaseTrack` regression guard is absent**: `databaseTrack` remains unchanged and no adjacent assertions pin nonempty `Muros`, `RecoveryZones`, or `KmVertical`. Apply-progress claims this task complete, contrary to the diff. |

## Task completion (tasks.md 1.1–1.16)

| Task | Status | Verification gate result | Notes |
|---|---|---|---|
| 1.1 Migration Up + Down | Partial | Build/tests pass; live goose migration gate unavailable/not run | Migration has required Up/Down halves; no migration-specific test file or DB smoke. |
| 1.2 Muros SQLC queries | Complete with verification caveat | `go test -race` passes; no live SQL execution | Query and generated method present; start-index ordering is in SQL. |
| 1.3 Recovery-zone SQLC queries | Complete with verification caveat | `go test -race` passes; no live SQL execution | Query and generated method present; start-index ordering is in SQL. |
| 1.4 Km vertical upsert/get | Complete with verification caveat | `go test -race` passes; no live SQL execution | Upsert SQL is correct by inspection; singleton replacement/isolation not exercised against PostgreSQL. |
| 1.5 Review `make generate` diff | Complete | Diff inspection passes | Only generated `models.go`, `querier.go`, and the three new `.sql.go` files changed. |
| 1.6 Extend `gpxQuerier` | Complete | `go build ./...` and tests pass | Six new methods are declared. |
| 1.7 Extend `StoredTrackDetail` | Complete | JSON-shape test and GET test pass | Tags are top-level and lack `omitempty`. |
| 1.8 `CreateDetail` signature/body | Complete with test caveat | Store tests pass | New write calls and no-op cases covered; transaction rollback on an injected child-write failure is not pinned. |
| 1.9 Extend interface mirrors | Complete | `go build ./...` and tests pass | Store and upload interfaces match; mechanical mocks compile. |
| 1.10 `GetDetail` rehydration | Complete | Store tests pass | Populated and empty/nil paths covered. |
| 1.11 Accessors | Complete | Store tests pass | `GetKmVertical` returns `(nil, nil)` for `pgx.ErrNoRows`. |
| 1.12 Remove upload wrapper | Complete with test caveat | Upload handler test passes | Direct detail serialization; test checks keys, but not nonempty detector values forwarded through captured fields. |
| 1.13 GET handler test | Complete | `TestGetGPXRehydratesMurosAndKmVerticalAndRecoveryZones` passes | Test asserts serialized populated fields. |
| 1.14 `databaseTrack` regression guard | **Incomplete** | **Not satisfied** | Fixture and guard have no new-field pin; this is an explicit spec/task requirement and apply-progress's completion claim is inaccurate. |
| 1.15 Final Go gates | Partial independently reproduced | `go build`, `go vet`, full `go test -race` pass; `gofmt -l .` clean; `make lint` failed to launch | Apply-progress reports the combined gate passed with golangci-lint 2.12.2, but this verification environment cannot reproduce lint (`golangci-lint: not found`). `make fmt` was not run because it may edit files; read-only gofmt check is clean. |
| 1.16 CI green on pushed branch | **Not complete / pending** | Not run | Apply-progress and user context say branch was not pushed; remote CI was not run. Exact unchecked marker: `#### 1.16. CI green on pushed branch (verification only, do not actually push)` followed by `- [ ]`. |

PR2 tasks 2.1–2.10 are outside this assigned PR1 slice and remain unchecked in the change; do not treat the entire change as complete.

## Acceptance criteria

1. `make fmt && make vet && make lint && make test` all green — **Reported green by apply-progress; not fully independently reproduced.** Local `go vet` and full race tests passed, gofmt check was clean, but `make lint` failed to launch because golangci-lint is unavailable here. No formatting command was run to preserve read-only verification.
2. Goose Up, Down, and second Up succeed — **Not verified**; migration was inspected but no live database smoke ran.
3. New upload detail contains detector outputs — **Implementation supports this**: handler forwards results to CreateDetail and store returns the supplied fields. Test only verifies the empty/nil case, so positive upload forwarding is under-pinned.
4. GET rehydrates the three fields — **Satisfied** by store implementation and GET serialization test.
5. Upload body no longer carries the three orphaned keys and equals `json.Marshal(detail)` — **Wording conflicts with the authoritative proposal/spec/design.** Those sources require the same top-level keys to remain and say direct `detail` serialization is byte-equivalent. Implementation follows the proposal/spec; keys remain at the same top-level paths. The literal acceptance wording is not satisfied, but removing those keys would violate the specified wire contract.
6. `make db-up && make migrate` smoke green — **Not verified**; no live DB was used.
7. Prior lossy substring pins are replaced — **Satisfied**; old assertions are removed and key presence/captured arguments are checked. Nonempty forwarding is not asserted in the current upload test.

## High-risk spot checklist

1. **Migration reversibility — PASS (static):** Up and Down sections exist; Down drops `gpx_km_vertical`, `gpx_recovery_zones`, `gpx_muros` in reverse order. Live goose execution unverified.
2. **Singleton constraint — PASS:** `UNIQUE` on `track_id` appears in migration and `schema.sql`.
3. **ON DELETE CASCADE — PASS:** all three table FKs reference `gpx_tracks(id) ON DELETE CASCADE` in migration and schema.
4. **Order preservation — PASS (query inspection):** both list queries contain `ORDER BY start_idx`.
5. **Wire shape preserved — PASS:** `gpx_upload.go` serializes detail directly; fields have top-level JSON tags. The handler test confirms key presence. This resolves the concern in favor of the proposal/spec's unchanged top-level wire paths.
6. **`StoredTrackDetail` rehydration — PASS:** `GetDetail` populates all three fields; list methods allocate nonnil empty slices; missing km vertical maps to nil and JSON `null`.
7. **No JSONB columns — PASS:** migration/schema use three independent relational tables; no JSONB columns introduced.
8. **GET rehydration test — PASS:** `TestGetGPXRehydratesMurosAndKmVerticalAndRecoveryZones` exists and tests populated serialization.
9. **Regression pin — FAIL:** no `databaseTrack` guard for the three fields exists, although task 1.14 is checked and apply-progress says it was added.
10. **Mock ripple — PASS:** mechanical zero-return stubs added to `activities_test.go`, `me_test.go`, `me_preferences_test.go`, `router_test.go`, and `auth/resolver_test.go`; they do not alter asserted test behavior. No `strava_connection_test.go` changes were necessary/present.
11. **No PR1 scope creep — PASS:** no `web/**` changes.
12. **Public upload API paths — PASS:** all three keys are still emitted at the original top-level paths (the GET-shaped detail itself now owns them); upload handler test checks presence.
13. **No backfill — PASS:** migration only creates new tables/indexes and drops those tables; no statements update or rewrite `gpx_tracks`.
14. **Generated SQLC output — PASS:** only `models.go`, `querier.go`, and three new `.sql.go` files changed under `internal/db/sqlc`; no old generated query file changed.
15. **`-race` tests — PASS:** locally ran both focused and full tests with `-race`; remote CI posture remains unverified.

## Local gate results

- `go build`: **PASS**, exit 0; `go build ./...` produced no output.
- `go vet`: **PASS**, exit 0; `go vet ./...` produced no output.
- Focused race tests: **PASS**, exit 0. Command: `go test -race -count=1 ./internal/gpx ./internal/http/handlers ./internal/db/... ./internal/auth/...`. Output: `internal/gpx`, `internal/http/handlers`, `internal/db`, `internal/db/status`, `internal/auth` all `ok`; `internal/db/sqlc` has no test files.
- Full race tests: **PASS**, exit 0. Command: `go test -race -count=1 ./...`. All test-bearing packages passed, including `internal/strava`; command package/sqlc packages with no tests reported `[no test files]`.
- `gofmt -l .`: **clean**, exit 0, no files listed.
- `make lint`: **UNAVAILABLE locally**, exit 2; `GOTOOLCHAIN=local golangci-lint run ./...` returned `/bin/sh: 1: golangci-lint: not found` (make target error 127). Apply-progress reports lint passed previously with v2.12.2; not reproduced here.
- Live migration checks (`goose Up`, `Down`, re-Up; `make db-up && make migrate`): **not run**, no DB migration smoke was available in this verification.

## Findings (severity-ordered)

- **Major — required regression pin omitted (task 1.14 / backend requirement 8).** `databaseTrack` is unchanged and has no guard assertions for `Muros`, `RecoveryZones`, or `KmVertical`. The apply-progress record says this task completed, but the branch diff and current test file do not support that claim. Restore the planned fixture guard before merge.
- **Major — migration behavior/acceptance smoke remains unverified.** Static SQL inspection confirms the migration shape, but neither a migration test nor live Up/Down/re-Up / `make db-up && make migrate` was run. This leaves the new production schema and rollback unproven; CI migration smoke is still required.
- **Minor — positive upload forwarding test is incomplete.** `gpx_upload_test.go` captures the fields but asserts only empty slices and nil km vertical. The implementation passes detector values through, but a nonempty detector-result test as described in task 1.12 would better pin the behavior.
- **Observation — acceptance wording is internally inconsistent.** It says the upload body no longer carries the three keys, while proposal/spec/design require them at the same top-level paths and say the body is equivalent to marshaled detail. Implementation follows the latter authoritative contract and keeps all three keys.
- **Observation — lint and remote CI are not independently verified in this run.** The apply agent reports local lint success, but the current environment lacks golangci-lint; remote CI was not run because the branch was not pushed.

## Recommendation

**changes-required** — address the missing explicit `databaseTrack` regression pin and obtain a successful migration smoke/CI result before merge. Implementation/schema inspection and all locally runnable build, vet, and race tests passed; no source or task artifacts were modified during verification.

## Risks for the parent

- Native status v2 says verification is ready/optional, `actionContext.mode` is `repo-local` with the repository as the allowed edit root, and native `nextRecommended` remains `apply`; verification does not alter that native recommendation. The whole change status reports 15/26 tasks complete, while PR1's 1.16 and all PR2 tasks remain pending.
- Do not interpret task 1.14's checked box or apply-progress wording as evidence: the guard is absent in the actual test file.
- Remote backend CI and actual PostgreSQL migration smoke were not run. Apply-progress documents local `make fmt vet lint test` success, but only build, vet, scoped/full race tests, and gofmt were independently reproduced here.
- Recommendation is not approval to archive PR1: archive admission remains governed by fresh native status and actual permissions/safety checks.

## Re-verify round 2 (after commit c617939)

### Regression pin now present
- File: `internal/gpx/store_test.go`
- Test: `TestDatabaseTrackFixtureKeepsClimbDerivedFields`
- Test passes: `ok github.com/fgjcarlos/ghamusinos/internal/gpx 1.016s` (exit 0).
- Pins all three climb-derived fields: `Muros` non-nil empty — yes; `RecoveryZones` non-nil empty — yes; `KmVertical` nil — yes.
- JSON shape pin: `require.Contains` pins `"muros":[]`, `"recovery_zones":[]`, and `"km_vertical":null` (all three assertions present and passed).
- Test calls `databaseTrack(...)` by name, then `GetDetail` with empty list results and `pgx.ErrNoRows`; the list-field checks would catch either list being dropped during rehydration. **Gap:** the nil `KmVertical` expectation cannot distinguish correct nil-on-no-row behavior from silently omitting the `KmVertical` assignment in `GetDetail`. Although the JSON shape checks key presence, that key can still serialize as `null` from the struct's zero value. The non-empty fixture/positive singleton regression described in task 1.14 and the spec's regression scenario is not established by this test.

### Full suite still passes
- `export PATH=/home/composedof2/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.4.linux-amd64/bin:$PATH; GOTOOLCHAIN=local go test -race -count=1 ./internal/gpx ./internal/http/handlers ./internal/db/... ./internal/auth/...` — exit 0. Output: `internal/gpx`, `internal/http/handlers`, `internal/db`, `internal/db/status`, and `internal/auth` all `ok`; `internal/db/sqlc` reports `[no test files]`.

### Remaining open items (not blocks; CI responsibility)
- Live migration smoke (PostgreSQL not available here; CI runs it).
- Lint (`golangci-lint` unavailable here; apply-progress reports pass; trust that prior run; CI runs it).
- Remote CI (branch not pushed; parent's responsibility). Task 1.16 remains unchecked: `- [ ]` under `#### 1.16. CI green on pushed branch (verification only, do not actually push)`.
- Acceptance wording remains inconsistent (“no longer carries the three keys” versus keys remaining at the same top-level paths); implementation follows proposal/spec and preserves the keys.

### Updated recommendation
- **changes-required** — the focused regression test is real and passes, but it does not prove that `GetDetail` preserves a populated/non-nil `KmVertical`; the exact loss mode from the previous finding can still pass. Migration smoke and CI remain follow-ups owned by CI/parent.
- Focused command: `export PATH=/home/composedof2/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.4.linux-amd64/bin:$PATH; GOTOOLCHAIN=local go test -race -count=1 -run TestDatabaseTrackFixtureKeepsClimbDerivedFields ./internal/gpx` — exit 0; output `ok github.com/fgjcarlos/ghamusinos/internal/gpx 1.016s`.
- Native status v2 remains authoritative: verify is ready/optional, native `nextRecommended` remains `apply`, and verification does not authorize archive. Whole-change task progress remains 15/26 complete (11 pending); do not infer archive readiness from this PR1 re-verification.

## Re-verify round 3 (after commits c617939 + b7b04fb)

### Test result for the regression pin
Command (with Go 1.26.4 and `GOTOOLCHAIN=local`):
```text
go test -race -count=1 -run TestDatabaseTrackFixtureKeepsClimbDerivedFields -v ./internal/gpx
=== RUN   TestDatabaseTrackFixtureKeepsClimbDerivedFields
=== RUN   TestDatabaseTrackFixtureKeepsClimbDerivedFields/empty_and_absent_rows_serialise_as_expected
=== RUN   TestDatabaseTrackFixtureKeepsClimbDerivedFields/populated_rows_round-trip_through_rehydration
--- PASS: TestDatabaseTrackFixtureKeepsClimbDerivedFields (0.00s)
    --- PASS: TestDatabaseTrackFixtureKeepsClimbDerivedFields/empty_and_absent_rows_serialise_as_expected (0.00s)
    --- PASS: TestDatabaseTrackFixtureKeepsClimbDerivedFields/populated_rows_round-trip_through_rehydration (0.00s)
PASS
ok  github.com/fgjcarlos/ghamusinos/internal/gpx  1.018s
EXIT_CODE=0
```

### Full suite result
Requested PR1 package suite command:
```text
go test -race -count=1 ./internal/gpx ./internal/http/handlers ./internal/db/... ./internal/auth/...
ok   github.com/fgjcarlos/ghamusinos/internal/gpx       1.047s
ok   github.com/fgjcarlos/ghamusinos/internal/http/handlers  1.383s
ok   github.com/fgjcarlos/ghamusinos/internal/db        1.119s
?    github.com/fgjcarlos/ghamusinos/internal/db/sqlc   [no test files]
ok   github.com/fgjcarlos/ghamusinos/internal/db/status 1.016s
ok   github.com/fgjcarlos/ghamusinos/internal/auth      1.683s
EXIT_CODE=0
```
This run covers the requested PR1 backend package set, not every package in `./...`; the earlier verify report records a prior full-repository race run.

### Build / vet / fmt
```text
go build ./...   — EXIT_CODE=0 (no output)
go vet ./...     — EXIT_CODE=0 (no output)
gofmt -l .       — EXIT_CODE=0 (no files listed)
```

### Subtest coverage assessment
- `empty_and_absent_rows_serialise_as_expected`: PASS — checks both empty result slices are non-nil, missing singleton is nil, and JSON includes `"muros":[]`, `"recovery_zones":[]`, `"km_vertical":null`.
- `populated_rows_round-trip_through_rehydration`: PASS — supplies populated SQLC rows to `GetDetail` and asserts populated Muros, RecoveryZones, and non-nil KmVertical with its expected gain; if the KmVertical assignment is dropped, the non-nil assertion fails.
- All previous round 2 failure modes now covered: yes — dropped populated KmVertical assignment fails the populated subtest; field rename fails compilation at the explicit field references and/or the serialized JSON-key assertions; broken empty/nil semantics fail the empty subtest.

The updated test at `internal/gpx/store_test.go` now closes the specific regression-pin gap identified in round 2. The pin is sufficient for the three stated failure modes; it is a focused store regression test, not evidence of live migration execution or remote CI.

### Status and remaining caveats
Native `gentle-ai.sdd-status` v2 was supplied and consumed: change `phase-1-3-km-vertical-and-king-climb` is `ready`, verification is optional and ready, `actionContext.mode` is `repo-local`, workspace root and allowed edit root are the repository. Native `nextRecommended` remains `apply`; task progress is 15/26 with 11 pending, including PR1 task 1.16 and PR2 tasks. Verification does not change that recommendation or authorize archive. Strict TDD is false in `openspec/config.yaml`. The migration up/down/re-up smoke and remote CI were not run in this round; see prior report caveats.

### Updated recommendation
- approve-with-followups — the regression pin now covers all three prior round-2 failure modes and the requested tests/build/vet/fmt checks pass. Retain live migration smoke and remote CI as outstanding follow-ups; do not infer whole-change completion.
