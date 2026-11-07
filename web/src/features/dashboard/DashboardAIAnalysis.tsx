// DashboardAIAnalysis: los controles de análisis con IA del panel de
// rendimiento (fase 1.5, issue #17, T8 — S3/S4/S7).
//
// La pantalla ya tiene el panel determinista (KPI, carga, zonas y
// deriva). Aquí sólo se añaden los dos análisis bajo demanda: el
// semanal y el de carga. Los dos_controls leen el estado global antes de
// pedir nada, así que con la IA apagada o sin consentimiento no sale
// ninguna petición y se explica dónde resolverlo.

import { AIAnalysisControl } from '../ai/AIAnalysisControl';
import { loadWindow, weeklyWindow } from './aiWindow';
import { PERIOD_OPTIONS, type DashboardPeriod } from './period';
import styles from './DashboardAIAnalysis.module.css';

export interface DashboardAIAnalysisProps {
  /** Periodo elegido en el panel. */
  period: DashboardPeriod;
  /** Último día de la ventana del panel (YYYY-MM-DD). */
  to: string;
}

export function DashboardAIAnalysis({ period, to }: DashboardAIAnalysisProps) {
  const days = PERIOD_OPTIONS.find((option) => option.id === period)?.days ?? 30;
  const weekly = weeklyWindow(to);
  const load = loadWindow(to, days);

  return (
    <section className={styles.section} aria-label="Análisis con IA">
      <header className={styles.header}>
        <h2 className={styles.title}>Análisis con IA</h2>
        <p className={styles.note}>
          Se piden bajo demanda: la IA sólo recibe métricas agregadas y sólo si has aceptado el
          envío.
        </p>
      </header>
      <div className={styles.grid}>
        <AIAnalysisControl
          kind="weekly"
          title={`Análisis semanal · últimos ${weekly.windowDays} días`}
          windowStart={weekly.windowStart}
          windowDays={weekly.windowDays}
        />
        <AIAnalysisControl
          kind="load"
          title={`Análisis de carga · últimos ${load.windowDays} días`}
          windowStart={load.windowStart}
          windowDays={load.windowDays}
        />
      </div>
    </section>
  );
}
