import { describe, expect, it } from 'vitest';
import { rawPointsToTrackPoints } from './rawPointsToTrackPoints';

describe('rawPointsToTrackPoints', () => {
  it('returns an empty array for empty input', () => {
    expect(rawPointsToTrackPoints([])).toEqual([]);
  });

  it('starts a single point at zero distance', () => {
    expect(rawPointsToTrackPoints([{ lat: 40.4, lng: -3.7, ele: 1000 }])).toEqual([
      { distance_m: 0, elevation_m: 1000 },
    ]);
  });

  it('keeps zero distance for identical consecutive points', () => {
    const result = rawPointsToTrackPoints([
      { lat: 40.4, lng: -3.7, ele: 1000 },
      { lat: 40.4, lng: -3.7, ele: 1010 },
    ]);
    expect(result[1]?.distance_m).toBe(0);
  });

  it('measures one kilometre along the equator within half a metre', () => {
    const result = rawPointsToTrackPoints([
      { lat: 0, lng: 0, ele: 0 },
      { lat: 0, lng: 0.0089932, ele: 0 },
    ]);
    expect(result[1]?.distance_m).toBeCloseTo(1000, 0);
    expect(Math.abs((result[1]?.distance_m ?? 0) - 1000)).toBeLessThanOrEqual(0.5);
  });

  it('matches the precomputed Madrid to Cercedilla cumulative-distance reference', () => {
    const input = [
      { lat: 40.45, lng: -3.7, ele: 700 },
      { lat: 40.520825, lng: -3.791675, ele: 800 },
      { lat: 40.59165, lng: -3.88335, ele: 900 },
      { lat: 40.662475, lng: -3.975025, ele: 1000 },
      { lat: 40.7333, lng: -4.0667, ele: 1188 },
    ];
    const groundTruth = [0, 11051.347, 22096.952, 33136.809, 44170.914];
    const result = rawPointsToTrackPoints(input);
    expect(result).toHaveLength(groundTruth.length);
    result.forEach((point, index) => {
      expect(Math.abs(point.distance_m - (groundTruth[index] ?? 0))).toBeLessThanOrEqual(1);
    });
  });

  it('preserves null elevation while accumulating distance', () => {
    const result = rawPointsToTrackPoints([
      { lat: 40.4, lng: -3.7, ele: 1000 },
      { lat: 40.41, lng: -3.69, ele: null },
      { lat: 40.42, lng: -3.68, ele: 1020 },
    ]);
    expect(result[1]?.elevation_m).toBeNull();
    expect(result[1]?.distance_m).toBeGreaterThan(0);
    expect(result[2]?.distance_m).toBeGreaterThan(result[1]?.distance_m ?? 0);
  });

  it('drops a NaN coordinate without poisoning later distances', () => {
    const result = rawPointsToTrackPoints([
      { lat: 40.4, lng: -3.7, ele: 1000 },
      { lat: Number.NaN, lng: -3.69, ele: 1010 },
      { lat: 40.42, lng: -3.68, ele: 1020 },
    ]);
    expect(result).toHaveLength(2);
    expect(result.every((point) => Number.isFinite(point.distance_m))).toBe(true);
    expect(result[1]?.distance_m).toBeGreaterThanOrEqual(result[0]?.distance_m ?? 0);
  });

  it('accepts Go lon wire points and gives lng precedence when both exist', () => {
    const inputLon = [
      { lat: 40.4, lon: -3.7, ele: 1000 },
      { lat: 40.41, lon: -3.69, ele: 1010 },
    ];
    const inputLng = [
      { lat: 40.4, lng: -3.7, ele: 1000 },
      { lat: 40.41, lng: -3.69, ele: 1010 },
    ];
    const inputMixed = [
      { lat: 40.4, lng: -3.7, ele: 1000 },
      { lat: 40.41, lng: -3.69, lon: 12, ele: 1010 },
    ];
    expect(rawPointsToTrackPoints(inputLon)).toEqual(rawPointsToTrackPoints(inputLng));
    expect(rawPointsToTrackPoints(inputMixed)[1]?.distance_m).toBe(
      rawPointsToTrackPoints(inputLng)[1]?.distance_m,
    );
  });

  it('drops entries without a finite longitude', () => {
    const result = rawPointsToTrackPoints([
      { lat: 40.4, lng: -3.7, ele: 1000 },
      { lat: 40.41, ele: 1010 },
    ]);
    expect(result).toHaveLength(1);
    expect(result[0]?.distance_m).toBe(0);
    expect(result.every((point) => Number.isFinite(point.distance_m))).toBe(true);
  });
});
