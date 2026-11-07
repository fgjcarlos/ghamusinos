import * as SunCalc from 'suncalc';
import { adjustedSpeedKmh, bearingDeg, cumulativeKm, slopePercentAt } from './trackCoords';
import type { SolarSegment, TrackPoint } from './types';

/**
 * Solar exposure for a planned race.
 *
 * This is a planning ESTIMATE, not a computed shadow model. `shade` only means
 * that the segment faces away from the sun (its aspect is more than 90 degrees
 * from the solar azimuth) or that the sun grazes the horizon on it; there is no
 * terrain, building or vegetation cast-shadow calculation.
 *
 * `suncalc` v2 reports `altitude` and a north-based clockwise `azimuth` in
 * degrees, so these values are used directly (see `node_modules/suncalc`).
 */

/** Normalises degrees into [0, 360); non-finite input collapses to 0. */
function normalize360(degrees: number): number {
  if (!Number.isFinite(degrees)) return 0;
  const wrapped = degrees % 360;
  return wrapped < 0 ? wrapped + 360 : wrapped;
}

/** Smallest angular distance between two compass directions, in [0, 180]. */
function angularDistance(a: number, b: number): number {
  const difference = Math.abs(normalize360(a) - normalize360(b));
  return Math.min(difference, 360 - difference);
}

function clamp01(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.min(1, Math.max(0, value));
}

/**
 * Merges consecutive raw segments that share a status. The merged intensity is
 * the maximum of its parts; aspect and slope become length-weighted averages,
 * falling back to the first value when the merged run has zero length.
 */
function mergeSegments(raw: SolarSegment[], weights: number[]): SolarSegment[] {
  const merged: SolarSegment[] = [];
  const mergedWeights: number[] = [];
  const aspectWeighted: number[] = [];
  const slopeWeighted: number[] = [];

  for (let index = 0; index < raw.length; index += 1) {
    const segment = raw[index];
    const weight = weights[index];
    const last = merged.length - 1;

    if (last >= 0 && merged[last].status === segment.status) {
      const totalWeight = mergedWeights[last] + weight;
      if (totalWeight > 0) {
        aspectWeighted[last] += segment.aspectDeg * weight;
        slopeWeighted[last] += segment.slopePercent * weight;
        merged[last].aspectDeg = normalize360(aspectWeighted[last] / totalWeight);
        merged[last].slopePercent = slopeWeighted[last] / totalWeight;
      }
      mergedWeights[last] = totalWeight;
      merged[last].endKm = segment.endKm;
      merged[last].intensity = Math.max(merged[last].intensity, segment.intensity);
      continue;
    }

    merged.push({ ...segment });
    mergedWeights.push(weight);
    aspectWeighted.push(segment.aspectDeg * weight);
    slopeWeighted.push(segment.slopePercent * weight);
  }

  return merged;
}

/**
 * Sun/shade/night status per track segment for a race starting at `startAt`.
 *
 * Each segment is evaluated at its midpoint position and at the time the runner
 * is estimated to reach that midpoint, using the base pace and each segment's
 * slope-adjusted speed from `adjustedSpeedKmh`. Consecutive segments with the
 * same status are merged.
 */
export function calculateSolarExposure(
  points: TrackPoint[],
  startAt: Date,
  basePaceMinPerKm: number,
): SolarSegment[] {
  if (points.length < 2) return [];
  if (!(startAt instanceof Date) || !Number.isFinite(startAt.getTime())) return [];
  if (!Number.isFinite(basePaceMinPerKm) || basePaceMinPerKm <= 0) return [];

  const distances = cumulativeKm(points);
  const baseSpeedKmh = 60 / basePaceMinPerKm;
  const raw: SolarSegment[] = [];
  const rawWeights: number[] = [];
  let elapsedMin = 0;

  for (let index = 1; index < points.length; index += 1) {
    const previous = points[index - 1];
    const current = points[index];
    const segmentKm = distances[index] - distances[index - 1];

    const slopePercent = slopePercentAt(points, index);
    const speedKmh = adjustedSpeedKmh(baseSpeedKmh, slopePercent);
    const durationMin = speedKmh > 0 ? (segmentKm / speedKmh) * 60 : 0;
    const midTime = new Date(startAt.getTime() + (elapsedMin + durationMin / 2) * 60_000);
    elapsedMin += durationMin;

    const midLat = (previous.lat + current.lat) / 2;
    const midLon = (previous.lon + current.lon) / 2;
    const sun = SunCalc.getPosition(midTime, midLat, midLon);
    const altitudeDeg = sun.altitude;
    const azimuthDeg = normalize360(sun.azimuth);

    const bearing = bearingDeg(previous, current);
    const deltaM = (current.ele ?? 0) - (previous.ele ?? 0);
    const aspectDeg = deltaM >= 0 ? normalize360(bearing) : normalize360(bearing + 180);
    const difference = angularDistance(azimuthDeg, aspectDeg);

    let status: SolarSegment['status'];
    let intensity: number;
    if (altitudeDeg <= 0) {
      status = 'night';
      intensity = 0;
    } else if (difference > 90) {
      status = 'shade';
      intensity = 0;
    } else {
      intensity = clamp01(
        Math.cos((difference * Math.PI) / 180) * Math.sin((altitudeDeg * Math.PI) / 180),
      );
      status = intensity > 0.1 ? 'sun' : 'shade';
    }

    raw.push({
      startKm: distances[index - 1],
      endKm: distances[index],
      status,
      intensity,
      aspectDeg,
      slopePercent,
    });
    rawWeights.push(segmentKm);
  }

  return mergeSegments(raw, rawWeights);
}
