# Spec Delta: phase-1-6-gpx-advanced-lab

> Domain: gpx-backend
> Source: openspec/changes/archive/2026-11-07-phase-1-6-gpx-advanced-lab/{proposal,design}.md
> Cross-project: `.` (Go)
> Stack: Go 1.26.10 + chi/v5 + pgx/v5 + riverqueue + goose/v3 + sqlc + testify + httptest
> Gating test command: `TZ=UTC GOTOOLCHAIN=go1.26.10 go test -race ./...`
> Quality gates: `go vet ./...`, `golangci-lint run ./...`

This delta adds the backend slice of phase 1.6: the Strava `time` stream and
its enqueue fix, the activity-streams read/refresh endpoints, and the
track-scoped weather and terrain proxies. The canonical
`openspec/specs/gpx-backend/spec.md` already exists; the archive step appends
these requirements to it.

All routes live inside the authenticated `/api/v1` group. Every handler
resolves the resource through the authenticated user, and neither
environmental endpoint accepts coordinates or URLs from the client.

## ADDED Requirements

### Requirement: the streams import requests `time` and is enqueued after every activity upsert (`GB-001`)

`internal/jobs/streams.go` SHALL request the Strava stream types `time`,
`heartrate`, `watts`, `cadence`, `altitude` and `latlng`.

Both production ingestion paths SHALL enqueue `ImportStravaStreamsArgs` after
a successful activity upsert, when a River inserter is configured:

- `ImportStravaWorker.Work` (backfill), once per imported activity; and
- `IngestActivityEventWorker.Work` (webhook), once per ingested activity.

The insert SHALL use `river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs:
true}}` so duplicate pending jobs collapse. A failure to enqueue SHALL be
logged and SHALL NOT fail the activity upsert or the webhook event.

**Project root**: `.`
**Gating test command**: `go test -race ./internal/jobs`
**Pin point**: `internal/jobs/streams.go`, `internal/jobs/workers.go`

#### Scenario: the webhook enqueues the streams import after the recalc

- GIVEN an `IngestActivityEventWorker` with a capturing River inserter
- WHEN it processes a valid activity event
- THEN the inserter receives exactly two jobs
- AND the first is `RecalcTrainingLoadArgs`
- AND the second is `ImportStravaStreamsArgs` with a non-empty `UserID` and
  the activity's `StravaActivityID`
- AND both use `UniqueOpts.ByArgs = true`

#### Scenario: the worker requests the time stream

- GIVEN an `ImportStravaStreamsWorker` with a fake fetcher
- WHEN it processes a job
- THEN the fetcher receives a stream-type list that contains `time`,
  `latlng` and `altitude`

#### Scenario: the backfill enqueues both jobs for each imported activity

- GIVEN an `ImportStravaWorker` with a River inserter and one fetched activity
- WHEN the backfill runs
- THEN the inserter receives a recalc job and a streams job for that activity

### Requirement: the authenticated read endpoint returns the activity streams (`GB-002`)

`GET /api/v1/activities/{id}/streams` SHALL be mounted in the authenticated
group. It SHALL:

- answer `401` with an RFC 9457 problem document when there is no
  authenticated user;
- parse `{id}` as the Strava external id (`int64`) and answer `400` when it is
  not numeric;
- resolve the activity with `GetActivityByExternalID` using the authenticated
  `user_id` and `external_source = 'strava'`, and answer `404` when it does
  not exist or belongs to another user;
- answer `200` with
  `{"activity_id":"<uuid>","started_at":"<RFC3339 UTC>","streams":{...}}`,
  where `streams` maps each stored `stream_type` to its raw JSONB value;
- answer `409 Streams Unavailable` when `time` or `latlng` is absent, `null`
  or an empty array, with a `detail` naming the missing types.

**Project root**: `.`
**Gating test command**: `go test -race ./internal/http/handlers`
**Pin point**: `internal/http/handlers/activity_streams.go`

#### Scenario: an activity with time and latlng returns every stored stream

- GIVEN an owned activity with `time` and `latlng` rows
- WHEN the authenticated endpoint is called
- THEN the response is `200`
- AND `streams.time` and `streams.latlng` carry the stored arrays
- AND `activity_id` and `started_at` identify the activity

#### Scenario: a missing time stream returns a 409 naming it

- GIVEN an owned activity with only a `latlng` row
- WHEN the endpoint is called
- THEN the response is `409` with title `Streams Unavailable`
- AND the detail contains `time`

#### Scenario: an activity from another user is not found

- GIVEN an activity that exists for a different `user_id`
- WHEN the endpoint is called by the authenticated user
- THEN the response is `404` and no stream is returned

### Requirement: the refresh endpoint re-enqueues the streams import (`GB-003`)

`POST /api/v1/activities/{id}/streams/refresh` SHALL be mounted in the
authenticated group. It SHALL resolve ownership exactly like `GB-002` and:

- answer `404` for an unknown or foreign activity;
- answer `503` when the River inserter is not configured or the insert fails;
- answer `202` with `{"status":"queued","job_id":"...","queued_at":"..."}`
  after enqueueing `ImportStravaStreamsArgs` for the authenticated user and
  the activity's external id, with `UniqueOpts.ByArgs = true`.

**Project root**: `.`
**Gating test command**: `go test -race ./internal/http/handlers`
**Pin point**: `internal/http/handlers/activity_streams.go`

#### Scenario: the refresh enqueues a job for the owned activity

- GIVEN an owned activity and a configured River inserter
- WHEN the refresh endpoint is called
- THEN the inserter receives `ImportStravaStreamsArgs` with the user and
  external id
- AND the response is `202` with `status = queued`

#### Scenario: no queue configured returns 503

