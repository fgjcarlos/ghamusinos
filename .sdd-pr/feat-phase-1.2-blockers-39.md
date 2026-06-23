# feat(jobs): River job queue foundation (#39)

## Objetivo
Setup de River (queue de jobs en Postgres) con lifecycle management y stubs de workers.

## Criterios de aceptación
- [x] River v0.39.0 + riverpgxv5 v0.39.0 añadidos a go.mod
- [x] `internal/jobs/` package con Kind, ImportStravaArgs, RefreshStravaTokenArgs, ImportStravaWorker, RefreshStravaTokenWorker
- [x] RiverClient con NewRiverClient/Start/Stop
- [x] Lifecycle: workers start en app.Run, stop en app context cancel
- [x] Integration test (testing.Short() gated) — stub job enqueue + verify
- [x] Aplicado a Phase 1.2 Strava: ImportStravaArgs + RefreshStravaTokenArgs listos

## Cambios
- `internal/jobs/jobs.go` (nuevo) — Kind constants, Args structs
- `internal/jobs/workers.go` (nuevo) — workers con Work methods
- `internal/jobs/river.go` (nuevo) — RiverClient lifecycle
- `internal/app/app.go` (modificado) — River init + graceful shutdown
- Tests: jobs_test.go, workers_test.go, river_test.go, integration_test.go

## Tests
- 8 tests passing
- Integration test skipped without DATABASE_URL (correct gating)

## Issue
Cierra #39.
