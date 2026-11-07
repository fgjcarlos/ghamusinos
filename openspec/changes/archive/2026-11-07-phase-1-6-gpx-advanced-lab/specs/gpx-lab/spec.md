# Spec Delta: phase-1-6-gpx-advanced-lab

> Domain: gpx-lab
> Source: openspec/changes/archive/2026-11-07-phase-1-6-gpx-advanced-lab/{proposal,design}.md
> Cross-project: `web/` (TypeScript client and UI)
> Stack: React 19 + TypeScript 6 + Vite 8 + Vitest 5 + @testing-library/react + jsdom + suncalc 2.1.2
> Gating test command: `pnpm -C web test:run`
> Quality gates: `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web format:check`

This delta adds the pure race-planning model, the same-origin API client and
hooks, the race-planning section mounted in the track detail page, and the
Post-Activity comparison. The canonical `openspec/specs/gpx-lab/spec.md`
already exists; the archive step appends these requirements to it.

Every model function is pure, uses named `{lat, lon, ele}` fields and returns
typed values; no model module performs network I/O. The UI only fetches
weather, terrain or activity streams on an explicit user action.

## ADDED Requirements

### Requirement: the geometry model derives distance, gain and time from named track points (`GL-001`)

`web/src/features/lab/RacePlanning/model/trackCoords.ts` SHALL expose:

- `toTrackPoints(input: unknown[])` → `TrackPoint[]`, accepting `{lat, lon}`
  or `{lat, lng}`, dropping non-finite coordinates and mapping an invalid
  `ele` to `null`;
- `haversineKm`, `cumulativeKm`, `totalDistanceKm`, `dPlusM`,
  `slopePercentAt`, `adjustedSpeedKmh`, `estimateTotalTimeMin`, `indexAtKm`
  and `bearingDeg`.

The slope-adjusted speed SHALL implement: uphill
`base * max(0.3, 1 - slope*0.025)`; moderate downhill
`base * (1 + |slope|*0.02)`; steep downhill
`base * max(0.7, 1.3 - (|slope|-15)*0.05)`. `estimateTotalTimeMin` SHALL
integrate `distance / adjustedSpeed` over segments and return minutes.
`indexAtKm` SHALL return the first index whose cumulative distance reaches
the target (with a sub-millimetre epsilon) and clamp out-of-range targets to
the first/last point.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/lab/RacePlanning/model/trackCoords.ts`

#### Scenario: one flat kilometre at 6 min/km takes six minutes

- GIVEN a 1 km flat track with valid elevations
- WHEN `estimateTotalTimeMin(points, 6)` is called
- THEN the result is approximately 6 minutes

#### Scenario: a climb takes longer than the flat and a descent less

- GIVEN a 1 km climb of +100 m and a 1 km descent of −100 m at the same base
  pace
- WHEN the total times are compared with the flat kilometre
- THEN the climb is slower than the flat
- AND the descent is faster than the flat

#### Scenario: an invalid point is dropped instead of poisoning distances

- GIVEN a raw point array containing `null`, a `NaN` latitude and one valid
  point
- WHEN `toTrackPoints` is called
- THEN only the valid point remains and its `ele` is finite or `null`

### Requirement: the model generates per-kilometre splits and evaluates cutoffs (`GL-002`)

`web/src/features/lab/RacePlanning/model/splits.ts` SHALL expose:

- `generateStrategicSplits(points, basePaceMinPerKm, fatigueFactor = 0.05)`
  → one `SplitRow` per whole kilometre, with the split time computed on the
  elevation-aware model and the fatigue factor applied as
  `pace * (1 + fatigueFactor * km / 10)`; each row carries `km`,
  `splitTimeMin`, `cumulativeTimeMin`, `gainM` and `paceMinPerKm`;
- `evaluateCutoffs(points, basePaceMinPerKm, cutoffs)` → one
  `CutoffResult` per valid input, with the elevation-aware `estimatedMin`,
  `isSafe = estimatedMin < timeLimitMin`, `marginMin` and a flat
  `requiredPaceMinPerKm = timeLimitMin / km`.

Invalid cutoffs (non-finite, `km <= 0`, `timeLimitMin <= 0`) SHALL be
filtered out. A track under 1 km SHALL yield no splits.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/lab/RacePlanning/model/splits.ts`

#### Scenario: a flat 10 km track yields ten splits with fatigue

- GIVEN a 10 km flat track and a 6 min/km base pace
- WHEN `generateStrategicSplits` is called with the default fatigue
- THEN ten rows are returned
- AND the first split is approximately 6 minutes
- AND the cumulative time grows by each split
- AND the last split is slower than the first

#### Scenario: a cutoff before its deadline is safe

- GIVEN a flat track and a cutoff at 5 km with a 40-minute limit
- WHEN `evaluateCutoffs` is called
- THEN the estimated arrival is approximately 30 minutes
- AND `isSafe` is true and the margin is approximately +10 minutes

