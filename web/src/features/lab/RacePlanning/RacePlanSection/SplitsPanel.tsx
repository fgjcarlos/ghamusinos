// Splits tab: per-kilometre strategic splits with an adjustable fatigue factor.

import { useId, useMemo, useState } from 'react';
import { DEFAULT_FATIGUE_FACTOR } from '../model/raceDay';
import { generateStrategicSplits } from '../model/splits';
import type { TrackPoint } from '../model/types';
import { formatDuration, formatPace } from './format';
import styles from './RacePlanSection.module.css';

export interface SplitsPanelProps {
  points: TrackPoint[];
  pace: number;
}

export function SplitsPanel({ points, pace }: SplitsPanelProps) {
  const inputId = useId();
  const [fatigueInput, setFatigueInput] = useState(String(DEFAULT_FATIGUE_FACTOR));

  const parsedFatigue = Number(fatigueInput);
  const fatigue = Number.isFinite(parsedFatigue)
    ? Math.min(0.2, Math.max(0, parsedFatigue))
    : DEFAULT_FATIGUE_FACTOR;

  const rows = useMemo(
    () => generateStrategicSplits(points, pace, fatigue),
    [points, pace, fatigue],
  );

  return (
    <div className={styles.stack}>
      <div className={styles.field}>
        <label htmlFor={inputId} className={styles.label}>
          Factor de fatiga
        </label>
        <input
          id={inputId}
          type="number"
          min={0}
          max={0.2}
          step={0.01}
          value={fatigueInput}
          onChange={(event) => setFatigueInput(event.target.value)}
          className={styles.input}
        />
      </div>

      {rows.length === 0 ? (
        <p className={styles.empty}>Este track no alcanza el primer kilómetro.</p>
      ) : (
        <table className={styles.table}>
          <caption className={styles.caption}>Splits estratégicos</caption>
          <thead>
            <tr>
              <th scope="col">Km</th>
              <th scope="col">Parcial</th>
              <th scope="col">Acumulado</th>
              <th scope="col">Desnivel</th>
              <th scope="col">Ritmo</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.km}>
                <td>{row.km}</td>
                <td>{formatDuration(row.splitTimeMin)}</td>
                <td>{formatDuration(row.cumulativeTimeMin)}</td>
                <td>{row.gainM} m</td>
                <td>{formatPace(row.paceMinPerKm)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
