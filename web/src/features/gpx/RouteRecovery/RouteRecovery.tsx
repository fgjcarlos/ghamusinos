// RouteRecovery: list of recovery zones (descents after a muro)
// detected by FindRecoveryZones. Issue 15, Fase 1.3. Each zone
// renders as a RecoveryCard with distance only — recovery has no
// gain_m or avg_slope_pct.

import { RecoveryCard } from './RecoveryCard';
import type { NormalizedRecoveryZone } from '../normalize';
import styles from './RouteRecovery.module.css';

export interface RouteRecoveryProps {
  data: NormalizedRecoveryZone[];
}

export function RouteRecovery({ data }: RouteRecoveryProps) {
  if (data.length === 0) {
    return (
      <section
        className={styles.empty}
        data-testid="route-recovery-empty"
        aria-label="Zonas de recuperación del track"
      >
        <h2 className={styles.title}>Recovery zones</h2>
        <p>No se detectaron zonas de recuperación en este track.</p>
      </section>
    );
  }
  return (
    <section
      className={styles.recovery}
      aria-label="Zonas de recuperación del track"
      data-testid="route-recovery"
    >
      <h2 className={styles.title}>Recovery zones ({data.length})</h2>
      <ul className={styles.list}>
        {data.map((r, i) => (
          <RecoveryCard key={`${r.start_idx}-${i}`} recovery={r} />
        ))}
      </ul>
    </section>
  );
}