#### Scenario: a cutoff exactly at the estimate is not safe

- GIVEN a cutoff whose time limit equals the estimated arrival
- WHEN `evaluateCutoffs` is called
- THEN `isSafe` is false because the comparison is strict `<`

### Requirement: race day provides markers, nutrition points and pace presets (`GL-003`)

`web/src/features/lab/RacePlanning/model/raceDay.ts` SHALL expose
`kmMarkers(points)`, `nutritionPoints(points, intervalKm = 10)`, the
`PACE_PRESETS` array (hike 7.5, easy 6.0, steady 5.0, fast 4.2 min/km) and
the defaults `DEFAULT_TARGET_PACE_MIN_PER_KM = 6.5`,
`DEFAULT_NUTRITION_INTERVAL_KM = 10` and `DEFAULT_FATIGUE_FACTOR = 0.05`.

`kmMarkers` SHALL return one marker per whole kilometre including km 0, with
the point index and cumulative gain, deduped by index. `nutritionPoints`
SHALL emit one point per interval strictly below the total distance, choosing
the flattest point within ±500 m of the interval and marking it `recovery`
when the chosen slope is <= 0, otherwise `scheduled`; duplicate indexes SHALL
not repeat.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/lab/RacePlanning/model/raceDay.ts`

#### Scenario: a 3 km track has four km markers

- GIVEN a 3 km flat track
- WHEN `kmMarkers` is called
- THEN markers exist for km 0, 1, 2 and 3
- AND each marker points at the first track index that reaches that kilometre

#### Scenario: nutrition every 10 km on a 25 km track

- GIVEN a 25 km track and the default interval
- WHEN `nutritionPoints` is called
- THEN points exist at km 10 and km 20
- AND no point is emitted at the finish

### Requirement: solar exposure classifies each segment as sun, shade or night (`GL-004`)

`web/src/features/lab/RacePlanning/model/solar.ts` SHALL expose
`calculateSolarExposure(points, startAt, basePaceMinPerKm)` and use
`suncalc@2.1.2`, whose `getPosition` returns `altitude` and north-based
clockwise `azimuth` already in degrees.

For each segment, the function SHALL compute the arrival instant from the
base pace and slope-adjusted speed, the sun position at the segment midpoint,
the segment bearing/aspect and the angular difference between the sun azimuth
and the aspect. It SHALL classify `night` when the sun altitude is <= 0,
`shade` when the difference is > 90°, and `sun` otherwise with an intensity
`cos(diff) * sin(altitude)` clamped to [0, 1]; intensities <= 0.1 become
`shade`. Consecutive segments with the same status SHALL merge, and
`aspectDeg` SHALL remain finite in [0, 360).

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/lab/RacePlanning/model/solar.ts`

#### Scenario: midnight is night and merges into one segment

- GIVEN a flat track and a local midnight start
- WHEN `calculateSolarExposure` is called
- THEN every segment has status `night`
- AND consecutive night segments merge into one covering the whole track

#### Scenario: a summer noon produces no night segment

- GIVEN a flat east-west track and a summer noon start
- WHEN the function is called
- THEN no segment has status `night`
- AND every `aspectDeg` is finite and in [0, 360)

### Requirement: the gear checklist derives mandatory and recommended items (`GL-005`)

`web/src/features/lab/RacePlanning/model/gear.ts` SHALL expose
`calculateRequiredGear(context: GearContext)` and apply, in order:

- duration > 3 h → whistle and thermal blanket (mandatory);
- duration > 6 h → first-aid kit and 1.5 L water reserve (mandatory);
- otherwise duration > 2 h → two 500 ml soft flasks (recommended);
- D+ > 800 m, or D+ > 500 m with duration > 4 h → trekking poles
  (recommended);
- max altitude > 2000 m → windbreaker (mandatory);
- min temperature < 5 °C or rain → waterproof jacket (mandatory) and thermal
  gloves (recommended);
- max wind > 30 km/h → buff (recommended).

Items SHALL be deduplicated by id with mandatory precedence, non-finite or
negative duration/D+/altitude SHALL be treated as 0, and the function SHALL
never throw. Ids SHALL be English; display names and reasons Spanish.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/lab/RacePlanning/model/gear.ts`

#### Scenario: a 3.5 h outing gets the 3 h mandatory items

- GIVEN a context with `durationHours = 3.5`, low D+ and low altitude
- WHEN `calculateRequiredGear` is called
- THEN `whistle` and `thermal-blanket` are mandatory
- AND the recommended list contains the soft flasks

#### Scenario: cold and rain add the waterproof layer exactly once

- GIVEN `minTemperatureC = 3` and `rainy = true`
- WHEN the checklist is built
- THEN `waterproof` is mandatory exactly once
- AND `gloves` is recommended exactly once

### Requirement: the API client and hooks keep the browser same-origin and on demand (`GL-006`)

`web/src/lib/api/lab.ts` SHALL expose `fetchTrackWeather`,
`fetchTrackTerrain`, `fetchActivityStreams` and `refreshActivityStreams`,
using same-origin `/api/v1` URLs, the bearer header and RFC 9457 problem
details mapped to `ApiError`.

The hooks SHALL:

- `useTrackWeather` / `useTrackTerrain` load only when `load()` is called,
  keep the last request wins, and expose `data`, `loading` and `error`;
- `useActivityStreams` load on `load()`, map a `409` to `needs-refresh` with
  the backend detail, and expose `refresh()` which enqueues the re-import and
  leaves the section in `needs-refresh`.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/lib/api/lab.ts`, `web/src/features/lab/RacePlanning/hooks/`

