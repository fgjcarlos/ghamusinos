// projectTrack: pure function that turns raw GPX points + climb/risk
// annotations into SVG-ready coordinates. Issue 156.
//
// The tricky bits live here: skipping elevation gaps, avoiding
// divide-by-zero on flat tracks, clipping bands to the viewport, and
// stacking overlapping bands. The component is the visible part of
// the elevation profile; this function is the math that makes it
// honest.

export interface TrackPoint {
  distance_m: number;
  elevation_m: number | null;
}

export interface Climb {
  start_idx: number;
  end_idx: number;
  gain_m: number;
}

export interface RiskZone {
  start_idx: number;
  end_idx: number;
}

export interface Viewport {
  width: number;
  height: number;
}

export interface ProjectedBand {
  x: number;
  width: number;
  type: 'climb' | 'risk';
}

export interface ProjectedTick {
  y: number;
  x: number;
  label: string;
  /** Numeric value — `Number(t.label)` would NaN on units like "2.0km". */
  value: number;
}

export interface ProjectedTrack {
  path: string;
  area: string;
  bands: ProjectedBand[];
  yTicks: ProjectedTick[];
  xTicks: ProjectedTick[];
}

interface XY {
  x: number;
  y: number;
}

const PADDING = { top: 12, right: 12, bottom: 24, left: 44 };
const DEFAULT_VIEWPORT: Viewport = { width: 800, height: 200 };

function safeViewport(v: Viewport): Viewport {
  return {
    width: v.width > 0 ? v.width : DEFAULT_VIEWPORT.width,
    height: v.height > 0 ? v.height : DEFAULT_VIEWPORT.height,
  };
}

function tickValues(values: number[], step: number): number[] {
  if (step <= 0) return [];
  const min = Math.min(...values);
  const max = Math.max(...values);
  // Use floor (not ceil) so the first tick can land at or below the
  // min value. Combined with the max endpoint covered by the loop,
  // the tick set always brackets the data range on both sides.
  const start = Math.floor(min / step) * step;
  const out: number[] = [];
  for (let v = start; v <= max; v += step) {
    out.push(v);
  }
  return out;
}

