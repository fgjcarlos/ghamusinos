// ActivityList: page-level container that shows PeriodSummary at top,
// one ActivityRow per activity, and Prev/Next pagination controls.
// Issue 158. Mostly presentational; pagination state lives in the
// parent container (ActivityListContainer).
//
// Fase 1.5 (T8): cada actividad lleva un disparador para pedir su
// análisis con IA. El control (AIAnalysisControl) se monta bajo la
// actividad elegida y sólo pide el análisis si la IA está disponible y
// el usuario ha consentido (S3/S7).

import { useState } from 'react';
import type { Activity } from '../../../lib/api/types';
import { AIAnalysisControl } from '../../ai/AIAnalysisControl';
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
  // Actividad cuyo análisis con IA está abierto (null = ninguna).
  const [aiActivityId, setAiActivityId] = useState<string | null>(null);

  return (
    <section className={styles.list} aria-label="Mis actividades">
      <header className={styles.header}>
        <h1 className={styles.title}>Mis actividades</h1>
      </header>

      {activities.length > 0 ? (
        <>
          <PeriodSummary activities={activities} />
          <ul className={styles.rows}>
            {activities.map((a) => {
              const activityId = a.id.valid ? a.id.bytes : '';
              const aiOpen = activityId !== '' && activityId === aiActivityId;
              return (
                <li key={a.external_id} className={styles.rowItem}>
                  <div className={styles.rowWrap}>
                    <ActivityRow activity={a} />
                    {activityId !== '' ? (
                      <button
                        type="button"
                        className={styles.aiTrigger}
                        data-testid="activity-ai-trigger"
                        aria-expanded={aiOpen}
                        onClick={() => setAiActivityId(aiOpen ? null : activityId)}
                      >
                        Analizar con IA
                      </button>
                    ) : null}
                  </div>
                  {aiOpen ? (
                    <div className={styles.aiPanel}>
                      <AIAnalysisControl
                        kind="activity"
                        activityId={activityId}
                        title="Análisis de la actividad"
                        autoStart
                      />
                    </div>
                  ) : null}
                </li>
              );
            })}
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