#### Scenario: the weather client sends the encoded instant and bearer token

- GIVEN a token, a track id and an RFC3339 instant
- WHEN `fetchTrackWeather` is called
- THEN the request goes to `/api/v1/gpx/{id}/weather?at=<encoded>`
- AND it carries `Authorization: Bearer <token>`

#### Scenario: a missing stream maps to the refresh action

- GIVEN `fetchActivityStreams` rejects with a `409` problem detail
- WHEN `useActivityStreams.load()` runs
- THEN the hook state is `needs-refresh`
- AND the detail is the backend message
- AND `refresh()` calls `refreshActivityStreams`

### Requirement: the race-planning section is mounted in the track detail page and degrades per panel (`GL-007`)

`web/src/features/lab/RacePlanning/RacePlanSection/RacePlanSection.tsx` SHALL
render eight accessible tabs (Carrera, Splits, Cortes, Terreno, Meteo, Solar,
Equipo, Post-actividad) and SHALL be mounted inside `TrackDetailPage` after
`RouteDetail`, receiving `trackId`, `points` and `token`.

The section SHALL NOT request weather, terrain or activity streams on mount.
Each tab SHALL be mounted on first visit and kept hidden afterwards. A
weather or terrain failure SHALL render only inside its own panel; the rest of
the section and the track detail SHALL remain usable. An empty track SHALL
render a clear insufficient-points state instead of crashing. UI copy SHALL
be Spanish; identifiers and comments English.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/lab/RacePlanning/RacePlanSection/RacePlanSection.tsx`, `web/src/features/lab/TrackDetailPage/TrackDetailPage.tsx`

#### Scenario: the initial render makes no third-party request

- GIVEN a rendered track detail page with a valid track
- WHEN the section mounts
- THEN `fetchTrackWeather` and `fetchTrackTerrain` are not called
- AND the Carrera tab is selected and shows the estimated time and km markers

#### Scenario: a terrain failure stays inside its panel

- GIVEN the terrain request rejects
- WHEN the user opens the Terreno tab and presses the button
- THEN the panel shows an alert
- AND the tab list and the Carrera panel remain usable

### Requirement: Post-Activity compares planned and real per-kilometre splits (`GL-008`)

`web/src/features/lab/RacePlanning/model/postActivity.ts` SHALL expose
`comparePlanVsReal(plannedPoints, activity, basePaceMinPerKm,
fatigueFactor?)`, where `activity` is `{latlng: [lat, lon][],
timeSeconds: number[]}`. It SHALL compute the real cumulative distance along
the activity, take the activity's own split at each whole planned kilometre,
and return one `PlanVsRealRow` per comparable kilometre with `km`,
`plannedSplitMin`, `realSplitMin`, `diffMin`, `plannedCumulativeMin`,
`realCumulativeMin` and
`fatigueIndex = max(0, diffMin / plannedSplitMin * 100)`. It SHALL stop when
the activity ends before the planned kilometre and return `[]` for invalid
inputs.

`PostActivityPanel` SHALL load the streams on demand, extract finite `time`
and `[lat, lon]` pairs defensively, render the comparison table, and offer a
`Re-sincronizar` action when the hook is in `needs-refresh`.

**Project root**: `web/`
**Gating test command**: `pnpm -C web test:run`
**Pin point**: `web/src/features/lab/RacePlanning/model/postActivity.ts`, `web/src/features/lab/RacePlanning/RacePlanSection/PostActivityPanel.tsx`

#### Scenario: an identical flat plan and activity has zero difference

- GIVEN a 3 km flat plan at 6 min/km and an activity whose `time` stream is
  `[0, 360, 720, 1080]` over the same distance
- WHEN `comparePlanVsReal` is called
- THEN three rows are returned
- AND each difference is approximately zero
- AND each fatigue index is approximately zero

#### Scenario: a slower activity reports positive fatigue

- GIVEN the same plan and an activity one minute per kilometre slower
- WHEN the comparison runs
- THEN the first difference is approximately +1 minute
- AND the first fatigue index is approximately 16.7 %

#### Scenario: missing streams offer the re-sync action

- GIVEN the activity-streams endpoint returns `409`
- WHEN the user opens Post-actividad and loads the activity
- THEN the panel shows the detail and a `Re-sincronizar` button
- AND pressing it calls the refresh endpoint
