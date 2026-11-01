// DashboardSummaryCardContainer: owns the weekly KPIs and the sparkline
// data (Fase 1.4, issue #16, DW-005). The presentational
// DashboardSummaryCard only renders.

import type { TrainingLoadPoint } from '../../lib/api/dashboard';
import { useDashboardSummary } from './hooks/useDashboardSummary';
import { useTrainingLoad } from './hooks/useTrainingLoad';
import { DashboardSummaryCard } from './DashboardSummaryCard/DashboardSummaryCard';
import styles from './DashboardStates.module.css';

/** Number of days shown by the weekly sparkline. */
const SPARKLINE_DAYS = 7;

export interface DashboardSummaryCardContainerProps {
  /** Inclusive start date (YYYY-MM-DD). */
  from: string;
  /** Inclusive end date (YYYY-MM-DD). */
  to: string;
}

/**
 * lastDays returns the TSS of the last `count` days of the series.
 */
function lastDays(series: TrainingLoadPoint[], count: number): number[] {
  return series.slice(-count).map((point) => point.tss);
}

export function DashboardSummaryCardContainer({ from, to }: DashboardSummaryCardContainerProps) {
  const token = import.meta.env.VITE_AUTH_TOKEN || '';
  const summary = useDashboardSummary(token);
  const load = useTrainingLoad(token, from, to);

  if (summary.loading) {
    return (
      <div className={styles.loading} role="status" data-testid="dashboard-summary-loading">
        Cargando resumen semanal…
      </div>
    );
  }

  if (summary.error) {
    return (
      <div className={styles.error} role="alert" data-testid="dashboard-summary-error">
        {summary.error}
      </div>
    );
  }

  if (!summary.data) return null;

  return (
    <DashboardSummaryCard
      summary={summary.data}
      sparkline={lastDays(load.data?.series ?? [], SPARKLINE_DAYS)}
    />
  );
}
