// Solar tab: sun/shade/night exposure for an explicit race start.

import { useId, useState } from 'react';
import { calculateSolarExposure } from '../model/solar';
import type { SolarSegment, TrackPoint } from '../model/types';
import { defaultRaceDateTimeLocal, solarStatusLabel } from './format';
import styles from './RacePlanSection.module.css';

export interface SolarPanelProps {
  points: TrackPoint[];
  pace: number;
}

const INVALID_MESSAGE = 'Introduce una fecha y hora válidas.';

export function SolarPanel({ points, pace }: SolarPanelProps) {
  const inputId = useId();
  const [at, setAt] = useState(() => defaultRaceDateTimeLocal());
  const [segments, setSegments] = useState<SolarSegment[] | null>(null);
  const [validation, setValidation] = useState<string | null>(null);

  function handleCalculate() {
    const startAt = new Date(at);
    if (at.trim() === '' || Number.isNaN(startAt.getTime())) {
      setValidation(INVALID_MESSAGE);
      return;
    }
    setValidation(null);
    setSegments(calculateSolarExposure(points, startAt, pace));
  }

  return (
    <div className={styles.stack}>
      <div className={styles.inlineFields}>
        <div className={styles.field}>
          <label htmlFor={inputId} className={styles.label}>
            Fecha y hora de salida
          </label>
          <input
            id={inputId}
            type="datetime-local"
            value={at}
            onChange={(event) => setAt(event.target.value)}
            className={styles.input}
          />
        </div>
        <button type="button" className={styles.action} onClick={handleCalculate}>
          Calcular exposición
        </button>
      </div>

      {validation !== null && (
        <p role="alert" className={styles.failed}>
          {validation}
        </p>
      )}

      {segments !== null && segments.length === 0 && (
        <p className={styles.empty}>Sin segmentos de exposición.</p>
      )}

      {segments !== null && segments.length > 0 && (
        <ul className={styles.segmentList}>
          {segments.map((segment) => (
            <li
              key={`${segment.startKm}-${segment.endKm}-${segment.status}`}
              className={styles.segmentItem}
            >
              <span className={styles.badge}>{solarStatusLabel(segment.status)}</span>
              <span className={styles.mono}>
                {segment.startKm.toFixed(2)}–{segment.endKm.toFixed(2)} km
              </span>
              <span className={styles.meta}>Intensidad {Math.round(segment.intensity * 100)}%</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
