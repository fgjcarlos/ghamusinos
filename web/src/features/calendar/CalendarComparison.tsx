import { useEffect, useMemo, useState } from 'react';
import {
  loadCalendarComparison,
  type ComparisonGroup,
  type ComparisonPeriod,
} from '../../lib/api/calendar';
import { EmptyState } from '../../ui';
import { resolveToken } from '../dashboard/chartTokens';
import type { EChartsCoreOption } from '../dashboard/echartsSetup';
import { useEChart } from '../dashboard/useEChart';
import styles from './Calendar.module.css';

const metrics = {
  duration_seconds: { label: 'Duración', unit: 'min', divisor: 60 },
  distance_m: { label: 'Distancia', unit: 'km', divisor: 1000 },
  elevation_gain_m: { label: 'Desnivel positivo', unit: 'm', divisor: 1 },
};
type Metric = keyof typeof metrics;
const number = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 2 });

function ComparisonBars({ periods, metric }: { periods: ComparisonPeriod[]; metric: Metric }) {
  const { divisor, unit } = metrics[metric];
  const option = useMemo<EChartsCoreOption>(
    () => ({
      color: [resolveToken('--gh-accent'), resolveToken('--gh-warn')],
      grid: { left: 52, right: 16, top: 46, bottom: 36 },
      legend: { textStyle: { color: resolveToken('--gh-text-2') } },
      tooltip: {
        trigger: 'axis',
        valueFormatter: (value: unknown) => `${number.format(Number(value))} ${unit}`,
      },
      xAxis: {
        type: 'category',
        data: periods.map((period) => period.period_start),
        axisLabel: { color: resolveToken('--gh-text-2'), hideOverlap: true },
      },
      yAxis: { type: 'value', name: unit, axisLabel: { color: resolveToken('--gh-text-2') } },
      series: [
        { name: 'Planificado', type: 'bar', data: periods.map((p) => p.planned[metric] / divisor) },
        { name: 'Realizado', type: 'bar', data: periods.map((p) => p.actual[metric] / divisor) },
      ],
    }),
    [periods, metric, divisor, unit],
  );
  const ref = useEChart(option);
  return <div ref={ref} className={styles.comparisonChart} aria-hidden="true" />;
}

export function CalendarComparison({
  token,
  from,
  to,
  group,
  revision = 0,
}: {
  token: string;
  from: string;
  to: string;
  group: ComparisonGroup;
  revision?: number;
}) {
  const [retry, setRetry] = useState(0);
  const [metric, setMetric] = useState<Metric>('duration_seconds');
  const key = `${from}:${to}:${group}:${revision}:${retry}`;
  const [result, setResult] = useState<{ key: string; periods: ComparisonPeriod[] | null }>();

  useEffect(() => {
    const controller = new AbortController();
    void loadCalendarComparison(token, from, to, group, controller.signal).then(
      ({ periods }) => {
        if (!controller.signal.aborted) setResult({ key, periods });
      },
      () => {
        if (!controller.signal.aborted) setResult({ key, periods: null });
      },
    );
    return () => controller.abort();
  }, [token, from, to, group, key]);

  const periods = result?.key === key ? result.periods : undefined;
  const { divisor, unit } = metrics[metric];
  return (
    <section className={styles.tray} aria-labelledby="comparison-title">
      <h2 id="comparison-title" className={styles.trayTitle}>
        Plan vs. real
      </h2>
      {periods === undefined ? (
        <p role="status">Cargando comparación…</p>
      ) : periods === null ? (
        <div className={styles.error} role="alert">
          No se pudo cargar la comparación.
          <button type="button" className={styles.navBtn} onClick={() => setRetry((n) => n + 1)}>
            Reintentar comparación
          </button>
        </div>
      ) : periods.length === 0 ? (
        <EmptyState message="No hay carga planificada ni realizada en este periodo." />
      ) : (
        <>
          <label className={styles.comparisonMetric}>
            Métrica
            <select value={metric} onChange={(event) => setMetric(event.target.value as Metric)}>
              {Object.entries(metrics).map(([key, { label, unit }]) => (
                <option key={key} value={key}>
                  {label} ({unit})
                </option>
              ))}
            </select>
          </label>
          <ComparisonBars periods={periods} metric={metric} />
          <details>
            <summary>Ver datos y cumplimiento</summary>
            <div className={styles.comparisonTable}>
              <table>
                <caption>
                  {metrics[metric].label} por {group === 'day' ? 'día' : 'semana'}
                </caption>
                <thead>
                  <tr>
                    <th scope="col">Inicio</th>
                    <th scope="col">Planificado ({unit})</th>
                    <th scope="col">Realizado ({unit})</th>
                    <th scope="col">Total sesiones</th>
                    <th scope="col">Completadas</th>
                    <th scope="col">Omitidas</th>
                    <th scope="col">Modificadas</th>
                  </tr>
                </thead>
                <tbody>
                  {periods.map(({ period_start, planned, actual, compliance }) => (
                    <tr key={period_start}>
                      <th scope="row">
                        <time dateTime={period_start}>{period_start}</time>
                      </th>
                      <td>{number.format(planned[metric] / divisor)}</td>
                      <td>{number.format(actual[metric] / divisor)}</td>
                      <td>{compliance.total}</td>
                      <td>{compliance.completed}</td>
                      <td>{compliance.skipped}</td>
                      <td>{compliance.modified}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </details>
        </>
      )}
    </section>
  );
}
