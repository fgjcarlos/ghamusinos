# Tasks: phase-1-6-gpx-advanced-lab

> Cadena sobre la rama integradora `feat/phase-1-6-gpx-lab` (base
> `main@2725d2e`). Delivery: feature-branch-chain, seis PRs hijas + PR
> tracker en borrador. Checks: `TZ=UTC GOTOOLCHAIN=go1.26.10 go test -race
> ./...`, `go vet ./...`, `golangci-lint run ./...`, `pnpm -C web test:run`,
> `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web format:check`.

## Slices

| Slice | PR | Unit | Commits |
| --- | --- | --- | --- |
| 1 | 1 | Activity streams: `time` ingestion, read/refresh endpoints, enqueue fix | `c7144f8` |
| 2 | 2 | Weather proxy | `1fad7d7` |
| 3 | 3 | Terrain proxy + route wiring | `572833a`, `6bff217` |
| 4 | 4 | Pure web model (geometry, splits/cutoffs, race day, solar, gear) | `ce4b2bb`, `825ee6d`, `9cafe97`, `21b0aa9`, `6e7c06d` |
| 5 | 5 | API client, hooks and race-planning UI | `93e07dd`, `2dd47a0` |
| 6 | 6 | Post-Activity model/panel and OpenSpec close-out | `aa41061`, docs |

## Work units

- [x] **T1 — Activity streams (S5, S11).** Add `time` to the Strava stream
  request; enqueue `import_strava_streams` after every activity upsert in the
  backfill and webhook paths; `GET /activities/{id}/streams`; `POST
  /activities/{id}/streams/refresh`; tests. Commit `c7144f8`.
- [x] **T2 — Weather proxy (S3, S7–S11).** `internal/weather` Open-Meteo
  client; `GET /gpx/{id}/weather?at=` with four roles, UTC hour selection,
  1 h cache after ownership, 422/503 mapping; tests. Commit `1fad7d7`.
- [x] **T3 — Terrain proxy (S3, S7–S11).** `internal/terrain` Overpass client
  and mapping; `GET /gpx/{id}/terrain`; 24 h cache; tests; routes wired.
  Commits `572833a`, `6bff217`.
- [x] **T4 — Geometry foundation (S7, S8).** `types.ts` + `trackCoords.ts`
  with the wire adapter, haversine, cumulative km, D+, slope, adjusted speed,
  total time, `indexAtKm` and bearing; 26 tests. Commit `ce4b2bb`.
- [x] **T5 — Splits and cutoffs (S3, S8).** `splits.ts`; 11 tests. Commit
  `825ee6d`.
- [x] **T6 — Race day (S3, S8).** `raceDay.ts`: markers, nutrition, presets;
  14 tests. Commit `9cafe97`.
- [x] **T7 — Solar exposure (S3, S8).** `solar.ts` with SunCalc v2; 8 tests.
  Commit `21b0aa9`.
- [x] **T8 — Gear checklist (S3, S8).** `gear.ts`; 18 tests. Commit
  `6e7c06d`.
- [x] **T9 — API client and hooks (S3, S7, S9, S10).** `lab.ts`,
  `useTrackWeather`, `useTrackTerrain`, `useActivityStreams`; 13 tests.
  Commit `93e07dd`.
- [x] **T10 — Race-planning UI (S3, S8, S9).** `RacePlanSection` and seven
  panels mounted in `TrackDetailPage`; 9 component tests. Commit `2dd47a0`.
- [x] **T11 — Post-Activity (S3, S5, S8).** `postActivity.ts` and
  `PostActivityPanel` with the 409 re-sync flow; 7 + 2 tests. Commit
  `aa41061`.
- [x] **T12 — OpenSpec close-out (S4, S12).** This change, canonical specs,
  archive and feature-record closure.

## Verification evidence

- Go: `go test -race ./...` green; `go vet ./...` clean; `golangci-lint run
  ./...` reports `0 issues`.
- Web: `pnpm -C web test:run` → 68 files / 435 tests passed; typecheck,
  lint and format check clean.
- No live third-party request in any test.
