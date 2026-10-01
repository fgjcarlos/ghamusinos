# Spec Delta: phase-1-4-performance-dashboard

> Domain: metrics-go
> Source: openspec/changes/phase-1-4-performance-dashboard/{proposal,exploration}.md
> Cross-project: `.` (Go)
> Stack: Go 1.26.0 + stdlib + testify
> Gating test command: `make test` (raw: `GOTOOLCHAIN=local go test -race ./...`)
> Quality gates: `make lint`, `make vet`, `make fmt`

This delta covers the pure-Go metrics package that PR1 ships under
`internal/metrics/`. Every requirement here is a pure function
(no I/O, no SQL, no HTTP) importable in isolation by PR2
(`internal/metrics/training_load.go` service layer) without
cyclic dependencies. No canonical `openspec/specs/metrics-go/spec.md`
exists yet; archive will copy this file into the canonical
location. Each function publishes at least one golden test
case from canonical literature (TrainingPeaks, Coggan, Minetti,
Pauley) and a degenerate-input sentinel.

## ADDED Requirements

### Requirement: `TSSCycling` matches the TrainingPeaks formula (`M-001`)

`internal/metrics/performance.go` `TSSCycling(ftpWatts int, durationSec int, npWatts int) float64`
SHALL return the TrainingPeaks TSS formula
`TSS = (durationSec × npWatts × IF) / (ftpWatts × 3600) × 100`
where `IF = npWatts / ftpWatts`. When `ftpWatts <= 0`,
`durationSec <= 0`, or `npWatts <= 0`, the function MUST
return `0` and SHALL NOT panic.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/metrics/performance.go` `TSSCycling`

#### Scenario: 1h at FTP yields TSS=100 (golden)

- GIVEN `ftpWatts = 250`, `durationSec = 3600`, `npWatts = 250`
- WHEN `TSSCycling(250, 3600, 250)` is invoked
- THEN it returns a value within float64 tolerance of `100.0`

#### Scenario: 30min at FTP yields TSS=50 (golden)

- GIVEN `ftpWatts = 250`, `durationSec = 1800`, `npWatts = 250`
- WHEN `TSSCycling(250, 1800, 250)` is invoked
- THEN it returns a value within float64 tolerance of `50.0`

#### Scenario: zero duration returns sentinel 0

- GIVEN `ftpWatts = 250`, `durationSec = 0`, `npWatts = 250`
- WHEN `TSSCycling(250, 0, 250)` is invoked
- THEN it returns `0.0`
- AND it does NOT panic and does NOT return `NaN`

### Requirement: `TSSRunning` matches the Balanced/PI formula (`M-002`)

`internal/metrics/performance.go`
`TSSRunning(thresholdSecPerKm int, durationSec int, actualSecPerKm int) float64`
SHALL compute `TSS_running = (durationSec / 3600) ×
(thresholdSecPerKm / actualSecPerKm)^2 × 100`. The pace ratio
is the ratio of actual to threshold velocity, expressed using
seconds per kilometer; therefore a faster-than-threshold pace
has `IF > 1`. Inputs `<= 0` MUST return `0` with no panic.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/metrics/performance.go` `TSSRunning`

#### Scenario: ~10km at threshold pace yields rTSS ≈ 100 (golden)

- GIVEN `thresholdSecPerKm = 240` (4:00/km), `durationSec = 2400`
  (40 min), `actualSecPerKm = 240`
- WHEN `TSSRunning(240, 2400, 240)` is invoked
- THEN it returns a value within float64 tolerance of `100.0`

#### Scenario: ~5km at threshold pace yields rTSS ≈ 50 (golden)

- GIVEN `thresholdSecPerKm = 240`, `durationSec = 1200`
  (20 min), `actualSecPerKm = 240`
- WHEN `TSSRunning(240, 1200, 240)` is invoked
- THEN it returns a value within float64 tolerance of `33.333…`

#### Scenario: faster-than-threshold pace yields IF greater than 1

- GIVEN `thresholdSecPerKm = 240` and `actualSecPerKm = 200`
- WHEN `TSSRunning(240, 3600, 200)` is invoked
- THEN the pace intensity factor `240 / 200` is greater than `1`
- AND the returned TSS is greater than `100.0`

### Requirement: `IntensityFactor` is the Coggan ratio (`M-003`)

