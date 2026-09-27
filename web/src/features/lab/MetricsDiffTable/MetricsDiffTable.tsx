// MetricsDiffTable: shows the comparison diff side-by-side per track.
// Issue 126. Pure presentational. Each row is a metric; each column
// (after the metric label) is a track. The cell at the "best track"
// index carries an accent border to highlight the winner.

import type { CompareMetric } from '../../../lib/api/types';
import styles from './MetricsDiffTable.module.css';

export interface MetricsDiffTableProps {
  metrics: Record<string, CompareMetric>;
  /** Order of tracks as displayed; aligns with diff.best_track indices. */
  trackNames: string[];
}

interface Row {
  label: string;
  metric: CompareMetric;
}

export function MetricsDiffTable({ metrics, trackNames }: MetricsDiffTableProps) {
  const rows: Row[] = Object.entries(metrics).map(([label, metric]) => ({
    label,
    metric,
  }));

  return (
    <section
      className={styles.table}
      aria-label="Tabla de métricas"
      data-testid="metrics-diff-table"
    >
      <table className={styles.grid}>
        <thead>
          <tr>
            <th scope="col" className={styles.metricHeader}>
              Métrica
            </th>
            {trackNames.map((name, i) => (
              <th key={i} scope="col" className={styles.trackHeader}>
                <span className={styles.trackBadge} style={{ background: trackColor(i) }} />
                {name}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, ri) => (
            <tr key={ri}>
              <th scope="row" className={styles.metricCell}>
                {row.label}
              </th>
              {trackNames.map((_, i) => {
                const value = row.metric.values[i];
                const isBest = i === row.metric.best_track;
                const unit = row.metric.unit;
                return (
                  <td
                    key={i}
                    className={isBest ? `${styles.cell} ${styles.cellBest}` : styles.cell}
                  >
                    {formatValue(value)} <span className={styles.unit}>{unit}</span>
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function trackColor(i: number): string {
  // Mirrors TRACK_COLORS in ComparisonMap.tsx — the diff table colours
  // tracks consistently across views.
  const colors = ['#7fc689', '#9e7a1f', '#8c3324'];
  return colors[i] ?? colors[colors.length - 1];
}

function formatValue(v: number | undefined | null): string {
  if (v === undefined || v === null) return '—';
  return Number.isFinite(v) ? v.toFixed(0) : '—';
}
