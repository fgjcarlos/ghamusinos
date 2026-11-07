import type { TrackPoint } from './types';

/** Mean Earth radius, matching the Go backend and `rawPointsToTrackPoints`. */
const EARTH_RADIUS_KM = 6371.0088;

function toRadians(degrees: number): number {
  return (degrees * Math.PI) / 180;
}

/**
 * Converts the GPX detail wire points (`{lat, lon, ele}` or the `lng` alias)
 * into the named `TrackPoint` shape used by every race-planning function.
 * Invalid coordinates are dropped; an invalid elevation becomes `null`.
 */
export function toTrackPoints(input: unknown[]): TrackPoint[] {
  const points: TrackPoint[] = [];
  for (const raw of input) {
    if (typeof raw !== 'object' || raw === null) continue;
    const candidate = raw as {
      lat?: unknown;
      lng?: unknown;
      lon?: unknown;
      ele?: unknown;
    };
    if (typeof candidate.lat !== 'number' || !Number.isFinite(candidate.lat)) continue;
    const lngValue = candidate.lng;
    const lonValue = candidate.lon;
    const lon = typeof lngValue === 'number' && Number.isFinite(lngValue) ? lngValue : lonValue;
    if (typeof lon !== 'number' || !Number.isFinite(lon)) continue;
    const ele =
      typeof candidate.ele === 'number' && Number.isFinite(candidate.ele) ? candidate.ele : null;
    points.push({ lat: candidate.lat, lon, ele });
  }
  return points;
}

/** Great-circle distance in kilometres between two named points. */
export function haversineKm(
  from: Pick<TrackPoint, 'lat' | 'lon'>,
  to: Pick<TrackPoint, 'lat' | 'lon'>,
): number {
  const lat1 = toRadians(from.lat);
  const lat2 = toRadians(to.lat);
  const deltaLat = toRadians(to.lat - from.lat);
  const deltaLon = toRadians(to.lon - from.lon);
  const a =
    Math.sin(deltaLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(deltaLon / 2) ** 2;
  return EARTH_RADIUS_KM * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}

/** Cumulative distance in kilometres for every point (first point is 0). */
export function cumulativeKm(points: TrackPoint[]): number[] {
  if (points.length === 0) return [];
  const distances = [0];
  let total = 0;
  for (let index = 1; index < points.length; index += 1) {
    total += haversineKm(points[index - 1], points[index]);
    distances.push(total);
  }
  return distances;
}

export function totalDistanceKm(points: TrackPoint[]): number {
  const distances = cumulativeKm(points);
  return distances.length === 0 ? 0 : distances[distances.length - 1];
}

/**
 * Cumulative positive elevation gain in metres. Deltas at or below the noise
 * threshold are ignored, and segments with a missing elevation contribute 0,
 * so a sparse GPS track does not fabricate gain.
 */
export function dPlusM(points: TrackPoint[], noiseThresholdM = 1): number {
  let gain = 0;
  for (let index = 1; index < points.length; index += 1) {
    const previous = points[index - 1].ele;
    const current = points[index].ele;
    if (previous === null || current === null) continue;
    const delta = current - previous;
    if (delta > noiseThresholdM) gain += delta;
  }
  return gain;
}

/** Slope in percent between `points[index - 1]` and `points[index]`. */
export function slopePercentAt(points: TrackPoint[], index: number): number {
  if (index <= 0 || index >= points.length) return 0;
  const previous = points[index - 1];
  const current = points[index];
  const distanceM = haversineKm(previous, current) * 1000;
  if (distanceM <= 0) return 0;
  const deltaM = (current.ele ?? 0) - (previous.ele ?? 0);
  return (deltaM / distanceM) * 100;
}

/**
 * Slope-adjusted speed in km/h, ported from the `old_ghamusinos` model:
 * uphill loses 2.5% of the flat speed per 1% of slope down to 30%; moderate
 * downhill gains 2% per 1%; very steep downhill loses speed again.
 */
export function adjustedSpeedKmh(baseSpeedKmh: number, slopePercent: number): number {
  if (slopePercent > 0) {
    return baseSpeedKmh * Math.max(0.3, 1 - slopePercent * 0.025);
  }
  if (slopePercent < 0) {
    const absolute = Math.abs(slopePercent);
    if (absolute < 15) {
      return baseSpeedKmh * (1 + absolute * 0.02);
    }
    return baseSpeedKmh * Math.max(0.7, 1.3 - (absolute - 15) * 0.05);
  }
  return baseSpeedKmh;
}

/**
 * Estimated moving time in minutes for a track at a flat base pace. Each
 * segment uses its own slope-adjusted speed, which is what makes the split
 * tables and cutoffs elevation-aware.
 */
export function estimateTotalTimeMin(points: TrackPoint[], basePaceMinPerKm: number): number {
  if (points.length < 2 || basePaceMinPerKm <= 0) return 0;
  const baseSpeedKmh = 60 / basePaceMinPerKm;
  let hours = 0;
  for (let index = 1; index < points.length; index += 1) {
    const distanceKm = haversineKm(points[index - 1], points[index]);
    if (distanceKm <= 0) continue;
    const speed = adjustedSpeedKmh(baseSpeedKmh, slopePercentAt(points, index));
    if (speed <= 0) continue;
    hours += distanceKm / speed;
  }
  return hours * 60;
}

/**
 * Index of the first point whose cumulative distance reaches `targetKm`.
 * Clamps to the first/last point for out-of-range targets.
 */
export function indexAtKm(points: TrackPoint[], targetKm: number): number {
  if (points.length === 0) return -1;
  if (targetKm <= 0) return 0;
  let total = 0;
  for (let index = 1; index < points.length; index += 1) {
    total += haversineKm(points[index - 1], points[index]);
    if (total + 1e-6 >= targetKm) return index;
  }
  return points.length - 1;
}

/** Initial bearing in degrees (0 = north, clockwise) from `from` to `to`. */
export function bearingDeg(
  from: Pick<TrackPoint, 'lat' | 'lon'>,
  to: Pick<TrackPoint, 'lat' | 'lon'>,
): number {
  const lat1 = toRadians(from.lat);
  const lat2 = toRadians(to.lat);
  const deltaLon = toRadians(to.lon - from.lon);
  const y = Math.sin(deltaLon) * Math.cos(lat2);
  const x = Math.cos(lat1) * Math.sin(lat2) - Math.sin(lat1) * Math.cos(lat2) * Math.cos(deltaLon);
  return ((Math.atan2(y, x) * 180) / Math.PI + 360) % 360;
}
