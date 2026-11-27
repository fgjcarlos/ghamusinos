// DailyLoadChart: the daily TSS of a report period. The load report
// only carries the daily TSS (no per-day CTL/ATL), so it is a small bar
// chart of its own instead of the dashboard's CTL/ATL/TSB line chart.
//
// ECharts paints on a canvas; the legend and the frame are real DOM text
// so they stay readable and assertable in jsdom.

import { useMemo } from 'react';
import type { DailyLoad } from '../../lib/api/reports';
import { resolveToken } from '../dashboard/chartTokens';
import type { EChartsCoreOption } from '../dashboard/echartsSetup';
import { useEChart } from '../dashboard/useEChart';
import styles from './Reports.module.css';

function buildDailyLoadOption(daily: DailyLoad[]): EChartsCoreOption {
  const accent = resolveToken('--gh-accent');
  const text = resolveToken('--gh-text-2');
  const line = resolveToken('--gh-line');
  return {
    grid: { left: 40, right: 16, top: 16, bottom: 28 },
    tooltip: { trigger: 'axis' },
    legend: { show: false },
    title: { show: false },
    xAxis: {
      type: 'category',
      data: daily.map((day) => day.date),
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
        name: 'TSS',
        type: 'bar',
        data: daily.map((day) => day.tss),
        itemStyle: { color: accent },
      },
    ],
  };
}

export function DailyLoadChart({ daily }: { daily: DailyLoad[] }) {
  const option = useMemo(() => buildDailyLoadOption(daily), [daily]);
  const containerRef = useEChart(option);
  return (
    <section aria-labelledby="daily-load-title">
      <h3 id="daily-load-title">Carga diaria</h3>
      <p className={styles.metadata} data-testid="daily-load-legend">
        TSS por día
      </p>
      <div ref={containerRef} className={styles.chart} data-testid="daily-load-chart" />
      {daily.length === 0 && <p>Sin días registrados en el periodo.</p>}
    </section>
  );
}
