import type { CutoffInput, CutoffResult, SplitRow, TrackPoint } from './types';
import { dPlusM, estimateTotalTimeMin, indexAtKm, totalDistanceKm } from './trackCoords';

/** Fatigue applied to the pace of each kilometre by default. */
const DEFAULT_FATIGUE_FACTOR = 0.05;

/** Splits and cutoffs are only meaningful once there is at least one whole km. */
const MIN_TRACK_KM = 1;

/**
 * One row per whole kilometre of the track.
 *
 * Each split is estimated with the slope-adjusted model over its own slice,
 * with the base pace stretched by an increasing fatigue factor (`1 + factor *
 * km / 10`). Times are left unrounded; the UI decides how to format them.
 */
export function generateStrategicSplits(
  points: TrackPoint[],
  basePaceMinPerKm: number,
  fatigueFactor = DEFAULT_FATIGUE_FACTOR,
): SplitRow[] {
  if (points.length < 2) return [];
  if (!Number.isFinite(basePaceMinPerKm) || basePaceMinPerKm <= 0) return [];
  const totalKm = totalDistanceKm(points);
  if (totalKm < MIN_TRACK_KM) return [];

  const rows: SplitRow[] = [];
  let cumulativeTimeMin = 0;
  const wholeKm = Math.floor(totalKm + 1e-6);

  for (let km = 1; km <= wholeKm; km += 1) {
    const startIndex = indexAtKm(points, km - 1);
    const endIndex = indexAtKm(points, km);
    if (endIndex <= startIndex) continue;

    const slice = points.slice(startIndex, endIndex + 1);
    const factor = 1 + fatigueFactor * (km / 10);
    const splitTimeMin = estimateTotalTimeMin(slice, basePaceMinPerKm * factor);
    cumulativeTimeMin += splitTimeMin;

    rows.push({
      km,
      splitTimeMin,
      cumulativeTimeMin,
      gainM: Math.round(dPlusM(slice)),
      // Every row spans one kilometre, so the pace is the split time itself.
      paceMinPerKm: splitTimeMin,
    });
  }

  return rows;
}

/**
 * Evaluates `(km, deadline)` cutoffs against the track's estimated arrival
 * times. Invalid cutoffs are dropped; valid ones keep their input order and
 * original fields.
 */
export function evaluateCutoffs(
  points: TrackPoint[],
  basePaceMinPerKm: number,
  cutoffs: CutoffInput[],
): CutoffResult[] {
  if (points.length < 2) return [];

  const results: CutoffResult[] = [];
  for (const cutoff of cutoffs) {
    if (!Number.isFinite(cutoff.km) || cutoff.km <= 0) continue;
    if (!Number.isFinite(cutoff.timeLimitMin) || cutoff.timeLimitMin <= 0) continue;

    const index = indexAtKm(points, cutoff.km);
    const estimatedMin = estimateTotalTimeMin(points.slice(0, index + 1), basePaceMinPerKm);

    results.push({
      ...cutoff,
      estimatedMin,
      isSafe: estimatedMin < cutoff.timeLimitMin,
      marginMin: cutoff.timeLimitMin - estimatedMin,
      requiredPaceMinPerKm: cutoff.timeLimitMin / cutoff.km,
    });
  }

  return results;
}
