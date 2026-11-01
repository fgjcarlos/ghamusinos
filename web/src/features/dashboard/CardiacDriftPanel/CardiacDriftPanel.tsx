// CardiacDriftPanel: per-activity cardiac drift.
// Fases 1.4 (issue #16, DW-006). Pure presentational — the container
// decides whether the panel is mounted at all (the backend reports
// `available: false` when no activity carries enough HR samples).

import type { CardiacDriftActivity } from '../../../lib/api/dashboard';
import styles from './CardiacDriftPanel.module.css';

export interface CardiacDriftPanelProps {
  /** One entry per activity with a computable drift. */
  activities: CardiacDriftActivity[];
}

/**
 * averageDrift returns the mean drift of the activities, or null when
 * there is nothing to average.
 */
function averageDrift(activities: CardiacDriftActivity[]): number | null {
  if (activities.length === 0) return null;
  const total = activities.reduce((sum, activity) => sum + activity.drift_pct, 0);
  return total / activities.length;
}

export function CardiacDriftPanel({ activities }: CardiacDriftPanelProps) {
  const average = averageDrift(activities);

  return (
    <section className={styles.card} data-testid="cardiac-drift-panel" aria-label="Deriva cardíaca">
      <header className={styles.header}>
        <h2 className={styles.title}>Deriva cardíaca</h2>
        {average !== null ? (
          <span className={styles.average} data-testid="cardiac-drift-average">
            media {average.toFixed(1)}%
          </span>
        ) : null}
      </header>
      <ul className={styles.list}>
        {activities.map((activity) => (
          <li key={activity.activity_id} className={styles.row}>
            <span className={styles.activity}>{activity.activity_id}</span>
            <span className={styles.drift}>{activity.drift_pct.toFixed(1)}%</span>
            <span className={styles.samples}>{activity.sample_count} muestras</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
