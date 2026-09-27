// RiskZonesPanel: colour-coded list of risk zones across tracks.
// Issue 126. Pure presentational. Each risk gets a SeverityPill
// (high / medium by severity threshold) and a category label.

import { SeverityPill } from '../../../ui/SeverityPill';
import type { GpxRiskZone } from '../../../lib/api/types';
import styles from './RiskZonesPanel.module.css';

export interface RiskZonesPanelProps {
  /** Flat list of risk zones with the track they belong to. */
  zones: { trackName: string; zone: GpxRiskZone }[];
}

const CATEGORY_LABEL: Record<GpxRiskZone['category'], string> = {
  steep: 'Pendiente',
  technical: 'Técnico',
  exposure: 'Exposición',
};

export function RiskZonesPanel({ zones }: RiskZonesPanelProps) {
  if (zones.length === 0) {
    return (
      <section className={styles.empty} data-testid="risk-zones-empty" aria-label="Zonas de riesgo">
        No se detectaron zonas de riesgo en los tracks comparados.
      </section>
    );
  }
  return (
    <section className={styles.panel} aria-label="Zonas de riesgo" data-testid="risk-zones-panel">
      <h2 className={styles.title}>Zonas de riesgo</h2>
      <ul className={styles.list}>
        {zones.map(({ trackName, zone }, i) => (
          <li key={i} className={styles.item}>
            <div className={styles.itemHeader}>
              <SeverityPill level={zone.severity >= 0.7 ? 'high' : 'medium'} />
              <span className={styles.category}>{CATEGORY_LABEL[zone.category]}</span>
            </div>
            <p className={styles.meta}>
              <strong>{trackName}</strong> · tramo {zone.start_idx}–{zone.end_idx} · severidad{' '}
              {zone.severity.toFixed(2)}
            </p>
          </li>
        ))}
      </ul>
    </section>
  );
}
