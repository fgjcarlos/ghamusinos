// RecoveryCard: a single recovery zone (descent after a muro) entry
// inside RouteRecovery. Pure presentational; only renders the
// distance and the start/end indices.

import type { NormalizedRecoveryZone } from '../normalize';
import styles from './RouteRecovery.module.css';

export interface RecoveryCardProps {
  recovery: NormalizedRecoveryZone;
}

export function RecoveryCard({ recovery }: RecoveryCardProps) {
  return (
    <li className={styles.item} data-testid="recovery-card">
      <header className={styles.itemHeader}>
        <span className={styles.itemIndex}>
          {recovery.start_idx}–{recovery.end_idx}
        </span>
      </header>
      <dl className={styles.stats}>
        <div>
          <dt>Distancia</dt>
          <dd>{(recovery.distance_m / 1000).toFixed(2)} km</dd>
        </div>
      </dl>
    </li>
  );
}
