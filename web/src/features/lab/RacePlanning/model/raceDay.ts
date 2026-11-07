/**
 * Race-day model: pace presets, kilometre milestones and nutrition points.
 *
 * Pure TypeScript, no network. Every coordinate decision is derived from the
 * track's cumulative distance, so the output stays deterministic for a given
 * track.
 */

import { cumulativeKm, dPlusM, indexAtKm, slopePercentAt, totalDistanceKm } from './trackCoords';
import type { KmMarker, NutritionPoint, RacePacePreset, TrackPoint } from './types';

/** Default base pace used when the user has no saved preference. */
export const DEFAULT_TARGET_PACE_MIN_PER_KM = 6.5;

/** Default distance between nutrition stops. */
export const DEFAULT_NUTRITION_INTERVAL_KM = 10;

/** Default per-kilometre fatigue factor for the strategic splits. */
export const DEFAULT_FATIGUE_FACTOR = 0.05;

/** Selectable paces. Labels are Spanish UI copy; ids and code are English. */
export const PACE_PRESETS: RacePacePreset[] = [
  { id: 'hike', label: 'Senderismo', paceMinPerKm: 7.5 },
  { id: 'easy', label: 'Suave', paceMinPerKm: 6.0 },
  { id: 'steady', label: 'Constante', paceMinPerKm: 5.0 },
  { id: 'fast', label: 'Rápido', paceMinPerKm: 4.2 },
];

/** Half-width of the flat-ground search window around a nutrition kilometre. */
const NUTRITION_WINDOW_KM = 0.5;

/**
 * One marker per whole kilometre, carrying the cumulative gain up to that
 * point. Markers that resolve to the same track point are deduplicated so a
 * sparse track never repeats an index.
 */
export function kmMarkers(points: TrackPoint[]): KmMarker[] {
  if (points.length === 0) return [];
  const maxKm = Math.floor(totalDistanceKm(points) + 1e-6);
  const markers: KmMarker[] = [];
  const seenIndexes = new Set<number>();
  for (let km = 0; km <= maxKm; km += 1) {
    const index = indexAtKm(points, km);
    if (seenIndexes.has(index)) continue;
    seenIndexes.add(index);
    const point = points[index];
    markers.push({
      km,
      index,
      lat: point.lat,
      lon: point.lon,
      cumulativeGainM: dPlusM(points.slice(0, index + 1)),
    });
  }
  return markers;
}

interface FlattestCandidate {
  index: number;
  slope: number;
}

/** Flattest point within `NUTRITION_WINDOW_KM`, lowest index on a slope tie. */
function flattestWithinWindow(
  points: TrackPoint[],
  cumulative: number[],
  targetKm: number,
): FlattestCandidate | null {
  let chosen: FlattestCandidate | null = null;
  for (let index = 0; index < points.length; index += 1) {
    if (Math.abs(cumulative[index] - targetKm) > NUTRITION_WINDOW_KM) continue;
    const slope = slopePercentAt(points, index);
    if (
      chosen === null ||
      Math.abs(slope) < Math.abs(chosen.slope) ||
      (Math.abs(slope) === Math.abs(chosen.slope) && index < chosen.index)
    ) {
      chosen = { index, slope };
    }
  }
  return chosen;
}

/**
 * Nutrition stops every `intervalKm` kilometres. Each stop picks the flattest
 * point within half a kilometre so food and drink land on runnable ground; a
 * point on flat or descending ground is a `recovery` stop, a climbing one is
 * `scheduled`. Stops that resolve to the same index are deduplicated, and the
 * exact finish is never a stop.
 */
export function nutritionPoints(
  points: TrackPoint[],
  intervalKm: number = DEFAULT_NUTRITION_INTERVAL_KM,
): NutritionPoint[] {
  if (points.length < 2) return [];
  if (!Number.isFinite(intervalKm) || intervalKm <= 0) return [];
  const totalKm = totalDistanceKm(points);
  const cumulative = cumulativeKm(points);
  const stops: NutritionPoint[] = [];
  const seenIndexes = new Set<number>();
  for (let km = intervalKm; km < totalKm; km += intervalKm) {
    const candidate = flattestWithinWindow(points, cumulative, km);
    const index = candidate === null ? indexAtKm(points, km) : candidate.index;
    if (seenIndexes.has(index)) continue;
    seenIndexes.add(index);
    const slope = candidate === null ? slopePercentAt(points, index) : candidate.slope;
    const point = points[index];
    stops.push({
      km,
      index,
      lat: point.lat,
      lon: point.lon,
      kind: slope <= 0 ? 'recovery' : 'scheduled',
    });
  }
  return stops;
}
