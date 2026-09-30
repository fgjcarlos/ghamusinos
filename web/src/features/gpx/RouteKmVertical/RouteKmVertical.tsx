// RouteKmVertical: singleton panel that surfaces the km_vertical
// detection result (issue 15, Fase 1.3). When the track has no
// qualifying ascent (gain_m continuous >= 50 m), the component
// renders an empty-state message instead of the metrics card.

import type { NormalizedKmVertical } from '../normalize';
import styles from './RouteKmVertical.module.css';

export interface RouteKmVerticalProps {
  /** Singleton value: null when the track has no qualifying km_vertical. */
  data: NormalizedKmVertical | null;
}

export function RouteKmVertical({ data }: RouteKmVerticalProps) {
  if (data === null) {
    return (
      <section
        className={styles.empty}
        data-testid="route-km-vertical-empty"
        aria-label="Km vertical del track"
      >
        <h2 className={styles.title}>Km Vertical</h2>
        <p>Este track no tiene un tramo de subida sostenida ≥ 50 m continuo.</p>
      </section>
    );
  }
  return (
    <section className={styles.card} aria-label="Km vertical del track">
      <header className={styles.header}>
        <h2 className={styles.title}>Km Vertical</h2>
      </header>
      <dl className={styles.stats} data-testid="route-km-vertical">
        <div>
          <dt>Desnivel</dt>
          <dd>{data.gain_m.toFixed(0)} m</dd>
        </div>
        <div>
          <dt>Distancia</dt>
          <dd>{(data.distance_m / 1000).toFixed(2)} km</dd>
        </div>
      </dl>
    </section>
  );
}