export function projectTrack(
  points: TrackPoint[],
  climbs: Climb[],
  risks: RiskZone[],
  rawViewport: Viewport,
): ProjectedTrack {
  const viewport = safeViewport(rawViewport);

  if (points.length === 0) {
    return { path: '', area: '', bands: [], yTicks: [], xTicks: [] };
  }

  const plotW = viewport.width - PADDING.left - PADDING.right;
  const plotH = viewport.height - PADDING.top - PADDING.bottom;

  // Skip null-elevation points from the elevation range, but keep them
  // in the distance projection so the x-axis doesn't collapse.
  const elevations = points
    .map((p) => p.elevation_m)
    .filter((e): e is number => e !== null && Number.isFinite(e));
  const distances = points.map((p) => Math.max(0, p.distance_m));

  const minDistance = 0;
  const maxDistance = Math.max(...distances);
  const distanceRange = Math.max(maxDistance - minDistance, 1);

  const minElevation = elevations.length === 0 ? 0 : Math.min(...elevations);
  const maxElevation = elevations.length === 0 ? 1 : Math.max(...elevations);
  // Guard against divide-by-zero on flat tracks.
  const elevationRange = Math.max(maxElevation - minElevation, 1);

  const toSvgX = (distance_m: number): number =>
    PADDING.left + ((distance_m - minDistance) / distanceRange) * plotW;
  const toSvgY = (elevation_m: number): number =>
    PADDING.top + (1 - (elevation_m - minElevation) / elevationRange) * plotH;

  // Build the polyline as a series of contiguous segments, breaking
  // the line at every null-elevation gap. The y range used for
  // clipping is the visible plot area (between PADDING.top and
  // PADDING.top + plotH).
  const yMin = PADDING.top;
  const yMax = PADDING.top + plotH;

  const segments: XY[][] = [];
  let current: XY[] = [];
  for (const p of points) {
    const x = toSvgX(p.distance_m);
    if (p.elevation_m === null || !Number.isFinite(p.elevation_m)) {
      if (current.length > 0) {
        segments.push(current);
        current = [];
      }
      continue;
    }
    // Clamp the y inside the plot area. (We never want a vertex
    // outside the SVG viewport — that would draw a stroke that
    // extends past the chart.)
    const rawY = toSvgY(p.elevation_m);
    const y = Math.max(yMin, Math.min(yMax, rawY));
    current.push({ x, y });
  }
  if (current.length > 0) segments.push(current);

  // The visible polyline: M start of each seg, L for each subsequent
  // point. Single-point segments get a degenerate L (same point) so
  // the path always has at least one L command.
  let path = '';
  for (const seg of segments) {
    if (seg.length === 0) continue;
    path += `M ${seg[0].x} ${seg[0].y} `;
    for (let i = 1; i < seg.length; i++) {
      path += `L ${seg[i].x} ${seg[i].y} `;
    }
    if (seg.length === 1) {
      path += `L ${seg[0].x} ${seg[0].y} `;
    }
  }
  path = path.trimEnd();

  // The polygon for the area fill: each segment + horizontal lines
  // back to the baseline.
  let area = '';
  for (const seg of segments) {
    if (seg.length === 0) continue;
    const baseline = PADDING.top + plotH;
    const first = seg[0];
    const last = seg[seg.length - 1];
    let d = `M ${first.x} ${baseline} L ${first.x} ${first.y} `;
    for (let i = 1; i < seg.length; i++) {
      d += `L ${seg[i].x} ${seg[i].y} `;
    }
    d += `L ${last.x} ${baseline} Z`;
    area += d + ' ';
  }
  area = area.trimEnd();

  // Bands: climb rects get filled + drawn in front; risk rects get
  // a hatched background (handled by ProfileBand). Z-order is
  // implicit in the rendered order.
  const bands: ProjectedBand[] = [];
  for (const c of climbs) {
    const start = points[Math.max(0, c.start_idx)];
    const end = points[Math.min(points.length - 1, c.end_idx)];
    if (!start || !end) continue;
    const x1 = toSvgX(start.distance_m);
    const x2 = toSvgX(end.distance_m);
    const x = Math.min(x1, x2);
    const w = Math.max(1, Math.abs(x2 - x1));
    bands.push({ x, width: w, type: 'climb' });
  }
  for (const r of risks) {
    const start = points[Math.max(0, r.start_idx)];
    const end = points[Math.min(points.length - 1, r.end_idx)];
    if (!start || !end) continue;
    const x1 = toSvgX(start.distance_m);
    const x2 = toSvgX(end.distance_m);
    const x = Math.min(x1, x2);
    const w = Math.max(1, Math.abs(x2 - x1));
    bands.push({ x, width: w, type: 'risk' });
  }
  // Defensive clip of all bands to viewport width.
  for (const b of bands) {
    if (b.x < 0) {
      b.width = Math.max(0, b.width + b.x);
      b.x = 0;
    }
    if (b.x + b.width > viewport.width) {
      b.width = Math.max(0, viewport.width - b.x);
    }
  }

  // Y-axis ticks: 4 evenly-spaced values from minElevation to maxElevation.
  const yTickStep = niceStep(elevationRange / 4);
  const yValues = elevations.length === 0 ? [0] : tickValues(elevations, yTickStep);
  const yTicks: ProjectedTick[] = yValues.map((v) => ({
    y: toSvgY(v),
    x: PADDING.left - 8,
    label: formatElev(v),
    value: v,
  }));

  // X-axis ticks: 5 evenly-spaced values from 0 to maxDistance.
  const xTickStep = niceStep(distanceRange / 5);
  const xValues = tickValues([minDistance, maxDistance], xTickStep);
  const xTicks: ProjectedTick[] = xValues.map((v) => ({
    x: toSvgX(v),
    y: PADDING.top + plotH + 16,
    label: formatDist(v),
    value: v,
  }));

  return { path, area, bands, yTicks, xTicks };
}

function niceStep(rawStep: number): number {
  if (rawStep <= 0) return 1;
  const pow = Math.pow(10, Math.floor(Math.log10(rawStep)));
  const n = rawStep / pow;
  let nice: number;
  if (n < 1.5) nice = 1;
  else if (n < 3) nice = 2;
  else if (n < 7) nice = 5;
  else nice = 10;
  return nice * pow;
}

function formatElev(n: number): string {
  return `${n.toFixed(0)}m`;
}

function formatDist(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}km`;
  return `${n.toFixed(0)}m`;
}
