// TDD contract for projectTrack (issue 156).
//
// The function is the "80% of the value" of the elevation profile:
// it turns raw GPX points into SVG path strings and band positions.
// The interesting cases are the edge cases — empty tracks, missing
// elevation gaps, flat tracks, negative elevations, bands at the
// viewport edges, overlapping bands. These are what would silently
// produce a "plausible but lying" chart if they were wrong.

import { describe, it, expect } from 'vitest';
import { projectTrack, type ProjectedBand, type ProjectedTick } from './projectTrack';

interface TrackPoint {
  distance_m: number;
  elevation_m: number | null;
}

interface Climb {
  start_idx: number;
  end_idx: number;
  gain_m: number;
}

interface RiskZone {
  start_idx: number;
  end_idx: number;
}

const VIEWPORT = { width: 800, height: 200 };

describe('projectTrack', () => {
  it('returns empty projection when the track has no points', () => {
    const result = projectTrack([], [], [], VIEWPORT);
    expect(result.path).toBe('');
    expect(result.area).toBe('');
    expect(result.bands).toEqual([]);
    expect(result.yTicks).toEqual([]);
    expect(result.xTicks).toEqual([]);
  });

  it('handles a single-point track without panicking', () => {
    const points: TrackPoint[] = [{ distance_m: 100, elevation_m: 50 }];
    const result = projectTrack(points, [], [], VIEWPORT);
    // Path should contain a single "M x,y L x,y" moveTo
    expect(result.path).toMatch(/^M\s/);
    expect(result.path).toMatch(/L\s/);
    expect(result.bands).toEqual([]);
  });

  it('skips points with null elevation rather than drawing a drop to zero', () => {
    const points: TrackPoint[] = [
      { distance_m: 0, elevation_m: 100 },
      { distance_m: 100, elevation_m: null },
      { distance_m: 200, elevation_m: 200 },
    ];
    const result = projectTrack(points, [], [], VIEWPORT);
    // Contract: the polyline must NOT dip to y = elevation(0) at the
    // gap. Every numeric y-coordinate in the path must be inside the
    // plot area (which sits between PADDING.top and PADDING.top + plotH).
    // The path is broken at the gap (two non-empty segments), so the
    // exact M/L count is implementation detail — we don't assert it.
    const yMatches = result.path.match(/[ML]\s*[\d.]+\s+([\d.]+)/g) || [];
    expect(yMatches.length).toBeGreaterThan(0);
    for (const m of yMatches) {
      const y = parseFloat(m.split(/\s+/).pop()!);
      expect(y).toBeGreaterThan(0);
      expect(y).toBeLessThan(VIEWPORT.height);
    }
  });

  it('handles a flat track without dividing by zero', () => {
    const points: TrackPoint[] = [
      { distance_m: 0, elevation_m: 100 },
      { distance_m: 100, elevation_m: 100 },
      { distance_m: 200, elevation_m: 100 },
    ];
    const result = projectTrack(points, [], [], VIEWPORT);
    expect(result.path).toMatch(/[ML]/);
    // Y range collapses to a single value; no NaN in path.
    expect(result.path).not.toMatch(/NaN/);
  });

  it('handles negative elevation (below sea level)', () => {
    const points: TrackPoint[] = [
      { distance_m: 0, elevation_m: -10 },
      { distance_m: 100, elevation_m: 5 },
      { distance_m: 200, elevation_m: -20 },
    ];
    const result = projectTrack(points, [], [], VIEWPORT);
    expect(result.path).not.toMatch(/NaN/);
    // All y-coords should still be inside the viewport.
    const yMatches = result.path.match(/[ML]\s*[\d.]+\s+([\d.]+)/g) || [];
    for (const m of yMatches) {
      const y = parseFloat(m.split(/\s+/).pop()!);
      expect(y).toBeGreaterThanOrEqual(0);
      expect(y).toBeLessThanOrEqual(VIEWPORT.height);
    }
  });

  it('keeps a climb that starts at index 0 inside the viewport', () => {
    const points: TrackPoint[] = [
      { distance_m: 0, elevation_m: 0 },
      { distance_m: 1000, elevation_m: 200 },
      { distance_m: 2000, elevation_m: 200 },
    ];
    const climbs: Climb[] = [{ start_idx: 0, end_idx: 1, gain_m: 200 }];
    const result = projectTrack(points, climbs, [], VIEWPORT);
    expect(result.bands).toHaveLength(1);
    expect(result.bands[0].x).toBeGreaterThanOrEqual(0);
    expect(result.bands[0].x + result.bands[0].width).toBeLessThanOrEqual(VIEWPORT.width);
  });

  it('keeps a climb that ends at the last index inside the viewport', () => {
    const points: TrackPoint[] = [
      { distance_m: 0, elevation_m: 0 },
      { distance_m: 500, elevation_m: 300 },
      { distance_m: 1000, elevation_m: 300 },
    ];
    const climbs: Climb[] = [{ start_idx: 1, end_idx: 2, gain_m: 300 }];
    const result = projectTrack(points, climbs, [], VIEWPORT);
    expect(result.bands).toHaveLength(1);
    expect(result.bands[0].x + result.bands[0].width).toBeLessThanOrEqual(VIEWPORT.width);
  });

  it('renders both bands when a climb and a risk zone overlap', () => {
    const points: TrackPoint[] = [
      { distance_m: 0, elevation_m: 0 },
      { distance_m: 500, elevation_m: 200 },
      { distance_m: 1000, elevation_m: 200 },
    ];
    const climbs: Climb[] = [{ start_idx: 0, end_idx: 2, gain_m: 200 }];
    const risks: RiskZone[] = [{ start_idx: 1, end_idx: 2 }];
    const result = projectTrack(points, climbs, risks, VIEWPORT);
    expect(result.bands).toHaveLength(2);
    const types = new Set(result.bands.map((b: ProjectedBand) => b.type));
    expect(types).toEqual(new Set(['climb', 'risk']));
  });

  it('does not panic when the viewport has zero dimensions', () => {
    const points: TrackPoint[] = [
      { distance_m: 0, elevation_m: 0 },
      { distance_m: 100, elevation_m: 50 },
    ];
    const result = projectTrack(points, [], [], { width: 0, height: 0 });
    // Should return some projection without NaN or undefined values
    expect(result.path).toBeDefined();
    expect(result.path).not.toMatch(/NaN|undefined/);
  });

  it('produces y-ticks covering the elevation range', () => {
    const points: TrackPoint[] = [
      { distance_m: 0, elevation_m: 100 },
      { distance_m: 500, elevation_m: 500 },
      { distance_m: 1000, elevation_m: 1000 },
    ];
    const result = projectTrack(points, [], [], VIEWPORT);
    expect(result.yTicks.length).toBeGreaterThanOrEqual(2);
    const values = result.yTicks
      .map((t: ProjectedTick) => t.value)
      .sort((a: number, b: number) => a - b);
    // Lowest value should be ≤ lowest elevation, highest ≥ highest
    expect(values[0]).toBeLessThanOrEqual(100);
    expect(values[values.length - 1]).toBeGreaterThanOrEqual(1000);
  });

  it('produces x-ticks covering the distance range', () => {
    const points: TrackPoint[] = [
      { distance_m: 0, elevation_m: 100 },
      { distance_m: 500, elevation_m: 200 },
      { distance_m: 1000, elevation_m: 300 },
      { distance_m: 1500, elevation_m: 400 },
      { distance_m: 2000, elevation_m: 500 },
    ];
    const result = projectTrack(points, [], [], VIEWPORT);
    expect(result.xTicks.length).toBeGreaterThanOrEqual(2);
    const values = result.xTicks
      .map((t: ProjectedTick) => t.value)
      .sort((a: number, b: number) => a - b);
    expect(values[0]).toBeLessThanOrEqual(0);
    expect(values[values.length - 1]).toBeGreaterThanOrEqual(2000);
  });
});
