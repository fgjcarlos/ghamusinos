// MuroCard: a single muro (short steep ascent) entry inside RouteMuros.
// Pure presentational; receives a NormalizedMuro and renders the
// numeric stats with a small severity icon when the slope is steep.

import type { NormalizedMuro } from '../normalize';
import styles from './RouteMuros.module.css';

export interface MuroCardProps {
  muro: NormalizedMuro;
}

export function MuroCard({ muro }: MuroCardProps) {
  // Severity icon threshold: 12% avg slope or higher is "hard".
  const isHard = muro.avg_slope_pct >= 12;
  return (
    <li className={styles.item} data-testid="muro-card">
      <header className={styles.itemHeader}>
        <span className={styles.itemIndex}>
          {muro.start_idx}–{muro.end_idx}
        </span>
        {isHard && <span className={styles.hardBadge}>Muro duro</span>}
      </header>
      <dl className={styles.stats}>
        <div>
          <dt>Desnivel</dt>
          <dd>{muro.gain_m.toFixed(0)} m</dd>
        </div>
        <div>
          <dt>Distancia</dt>
          <dd>{(muro.distance_m / 1000).toFixed(2)} km</dd>
        </div>
        <div>
          <dt>Pendiente</dt>
          <dd>{muro.avg_slope_pct.toFixed(1)}%</dd>
        </div>
      </dl>
    </li>
  );
}
