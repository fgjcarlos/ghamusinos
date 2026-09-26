// ElevationProfile: SVG renderer for a track's elevation curve.
// Issue 156. Pure presentational — receives already-projected
// coordinates from projectTrack and renders them as SVG primitives.

import type { ProjectedTrack, ProjectedTick } from './projectTrack';
import { ProfileBand } from './ProfileBand';
import styles from './ElevationProfile.module.css';

export interface ElevationProfileProps {
  /** Result of projectTrack — geometry already computed. */
  projected: ProjectedTrack;
  /** Explicit width of the SVG viewBox. */
  width: number;
  /** Explicit height of the SVG viewBox. */
  height: number;
}

const PADDING_TOP = 12;
const PADDING_BOTTOM = 24;

export function ElevationProfile({ projected, width, height }: ElevationProfileProps) {
  const ariaLabel = buildAriaLabel(projected);
  const plotHeight = Math.max(0, height - PADDING_TOP - PADDING_BOTTOM);

  return (
    <svg
      role="img"
      aria-label={ariaLabel}
      viewBox={`0 0 ${width} ${height}`}
      width={width}
      height={height}
      className={styles.profile}
      data-testid="elevation-profile"
    >
      {/* Area fill (translucent) under the curve. */}
      <path d={projected.area} className={styles.area} />
      {/* Polyline of the elevation curve itself. */}
      <path d={projected.path} className={styles.line} />
      {/* Highlight bands for climbs and risk zones. */}
      {projected.bands.map((band, i) => (
        <ProfileBand
          key={`${band.type}-${i}`}
          band={band}
          y={PADDING_TOP}
          height={plotHeight}
          label={bandLabel(band, i)}
        />
      ))}
      {/* Y-axis tick labels (left margin). */}
      {projected.yTicks.map((tick, i) => (
        <text
          key={`y-${i}`}
          x={tick.x}
          y={tick.y}
          textAnchor="end"
          dominantBaseline="middle"
          className={styles.tickLabel}
        >
          {tick.label}
        </text>
      ))}
      {/* X-axis tick labels (bottom margin). */}
      {projected.xTicks.map((tick, i) => (
        <text key={`x-${i}`} x={tick.x} y={tick.y} textAnchor="middle" className={styles.tickLabel}>
          {tick.label}
        </text>
      ))}
      {/* Baseline at the bottom of the plot area. */}
      <line
        x1={0}
        x2={width}
        y1={height - PADDING_BOTTOM}
        y2={height - PADDING_BOTTOM}
        className={styles.baseline}
      />
    </svg>
  );
}

function bandLabel(band: ProjectedTrack['bands'][number], i: number): string {
  return band.type === 'climb' ? `Subida ${i + 1}` : `Zona de riesgo ${i + 1}`;
}

function buildAriaLabel(projected: ProjectedTrack): string {
  if (projected.yTicks.length < 2) return 'Perfil de elevación';
  const first: ProjectedTick = projected.yTicks[0];
  const last: ProjectedTick = projected.yTicks[projected.yTicks.length - 1];
  return `Perfil de elevación: ${first.label} a ${last.label} de altitud`;
}
