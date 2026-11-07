# Design: phase-1-6-gpx-advanced-lab

> Arquitectura de la planificación de carrera y del contexto ambiental
> sobre un track GPX guardado. Base: `main@2725d2e`.

## 1. Constraints

- **CSP**: la SPA se sirve con `connect-src 'self'`
  (`internal/http/security_middleware.go`). El navegador no puede llamar a
  Open-Meteo ni a Overpass.
- **Datos**: el track es del usuario autenticado; el servidor ya tiene sus
  puntos (`gpx.StoredTrackDetail.Track.Track.Points`, con `lat`, `lon`, `ele`).
- **Sin tabla de planes**: la planificación es efímera, derivada del track y
  de los parámetros de la sesión.
- **Frontend estricto**: `strict`, `noUnusedLocals`,
  `exactOptionalPropertyTypes`; sin `any`.
- **Tests sin red**: los clientes HTTP se prueban con `httptest` y el
  cliente web con `fetch` simulado.

## 2. Architecture

```text
TrackDetailPage
 └── RacePlanSection (tabs)
      ├── model/*            pure TypeScript, no network
      ├── hooks/*            request lifecycle only
      └── lib/api/lab.ts     fetch('/api/v1/...')

Go /api/v1 (authenticated)
 ├── /activities/{id}/streams          → activity_streams (Strava)
 ├── /activities/{id}/streams/refresh  → river: import_strava_streams
 ├── /gpx/{id}/weather?at=...          → Open-Meteo (1 h cache)
 └── /gpx/{id}/terrain                 → Overpass (24 h cache)
```

The browser only talks to the same origin. The Go handlers read the stored
track with `GetDetail(ctx, userID, trackID, 0)` (which enforces ownership),
derive the coordinates server-side, and call the third party. Gore:

- no client-supplied coordinates or URLs, so no SSRF surface;
- the third party never sees the user identity or token;
- the server can cache and time out.

## 3. Backend design

### 3.1 Activity streams

**Problem found:** `ImportStravaStreamsWorker` was registered but no
production path enqueued `ImportStravaStreamsArgs`. `activity_streams` was
therefore empty for real users, silently degrading HR zones and cardiac
drift too.

**Fix:** enqueue the job after every successful activity upsert:

- `ImportStravaWorker` (backfill loop) → one job per activity.
- `IngestActivityEventWorker` (webhook) → one job per activity.

Both use `river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}` to
collapse duplicate pending jobs. The stream request now includes `time`.

**Read endpoint:** `GET /api/v1/activities/{id}/streams`

- resolves the activity by `(user_id, external_source='strava', external_id)`
  with the same query as `GetActivity`, so cross-user access is impossible;
- returns every stored stream as raw JSON:
  `{"activity_id":"<uuid>","started_at":"...","streams":{"time":[...],"latlng":[[lat,lon],...],"altitude":[...]}}`;
- `409 Streams Unavailable` with `missing: time, latlng` in the detail when a
  required stream is absent or an empty array/`null`.

**Refresh endpoint:** `POST /api/v1/activities/{id}/streams/refresh`

- same ownership check;
- `202 {"status":"queued","job_id":"...","queued_at":"..."}`;
- `503` when the River queue is not configured or the insert fails;
- `404` for an unknown activity.

**Backfill note:** existing activities do not gain `time` automatically. The
UI offers the refresh action when the read returns `409`. A bulk backfill is
a follow-up; the per-activity action is enough for the lab.

### 3.2 Weather proxy

`GET /api/v1/gpx/{id}/weather?at=<RFC3339>`

- `at` is required; invalid → `400`.
- Load the track with `GetDetail(..., 0)`; unknown/unowned → `404`.
- Select four points, deduped by index preserving order: start, highest
  (`ele` max, nil elevations ignored), midpoint, end.
- Call Open-Meteo:
  `GET https://api.open-meteo.com/v1/forecast?latitude=..&longitude=..&hourly=temperature_2m,weathercode,windspeed_10m&forecast_days=16&timezone=UTC`
  and select the hourly entry at `at.UTC().Truncate(time.Hour)`.
- `ErrOutOfRange` skips that point; if every point is out of range → `422`.
  Any other upstream failure → `503`.
- Response:
  `{"track_id":"<uuid>","at":"<RFC3339 UTC>","timezone":"UTC","points":[{"role":"start","km":0,"lat":...,"lon":...,"ele":...,"time":"...","temperature_c":...,"wind_kmh":...,"weather_code":...,"label":"...","available":true}]}`
- Cache: in-memory, 1 h, keyed by track id + UTC hour. The cache is consulted
  **after** `GetDetail`, so a cache hit still proves ownership.
- The client sends the race instant as an ISO string; the UI converts a
  `datetime-local` value (local wall time) with `new Date(value).toISOString()`.

### 3.3 Terrain proxy

`GET /api/v1/gpx/{id}/terrain`

- Load the track (ownership enforced), build a decimated polyline of at most
  80 points, and query Overpass:
  `[out:json][timeout:25];way(around:50.0,lat1,lon1,...)["highway"];out tags geom;`
  as a form-encoded `data=` POST with a descriptive `User-Agent`.
- For each ~100 m sample, find the closest way **segment** (point-to-segment
  distance, not nearest node) within 50 m.
- Map `surface`/`highway` to `{paved|gravel|trail|technical|unknown, 1..5}`;
  merge consecutive samples with the same `(surface, name)`.
- Response:
  `{"track_id":"<uuid>","available":true,"segments":[{"start_km":0,"end_km":1.2,"surface":"paved","technicality":1,"name":"Camino"}]}`
- No ways / no match → `200 {"available":false,"segments":[]}` (valid result).
- Upstream error/timeout → `503`.
- Cache: in-memory, 24 h, keyed by track id, consulted after `GetDetail`.

