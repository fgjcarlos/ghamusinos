// TDD contract for the plan-vs-real model (phase 1.6, issue #18, S3/S5/S8).
//
// The model compares the planned per-kilometre splits of a GPX track with the
// real splits of a Strava activity. Real distance comes from the activity's
// own `latlng` stream, so a route that differs from the plan is still a valid
// comparison.

import { describe, expect, it } from 'vitest';
import { comparePlanVsReal, type ActivitySample } from './postActivity';
import type { TrackPoint } from './types';

/** Mean Earth radius in kilometres, matching `trackCoords`. */
const EARTH_RADIUS_KM = 6371.0088;

/** One kilometre of longitude at the equator, in degrees. */
const DEG_PER_KM = 180 / (Math.PI * EARTH_RADIUS_KM);

/** Flat straight track of exactly `km` kilometres, one point per kilometre. */
function flatTrack(km: number): TrackPoint[] {
  return Array.from({ length: km + 1 }, (_, index) => ({
    lat: 0,
    lon: index * DEG_PER_KM,
    ele: 0,
  }));
}

/** Activity `latlng` points one kilometre apart along the equator (Strava order). */
function flatLatLng(km: number): Array<[number, number]> {
  return Array.from({ length: km + 1 }, (_, index): [number, number] => [0, index * DEG_PER_KM]);
}

describe('comparePlanVsReal', () => {
  it('matches an identical flat plan and activity with zero difference and fatigue', () => {
    const plan = flatTrack(3);
    const activity: ActivitySample = {
      latlng: flatLatLng(3),
      timeSeconds: [0, 360, 720, 1080],
    };

    const rows = comparePlanVsReal(plan, activity, 6, 0);

    expect(rows).toHaveLength(3);
    expect(rows.map((row) => row.km)).toEqual([1, 2, 3]);

    for (const row of rows) {
      expect(row.diffMin).toBeCloseTo(0, 6);
      expect(row.fatigueIndex).toBeCloseTo(0, 6);
      expect(row.realSplitMin).toBeCloseTo(6, 6);
      expect(row.realCumulativeMin).toBeCloseTo(row.km * 6, 6);
      expect(row.plannedCumulativeMin).toBeCloseTo(row.km * 6, 6);
    }
  });

  it('reports a positive difference and fatigue for a slower real activity', () => {
    const plan = flatTrack(3);
    // One extra minute per kilometre than the 6 min/km plan.
    const activity: ActivitySample = {
      latlng: flatLatLng(3),
      timeSeconds: [0, 420, 840, 1260],
    };

    const rows = comparePlanVsReal(plan, activity, 6, 0);

    expect(rows).toHaveLength(3);
    for (const row of rows) {
      expect(row.diffMin).toBeCloseTo(1, 6);
    }
    expect(rows[0].fatigueIndex).toBeCloseTo((1 / 6) * 100, 6);
  });

  it('stops emitting rows when the activity ends before the planned distance', () => {
    const plan = flatTrack(5);
    const activity: ActivitySample = {
      latlng: flatLatLng(2),
      timeSeconds: [0, 360, 720],
    };

    const rows = comparePlanVsReal(plan, activity, 6, 0);

    expect(rows).toHaveLength(2);
    expect(rows.map((row) => row.km)).toEqual([1, 2]);
  });

  it('returns [] for an empty or short plan', () => {
    const activity: ActivitySample = {
      latlng: flatLatLng(2),
      timeSeconds: [0, 360, 720],
    };

    expect(comparePlanVsReal([], activity, 6)).toEqual([]);
    expect(comparePlanVsReal([{ lat: 0, lon: 0, ele: 0 }], activity, 6)).toEqual([]);
  });

  it('returns [] for an empty or short activity', () => {
    const plan = flatTrack(3);

    expect(comparePlanVsReal(plan, { latlng: [], timeSeconds: [] }, 6)).toEqual([]);
    expect(comparePlanVsReal(plan, { latlng: [[0, 0]], timeSeconds: [0] }, 6)).toEqual([]);
    expect(comparePlanVsReal(plan, { latlng: flatLatLng(2), timeSeconds: [0] }, 6)).toEqual([]);
  });

  it('returns [] for a non-positive or non-finite base pace', () => {
    const plan = flatTrack(3);
    const activity: ActivitySample = {
      latlng: flatLatLng(3),
      timeSeconds: [0, 360, 720, 1080],
    };

    expect(comparePlanVsReal(plan, activity, 0)).toEqual([]);
    expect(comparePlanVsReal(plan, activity, -6)).toEqual([]);
    expect(comparePlanVsReal(plan, activity, Number.NaN)).toEqual([]);
    expect(comparePlanVsReal(plan, activity, Number.POSITIVE_INFINITY)).toEqual([]);
  });

  it('compares a hilly plan with a flat activity and keeps every value finite', () => {
    const hilly: TrackPoint[] = [
      { lat: 0, lon: 0, ele: 0 },
      { lat: 0, lon: DEG_PER_KM, ele: 100 },
      { lat: 0, lon: 2 * DEG_PER_KM, ele: 0 },
    ];
    const flatActivity: ActivitySample = {
      latlng: flatLatLng(2),
      timeSeconds: [0, 360, 720],
    };

    const rows = comparePlanVsReal(hilly, flatActivity, 6, 0);

    expect(rows).toHaveLength(2);
    // The climb is slower than the flat real kilometre; the descent is faster.
    expect(rows[0].plannedSplitMin).toBeGreaterThan(rows[0].realSplitMin);
    expect(rows.some((row) => Math.abs(row.plannedSplitMin - row.realSplitMin) > 0.5)).toBe(true);
    for (const row of rows) {
      expect(Number.isFinite(row.plannedSplitMin)).toBe(true);
      expect(Number.isFinite(row.realSplitMin)).toBe(true);
      expect(Number.isFinite(row.diffMin)).toBe(true);
      expect(Number.isFinite(row.plannedCumulativeMin)).toBe(true);
      expect(Number.isFinite(row.realCumulativeMin)).toBe(true);
      expect(Number.isFinite(row.fatigueIndex)).toBe(true);
    }
  });
});
