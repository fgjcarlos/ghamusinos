// Carrera tab: pace presets, estimated time, km milestones and nutrition.

import { useMemo } from 'react';
import { kmMarkers, nutritionPoints, PACE_PRESETS } from '../model/raceDay';
import { dPlusM } from '../model/trackCoords';
import type { TrackPoint } from '../model/types';
import { formatDuration, formatPace } from './format';
import styles from './RacePlanSection.module.css';

export interface RaceDayPanelProps {
  points: TrackPoint[];
  pace: number;
  totalKm: number;
  estimatedTimeMin: number;
  onPaceChange: (pace: number) => void;
}

export function RaceDayPanel({
  points,
  pace,
  totalKm,
  estimatedTimeMin,
  onPaceChange,
}: RaceDayPanelProps) {
  const markers = useMemo(() => kmMarkers(points), [points]);
  const nutrition = useMemo(() => nutritionPoints(points), [points]);
  const gain = useMemo(() => dPlusM(points), [points]);

  return (
    <div className={styles.stack}>
      <h3 className={styles.heading}>Tiempo estimado: {formatDuration(estimatedTimeMin)}</h3>
      <p className={styles.meta}>
        Distancia total: {totalKm.toFixed(2)} km · Desnivel positivo: {Math.round(gain)} m
      </p>

      <div className={styles.paceGroup} role="group" aria-label="Ritmo base">
        {PACE_PRESETS.map((preset) => {
          const selected = preset.paceMinPerKm === pace;
          return (
            <button
              key={preset.id}
              type="button"
              aria-pressed={selected}
              className={selected ? `${styles.chip} ${styles.chipActive}` : styles.chip}
              onClick={() => onPaceChange(preset.paceMinPerKm)}
            >
              {preset.label} · {formatPace(preset.paceMinPerKm)}
            </button>
          );
        })}
      </div>

      <table className={styles.table}>
        <caption className={styles.caption}>Hitos por kilómetro</caption>
        <thead>
          <tr>
            <th scope="col">Km</th>
            <th scope="col">Desnivel acumulado</th>
          </tr>
        </thead>
        <tbody>
          {markers.map((marker) => (
            <tr key={`${marker.km}-${marker.index}`}>
              <td>{marker.km}</td>
              <td>{Math.round(marker.cumulativeGainM)} m</td>
            </tr>
          ))}
        </tbody>
      </table>

      <div>
        <h4 className={styles.subheading}>Puntos de nutrición</h4>
        {nutrition.length === 0 ? (
          <p className={styles.meta}>Sin puntos de nutrición para esta distancia.</p>
        ) : (
          <ul className={styles.list}>
            {nutrition.map((stop) => (
              <li key={`${stop.km}-${stop.index}`}>
                Km {stop.km} · {stop.kind === 'recovery' ? 'Recuperación' : 'Planificado'}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
