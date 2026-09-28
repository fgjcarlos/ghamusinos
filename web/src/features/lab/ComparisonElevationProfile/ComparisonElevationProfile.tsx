// ComparisonElevationProfile: SVG overlay of up to 3 elevation
// profiles stacked in the same chart, colour-coded per track. Issue
// 126. The point projection lives in the shared ElevationProfile
// helper (`projectTrack`) — each track gets its own projection.

import { useMemo } from 'react';
import {
  projectTrack,
  type ProjectedTrack,
  type Climb,
  type RiskZone,
} from '../../gpx/ElevationProfile/projectTrack';
import { rawPointsToTrackPoints } from '../../gpx/rawPointsToTrackPoints';
import styles from './ComparisonElevationProfile.module.css';

// Mirrors the values in ComparisonMap.tsx / MetricsDiffTable.tsx so
// the same colour follows the track across views.
const TRACK_COLORS = ['#7fc689', '#9e7a1f', '#8c3324'] as const;

// Raw API points may use Go's `lon` wire key or the normalized `lng` alias.
type RawPoint = { lat: number; lng?: number; lon?: number; ele?: number | null };

export interface ComparisonElevationProfileProps {
  /** Per-track raw elevation points ({ lat, lng | lon, ele? }). */
  tracks: { name: string; points: RawPoint[] }[];
  height?: number;
}

function colorFor(i: number): string {
  return TRACK_COLORS[i] ?? TRACK_COLORS[TRACK_COLORS.length - 1];
}

export function ComparisonElevationProfile({
  tracks,
  height = 200,
}: ComparisonElevationProfileProps) {
  const projections = useMemo<ProjectedTrack[]>(
    () =>
      tracks.map((t) =>
        projectTrack(rawPointsToTrackPoints(t.points), [] as Climb[], [] as RiskZone[], {
          width: 800,
          height,
        }),
      ),
    [tracks, height],
  );

  // Stack the SVGs at the same size, each colored, sharing the same
  // coordinate system (we re-render with the same viewport).
  return (
    <section
      className={styles.profile}
      aria-label="Perfiles de elevación comparados"
      data-testid="comparison-elevation-profile"
    >
      <div className={styles.stack} style={{ height: `${height}px` }}>
        {projections.map((projected, i) => (
          <SingleProfile
            key={i}
            name={tracks[i]?.name ?? `Track ${i + 1}`}
            color={colorFor(i)}
            projected={projected}
            total={tracks.length}
          />
        ))}
      </div>
      <ul className={styles.legend} aria-label="Leyenda">
        {tracks.map((t, i) => (
          <li key={i} className={styles.legendItem}>
            <span
              className={styles.swatch}
              style={{ background: colorFor(i) }}
              aria-hidden="true"
            />
            {t.name}
          </li>
        ))}
      </ul>
    </section>
  );
}

interface SingleProfileProps {
  name: string;
  color: string;
  projected: ProjectedTrack;
  total: number;
}

function SingleProfile({ name, color, projected, total }: SingleProfileProps) {
  const w = projected.xTicks.length ? Math.max(...projected.xTicks.map((t) => t.x)) : 800;
  const h = projected.yTicks.length ? Math.max(...projected.yTicks.map((t) => t.y)) : 200;
  return (
    <svg
      role="img"
      aria-label={`Perfil de elevación: ${name}`}
      viewBox={`0 0 ${w} ${h}`}
      width={w}
      height={h}
      data-track-index={0}
      style={{
        position: 'absolute',
        top: 0,
        left: 0,
        width: '100%',
        height: '100%',
        opacity: total > 1 ? 0.75 : 1,
        pointerEvents: 'none',
      }}
    >
      <path d={projected.area} fill={color} fillOpacity={0.06} stroke="none" />
      <path
        d={projected.path}
        fill="none"
        stroke={color}
        strokeWidth={1.6}
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}
