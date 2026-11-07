import { describe, expect, it } from 'vitest';
import { calculateSolarExposure } from './solar';
import { cumulativeKm } from './trackCoords';
import type { TrackPoint } from './types';

const LAT = 45;
const KM_PER_DEG_LAT = 111.13;

/** A clear summer noon at a mid-latitude on the Greenwich meridian. */
const SUMMER_NOON = new Date('2026-06-21T12:00:00Z');
/** Deep winter night at the same place: the sun is far below the horizon. */
const WINTER_MIDNIGHT = new Date('2026-01-15T00:00:00Z');

function eastWest(km: number, steps = 10): TrackPoint[] {
  const spanDeg = km / (KM_PER_DEG_LAT * Math.cos((LAT * Math.PI) / 180));
  return Array.from({ length: steps + 1 }, (_, index) => ({
    lat: LAT,
    lon: -spanDeg / 2 + (spanDeg * index) / steps,
    ele: 0,
  }));
}

function southLine(km: number, steps = 10): TrackPoint[] {
  const spanDeg = km / KM_PER_DEG_LAT;
  return Array.from({ length: steps + 1 }, (_, index) => ({
    lat: LAT - (spanDeg * index) / steps,
    lon: 0,
    ele: 0,
  }));
}

/** Heads south for `kmEach`, then turns around and heads north over the same line. */
function southThenNorth(kmEach: number, steps = 5): TrackPoint[] {
  const spanDeg = kmEach / KM_PER_DEG_LAT;
  const south = Array.from({ length: steps + 1 }, (_, index) => ({
    lat: LAT - (spanDeg * index) / steps,
    lon: 0,
    ele: 0,
  }));
  const north = Array.from({ length: steps }, (_, index) => ({
    lat: LAT - spanDeg + (spanDeg * (index + 1)) / steps,
    lon: 0,
    ele: 0,
  }));
  return [...south, ...north];
}

function slopedLine(km: number, deltaM: number, steps = 5): TrackPoint[] {
  const spanDeg = km / KM_PER_DEG_LAT;
  return Array.from({ length: steps + 1 }, (_, index) => ({
    lat: LAT - (spanDeg * index) / steps,
    lon: 0,
    ele: (deltaM * index) / steps,
  }));
}

function totalKm(points: TrackPoint[]): number {
  const distances = cumulativeKm(points);
  return distances[distances.length - 1];
}

describe('calculateSolarExposure guards', () => {
  it('returns [] for an empty or single-point track', () => {
    expect(calculateSolarExposure([], SUMMER_NOON, 6)).toEqual([]);
    expect(calculateSolarExposure([{ lat: LAT, lon: 0, ele: 0 }], SUMMER_NOON, 6)).toEqual([]);
  });

  it('returns [] for an invalid start date', () => {
    expect(calculateSolarExposure(eastWest(1), new Date('not-a-date'), 6)).toEqual([]);
  });

  it('returns [] for a non-positive or non-finite base pace', () => {
    expect(calculateSolarExposure(eastWest(1), SUMMER_NOON, 0)).toEqual([]);
    expect(calculateSolarExposure(eastWest(1), SUMMER_NOON, -6)).toEqual([]);
    expect(calculateSolarExposure(eastWest(1), SUMMER_NOON, Number.NaN)).toEqual([]);
    expect(calculateSolarExposure(eastWest(1), SUMMER_NOON, Number.POSITIVE_INFINITY)).toEqual([]);
  });
});

describe('calculateSolarExposure daylight', () => {
  it('never reports night at summer noon and keeps aspect in [0, 360)', () => {
    const segments = calculateSolarExposure(eastWest(3), SUMMER_NOON, 6);

    expect(segments.length).toBeGreaterThan(0);
    for (const segment of segments) {
      expect(segment.status).not.toBe('night');
      expect(['sun', 'shade']).toContain(segment.status);
      expect(Number.isFinite(segment.aspectDeg)).toBe(true);
      expect(segment.aspectDeg).toBeGreaterThanOrEqual(0);
      expect(segment.aspectDeg).toBeLessThan(360);
      expect(Number.isFinite(segment.intensity)).toBe(true);
      expect(segment.intensity).toBeGreaterThanOrEqual(0);
      expect(segment.intensity).toBeLessThanOrEqual(1);
    }
  });

  it('merges a constant-sun track into one segment spanning the whole track', () => {
    const points = southLine(3);
    const segments = calculateSolarExposure(points, SUMMER_NOON, 6);

    expect(segments).toHaveLength(1);
    expect(segments[0].status).toBe('sun');
    expect(segments[0].intensity).toBeGreaterThan(0.1);
    expect(segments[0].startKm).toBe(0);
    expect(segments[0].endKm).toBeCloseTo(totalKm(points), 6);
  });

  it('emits ordered, non-overlapping segments with positive length', () => {
    const segments = calculateSolarExposure(southThenNorth(1), SUMMER_NOON, 6);

    expect(segments.length).toBeGreaterThanOrEqual(2);
    for (const segment of segments) {
      expect(segment.startKm).toBeLessThan(segment.endKm);
    }
    for (let index = 1; index < segments.length; index += 1) {
      expect(segments[index].startKm).toBeGreaterThanOrEqual(segments[index - 1].endKm - 1e-9);
    }
  });
});

describe('calculateSolarExposure night', () => {
  it('reports night for every segment at winter midnight and merges them into one', () => {
    const points = eastWest(3);
    const segments = calculateSolarExposure(points, WINTER_MIDNIGHT, 6);

    expect(segments).toHaveLength(1);
    expect(segments[0].status).toBe('night');
    expect(segments[0].intensity).toBe(0);
    expect(segments[0].startKm).toBe(0);
    expect(segments[0].endKm).toBeCloseTo(totalKm(points), 6);
  });

  it('keeps slope percentages finite and signed for a climb and a descent', () => {
    const climb = calculateSolarExposure(slopedLine(1, 120), WINTER_MIDNIGHT, 6);
    expect(climb).toHaveLength(1);
    expect(Number.isFinite(climb[0].slopePercent)).toBe(true);
    expect(climb[0].slopePercent).toBeGreaterThan(0);

    const descent = calculateSolarExposure(slopedLine(1, -120), WINTER_MIDNIGHT, 6);
    expect(descent).toHaveLength(1);
    expect(Number.isFinite(descent[0].slopePercent)).toBe(true);
    expect(descent[0].slopePercent).toBeLessThan(0);
  });
});
