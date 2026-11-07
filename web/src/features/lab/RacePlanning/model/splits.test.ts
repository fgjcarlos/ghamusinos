import { describe, expect, it } from 'vitest';
import { evaluateCutoffs, generateStrategicSplits } from './splits';
import type { TrackPoint } from './types';

/** Mean Earth radius in kilometres, matching `trackCoords`. */
const EARTH_RADIUS_KM = 6371.0088;

/**
 * One kilometre of longitude at the equator, in degrees. Derived from the same
 * radius `trackCoords` uses so a generated "kilometre" measures one kilometre
 * within floating-point noise.
 */
const DEG_PER_KM = 180 / (Math.PI * EARTH_RADIUS_KM);

/** Flat straight line of `km` kilometres at the equator, every `steps`-th point. */
function flatLine(km: number, steps = 10): TrackPoint[] {
  return Array.from({ length: steps + 1 }, (_, index) => ({
    lat: 0,
    lon: ((km * index) / steps) * DEG_PER_KM,
    ele: 0,
  }));
}

/**
 * Flat 10 km track measured a hair over 10 km. The overshoot keeps the strict
 * `estimated < limit` boundary deterministic instead of being decided by the
 * last bit of float noise in a measured 10.000000 km track.
 */
function flatTenKmTrack(): TrackPoint[] {
  return flatLine(10.001, 1000);
}

describe('generateStrategicSplits', () => {
  it('returns [] for a track with fewer than two points', () => {
    expect(generateStrategicSplits([], 6)).toEqual([]);
    expect(generateStrategicSplits([{ lat: 0, lon: 0, ele: 0 }], 6)).toEqual([]);
  });

  it('returns [] for a non-positive or non-finite base pace', () => {
    const track = flatLine(2);
    expect(generateStrategicSplits(track, 0)).toEqual([]);
    expect(generateStrategicSplits(track, -6)).toEqual([]);
    expect(generateStrategicSplits(track, Number.NaN)).toEqual([]);
    expect(generateStrategicSplits(track, Number.POSITIVE_INFINITY)).toEqual([]);
  });

  it('returns [] for a track under one kilometre', () => {
    expect(generateStrategicSplits(flatLine(0.5), 6)).toEqual([]);
  });

  it('produces one row per whole kilometre with cumulative fatigue on a flat 10 km track', () => {
    const rows = generateStrategicSplits(flatLine(10, 100), 6);

    expect(rows).toHaveLength(10);
    expect(rows.map((row) => row.km)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);

    expect(rows[0].splitTimeMin).toBeCloseTo(6, 1);
    expect(rows[0].cumulativeTimeMin).toBeCloseTo(6, 1);
    expect(rows[1].cumulativeTimeMin).toBeCloseTo(12, 0);
    expect(rows[9].splitTimeMin).toBeGreaterThan(rows[0].splitTimeMin);

    for (let index = 1; index < rows.length; index += 1) {
      expect(rows[index].cumulativeTimeMin).toBeCloseTo(
        rows[index - 1].cumulativeTimeMin + rows[index].splitTimeMin,
        6,
      );
    }

    // Flat track: no gain, and the one-kilometre pace is the split time.
    expect(rows.every((row) => row.gainM === 0)).toBe(true);
    expect(rows[0].paceMinPerKm).toBeCloseTo(rows[0].splitTimeMin, 9);
  });

  it('keeps every flat split at the base pace when fatigueFactor is 0', () => {
    const rows = generateStrategicSplits(flatLine(10, 100), 6, 0);

    expect(rows).toHaveLength(10);
    for (const row of rows) {
      expect(row.splitTimeMin).toBeCloseTo(6, 1);
    }
  });

  it('reports gain and slows the climb on a hilly 2 km track', () => {
    const track: TrackPoint[] = [
      { lat: 0, lon: 0, ele: 0 },
      { lat: 0, lon: DEG_PER_KM, ele: 100 },
      { lat: 0, lon: 2 * DEG_PER_KM, ele: 0 },
    ];

    const rows = generateStrategicSplits(track, 6, 0);

    expect(rows).toHaveLength(2);
    expect(rows[0].gainM).toBeCloseTo(100, 5);
    // The climb is slower than a flat kilometre at the same base pace.
    expect(rows[0].splitTimeMin).toBeGreaterThan(6);
    // The descent is faster than the climb.
    expect(rows[1].splitTimeMin).toBeLessThan(rows[0].splitTimeMin);
  });
});

describe('evaluateCutoffs', () => {
  it('returns [] for a track with fewer than two points', () => {
    expect(evaluateCutoffs([], 6, [{ km: 5, timeLimitMin: 25 }])).toEqual([]);
    expect(evaluateCutoffs([{ lat: 0, lon: 0, ele: 0 }], 6, [{ km: 5, timeLimitMin: 25 }])).toEqual(
      [],
    );
  });

  it('flags a late cutoff as unsafe with a negative margin and required pace', () => {
    const [result] = evaluateCutoffs(flatTenKmTrack(), 6, [{ km: 5, timeLimitMin: 25 }]);

    expect(result.estimatedMin).toBeCloseTo(30, 1);
    expect(result.isSafe).toBe(false);
    expect(result.marginMin).toBeCloseTo(-5, 1);
    expect(result.requiredPaceMinPerKm).toBeCloseTo(5, 9);
  });

  it('flags an early cutoff as safe with a positive margin', () => {
    const [result] = evaluateCutoffs(flatTenKmTrack(), 6, [{ km: 5, timeLimitMin: 40 }]);

    expect(result.estimatedMin).toBeCloseTo(30, 1);
    expect(result.isSafe).toBe(true);
    expect(result.marginMin).toBeCloseTo(10, 1);
  });

  it('treats an arrival at the limit as not safe (strict less-than)', () => {
    const [result] = evaluateCutoffs(flatTenKmTrack(), 6, [{ km: 10, timeLimitMin: 60 }]);

    expect(result.estimatedMin).toBeCloseTo(60, 1);
    expect(result.isSafe).toBe(false);
    expect(result.marginMin).toBeLessThan(0);
  });

  it('filters invalid cutoffs and preserves input order and original fields', () => {
    const cutoffs = [
      { km: 3, timeLimitMin: 30, label: 'CP1' },
      { km: 0, timeLimitMin: 30 },
      { km: 5, timeLimitMin: 0 },
      { km: Number.NaN, timeLimitMin: 30 },
      { km: 5, timeLimitMin: Number.POSITIVE_INFINITY },
      { km: -1, timeLimitMin: 10 },
      { km: 8, timeLimitMin: 50, label: 'CP2' },
    ];

    const results = evaluateCutoffs(flatTenKmTrack(), 6, cutoffs);

    expect(results.map((result) => result.km)).toEqual([3, 8]);
    expect(results[0]).toMatchObject({ km: 3, timeLimitMin: 30, label: 'CP1' });
    expect(results[1]).toMatchObject({ km: 8, timeLimitMin: 50, label: 'CP2' });
  });
});