`internal/metrics/performance.go`
`IntensityFactor(ftpWatts int, npWatts int) float64`
SHALL return `npWatts / ftpWatts`. `ftpWatts <= 0` MUST return
`0`. Documented typical range `[0.5, 1.2]`.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/metrics/performance.go` `IntensityFactor`

#### Scenario: NP equal to FTP yields IF=1.0 (golden)

- GIVEN `ftpWatts = 250`, `npWatts = 250`
- WHEN `IntensityFactor(250, 250)` is invoked
- THEN it returns a value within float64 tolerance of `1.0`

#### Scenario: NP equal to half FTP yields IF=0.5 (golden)

- GIVEN `ftpWatts = 250`, `npWatts = 125`
- WHEN `IntensityFactor(250, 125)` is invoked
- THEN it returns a value within float64 tolerance of `0.5`

### Requirement: `GradeAdjustedPace` uses the Minetti-normalized curve (`M-004`)

`internal/metrics/performance.go`
`GradeAdjustedPace(grade float64, paceSecPerKm float64) float64`
SHALL return `paceSecPerKm × (C(grade) / C(0))` where
`C(g) = 155.4·g⁵ − 30.4·g⁴ − 43.3·g³ + 46.3·g² + 19.5·g + 3.6`
(Minetti 2002 normalized curve — the convention Strava uses
since 2018 per the proposal's E6 risk). `grade` is clamped to
`[-0.35, +0.35]`. `paceSecPerKm <= 0` MUST return `0`.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/metrics/performance.go` `GradeAdjustedPace`

#### Scenario: grade=0 returns the unchanged pace (golden)

- GIVEN `grade = 0.0`, `paceSecPerKm = 360` (6:00/km)
- WHEN `GradeAdjustedPace(0.0, 360)` is invoked
- THEN it returns a value within float64 tolerance of `360.0`

#### Scenario: grade=+10% roughly doubles the pace (golden)

- GIVEN `grade = 0.10`, `paceSecPerKm = 360`
- WHEN `GradeAdjustedPace(0.10, 360)` is invoked
- THEN it returns a value within float64 tolerance of `720.0`
  (multiplier ≈ 2.0 on the normalized Minetti curve at g=+0.10)

#### Scenario: grade=-10% roughly halves the pace (golden)

- GIVEN `grade = -0.10`, `paceSecPerKm = 360`
- WHEN `GradeAdjustedPace(-0.10, 360)` is invoked
- THEN it returns a value within float64 tolerance of `162.0`
  (multiplier ≈ 0.45 on the normalized Minetti curve at g=-0.10)

### Requirement: `EfficiencyFactor` is the Coggan NP/HR ratio (`M-005`)

`internal/metrics/performance.go`
`EfficiencyFactor(npWatts int, avgHR int) float64` SHALL return
`npWatts / avgHR`. `avgHR <= 0` MUST return `0`
(`activities.avg_hr` is nullable per migration 00004).

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/metrics/performance.go` `EfficiencyFactor`

#### Scenario: NP=200W with HR=150 yields EF=1.33 (golden)

- GIVEN `npWatts = 200`, `avgHR = 150`
- WHEN `EfficiencyFactor(200, 150)` is invoked
- THEN it returns a value within float64 tolerance of `1.333…`

#### Scenario: zero HR returns sentinel 0

- GIVEN `npWatts = 200`, `avgHR = 0`
- WHEN `EfficiencyFactor(200, 0)` is invoked
- THEN it returns `0.0`
- AND it does NOT panic and does NOT return `+Inf`

### Requirement: `CardiacDrift` and `CardiacDriftSeries` (`M-006`)

`internal/metrics/health.go`
`CardiacDrift(hrStart float64, hrEnd float64) float64` SHALL
return `(hrEnd - hrStart) / hrStart * 100` (Pauley convention).
`CardiacDriftSeries(samples []int) float64` SHALL compare the
average of the first quartile vs the average of the third
quartile of `samples` (more conservative than first-half vs
second-half per the exploration; avoids warm-up bias). When
`hrStart <= 0`, the empty-series case, or the
fewer-than-two-samples case applies, the function MUST return
`0`. The web UI SHALL NOT render the Cardiac Drift panel when
no HR streams are present (this requirement belongs to
dashboard-web `DW-005`; PR1 only ships the math).

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/metrics/health.go` `CardiacDrift`, `CardiacDriftSeries`

#### Scenario: HR drift 100 → 110 yields 10% (golden)

- GIVEN `hrStart = 100`, `hrEnd = 110`
- WHEN `CardiacDrift(100, 110)` is invoked
- THEN it returns a value within float64 tolerance of `10.0`

#### Scenario: HR_start=0 returns sentinel 0

- GIVEN `hrStart = 0` and `hrEnd = 110`
- WHEN `CardiacDrift(0, 110)` is invoked
- THEN it returns `0.0`
- AND it does NOT panic and does NOT return `NaN`

#### Scenario: empty series returns sentinel 0

- GIVEN `samples = []int{}` (no HR samples available)
- WHEN `CardiacDriftSeries([]int{})` is invoked
- THEN it returns `0.0`
- AND it does NOT panic and does NOT return `NaN`

### Requirement: `CTL` is the 42-day EMA of TSS (`M-007`)

