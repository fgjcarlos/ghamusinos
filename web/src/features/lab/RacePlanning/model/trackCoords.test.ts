import { describe, expect, it } from 'vitest';
import {
  adjustedSpeedKmh,
  bearingDeg,
  cumulativeKm,
  dPlusM,
  estimateTotalTimeMin,
  haversineKm,
  indexAtKm,
  slopePercentAt,
  toTrackPoints,
  totalDistanceKm,
} from './trackCoords';
import type { TrackPoint } from './types';

/** One kilometre of longitude at the equator, in degrees. */
const DEG_PER_KM = 0.0089932;

function flatLine(km: number, steps = 10): TrackPoint[] {
  return Array.from({ length: steps + 1 }, (_, index) => ({
    lat: 0,
    lon: ((km * index) / steps) * DEG_PER_KM,
    ele: 0,
  }));
}

describe('toTrackPoints', () => {
  it('returns an empty array for empty input', () => {
    expect(toTrackPoints([])).toEqual([]);
  });

  it('drops non-objects and invalid coordinates', () => {
    expect(
      toTrackPoints([
        null,
        'nope',
        { lat: Number.NaN, lon: 0 },
        { lat: 40, lon: Number.POSITIVE_INFINITY },
        { lat: 40, lon: -3, ele: 700 },
      ]),
    ).toEqual([{ lat: 40, lon: -3, ele: 700 }]);
  });

  it('accepts the lng alias and normalises a missing elevation to null', () => {
    expect(toTrackPoints([{ lat: 40, lng: -3 }])).toEqual([{ lat: 40, lon: -3, ele: null }]);
  });

  it('keeps a valid zero elevation', () => {
    expect(toTrackPoints([{ lat: 40, lon: -3, ele: 0 }])).toEqual([{ lat: 40, lon: -3, ele: 0 }]);
  });
});

describe('haversineKm', () => {
  it('measures one kilometre along the equator within one metre', () => {
    expect(haversineKm({ lat: 0, lon: 0 }, { lat: 0, lon: DEG_PER_KM })).toBeCloseTo(1, 2);
  });

  it('returns zero for identical points', () => {
    expect(haversineKm({ lat: 40, lon: -3 }, { lat: 40, lon: -3 })).toBe(0);
  });
});

describe('cumulativeKm and totalDistanceKm', () => {
  it('starts at zero and grows monotonically', () => {
    const points = flatLine(2, 2);
    const distances = cumulativeKm(points);
    expect(distances[0]).toBe(0);
    expect(distances[1]).toBeGreaterThan(0);
    expect(distances[2]).toBeGreaterThan(distances[1]);
    expect(totalDistanceKm(points)).toBeCloseTo(2, 2);
  });

  it('returns zero for an empty track', () => {
    expect(totalDistanceKm([])).toBe(0);
    expect(cumulativeKm([])).toEqual([]);
  });
});

describe('dPlusM', () => {
  it('ignores deltas below the noise threshold', () => {
    const points: TrackPoint[] = [
      { lat: 0, lon: 0, ele: 100 },
      { lat: 0, lon: 0.001, ele: 100.5 },
      { lat: 0, lon: 0.002, ele: 110.5 },
    ];
    expect(dPlusM(points)).toBeCloseTo(10, 0);
  });

  it('skips segments with a missing elevation', () => {
    const points: TrackPoint[] = [
      { lat: 0, lon: 0, ele: 100 },
      { lat: 0, lon: 0.001, ele: null },
      { lat: 0, lon: 0.002, ele: 150 },
    ];
    expect(dPlusM(points)).toBe(0);
  });
});

describe('slopePercentAt', () => {
  it('computes rise over run in percent', () => {
    const points: TrackPoint[] = [
      { lat: 0, lon: 0, ele: 0 },
      { lat: 0, lon: 0.0089932, ele: 100 },
    ];
    expect(slopePercentAt(points, 1)).toBeCloseTo(10, 1);
  });

  it('returns zero for an out-of-range index or zero distance', () => {
    const points: TrackPoint[] = [
      { lat: 0, lon: 0, ele: 0 },
      { lat: 0, lon: 0, ele: 100 },
    ];
    expect(slopePercentAt(points, 0)).toBe(0);
    expect(slopePercentAt(points, 1)).toBe(0);
  });
});

describe('adjustedSpeedKmh', () => {
  it('returns the flat speed on zero slope', () => {
    expect(adjustedSpeedKmh(10, 0)).toBe(10);
  });

  it('loses 2.5% of speed per uphill percent', () => {
    expect(adjustedSpeedKmh(10, 4)).toBeCloseTo(9, 6);
  });

  it('clamps extreme uphill to 30% of the flat speed', () => {
    expect(adjustedSpeedKmh(10, 40)).toBeCloseTo(3, 6);
  });

  it('gains 2% per downhill percent below 15%', () => {
    expect(adjustedSpeedKmh(10, -10)).toBeCloseTo(12, 6);
  });

  it('loses speed again on very steep downhill', () => {
    expect(adjustedSpeedKmh(10, -25)).toBeCloseTo(8, 6);
  });
});

describe('estimateTotalTimeMin', () => {
  it('returns zero for fewer than two points', () => {
    expect(estimateTotalTimeMin([], 6)).toBe(0);
    expect(estimateTotalTimeMin([{ lat: 0, lon: 0, ele: 0 }], 6)).toBe(0);
  });

  it('takes 60 minutes for 10 flat kilometres at 6 min/km', () => {
    expect(estimateTotalTimeMin(flatLine(10), 6)).toBeCloseTo(60, 1);
  });

  it('takes longer on a climb than on the flat at the same distance', () => {
    const flat = flatLine(1);
    const climb: TrackPoint[] = [
      { lat: 0, lon: 0, ele: 0 },
      { lat: 0, lon: DEG_PER_KM, ele: 100 },
    ];
    expect(estimateTotalTimeMin(climb, 6)).toBeGreaterThan(estimateTotalTimeMin(flat, 6));
  });

  it('takes less time on a moderate descent than on the flat', () => {
    const descent: TrackPoint[] = [
      { lat: 0, lon: 0, ele: 100 },
      { lat: 0, lon: DEG_PER_KM, ele: 0 },
    ];
    expect(estimateTotalTimeMin(descent, 6)).toBeLessThan(estimateTotalTimeMin(flatLine(1), 6));
  });
});

describe('indexAtKm', () => {
  const points = flatLine(3, 3);

  it('returns the first index at or below zero', () => {
    expect(indexAtKm(points, 0)).toBe(0);
    expect(indexAtKm(points, -1)).toBe(0);
  });

  it('returns the point that reaches the target kilometre', () => {
    expect(indexAtKm(points, 1)).toBe(1);
    expect(indexAtKm(points, 2)).toBe(2);
    expect(indexAtKm(points, 3)).toBe(3);
  });

  it('clamps beyond the track end', () => {
    expect(indexAtKm(points, 99)).toBe(points.length - 1);
  });

  it('returns -1 for an empty track', () => {
    expect(indexAtKm([], 1)).toBe(-1);
  });
});

describe('bearingDeg', () => {
  it('reports north and east correctly', () => {
    expect(bearingDeg({ lat: 0, lon: 0 }, { lat: 1, lon: 0 })).toBeCloseTo(0, 3);
    expect(bearingDeg({ lat: 0, lon: 0 }, { lat: 0, lon: 1 })).toBeCloseTo(90, 3);
  });
});
