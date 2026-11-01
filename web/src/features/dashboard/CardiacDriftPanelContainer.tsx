// CardiacDriftPanelContainer: owns the cardiac drift of the window
// (Fase 1.4, issue #16, DW-006). The panel is only mounted when the
// backend reports `available: true`; otherwise the parent shows the
// "needs HR streams" notice.

import { useCardiacDrift } from './hooks/useCardiacDrift';
import { CardiacDriftPanel } from './CardiacDriftPanel/CardiacDriftPanel';
import styles from './DashboardStates.module.css';

export interface CardiacDriftPanelContainerProps {
  /** Inclusive start date (YYYY-MM-DD). */
  from: string;
  /** Inclusive end date (YYYY-MM-DD). */
  to: string;
}

export function CardiacDriftPanelContainer({ from, to }: CardiacDriftPanelContainerProps) {
  const token = import.meta.env.VITE_AUTH_TOKEN || '';
  const { data, loading, error } = useCardiacDrift(token, from, to);

  if (loading) {
    return (
      <div className={styles.loading} role="status" data-testid="cardiac-drift-loading">
        Calculando deriva cardíaca…
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.error} role="alert" data-testid="cardiac-drift-error">
        {error}
      </div>
    );
  }

  if (!data?.available) {
    return (
      <p className={styles.notice} data-testid="cardiac-drift-empty">
        necesita streams HR para calcular
      </p>
    );
  }

  return <CardiacDriftPanel activities={data.activities} />;
}