`internal/metrics/fatigue.go`
`CTL(daily []DailyLoad) float64` SHALL compute the final
chronic training load via exponential moving average with
time constant `TauCTL = 42` days:
`CTL_t = CTL_{t-1} + (TSS_t − CTL_{t-1}) × (1 − exp(−1/42))`.
`CTL_0` MUST be `0`, with no warm-up period; days before the
first supplied activity do not influence the EMA. Equivalently,
for a series of `n_days`, only the supplied daily values are
processed (no synthetic pre-history). `TauCTL` SHALL be exported
so PR2's service layer can reuse the same constant without
duplicating the literal.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/metrics/fatigue.go` `CTL`, `TauCTL`

#### Scenario: constant TSS=100 over 42 days converges to CTL≈100 (golden)

- GIVEN a `daily` slice of 42 entries with `TSS = 100` each day
- WHEN `CTL(daily)` is invoked
- THEN it returns a value within float64 tolerance of `100.0`
  (the EMA converges to the constant input after ≥ τ samples)

#### Scenario: empty series returns 0

- GIVEN `daily = []DailyLoad{}`
- WHEN `CTL(daily)` is invoked
- THEN it returns `0.0`

#### Scenario: first activity cold-starts the EMA without warm-up

- GIVEN the first activity occurs on day N with `TSS = 100`
- WHEN `CTL` is invoked with only the daily values beginning on day N
- THEN day N's CTL is computed from initial zero and that day's TSS
- AND days before day N do not affect the result

### Requirement: `ATL` is the 7-day EMA of TSS (`M-008`)

`internal/metrics/fatigue.go`
`ATL(daily []DailyLoad) float64` SHALL compute the final acute
training load via EMA with `TauATL = 7` days:
`ATL_t = ATL_{t-1} + (TSS_t − ATL_{t-1}) × (1 − exp(−1/7))`.
Same initialization and sentinel rules as `CTL`.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/metrics/fatigue.go` `ATL`, `TauATL`

#### Scenario: TSS=0 over 7 days drops ATL from 100 to ≈41.7 (golden)

- GIVEN a first `daily[0]` with `TSS = 100` and `ATL(0) = 100`
  followed by 7 entries with `TSS = 0`
- WHEN the 7-day zero window is applied (full series of 8 days)
- THEN the resulting ATL is within float64 tolerance of `41.7`

#### Scenario: empty series returns 0

- GIVEN `daily = []DailyLoad{}`
- WHEN `ATL(daily)` is invoked
- THEN it returns `0.0`

### Requirement: `TSB` is the CTL minus ATL balance (`M-009`)

`internal/metrics/fatigue.go`
`TSB(ctl, atl float64) float64` SHALL return `ctl − atl`.
Positive TSB indicates freshness; negative indicates accumulated
fatigue. No sentinels required; the function is a pure
subtraction.

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/metrics/fatigue.go` `TSB`

#### Scenario: CTL=42.1 and ATL=31.0 yields TSB=11.1 (golden)

- GIVEN `ctl = 42.1`, `atl = 31.0`
- WHEN `TSB(42.1, 31.0)` is invoked
- THEN it returns a value within float64 tolerance of `11.1`

#### Scenario: equal CTL and ATL yields TSB=0

- GIVEN `ctl = 50.0`, `atl = 50.0`
- WHEN `TSB(50.0, 50.0)` is invoked
- THEN it returns `0.0`

### Requirement: `FillMissingDays` returns one row per calendar day (`M-010`)

`internal/metrics/fatigue.go`
`FillMissingDays(from time.Time, to time.Time, raw []DailyLoad) []DailyLoad`
SHALL return exactly `(to.Sub(from).Hours()/24) + 1` rows in
UTC-day granularity, one per calendar day in the inclusive
range `[from, to]`. Days present in `raw` (matched by
`Day` field) MUST be carried over with their original
`TSS`. Days absent from `raw` MUST be returned with
`TSS = 0` so the downstream EMA never sees a hole. The
`from > to` case MUST return `nil` or an empty slice without
panicking (caller guarantees valid range, but the function
defends).

**Project root**: `.`
**Gating test command**: `make test`
**Pin point**: `internal/metrics/fatigue.go` `FillMissingDays`

#### Scenario: 7-day range with 3 missing days fills gaps with TSS=0 (golden)

- GIVEN `from = 2026-09-01 UTC`, `to = 2026-09-07 UTC`
  (7 calendar days inclusive) and `raw = []DailyLoad{ {Day: 2026-09-02, TSS: 80}, {Day: 2026-09-04, TSS: 60}, {Day: 2026-09-06, TSS: 90} }`
- WHEN `FillMissingDays(2026-09-01, 2026-09-07, raw)` is invoked
- THEN the returned slice has exactly 7 elements
- AND `result[1].TSS == 80` (09-02), `result[3].TSS == 60` (09-04), `result[5].TSS == 90` (09-06)
- AND `result[0].TSS == 0`, `result[2].TSS == 0`, `result[4].TSS == 0`, `result[6].TSS == 0` (the missing days)

#### Scenario: `from > to` returns empty without panic

- GIVEN `from = 2026-09-07`, `to = 2026-09-01`
- WHEN `FillMissingDays(2026-09-07, 2026-09-01, nil)` is invoked
- THEN the returned slice is `nil` or empty
- AND the function does NOT panic
