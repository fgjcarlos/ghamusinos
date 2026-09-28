// HRZonePreview: shows the 5 HR zones derived from hr_max. Issue 159.
// Thresholds must match internal/jobs/streams.go:135 — 60 / 70 / 80
// / 90 % — so what the user sees here matches what the backend
// computes at Strava import time.
//
// Pure presentational: takes hr_max, returns the zone structure;
// the parent owns fetch + saving state.

import { useMemo } from 'react';
import styles from './HRZonePreview.module.css';

export interface HRZone {
  label: string;
  threshold_pct: number;
  low_pct: number;
  high_pct: number;
}

export interface HRZonePreviewProps {
  hr_max: number | null;
}

function pct(n: number): string {
  return `${n.toFixed(0)}%`;
}

export function HRZonePreview({ hr_max }: HRZonePreviewProps) {
  const zones = useMemo<HRZone[]>(() => {
    if (!hr_max || hr_max <= 0) return [];
    // The 5 zones follow the standard convention: Z1 < 60%, Z2 60–70%,
    // Z3 70–80%, Z4 80–90%, Z5 ≥ 90%. Boundaries match
    // calcHRZonesScaled (`hr < threshold`), so the upper bound is
    // exclusive and the lower bound inclusive.
    const upperBounds = [60, 70, 80, 90, 100];
    return upperBounds.map((upper, i) => ({
      label: `Z${i + 1}`,
      threshold_pct: upper,
      low_pct: i === 0 ? 0 : upperBounds[i - 1],
      high_pct: upper,
    }));
  }, [hr_max]);

  if (!hr_max || hr_max <= 0) {
    return (
      <section
        className={styles.empty}
        data-testid="hr-zones-empty"
        aria-label="Vista previa de zonas de frecuencia cardíaca"
      >
        <p>
          Introduce tu FC máxima para ver tus zonas. Las zonas se calculan con los mismos umbrales
          que usa el backend al importar actividades (60 / 70 / 80 / 90 %).
        </p>
      </section>
    );
  }

  return (
    <section
      className={styles.preview}
      data-testid="hr-zones-preview"
      aria-label="Zonas de frecuencia cardíaca"
    >
      <header className={styles.header}>
        <h2 className={styles.title}>Zonas de FC</h2>
        <p className={styles.subtitle}>
          Calculadas al 60 / 70 / 80 / 90 % de tu FC máxima ({hr_max} bpm)
        </p>
      </header>
      <ol className={styles.zones}>
        {zones.map((z) => (
          <li key={z.label} className={styles.zone} data-zone={z.label}>
            <span className={styles.zoneLabel}>{z.label}</span>
            <span className={styles.zoneRange}>
              {pct(z.low_pct)}–{pct(z.high_pct)}
            </span>
            <span className={styles.zoneBpm}>
              {hr_max > 0
                ? `${Math.round((hr_max * z.low_pct) / 100)}–${Math.round(
                    (hr_max * z.high_pct) / 100,
                  )} bpm`
                : ''}
            </span>
          </li>
        ))}
      </ol>
    </section>
  );
}
