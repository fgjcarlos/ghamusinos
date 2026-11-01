// TrainingLoadChartContainer: owns the CTL/ATL/TSB series
// (Fase 1.4, issue #16, DW-004). The presentational TrainingLoadChart
// only renders.

import { useTrainingLoad } from './hooks/useTrainingLoad';
import { TrainingLoadChart } from './TrainingLoadChart/TrainingLoadChart';
import styles from './DashboardStates.module.css';

export interface TrainingLoadChartContainerProps {
  /** Inclusive start date (YYYY-MM-DD). */
  from: string;
  /** Inclusive end date (YYYY-MM-DD). */
  to: string;
}

export function TrainingLoadChartContainer({ from, to }: TrainingLoadChartContainerProps) {
  const token = import.meta.env.VITE_AUTH_TOKEN || '';
  const { data, loading, error } = useTrainingLoad(token, from, to);

  if (loading) {
    return (
      <div className={styles.loading} role="status" data-testid="training-load-loading">
        Cargando carga de entrenamiento…
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.error} role="alert" data-testid="training-load-error">
        {error}
      </div>
    );
  }

  return <TrainingLoadChart series={data?.series ?? []} />;
}
