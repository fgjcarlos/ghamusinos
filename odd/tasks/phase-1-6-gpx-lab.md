# Phase 1.6 — GPX advanced lab (#18)

Objective: deliver race planning and environmental context on top of a stored GPX track, without changing the deterministic core. Base: `main@2725d2e`; branch: `feat/phase-1-6-gpx-lab` (tracker/final integration). Delivery: feature-branch-chain, 5 child PRs + draft tracker PR. Forecast: 6000–8000 authored lines. Checks: `TZ=UTC GOTOOLCHAIN=go1.26.10 go test -race ./...`; `go vet ./...`; `golangci-lint run`; `pnpm -C web test:run`; `pnpm -C web typecheck`; `pnpm -C web lint`; `pnpm -C web format:check`; `make build`.

## Specs

S1. User request: "Vamos ahora a por la issue #18".

S2. Epic objective (issue #18, verbatim): "Planificación de carrera y contexto ambiental (ya prototipado en old_ghamusinos)."

S3. Epic deliverables (issue #18, verbatim):
- "Race Day (tiempo, ritmo, nutrición, hitos KM)"
- "Cutoff Calculator + Strategic Splits"
- "Terrain Info (OSM/Overpass), Weather (Open-Meteo), Solar Exposure (SunCalc)"
- "Gear Checklist + Post-Activity (plan vs real)"

S4. Closing criterion (roadmap, verbatim): "el laboratorio ofrece preparación de carrera completa sobre un track."

S5. Post-Activity scope (user choice, verbatim): "Incluir backend (recomendado)" — "Añadir el stream `time` a la ingesta de Strava y un endpoint autenticado de solo lectura que exponga latlng+altitude+time de la actividad. El laboratorio compara plan vs actividad real y la épica queda completa. Suma superficie de API y toca el job de ingesta (riesgo alto: auth y datos)."

S6. Delivery strategy (user choice, verbatim): "Cadena sobre rama de feature (recomendado)" — one `feat/phase-1-6-gpx-lab` integration branch with 5 chained child PRs, each targeting the previous child branch; a draft tracker PR targets `main`.

S7. Architecture constraints (assumptions verified against repo evidence):
- The SPA is served with CSP `connect-src 'self'` (`internal/http/security_middleware.go`), so the browser MUST NOT call Open-Meteo or Overpass directly. Both go through authenticated Go endpoints.
- Backend endpoints derive coordinates from the user's stored track, never from a client-supplied URL (no SSRF surface).
- All race math is pure client-side TypeScript, consumes `{lat, lon, ele}` named fields, and uses no `any`.
- UI copy is Spanish; identifiers, comments, and commit messages are English.
- Tests never hit live third-party services; HTTP is stubbed with `httptest`/`fetch` mocks.

S8. Behavioral acceptance criteria per deliverable:
- Race Day: estimated total time from a base pace (min/km) with slope-adjusted speed; selectable pace presets; nutrition points every N km (default 10); KM milestones with cumulative distance and gain.
- Cutoff Calculator: for each `(km, deadline)` cutoff, estimated arrival time, safe/unsafe flag, margin, and required average pace to meet it.
- Strategic Splits: one row per km with split time, cumulative time, gain, pace, and an adjustable fatigue factor (default 0.05).
- Terrain Info: surface type and technicality 1–5 per track segment from OSM; empty state when OSM returns nothing; endpoint failure degrades the section only.
- Weather: forecast at 4 strategic points (start, highest point, midpoint, end) for the chosen race date/hour; server-side cache of 1 h; failure degrades the section only.
- Solar Exposure: sun/shade/night status per segment for an explicit date and timezone, with intensity and aspect.
- Gear Checklist: mandatory and recommended items derived from duration, D+, maximum altitude, and weather; checked state stays in the section.
- Post-Activity: per-km planned vs real split, difference, cumulative times, and fatigue index. When the activity lacks `time` or `latlng` streams, the API returns `409` with the missing stream types and the UI offers a re-sync action.

S9. Degradation: a weather or terrain failure MUST NOT block `TrackDetailPage` nor the other race-planning tabs.

S10. On-demand only: weather and terrain calls start from an explicit user action in their tab; loading the track detail performs no third-party request.

S11. Security and privacy: the streams endpoint is scoped to the authenticated user; the refresh action only enqueues a job for an activity the user owns; third-party requests carry track coordinates but no user identity, email, or token.

S12. Closure: canonical OpenSpec specs under `openspec/specs/`, the change archived under `openspec/changes/archive/`, and this record closed with evidence.

## Tasks

| ID | Linked | Route | Slice | Status | Evidence |
|---|---|---|---|---|---|
| T1 | S5, S11 | parent inline | 1 — backend streams | done | `time` added to stream ingestion; the previously orphaned streams job is now enqueued from both backfill and webhook paths; `GET /activities/{id}/streams` (409 + missing types) and `POST .../streams/refresh` (202) with tests; full Go race suite, vet and golangci-lint green. Commit: `c7144f8`. |
| T2 | S3, S7, S8, S9, S10, S11 | delegated (parallel writers) | 2 — backend weather | done | `internal/weather` Open-Meteo client + `GET /gpx/{id}/weather?at=RFC3339`; four deduped roles, UTC hour selection, 1 h cache after the ownership check, 422/503 degradation; tests. Commit: `1fad7d7`. |
| T3 | S3, S7, S8, S9, S10, S11 | delegated (parallel writers) | 3 — backend terrain | done | `internal/terrain` Overpass client + `GET /gpx/{id}/terrain`; `around:50` query, point-to-segment attribution, surface/technicality mapping, 24 h cache; tests. Commit: `572833a`, routes wired in `6bff217`. |
| T4 | S7, S8 | parent inline | 4 — web model | done | `types.ts` + `trackCoords.ts`: wire adapter, haversine, cumulative distance, D+, slope, slope-adjusted speed, total time, km index, bearing; 26 tests. Commit: `ce4b2bb`. |
| T5 | S3, S8 | delegated (parallel writers) | 4 — web model | done | `splits.ts`: per-km fatigue-adjusted splits and cutoff evaluation; 11 tests. Commit: `825ee6d`. |
| T6 | S3, S8 | delegated (parallel writers) | 4 — web model | done | `raceDay.ts`: km markers, nutrition points with ±500 m flattest selection, pace presets and defaults; 14 tests. Commit: `9cafe97`. |
| T7 | S3, S8 | delegated (parallel writers) | 4 — web model | done | `solar.ts`: per-segment SunCalc position at the arrival instant, sun/shade/night classification and merging; 8 tests. `suncalc@2.1.2` returns degrees (v2), so `@types/suncalc` was removed. Commit: `21b0aa9`. |
| T8 | S3, S8 | delegated (parallel writers) | 4 — web model | done | `gear.ts`: duration/D+/altitude/weather rules with mandatory precedence and dedupe; 18 tests. Commit: `6e7c06d`. |
| T9 | S3, S7, S9, S10 | parent inline | 4 — web client/UI | pending | TS API client + hooks for weather, terrain, activity streams; tests. |
| T10 | S3, S8, S9 | parent inline | 4 — web client/UI | pending | `RacePlanSection` tabs mounted in `TrackDetailPage`; component tests. |
| T11 | S3, S5, S8 | parent inline | 5 — post-activity + docs | pending | Plan-vs-real model + UI, re-sync flow; tests. |
| T12 | S4, S12 | parent inline | 5 — post-activity + docs | pending | OpenSpec change, canonical specs, archive, ODD closure. |

Route note: every task starts inline under the parent. A writer is delegated only if the context backstop fires or two independent units with disjoint edit surfaces are launched together; the trigger and surface are recorded here at that moment.

## Log

L1. User request (2026-11-07, verbatim): "Vamos ahora a por la issue #18".

L2. Exploration (2026-11-07). Issue #18 is the phase 1.6 epic, not a leaf task. The prototype lives at `/home/composedof2/Dev/old_ghamusinos` (Astro + React) and concentrates the logic in `src/lib/utils.ts` (1669 lines). The current app already has a GPX lab (`web/src/features/lab`, `web/src/features/gpx`) and a Go GPX store. The explorer's map is summarized in S7 and the task list above.

L3. Blocking findings from exploration:
- CSP `connect-src 'self'` makes direct browser calls to Open-Meteo/Overpass impossible. Chosen solution: authenticated Go endpoints that derive coordinates from the stored track.
- `suncalc` is not a current dependency; it must be added to `web/package.json`.
- GPX `Point.Time` is parsed but discarded at storage (`internal/gpx/store.go:521-531`, `:582-599`). Race planning does not need it; Post-Activity uses Strava streams instead.
- Activity streams currently import `heartrate`, `watts`, `cadence`, `altitude`, `latlng` but not `time` (`internal/jobs/streams.go`). Post-Activity therefore needs the `time` stream plus a read endpoint and a refresh path for existing activities.
- Activities expose no coordinates or streams through HTTP today.

L4. User decisions (2026-11-07, verbatim answers): Post-Activity → "Incluir backend (recomendado)". Delivery → "Cadena sobre rama de feature (recomendado)".

L5. Plan shape: 5 child slices — (1) backend streams, (2) backend weather+terrain, (3) web pure model, (4) web client+UI, (5) Post-Activity+docs. Each child branch is created from the previous child branch; the tracker `feat/phase-1-6-gpx-lab` merges to `main` only after the chain completes.

L6. Known limitations to carry into the specs and PR bodies:
- Weather forecasts are only available for the Open-Meteo horizon; dates outside it must show a clear "not available" state.
- Overpass is rate-limited and may be slow; the endpoint must time out and cache.
- OSM surface data is incomplete by nature; "unknown" technicality is a valid result, not an error.
- Existing Strava activities will not have the `time` stream until a refresh is queued; this is an explicit UX state, not a silent failure.

L7. T1 delivered on 2026-11-07 (commit `c7144f8`). Two findings while implementing:
- The `ImportStravaStreamsWorker` was registered but **nothing enqueued it** in production. Only the tests referenced `ImportStravaStreamsArgs`. As a result `activity_streams` stayed empty for real users, which also silently broke HR zones and cardiac drift. Fixed by enqueueing the job after every successful activity upsert in both the backfill loop and the webhook ingest worker, with `UniqueOpts{ByArgs: true}`.
- `time` is now requested from Strava alongside `heartrate`, `watts`, `cadence`, `altitude` and `latlng`.

Endpoints: `GET /api/v1/activities/{id}/streams` returns every stored stream as raw JSON and `409 Streams Unavailable` when `time` or `latlng` is missing/empty; `POST /api/v1/activities/{id}/streams/refresh` enqueues the import for one owned activity and returns `202`. Both resolve the activity by external id and user, reusing the `GetActivityByExternalID` ownership filter.

Verification observed: `TZ=UTC GOTOOLCHAIN=go1.26.10 go test -race -count=1 ./...` green; `go vet ./...` clean; `golangci-lint run ./...` reports `0 issues`. The commit is 635 added lines (handler 223 + handler tests 218 + enqueue fix 43 + generated/mocks the rest), above the advisory 400-line heuristic but one cohesive deliverable; no cosmetic reduction applied.

L8. T2 and T3 delivered on 2026-11-07 by two parallel `gentle-ai-worker` writers with disjoint edit surfaces (`internal/weather/**` + `lab_weather*`; `internal/terrain/**` + `lab_terrain*`), then integrated by the parent.
- Weather: `internal/weather` (Open-Meteo client, UTC hourly selection, `ErrOutOfRange`, Spanish WMO labels) and `GET /api/v1/gpx/{id}/weather?at=RFC3339`. Four deduped roles (start, highest, midpoint, end), 1 h cache keyed by track+hour, cache consulted only after `GetDetail` so ownership is always enforced. Commit `1fad7d7`.
- Terrain: `internal/terrain` (Overpass `around:50` query over an 80-point decimation, point-to-segment attribution with a 50 m cutoff, `MapSurface`/`MapSegments`) and `GET /api/v1/gpx/{id}/terrain`. 24 h cache keyed by track id, consulted after `GetDetail`. Commit `572833a`.
- Integration: routes wired in `6bff217`; both are track-scoped and derive coordinates from the stored track, never from a client-supplied URL (no SSRF).
- Parent lint fixes: errcheck on `resp.Body.Close` and test `fmt.Fprint`, staticcheck S1016/QF1012 in the Overpass client, and the two route lists in `internal/app/routes_test.go`.
- Verification observed: `TZ=UTC GOTOOLCHAIN=go1.26.10 go test -race -count=1 ./...` green; `go vet ./...` clean; `golangci-lint run ./...` reports `0 issues`.
- Deliberate deviations recorded by the writers: terrain `MapSurface` returns the kind labels `paved/gravel/trail/technical/unknown` (the web layer maps them to display text); `start_km`/`end_km` are rounded to 2 decimals; the Overpass query is sent as a form-encoded `data=` POST.

L9. Chain boundaries revised after measuring the units (additions+deletions):
- PR 1 streams: `f755e17..106b8d7` (~650 lines).
- PR 2 weather: `106b8d7..1fad7d7` (~930 lines).
- PR 3 terrain: `1fad7d7..6bff217` (~1120 lines).
- PR 4 web pure model (T4-T8): pending; split again if it exceeds ~1200 lines.
- PR 5 web client + UI (T9-T10): pending.
- PR 6 Post-Activity + docs (T11-T12): pending.
Branch choreography is deferred until delivery: the tracker `feat/phase-1-6-gpx-lab` will be reset to the feature-doc commit `f755e17`; each child branch is created at its boundary commit and targets the previous child branch. Push and PR creation still require explicit user authorization.

L10. T4-T8 delivered on 2026-11-07. T4 (geometry foundation) was written inline; T5-T8 ran as four parallel `gentle-ai-worker` writers with disjoint file surfaces after `types.ts`/`trackCoords.ts` were frozen.
- Foundation: `web/src/features/lab/RacePlanning/model/types.ts` and `trackCoords.ts` (wire adapter, haversine, cumulative distance, D+, slope, slope-adjusted speed, total time, km index, bearing), 26 tests. Commit `ce4b2bb`.
- `splits.ts` (commit `825ee6d`), `raceDay.ts` (`9cafe97`), `solar.ts` (`21b0aa9`), `gear.ts` (`6e7c06d`), each with its colocated tests.
- The solar worker found that the installed `suncalc@2.1.2` is a v2 ESM rewrite that returns `altitude` and north-based clockwise `azimuth` already in **degrees** and ships its own typings. Applying the legacy v1 radians conversion would have made noon altitude ~3921° and disabled the `night` branch. The implementation uses the v2 values directly; `@types/suncalc` was removed because the package types it.
- Verification observed: `pnpm -C web test:run` → 64 files / 404 tests passed; `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web format:check` all clean.
- Deliberate simplifications recorded: splits use one row per whole kilometre with the prototype's fatigue model (+5% pace per 10 km); cutoffs use a flat `timeLimit / km` required pace (elevation-aware arrival, flat required-pace estimate); solar `shade` is an aspect/intensity heuristic with no cast-shadow terrain model; nutrition prefers the flattest point within ±500 m and labels `recovery` when the chosen slope is <= 0.
