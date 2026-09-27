// RouteRisks: list of risk zones (steep / technical / exposure).
// Issue 157. Each risk is tagged with a SeverityPill based on
// severity (>= 0.7 → high, < 0.7 → medium).

import { SeverityPill } from '../../../ui/SeverityPill';
import type { NormalizedRiskZone } from '../normalize';
import styles from './RouteRisks.module.css';

export interface RouteRisksProps {
  risks: NormalizedRiskZone[];
}

const CATEGORY_LABEL: Record<NormalizedRiskZone['category'], string> = {
  steep: 'Pendiente',
  technical: 'Técnico',
  exposure: 'Exposición',
};

export function RouteRisks({ risks }: RouteRisksProps) {
  if (risks.length === 0) {
    return (
      <section className={styles.empty} data-testid="route-risks-empty">
        <h2 className={styles.title}>Zonas de riesgo</h2>
        <p>No se detectaron zonas de riesgo en este track.</p>
      </section>
    );
  }
  return (
    <section className={styles.risks} aria-label="Zonas de riesgo del track">
      <h2 className={styles.title}>Zonas de riesgo ({risks.length})</h2>
      <ul className={styles.list}>
        {risks.map((r, i) => (
          <li key={i} className={styles.item} data-testid="risk">
            <div className={styles.itemHeader}>
              <SeverityPill level={r.severity >= 0.7 ? 'high' : 'medium'} />
              <span className={styles.categoryLabel}>{CATEGORY_LABEL[r.category]}</span>
            </div>
            <p className={styles.meta}>
              Tramo {r.start_idx}–{r.end_idx} · severidad {r.severity.toFixed(2)}
            </p>
          </li>
        ))}
      </ul>
    </section>
  );
}