### 3.4 Route wiring

Both environment routes live inside the authenticated
`/api/v1/gpx` group, next to `GET /{id}`. `internal/app/routes_test.go`
pins the complete production route list.

## 4. Web design

### 4.1 Pure model (`web/src/features/lab/RacePlanning/model/`)

| Module | Responsibility |
| --- | --- |
| `types.ts` | Named shapes shared by the model and UI. |
| `trackCoords.ts` | `toTrackPoints` wire adapter, haversine, cumulative km, D+, slope, slope-adjusted speed, total time, `indexAtKm`, bearing. |
| `splits.ts` | `generateStrategicSplits` (one row per whole km, fatigue factor), `evaluateCutoffs`. |
| `raceDay.ts` | Pace presets, `kmMarkers`, `nutritionPoints`, defaults. |
| `solar.ts` | `calculateSolarExposure` with `suncalc@2.1.2`. |
| `gear.ts` | `calculateRequiredGear` from duration/D+/altitude/weather. |
| `postActivity.ts` | `comparePlanVsReal` from the activity's own cumulative distance. |

Key formulas (ported from the prototype, but typed and tested):

- **Slope-adjusted speed**: uphill `base * max(0.3, 1 - slope*0.025)`;
  moderate downhill `base * (1 + |slope|*0.02)`; steep downhill
  `base * max(0.7, 1.3 - (|slope|-15)*0.05)`.
- **Total time**: sum of `distance / adjustedSpeed` over segments, in minutes.
- **Splits**: for each whole km, `estimateTotalTimeMin(slice, pace * (1 + fatigue * km/10))`,
  with `fatigue = 0.05` by default.
- **Cutoffs**: estimated arrival with the elevation-aware model; required
  pace is the flat `timeLimit / km` estimate (documented as approximate).
- **Nutrition**: one point every `intervalKm` (10 default), moved to the
  flattest point within ±500 m; `recovery` when the chosen slope is <= 0.
- **Solar**: SunCalc v2 `getPosition` at the segment midpoint arrival time;
  `altitude`/`azimuth` already in degrees; `night` below the horizon,
  `shade` when the aspect is more than 90° from the sun, else `sun` with
  `intensity = cos(diff) * sin(altitude)`.
- **Gear**: duration > 3 h / > 6 h, D+ > 800 (or > 500 with > 4 h),
  altitude > 2000 m, cold/rain, wind > 30 km/h.
- **Post-Activity**: real cumulative km along `latlng`; real split at each
  whole km from `time`; `diff = real - planned`;
  `fatigue = max(0, diff / planned * 100)`.

### 4.2 API client and hooks

`web/src/lib/api/lab.ts` mirrors `ai.ts`: same-origin `fetch`, bearer
header, problem+json parsed into `ApiError`.

Hooks:

- `useTrackWeather` / `useTrackTerrain`: on-demand, last request wins.
- `useActivityStreams`: `409` → `needs-refresh`; `refresh()` enqueues the
  import and leaves the section in `needs-refresh` with an "en curso"
  message; the user retries `load()`.

### 4.3 UI

`RacePlanSection` is mounted inside `TrackDetailPage` after `RouteDetail`.
It has eight tabs: Carrera, Splits, Cortes, Terreno, Meteo, Solar, Equipo,
Post-actividad.

- Panels are lazily mounted on first visit and then kept hidden, so tab
  switches preserve inputs and results.
- Weather and terrain only fetch when their button is pressed (S10).
- A weather/terrain failure renders only inside its panel (S9).
- Accessible tabs: `role="tab"`, `aria-selected`, `aria-controls`,
  arrow-key navigation, `role="tabpanel"` + `aria-labelledby`.
- Spanish UI copy; English identifiers and comments.

## 5. Testing strategy

- **Go**: handler tests with `httptest`; injected fake streams/weather/
  terrain stores and fetchers; the Overpass/Open-Meteo clients tested with
  `httptest.Server`; the backfill/webhook enqueue tested with a capturing
  River inserter.
- **Web**: pure model tests for every rule; API client tests with stubbed
  `fetch`; hook tests with `renderHook`; component tests with the API module
  mocked and user interactions via `@testing-library/react`.
- No live third-party request in any test.

Observed gates:

- `TZ=UTC GOTOOLCHAIN=go1.26.10 go test -race -count=1 ./...` → green.
- `go vet ./...` and `golangci-lint run ./...` → clean.
- `pnpm -C web test:run` → 68 files / 435 tests passed.
- `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web format:check` → clean.

## 6. Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Widen CSP `connect-src` and call third parties from the browser | Exposes the user's IP and route to third parties, loses server caching, and weakens a security control for a feature. |
| Accept client-supplied coordinates in the proxy | Creates an SSRF/open-proxy surface; the track is already on the server. |
| Align plan and activity points index by index | Fragile when the real route differs; the activity's own cumulative distance is the correct comparison. |
| Add a race-plan persistence table | Not required by the issue; the plan is derived and ephemeral. |
| Use SunCalc v1 radians | The installed v2 returns degrees; the conversion would break the night branch. |
| A recent-activity picker in Post-Activity | Nice UX, but the id from the activity URL is enough for the deliverable; noted as a follow-up. |

## 7. Known limitations

- Weekly/seasonal weather beyond the Open-Meteo horizon is unavailable.
- Overpass coverage is incomplete; `unknown` and `available:false` are normal.
- `shade` is an orientation heuristic, not a cast-shadow simulation.
- Required cutoff pace is a flat estimate; the arrival time is elevation-aware.
- Caches are per process; multiple replicas each call the upstream once.
- Existing activities need the per-activity refresh to gain `time`.
- Post-Activity uses the Strava external id typed by the user; a picker is a
  follow-up.
