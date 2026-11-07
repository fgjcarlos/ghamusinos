// Cortes tab: cutoff calculator with per-cutoff safe/unsafe evaluation.

import { useId, useMemo, useState } from 'react';
import { evaluateCutoffs } from '../model/splits';
import type { CutoffInput, TrackPoint } from '../model/types';
import { formatDuration, formatPace, formatSignedDuration } from './format';
import styles from './RacePlanSection.module.css';

export interface CutoffsPanelProps {
  points: TrackPoint[];
  pace: number;
}

const INVALID_MESSAGE = 'Introduce un kilómetro y un tiempo límite válidos.';

export function CutoffsPanel({ points, pace }: CutoffsPanelProps) {
  const kmId = useId();
  const limitId = useId();
  const [cutoffs, setCutoffs] = useState<CutoffInput[]>([]);
  const [kmInput, setKmInput] = useState('');
  const [limitInput, setLimitInput] = useState('');
  const [message, setMessage] = useState<string | null>(null);

  const results = useMemo(() => evaluateCutoffs(points, pace, cutoffs), [points, pace, cutoffs]);

  function addCutoff() {
    const km = Number(kmInput);
    const timeLimitMin = Number(limitInput);
    if (
      kmInput.trim() === '' ||
      limitInput.trim() === '' ||
      !Number.isFinite(km) ||
      km <= 0 ||
      !Number.isFinite(timeLimitMin) ||
      timeLimitMin <= 0
    ) {
      setMessage(INVALID_MESSAGE);
      return;
    }
    setMessage(null);
    setCutoffs((previous) => [...previous, { km, timeLimitMin }]);
    setKmInput('');
    setLimitInput('');
  }

  return (
    <div className={styles.stack}>
      <div className={styles.inlineFields}>
        <div className={styles.field}>
          <label htmlFor={kmId} className={styles.label}>
            Kilómetro del corte
          </label>
          <input
            id={kmId}
            type="number"
            min={0}
            step={0.1}
            value={kmInput}
            onChange={(event) => setKmInput(event.target.value)}
            className={styles.input}
          />
        </div>
        <div className={styles.field}>
          <label htmlFor={limitId} className={styles.label}>
            Tiempo límite (min)
          </label>
          <input
            id={limitId}
            type="number"
            min={0}
            step={1}
            value={limitInput}
            onChange={(event) => setLimitInput(event.target.value)}
            className={styles.input}
          />
        </div>
        <button type="button" className={styles.action} onClick={addCutoff}>
          Añadir corte
        </button>
      </div>

      {message !== null && (
        <p role="alert" className={styles.failed}>
          {message}
        </p>
      )}

      {results.length === 0 ? (
        <p className={styles.meta}>Todavía no hay cortes.</p>
      ) : (
        <table className={styles.table}>
          <caption className={styles.caption}>Evaluación de cortes</caption>
          <thead>
            <tr>
              <th scope="col">Km</th>
              <th scope="col">Llegada</th>
              <th scope="col">Límite</th>
              <th scope="col">Margen</th>
              <th scope="col">Estado</th>
              <th scope="col">Ritmo requerido</th>
            </tr>
          </thead>
          <tbody>
            {results.map((result) => (
              <tr key={`${result.km}-${result.timeLimitMin}`}>
                <td>{result.km}</td>
                <td>{formatDuration(result.estimatedMin)}</td>
                <td>{formatDuration(result.timeLimitMin)}</td>
                <td>{formatSignedDuration(result.marginMin)}</td>
                <td>
                  <span className={result.isSafe ? styles.safe : styles.unsafe}>
                    {result.isSafe ? 'Seguro' : 'Riesgo'}
                  </span>
                </td>
                <td>{formatPace(result.requiredPaceMinPerKm)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
