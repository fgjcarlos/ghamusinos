// RouteMuros: list of muros (short, steep climbs detected by
// FindMuros). Issue 15, Fase 1.3. Each muro is rendered as a
// MuroCard with the numeric metrics (gain_m, distance_m, avg_slope_pct)
// and a small severity icon when avg_slope_pct >= 12.

import { MuroCard } from './MuroCard';
import type { NormalizedMuro } from '../normalize';
import styles from './RouteMuros.module.css';

export interface RouteMurosProps {
  data: NormalizedMuro[];
}

export function RouteMuros({ data }: RouteMurosProps) {
  if (data.length === 0) {
    return (
      <section
        className={styles.empty}
        data-testid="route-muros-empty"
        aria-label="Muros del track"
      >
        <h2 className={styles.title}>Muros</h2>
        <p>No se detectaron muros relevantes en este track.</p>
      </section>
    );
  }
  return (
    <section className={styles.muros} aria-label="Muros del track">
      <h2 className={styles.title}>Muros ({data.length})</h2>
      <ul className={styles.list}>
        {data.map((m, i) => (
          <MuroCard key={`${m.start_idx}-${i}`} muro={m} />
        ))}
      </ul>
    </section>
  );
}
