// HRZonesChart: minutes per heart-rate zone (z1..z5).
// Fases 1.4 (issue #16, DW-004). Pure presentational — the container
// owns the data; this component only draws it.
//
// When the backend had to approximate the zones (no HR streams
// ingested) it answers `degraded: true` and the chart says so
// explicitly. With real zones the label is not rendered at all.

import { useMemo } from 'react';
import type { HRZoneBucket } from '../../../lib/api/dashboard';
import { resolveToken } from '../chartTokens';
import type { EChartsCoreOption } from '../echartsSetup';
import { useEChart } from '../useEChart';
import styles from './HRZonesChart.module.css';

export interface HRZonesChartProps {
  /** The five buckets, z1..z5. */
  zones: HRZoneBucket[];
  /** True when the backend approximated the zones. */
  degraded: boolean;
}

/** Colour token per zone, from the design tokens only. */
const ZONE_TOKENS = [
  '--gh-accent-deep',
  '--gh-accent-dim',
  '--gh-accent',
  '--gh-warn',
  '--gh-risk',
];

const ZONE_IDS = ['z1', 'z2', 'z3', 'z4', 'z5'] as const;

/**
 * buildHRZonesOption maps the buckets to a stacked bar option.
 */
function buildHRZonesOption(zones: HRZoneBucket[]): EChartsCoreOption {
  const text = resolveToken('--gh-text-2');
  const line = resolveToken('--gh-line');

  const series = ZONE_IDS.map((zoneId, index) => {
    const bucket = zones.find((candidate) => candidate.zone === zoneId);
    const color = resolveToken(ZONE_TOKENS[index]);
    return {
      name: zoneId,
      type: 'bar',
      stack: 'zones',
      barMaxWidth: 96,
      data: [bucket ? bucket.minutes : 0],
      itemStyle: { color },
    };
  });

  return {
    grid: { left: 44, right: 16, top: 16, bottom: 28 },
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: { show: false },
    title: { show: false },
    xAxis: {
      type: 'category',
      data: ['minutos'],
      axisLabel: { color: text },
      axisLine: { lineStyle: { color: line } },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: text },
      splitLine: { lineStyle: { color: line } },
    },
    series,
  };
}

export function HRZonesChart({ zones, degraded }: HRZonesChartProps) {
  const option = useMemo(() => buildHRZonesOption(zones), [zones]);
  const containerRef = useEChart(option);

  return (
    <section className={styles.card} aria-label="Zonas de frecuencia cardíaca">
      <header className={styles.header}>
        <h2 className={styles.title}>Zonas de frecuencia cardíaca</h2>
        <ul className={styles.legend} data-testid="hr-zones-legend">
          {ZONE_IDS.map((zoneId, index) => (
            <li key={zoneId} className={`${styles.legendItem} ${styles[`legendZone${index + 1}`]}`}>
              {zoneId}
            </li>
          ))}
        </ul>
      </header>
      {degraded ? (
        <p className={styles.degraded} data-testid="hr-zones-degraded-label">
          aproximación (sin streams HR)
        </p>
      ) : null}
      <div ref={containerRef} className={styles.chart} data-testid="hr-zones-chart" />
    </section>
  );
}
