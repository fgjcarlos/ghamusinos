// PeriodSummary: 4-week totals calculated client-side over the loaded
// page of activities. Issue 158. Pure presentational.
//
// Honest disclosure: text says "de lo mostrado en esta página" because
// totals are computed over the current page, not the backend's total.

import type { Activity, PgTypeNumeric } from '../../../lib/api/types';
import styles from './PeriodSummary.module.css';

function pgNumericToNumber(n: PgTypeNumeric | null): number | null {
  if (!n || !n.valid) return null;
  return Number(BigInt(n.bigint)) * Math.pow(10, -n.exp) * n.sign;
}

export interface PeriodSummaryProps {
  activities: Activity[];
}

export function PeriodSummary({ activities }: PeriodSummaryProps) {
  const count = activities.length;
  const totalDistance = activities.reduce<number | null>((acc, a) => {
    const v = pgNumericToNumber(a.distance_meters);
    if (v === null) return acc;
    if (acc === null) return v;
    return acc + v;
  }, null);
  const totalMoving = activities.reduce((acc, a) => acc + a.moving_seconds, 0);

  return (
    <section className={styles.summary} aria-label="Resumen de lo mostrado">
      <header className={styles.header}>
        <h2 className={styles.title}>Últimas 4 semanas</h2>
        <span className={styles.note}>de lo mostrado en esta página</span>
      </header>
      <div className={styles.tiles}>
        <div className={styles.tile}>
          <span className={styles.tileLabel}>Actividades</span>
          <span className={styles.tileValue}>{count}</span>
        </div>
        {totalDistance !== null && (
          <div className={styles.tile}>
            <span className={styles.tileLabel}>Distancia</span>
            <span className={styles.tileValue}>{(totalDistance / 1000).toFixed(1)} km</span>
          </div>
        )}
        <div className={styles.tile}>
          <span className={styles.tileLabel}>Tiempo en movimiento</span>
          <span className={styles.tileValue}>
            {Math.floor(totalMoving / 3600)}h {Math.floor((totalMoving % 3600) / 60)}m
          </span>
        </div>
      </div>
    </section>
  );
}
