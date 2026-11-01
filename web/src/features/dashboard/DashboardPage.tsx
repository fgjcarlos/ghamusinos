// DashboardPage: the /dashboard screen (Fase 1.4, issue #16, DW-001..006).
//
// The page owns the selected period and the from/to window derived from
// it; every feature below is a container that owns its own data.

import { useMemo, useState } from 'react';
import { useDashboardSummary } from './hooks/useDashboardSummary';
import { DEFAULT_PERIOD, periodRange, type DashboardPeriod } from './period';
import { PeriodSelector } from './PeriodSelector/PeriodSelector';
import { DashboardSummaryCardContainer } from './DashboardSummaryCardContainer';
import { TrainingLoadChartContainer } from './TrainingLoadChartContainer';
import { HRZonesChartContainer } from './HRZonesChartContainer';
import { CardiacDriftPanelContainer } from './CardiacDriftPanelContainer';
import styles from './DashboardPage.module.css';

export function DashboardPage() {
  const [period, setPeriod] = useState<DashboardPeriod>(DEFAULT_PERIOD);
  const { from, to } = useMemo(() => periodRange(period), [period]);
  const token = import.meta.env.VITE_AUTH_TOKEN || '';
  const summary = useDashboardSummary(token);

  // With no activity in the current week there is nothing to chart:
  // the summary card renders the empty state and the charts stay
  // unmounted. The charts also wait for the summary to resolve, so a
  // first visit does not fire three requests that are unmounted again
  // a moment later (and the empty state does not flash the charts).
  const summaryResolved = !summary.loading && summary.error === null;
  const withoutData = summary.data !== null && summary.data.weekly_activities_count === 0;
  const showCharts = summaryResolved && !withoutData;

  return (
    <section className={styles.page} aria-label="Rendimiento">
      <header className={styles.header}>
        <h1 className={styles.title}>Rendimiento</h1>
        <PeriodSelector value={period} onChange={setPeriod} />
      </header>

      <DashboardSummaryCardContainer from={from} to={to} />

      {showCharts ? (
        <>
          <TrainingLoadChartContainer from={from} to={to} />
          <HRZonesChartContainer from={from} to={to} />
          <CardiacDriftPanelContainer from={from} to={to} />
        </>
      ) : null}
    </section>
  );
}
