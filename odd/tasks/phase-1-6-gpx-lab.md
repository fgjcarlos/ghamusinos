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
| T2 | S3, S7, S8, S9, S10, S11 | parent inline | 2 — backend weather | pending | Track-scoped Open-Meteo endpoint, 4 strategic points, 1 h cache; tests. |
| T3 | S3, S7, S8, S9, S10, S11 | parent inline | 2 — backend terrain | pending | Track-scoped Overpass endpoint, surface/technicality mapping, cache; tests. |
| T4 | S7, S8 | parent inline | 3 — web model | pending | Coordinate adapter, haversine/cumulative distance, D+, adjusted speed, total time; tests. |
| T5 | S3, S8 | parent inline | 3 — web model | pending | Strategic splits + cutoff calculator; tests. |
| T6 | S3, S8 | parent inline | 3 — web model | pending | KM markers, nutrition points, race-day presets; tests. |
| T7 | S3, S8 | parent inline | 3 — web model | pending | Solar exposure with `suncalc`, explicit date/timezone; tests. |
| T8 | S3, S8 | parent inline | 3 — web model | pending | Gear checklist rules from duration/D+/altitude/weather; tests. |
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
