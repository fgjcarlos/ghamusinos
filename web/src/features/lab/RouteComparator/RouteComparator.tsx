// RouteComparator: page-level composition for the route comparator.
// Issue 126. Pure presentational — receives the normalized
// CompareResponse from ComparisonMode (or any container) and
// composes the multi-track map, the diff table, the risk zones
// panel and the elevation profile overlay.
//
// This is a temporary stub: the full sub-components (ComparisonMap,
// MetricsDiffTable, RiskZonesPanel, ComparisonElevationProfile) are
// written in the next commits of this PR.

import type { CompareResponse } from '../../../lib/api/types';
import styles from './RouteComparator.module.css';

export interface RouteComparatorProps {
  data: CompareResponse;
}

export function RouteComparator({ data }: RouteComparatorProps) {
  return (
    <article
      className={styles.comparator}
      data-testid="route-comparator"
      aria-label="Comparador de rutas"
    >
      <header className={styles.header}>
        <h1 className={styles.title}>Comparador de rutas</h1>
        <p className={styles.meta}>{data.tracks.length} tracks comparados</p>
      </header>
      <section className={styles.body} data-testid="route-comparator-body">
        {/* Placeholder for sub-components (ComparisonMap, MetricsDiffTable,
            RiskZonesPanel, ComparisonElevationProfile) — full
            implementations land in the next commits of this PR. */}
        <div className={styles.placeholder} data-testid="route-comparator-placeholder">
          <p>
            Comparación cargada con {data.tracks.length} tracks. Las sub-vistas (mapa, tabla,
            perfil, riesgos) se montan en commits separados de este mismo PR.
          </p>
        </div>
      </section>
    </article>
  );
}
