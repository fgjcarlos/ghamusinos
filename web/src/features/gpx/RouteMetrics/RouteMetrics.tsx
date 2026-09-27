// RouteMetrics: 4 main + 6 derived metrics for the route detail page.
// Issue 157. Pure presentational. Uses MetricTile from src/ui/.

import { MetricTile } from '../../../ui/MetricTile';
import type { NormalizedAnalysis } from '../normalize';
import styles from './RouteMetrics.module.css';

export interface RouteMetricsProps {
  analysis: NormalizedAnalysis;
}

function formatSeconds(s: number): string {
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function formatKm(m: number): string {
  return `${(m / 1000).toFixed(1)} km`;
}

function formatM(m: number): string {
  return `${m.toFixed(0)} m`;
}

function formatPct(p: number): string {
  return `${p.toFixed(1)}%`;
}

function formatNumber(n: number): string {
  return n.toFixed(1);
}

export function RouteMetrics({ analysis }: RouteMetricsProps) {
  return (
    <section className={styles.metrics} aria-label="Métricas del track">
      <h2 className={styles.title}>Métricas</h2>
      <div className={styles.tiles}>
        {/* 4 principales */}
        <MetricTile label="Distancia" value={formatKm(analysis.distance_m)} />
        <MetricTile label="Desnivel +" value={formatM(analysis.d_plus_m)} />
        <MetricTile label="Desnivel −" value={formatM(analysis.d_minus_m)} />
        <MetricTile label="Tiempo en mov." value={formatSeconds(analysis.moving_time_s)} />

        {/* 6 derivadas */}
        <MetricTile label="Pendiente media" value={formatPct(analysis.avg_slope_pct)} />
        <MetricTile label="Pendiente máx" value={formatPct(analysis.max_slope_pct)} />
        <MetricTile label="ITRA" value={String(analysis.itra_points)} />
        <MetricTile label="Índice esfuerzo" value={formatNumber(analysis.effort_index)} />
        <MetricTile label="VAM estimada" value={formatM(analysis.estimated_vam)} />
        <MetricTile label="Corribilidad" value={formatPct(analysis.runnability_pct)} />
      </div>
    </section>
  );
}
