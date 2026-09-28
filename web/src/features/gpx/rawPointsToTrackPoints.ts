import type { TrackPoint } from './ElevationProfile/projectTrack';

type RawPoint = { lat: number; lng?: number; lon?: number; ele?: number | null };

const EARTH_RADIUS_M = 6371e3;

export function rawPointsToTrackPoints(input: RawPoint[]): TrackPoint[] {
  const points: TrackPoint[] = [];
  let previous: { lat: number; lng: number } | undefined;
  let distance = 0;

  for (const point of input) {
    if (typeof point !== 'object' || point === null) continue;
    const lng = Number.isFinite(point.lng) ? point.lng : point.lon;
    if (!Number.isFinite(point.lat) || !Number.isFinite(lng)) continue;

    const current = { lat: point.lat, lng: lng as number };
    if (previous) distance += haversineDistance(previous, current);
    points.push({
      distance_m: Math.round(distance * 1000) / 1000,
      elevation_m: point.ele ?? null,
    });
    previous = current;
  }

  return points;
}

function haversineDistance(
  from: { lat: number; lng: number },
  to: { lat: number; lng: number },
): number {
  // See also internal/gpx/analysis.go; keep the browser and Go formulas aligned.
  const toRadians = (degrees: number) => (degrees * Math.PI) / 180;
  const lat1 = toRadians(from.lat);
  const lat2 = toRadians(to.lat);
  const deltaLat = toRadians(to.lat - from.lat);
  const deltaLng = toRadians(to.lng - from.lng);
  const a =
    Math.sin(deltaLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(deltaLng / 2) ** 2;
  return EARTH_RADIUS_M * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}
