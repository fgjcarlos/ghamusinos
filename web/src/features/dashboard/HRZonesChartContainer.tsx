// HRZonesChartContainer: owns the minutes per heart-rate zone
// (Fase 1.4, issue #16, DW-004). The presentational HRZonesChart only
// renders.

import { useHRZones } from './hooks/useHRZones';
import { HRZonesChart } from './HRZonesChart/HRZonesChart';
import styles from './DashboardStates.module.css';

export interface HRZonesChartContainerProps {
  /** Inclusive start date (YYYY-MM-DD). */
  from: string;
  /** Inclusive end date (YYYY-MM-DD). */
  to: string;
}

export function HRZonesChartContainer({ from, to }: HRZonesChartContainerProps) {
  const token = import.meta.env.VITE_AUTH_TOKEN || '';
  const { data, loading, error } = useHRZones(token, from, to);

  if (loading) {
    return (
      <div className={styles.loading} role="status" data-testid="hr-zones-loading">
        Cargando zonas de frecuencia cardíaca…
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.error} role="alert" data-testid="hr-zones-error">
        {error}
      </div>
    );
  }

  return <HRZonesChart zones={data?.zones ?? []} degraded={data?.degraded ?? false} />;
}
