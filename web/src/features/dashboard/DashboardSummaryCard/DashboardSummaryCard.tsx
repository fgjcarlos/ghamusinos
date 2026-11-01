// DashboardSummaryCard: weekly KPIs + the weekly sparkline.
// Fases 1.4 (issue #16, DW-005). Pure presentational — the container
// owns the data; this component only draws it.
//
// With no activities in the current week the card shows the empty
// state instead of a row of zeros, so a brand new user is not told
// "0 m" but "sync your activities".

import type { DashboardSummary } from '../../../lib/api/dashboard';
import { EmptyState } from '../../../ui/EmptyState';
import { MetricTile } from '../../../ui/MetricTile';
import styles from './DashboardSummaryCard.module.css';

export interface DashboardSummaryCardProps {
  /** Weekly KPIs as returned by GET /dashboard/summary. */
  summary: DashboardSummary;
  /** TSS of the last 7 days — drives the sparkline. */
  sparkline: number[];
}

/**
 * formatTrend renders a trend percentage. `null` means "no previous
 * window to compare against", shown as an em dash.
 */
function formatTrend(value: number | null): string {
  if (value === null) return '—';
  const rounded = value.toFixed(1);
  return value > 0 ? `+${rounded}` : rounded;
}

export function DashboardSummaryCard({ summary, sparkline }: DashboardSummaryCardProps) {
  if (summary.weekly_activities_count === 0) {
    return (
      <section className={styles.card} data-testid="dashboard-summary-card">
        <div data-testid="dashboard-empty-state">
          <EmptyState message="Sincroniza tus actividades de Strava para empezar" />
        </div>
      </section>
    );
  }

  return (
    <section className={styles.card} data-testid="dashboard-summary-card">
      <div className={styles.tiles}>
        <div className={styles.tile} data-testid="metric-weekly-volume" title="weekly_volume_m">
          <MetricTile
            label="Volumen semanal"
            value={Math.round(summary.weekly_volume_m)}
            unit="m"
          />
        </div>
        <div
          className={styles.tile}
          data-testid="metric-weekly-elevation"
          title="weekly_elevation_m"
        >
          <MetricTile
            label="Desnivel semanal"
            value={Math.round(summary.weekly_elevation_m)}
            unit="m"
          />
        </div>
        <div
          className={styles.tile}
          data-testid="metric-weekly-activities"
          title="weekly_activities_count"
        >
          <MetricTile
            label="Actividades esta semana"
            value={summary.weekly_activities_count}
            unit="sesiones"
          />
        </div>
        <div
          className={styles.tile}
          data-testid="metric-trend"
          title="trend_7d_pct / trend_30d_pct"
        >
          <MetricTile
            label="Tendencia 7d / 30d"
            value={`${formatTrend(summary.trend_7d_pct)} / ${formatTrend(summary.trend_30d_pct)}`}
            unit="%"
          />
        </div>
      </div>
      <WeeklySparkline values={sparkline} />
    </section>
  );
}

const SPARKLINE_WIDTH = 160;
const SPARKLINE_HEIGHT = 36;

/**
 * WeeklySparkline draws the TSS of the last 7 days as a plain SVG
 * polyline. It stays in the DOM (SVG, not canvas) so the tests can
 * read it and the browser can scale it without extra machinery.
 */
function WeeklySparkline({ values }: { values: number[] }) {
  const points = sparklinePoints(values);
  return (
    <figure className={styles.sparkline} data-testid="weekly-sparkline">
      <svg
        role="img"
        aria-label="Carga de las últimas 7 sesiones"
        viewBox={`0 0 ${SPARKLINE_WIDTH} ${SPARKLINE_HEIGHT}`}
        preserveAspectRatio="none"
        className={styles.sparklineSvg}
      >
        {points.length > 0 ? <polyline points={points} className={styles.sparklineLine} /> : null}
      </svg>
      <figcaption className={styles.sparklineCaption}>Últimos 7 días</figcaption>
    </figure>
  );
}

/**
 * sparklinePoints maps the values to a polyline inside the viewBox.
 */
function sparklinePoints(values: number[]): string {
  if (values.length === 0) return '';
  const max = Math.max(...values);
  const min = Math.min(...values, 0);
  const span = max - min || 1;
  const step = values.length > 1 ? SPARKLINE_WIDTH / (values.length - 1) : SPARKLINE_WIDTH;
  return values
    .map((value, index) => {
      const x = index * step;
      const y = SPARKLINE_HEIGHT - ((value - min) / span) * SPARKLINE_HEIGHT;
      return `${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(' ');
}
