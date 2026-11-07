import { describe, expect, it } from 'vitest';
import {
  DEFAULT_FATIGUE_FACTOR,
  DEFAULT_NUTRITION_INTERVAL_KM,
  DEFAULT_TARGET_PACE_MIN_PER_KM,
  PACE_PRESETS,
  kmMarkers,
  nutritionPoints,
} from './raceDay';
import type { TrackPoint } from './types';

/** One kilometre of longitude at the equator, in degrees. */
const DEG_PER_KM = 1 / ((6371.0088 * Math.PI) / 180);

function flatLine(km: number, stepsPerKm = 10): TrackPoint[] {
  const steps = Math.round(km * stepsPerKm);
  return Array.from({ length: steps + 1 }, (_, index) => ({
    lat: 0,
    lon: ((km * index) / steps) * DEG_PER_KM,
    ele: 0,
  }));
}

/** A steady climb gaining `gainPerKm` metres over every kilometre. */
function climbLine(km: number, gainPerKm: number, stepsPerKm = 10): TrackPoint[] {
  const steps = Math.round(km * stepsPerKm);
  return Array.from({ length: steps + 1 }, (_, index) => ({
    lat: 0,
    lon: ((km * index) / steps) * DEG_PER_KM,
    ele: (index / steps) * km * gainPerKm,
  }));
}

describe('kmMarkers', () => {
  it('returns no markers for an empty track', () => {
    expect(kmMarkers([])).toEqual([]);
  });

  it('returns one marker at km 0 for a single point', () => {
    expect(kmMarkers([{ lat: 40, lon: -3, ele: 700 }])).toEqual([
      { km: 0, index: 0, lat: 40, lon: -3, cumulativeGainM: 0 },
    ]);
  });

  it('emits one marker per kilometre on a 3 km flat line', () => {
    const markers = kmMarkers(flatLine(3));
    expect(markers.map((marker) => marker.km)).toEqual([0, 1, 2, 3]);
    expect(markers.map((marker) => marker.index)).toEqual([0, 10, 20, 30]);
    for (const marker of markers) {
      expect(marker.cumulativeGainM).toBe(0);
    }
    expect(markers[1].lat).toBe(0);
    expect(markers[1].lon).toBeCloseTo(DEG_PER_KM, 6);
  });

  it('accumulates gain on a climb', () => {
    const markers = kmMarkers(climbLine(3, 100));
    expect(markers[0].cumulativeGainM).toBe(0);
    expect(markers[1].cumulativeGainM).toBeCloseTo(100, 0);
    expect(markers[2].cumulativeGainM).toBeCloseTo(200, 0);
    expect(markers[3].cumulativeGainM).toBeCloseTo(300, 0);
    expect(markers[3].cumulativeGainM).toBeGreaterThan(markers[0].cumulativeGainM);
  });

  it('deduplicates markers that fall on the same point of a sparse track', () => {
    const sparse: TrackPoint[] = [
      { lat: 0, lon: 0, ele: 0 },
      { lat: 0, lon: 5 * DEG_PER_KM, ele: 0 },
    ];
    const markers = kmMarkers(sparse);
    expect(markers.map((marker) => marker.km)).toEqual([0, 1]);
    expect(markers.map((marker) => marker.index)).toEqual([0, 1]);
  });
});

describe('nutritionPoints', () => {
  it('returns no points for fewer than two track points', () => {
    expect(nutritionPoints([])).toEqual([]);
    expect(nutritionPoints([{ lat: 0, lon: 0, ele: 0 }])).toEqual([]);
  });

  it('returns no points for a non-positive or non-finite interval', () => {
    const points = flatLine(25, 1);
    expect(nutritionPoints(points, 0)).toEqual([]);
    expect(nutritionPoints(points, -5)).toEqual([]);
    expect(nutritionPoints(points, Number.NaN)).toEqual([]);
    expect(nutritionPoints(points, Number.POSITIVE_INFINITY)).toEqual([]);
  });

  it('returns no points when the interval is larger than the track', () => {
    expect(nutritionPoints(flatLine(5, 1), 10)).toEqual([]);
  });

  it('emits points every 10 km on a 25 km line, never at the exact finish', () => {
    const points = nutritionPoints(flatLine(25, 1));
    expect(points.map((point) => point.km)).toEqual([10, 20]);
    expect(points[0].index).not.toBe(points[1].index);
    for (const point of points) {
      expect(point.lat).toBe(0);
      expect(point.lon).toBeCloseTo(point.km * DEG_PER_KM, 6);
    }
  });

  it('decides recovery versus scheduled by the chosen point slope', () => {
    const flatPoints = nutritionPoints(flatLine(3, 1), 1);
    expect(flatPoints.map((point) => point.km)).toEqual([1, 2]);
    for (const point of flatPoints) {
      expect(point.kind).toBe('recovery');
    }

    const climbPoints = nutritionPoints(climbLine(3, 100, 1), 1);
    expect(climbPoints.map((point) => point.km)).toEqual([1, 2]);
    for (const point of climbPoints) {
      expect(point.kind).toBe('scheduled');
    }
  });

  it('chooses the flattest point inside the +/-500 m window', () => {
    // 3 km with ten steps per kilometre: a climb to index 16, a flat stretch
    // from index 16 to index 20, then a climb again.
    const points: TrackPoint[] = Array.from({ length: 31 }, (_, index) => ({
      lat: 0,
      lon: (index / 10) * DEG_PER_KM,
      ele: index <= 16 ? index * 10 : index <= 20 ? 160 : 160 + (index - 20) * 10,
    }));
    const nutrition = nutritionPoints(points, 2);
    expect(nutrition).toHaveLength(1);
    expect(nutrition[0].km).toBe(2);
    // Indexes 17..20 are inside the window with zero slope; the lowest wins.
    expect(nutrition[0].index).toBe(17);
    expect(nutrition[0].kind).toBe('recovery');
  });

  it('deduplicates points that map to the same index', () => {
    const sparse: TrackPoint[] = [
      { lat: 0, lon: 0, ele: 0 },
      { lat: 0, lon: 10 * DEG_PER_KM, ele: 0 },
    ];
    const points = nutritionPoints(sparse, 4);
    expect(points).toHaveLength(1);
    expect(points[0].km).toBe(4);
    expect(points[0].index).toBe(1);
  });
});

describe('race-day constants and presets', () => {
  it('exposes the default constants', () => {
    expect(DEFAULT_TARGET_PACE_MIN_PER_KM).toBe(6.5);
    expect(DEFAULT_NUTRITION_INTERVAL_KM).toBe(10);
    expect(DEFAULT_FATIGUE_FACTOR).toBe(0.05);
  });

  it('exposes the pace presets in order', () => {
    expect(PACE_PRESETS).toHaveLength(4);
    expect(PACE_PRESETS.map((preset) => preset.id)).toEqual(['hike', 'easy', 'steady', 'fast']);
    expect(PACE_PRESETS.map((preset) => preset.label)).toEqual([
      'Senderismo',
      'Suave',
      'Constante',
      'Rápido',
    ]);
    expect(PACE_PRESETS.map((preset) => preset.paceMinPerKm)).toEqual([7.5, 6, 5, 4.2]);
  });
});
