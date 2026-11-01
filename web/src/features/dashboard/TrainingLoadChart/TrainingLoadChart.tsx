// TrainingLoadChart: CTL / ATL / TSB over time.
// Fases 1.4 (issue #16, DW-004). Pure presentational — the container
// owns the data; this component only draws it.
//
// The chart is an ECharts line chart mounted on a canvas. The legend
// is rendered as real DOM text (CTL, ATL, TSB) so it stays readable
// with a screen reader and assertable in the tests.

import { useMemo } from 'react';
import type { TrainingLoadPoint } from '../../../lib/api/dashboard';
import { resolveToken } from '../chartTokens';
import type { EChartsCoreOption } from '../echartsSetup';
import { useEChart } from '../useEChart';
import styles from './TrainingLoadChart.module.css';

export interface TrainingLoadChartProps {
  /** One entry per day of the selected window. */
  series: TrainingLoadPoint[];
}

/**
 * buildTrainingLoadOption maps the series to the ECharts option.
 * Colours come from the design tokens, resolved at runtime.
 */
function buildTrainingLoadOption(series: TrainingLoadPoint[]): EChartsCoreOption {
  const accent = resolveToken('--gh-accent');
  const warn = resolveToken('--gh-warn');
  const risk = resolveToken('--gh-risk');
  const text = resolveToken('--gh-text-2');
  const line = resolveToken('--gh-line');

  return {
    grid: { left: 44, right: 16, top: 16, bottom: 28 },
    tooltip: { trigger: 'axis' },
    // The legend is rendered as DOM next to the chart.
    legend: { show: false },
    title: { show: false },
    xAxis: {
      type: 'category',
      data: series.map((point) => point.day),
      axisLabel: { color: text, hideOverlap: true },
      axisLine: { lineStyle: { color: line } },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: text },
      splitLine: { lineStyle: { color: line } },
    },
    series: [
      {
        name: 'CTL',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: series.map((point) => point.ctl),
        itemStyle: { color: accent },
        lineStyle: { color: accent },
      },
      {
        name: 'ATL',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: series.map((point) => point.atl),
        itemStyle: { color: warn },
        lineStyle: { color: warn },
      },
      {
        name: 'TSB',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: series.map((point) => point.tsb),
        itemStyle: { color: risk },
        lineStyle: { color: risk },
      },
    ],
  };
}

export function TrainingLoadChart({ series }: TrainingLoadChartProps) {
  const option = useMemo(() => buildTrainingLoadOption(series), [series]);
  const containerRef = useEChart(option);

  return (
    <section className={styles.card} aria-label="Carga de entrenamiento">
      <header className={styles.header}>
        <h2 className={styles.title}>Carga de entrenamiento</h2>
        <ul className={styles.legend} data-testid="training-load-legend">
          <li className={`${styles.legendItem} ${styles.legendCtl}`}>CTL</li>
          <li className={`${styles.legendItem} ${styles.legendAtl}`}>ATL</li>
          <li className={`${styles.legendItem} ${styles.legendTsb}`}>TSB</li>
        </ul>
      </header>
      <div ref={containerRef} className={styles.chart} data-testid="training-load-chart" />
    </section>
  );
}
