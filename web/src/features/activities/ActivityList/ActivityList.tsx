// ActivityList: page-level container that shows PeriodSummary at top,
// one ActivityRow per activity, and Prev/Next pagination controls.
// Issue 158. Pure presentational; pagination state lives in the
// parent container (ActivityListContainer).

import type { Activity } from '../../../lib/api/types';
import { ActivityRow } from '../ActivityRow/ActivityRow';
import { PeriodSummary } from '../PeriodSummary/PeriodSummary';
import styles from './ActivityList.module.css';

export interface ActivityListProps {
  activities: Activity[];
  page: number;
  hasNext: boolean;
  onPageChange: (page: number) => void;
}

export function ActivityList({ activities, page, hasNext, onPageChange }: ActivityListProps) {
  return (
    <section className={styles.list} aria-label="Mis actividades">
      <header className={styles.header}>
        <h1 className={styles.title}>Mis actividades</h1>
      </header>

      {activities.length > 0 ? (
        <>
          <PeriodSummary activities={activities} />
          <ul className={styles.rows}>
            {activities.map((a) => (
              <li key={a.external_id} className={styles.rowItem}>
                <ActivityRow activity={a} />
              </li>
            ))}
          </ul>
        </>
      ) : (
        <div className={styles.empty}>
          <p>No hay actividades todavía en esta página.</p>
          <p className={styles.emptyHint}>
            Sin actividades sincronizadas. Conecta Strava desde tu perfil para empezar.
          </p>
        </div>
      )}

      <nav className={styles.pager} aria-label="Paginación">
        <button
          type="button"
          className={styles.pagerBtn}
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          ← Anterior
        </button>
        <span className={styles.pagerInfo}>Página {page}</span>
        <button
          type="button"
          className={styles.pagerBtn}
          disabled={!hasNext}
          onClick={() => onPageChange(page + 1)}
        >
          Siguiente →
        </button>
      </nav>
    </section>
  );
}
