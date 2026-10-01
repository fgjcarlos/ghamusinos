# Metrics formula contract

This package implements pure Go functions for phase 1.4. Public signatures and sentinel behavior follow the `metrics-go` delta requirements M-001–M-010. Durations are seconds, cycling power is watts, paces are seconds per kilometer, grade is a fraction (`0.10` means 10%), heart rate is bpm, and load is daily TSS. Non-positive inputs documented below return zero rather than panicking or producing non-finite values.

## M-001 — Cycling Training Stress Score

`TSSCycling(ftpWatts, durationSec, npWatts int) float64` uses the TrainingPeaks/Coggan formula `TSS = (durationSec × npWatts × IF) / (ftpWatts × 3600) × 100`, where `IF = npWatts / ftpWatts`. Non-positive FTP, duration, or NP returns zero. Golden: one hour at 250 W FTP and NP returns 100; 30 minutes returns 50.

## M-002 — Running Training Stress Score

`TSSRunning(thresholdSecPerKm, durationSec, actualSecPerKm int) float64` returns `(durationSec / 3600) × (thresholdSecPerKm / actualSecPerKm)^2 × 100`, the pace representation of the actual-to-threshold velocity ratio (Running Balanced/PI). Faster-than-threshold pace has IF > 1. Any non-positive input returns zero. Golden: one hour at threshold pace returns 100; a 200 sec/km actual pace against a 240 sec/km threshold returns more than 100.

## M-003 — Intensity Factor

`IntensityFactor(ftpWatts, npWatts int) float64` returns NP/FTP (Coggan); FTP <= 0 returns zero. `IntensityFactorRunning(velocity, thresholdVelocity float64) float64` returns velocity/thresholdVelocity; a non-positive threshold velocity returns zero. Golden: NP=FTP gives 1; NP=0.7×FTP gives 0.7.

## M-004 — Grade Adjusted Pace

`GradeAdjustedPace(grade, paceSecPerKm float64) float64` returns `pace × C(grade)/C(0)` using the Minetti 2002 running energy-cost curve `C(g)=155.4g⁵−30.4g⁴−43.3g³+46.3g²+19.5g+3.6`, normalized to level ground. Grade is a fraction clamped to [-0.35, 0.35]; non-positive pace returns zero. Golden: grade 0 preserves a 360 sec/km pace.

## M-005 — Efficiency Factor

`EfficiencyFactor(npWatts, avgHR int) float64` returns NP/average heart rate (Coggan). Non-positive average HR returns zero. Golden: 200 W / 150 bpm = 1.333…

## M-006 — Cardiac Drift

`CardiacDrift(hrStart, hrEnd float64) float64` returns `(HR_end−HR_start)/HR_start × 100` (Pauley); HR_start <= 0 returns zero. `CardiacDriftSeries(samples []int) float64` compares the average of the first quartile with the average of the third quartile and returns their percentage drift; fewer than two samples returns zero. Golden: 100→110 bpm is 10%; empty samples return zero.

## M-007–M-009 — Training load

`DailyLoad` carries `Day time.Time` and `TSS float64`. `CTL(daily)` and `ATL(daily)` return the final Coggan Performance Management Chart exponential moving average with τ=42 and τ=7 days respectively: `load_t = load_(t−1) + (TSS_t−load_(t−1)) × (1−exp(−1/τ))`. Exported `TauCTL=42` and `TauATL=7` are the shared constants. An empty series returns zero; a non-empty EMA is seeded from its first supplied TSS value, with no synthetic warm-up. Values before the first supplied day do not influence the result. `TSB(ctl, atl)` returns `ctl−atl`. Golden: constant TSS=100 over 42 days yields CTL approximately 100; CTL 42.1 minus ATL 31.0 yields 11.1.

## M-010 — Missing calendar days

`FillMissingDays(from, to time.Time, raw []DailyLoad) []DailyLoad` returns one row for every inclusive UTC calendar date, preserving supplied TSS and setting missing days to zero. A reversed range returns an empty slice. Input dates are normalized to UTC-day boundaries. Golden: three supplied loads within a seven-day range produce seven ordered rows, with zero TSS on the other four dates.