- GIVEN no River inserter
- WHEN the refresh endpoint is called for an owned activity
- THEN the response is `503` and no job is created

### Requirement: the weather endpoint forecasts four strategic points of the stored track (`GB-004`)

`GET /api/v1/gpx/{id}/weather` SHALL be mounted in the authenticated
`/api/v1/gpx` group. It SHALL:

- answer `401` without an authenticated user, `404` for an invalid, unknown
  or foreign track, and `404` when the store returns `pgx.ErrNoRows`;
- require `at` as RFC3339 and answer `400` when it is missing or invalid;
- load the track with `GetDetail(..., resolution 0)` and derive the
  coordinates exclusively from the stored points;
- select four roles, deduped by point index preserving order: `start` (index
  0), `highest` (maximum non-nil `ele`), `midpoint` (len/2) and `end`
  (len-1);
- query Open-Meteo at
  `https://api.open-meteo.com/v1/forecast` with
  `hourly=temperature_2m,weathercode,windspeed_10m`, `forecast_days=16` and
  `timezone=UTC`, selecting the hour at `at.UTC().Truncate(time.Hour)`;
- skip a point that is outside the forecast horizon and answer `422` when
  every point is out of range;
- answer `503` when a point fails for any other upstream reason;
- answer `200` with
  `{"track_id","at","timezone":"UTC","points":[{"role","km","lat","lon","ele","time","temperature_c","wind_kmh","weather_code","label","available"}]}`,
  including only available points in role order;
- cache successful `200` bodies in memory for 1 h keyed by track id + UTC
  hour, consulting the cache only after `GetDetail` has enforced ownership.

**Project root**: `.`
**Gating test command**: `go test -race ./internal/weather ./internal/http/handlers`
**Pin point**: `internal/http/handlers/lab_weather.go`, `internal/weather/weather.go`

#### Scenario: a normal forecast returns the four deduped roles in order

- GIVEN an owned track with a highest point between the start and the end
- WHEN the endpoint is called with a valid `at`
- THEN the response is `200`
- AND the roles are `start`, `highest`, `midpoint`, `end` in that order
- AND each point carries a finite temperature, wind speed and WMO label

#### Scenario: a short track never repeats a point

- GIVEN a track whose start, highest, midpoint and end collapse to fewer
  indices
- WHEN the endpoint is called
- THEN each index appears at most once, keeping the first role that claimed it

#### Scenario: an out-of-horizon instant returns 422

- GIVEN an `at` outside the Open-Meteo forecast window for every point
- WHEN the endpoint is called
- THEN the response is `422` with a clear problem detail

#### Scenario: a repeated request is served from cache

- GIVEN two identical requests for the same track and UTC hour
- WHEN both are served
- THEN the upstream fetcher is called once

### Requirement: the terrain endpoint maps OSM surfaces to technicality segments (`GB-005`)

`GET /api/v1/gpx/{id}/terrain` SHALL be mounted in the authenticated
`/api/v1/gpx` group. It SHALL:

- apply the same auth/404/ownership rules as `GB-004`;
- derive the coordinates exclusively from the stored track;
- query Overpass with a decimated polyline (at most 80 points) using
  `around:50.0`, a `["highway"]` filter and `out tags geom`, as a
  form-encoded `data=` POST with a descriptive `User-Agent` and a 25 s
  timeout;
- attribute each ~100 m sample to the closest way **segment** within 50 m
  (point-to-segment distance), map `surface`/`highway` to
  `paved|gravel|trail|technical|unknown` + technicality 1–5, and merge
  consecutive samples with the same `(surface, name)`;
- answer `200` with
  `{"track_id","available":true,"segments":[{"start_km","end_km","surface","technicality","name"}]}`;
- answer `200 {"track_id","available":false,"segments":[]}` when there are no
  ways or no match (a valid result);
- answer `503` when Overpass fails or times out;
- cache successful `200` bodies in memory for 24 h keyed by track id,
  consulting the cache only after `GetDetail` has enforced ownership.

**Project root**: `.`
**Gating test command**: `go test -race ./internal/terrain ./internal/http/handlers`
**Pin point**: `internal/http/handlers/lab_terrain.go`, `internal/terrain/client.go`, `internal/terrain/segments.go`

#### Scenario: a way near the track becomes a surface segment

- GIVEN an owned track and an Overpass response with a paved way within 50 m
- WHEN the endpoint is called
- THEN the response is `200` with at least one segment
- AND the segment has a km range, a surface kind and technicality 1–5

#### Scenario: no ways is an empty but successful result

- GIVEN an Overpass response with no matching ways
- WHEN the endpoint is called
- THEN the response is `200` with `available:false` and `segments:[]`

#### Scenario: an upstream failure degrades the section

- GIVEN an Overpass error or timeout
- WHEN the endpoint is called
- THEN the response is `503` with an RFC 9457 problem document

### Requirement: both environmental endpoints are track-scoped and authenticated (`GB-006`)

The weather and terrain routes SHALL be mounted only inside the
authenticated `/api/v1/gpx` group. Their handlers SHALL NOT read
coordinates or upstream URLs from the query string or body; the only input
besides the track id is the weather `at` instant. The complete production
route list test SHALL include
`GET /api/v1/gpx/{id}/weather` and `GET /api/v1/gpx/{id}/terrain`.

**Project root**: `.`
**Gating test command**: `go test -race ./internal/app ./internal/http`
**Pin point**: `internal/http/router.go`, `internal/app/routes_test.go`

#### Scenario: an anonymous environmental request is rejected

- GIVEN no authenticated user
- WHEN either environmental route is called
- THEN the response is `401` and no upstream request is made

#### Scenario: the route table is pinned

- GIVEN the production router is built
- WHEN the route list is collected
- THEN it contains both environmental routes under `/api/v1/gpx`
