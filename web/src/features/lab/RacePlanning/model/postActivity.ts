/**
 * Plan-vs-real model for the Post-Activity tab (phase 1.6, issue #18, S3/S5/S8).
 *
 * Pure TypeScript, no network. The planned splits come from the stored GPX
 * track; the real splits come from the activity's own `latlng` + `time`
 * streams. Because the real cumulative distance is measured along the actual
 * path, a route that differs from the plan is still a valid comparison.
 */

import { generateStrategicSplits } from './splits';
import { cumulativeKm } from './trackCoords';
import type { PlanVsRealRow, TrackPoint } from './types';

/**
 * Raw streams of one Strava activity. `latlng` uses Strava's `[lat, lon]`
 * order; `timeSeconds` holds second offsets from the activity start.
 */
export interface ActivitySample {
  latlng: Array<[number, number]>;
  timeSeconds: number[];
}

/** Tolerance when matching an activity point to a whole planned kilometre. */
const KM_EPSILON = 1e-6;

/**
 * One row per planned kilometre that the activity also reached.
 *
 * Real distance is cumulative along the activity, and the real split is the
 * difference of consecutive activity timestamps at each whole kilometre.
 * The fatigue index is the real overshoot as a percentage of the planned
 * split, clamped at `0` so a faster real kilometre never reports negative
 * fatigue.
 */
export function comparePlanVsReal(
  plannedPoints: TrackPoint[],
  activity: ActivitySample,
  basePaceMinPerKm: number,
  fatigueFactor?: number,
): PlanVsRealRow[] {
  if (plannedPoints.length < 2) return [];
  if (!Number.isFinite(basePaceMinPerKm) || basePaceMinPerKm <= 0) return [];

  const { latlng, timeSeconds } = activity;
  if (timeSeconds.length < 2) return [];
  const samples = Math.min(latlng.length, timeSeconds.length);
  if (samples < 2) return [];

  const activityPoints: TrackPoint[] = [];
  for (let index = 0; index < samples; index += 1) {
    const [lat, lon] = latlng[index];
    activityPoints.push({ lat, lon, ele: null });
  }
  const activityCumulativeKm = cumulativeKm(activityPoints);

  const plannedRows = generateStrategicSplits(plannedPoints, basePaceMinPerKm, fatigueFactor);

  const rows: PlanVsRealRow[] = [];
  let previousRealCumulativeMin = 0;
  let searchStart = 0;

  for (const planned of plannedRows) {
    let index = -1;
    for (let candidate = searchStart; candidate < activityCumulativeKm.length; candidate += 1) {
      if (activityCumulativeKm[candidate] >= planned.km - KM_EPSILON) {
        index = candidate;
        break;
      }
    }
    // The activity ended before this planned kilometre.
    if (index < 0) break;

    const realCumulativeMin = timeSeconds[index] / 60;
    const realSplitMin = realCumulativeMin - previousRealCumulativeMin;
    const diffMin = realSplitMin - planned.splitTimeMin;
    const fatigueIndex =
      planned.splitTimeMin > 0 ? Math.max(0, (diffMin / planned.splitTimeMin) * 100) : 0;

    rows.push({
      km: planned.km,
      plannedSplitMin: planned.splitTimeMin,
      realSplitMin,
      diffMin,
      plannedCumulativeMin: planned.cumulativeTimeMin,
      realCumulativeMin,
      fatigueIndex,
    });

    previousRealCumulativeMin = realCumulativeMin;
    searchStart = index;
  }

  return rows;
}
