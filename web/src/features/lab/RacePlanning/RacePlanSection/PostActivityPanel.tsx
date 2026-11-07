// Post-actividad tab: compares the planned splits with a Strava activity.
//
// Nothing is fetched on mount. The user pastes an activity id from the Strava
// URL and presses `Cargar streams`; a 409 from the backend means the activity
// predates the `time` import, so the panel offers a re-sync action instead of
// a table (S8).

import { useId, useState } from 'react';
import { useActivityStreams } from '../hooks/useActivityStreams';
import { comparePlanVsReal, type ActivitySample } from '../model/postActivity';
import type { TrackPoint } from '../model/types';
import { formatDuration, formatPace, formatSignedDuration } from './format';
import styles from './RacePlanSection.module.css';

export interface PostActivityPanelProps {
  token: string;
  plannedPoints: TrackPoint[];
  pace: number;
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

/**
 * Pulls `time` (second offsets) and `latlng` (`[lat, lon]` pairs) out of the
 * opaque streams record. Anything that is not a finite number or a numeric
 * pair is ignored; `null` means the activity lacks usable streams.
 */
function extractSample(streams: Record<string, unknown>): ActivitySample | null {
  const rawTime = streams.time;
  const rawLatLng = streams.latlng;
  if (!Array.isArray(rawTime) || !Array.isArray(rawLatLng)) return null;

  const timeSeconds = rawTime.filter(isFiniteNumber);

  const latlng: Array<[number, number]> = [];
  for (const pair of rawLatLng) {
    if (!Array.isArray(pair) || pair.length < 2) continue;
    const [lat, lon] = pair;
    if (!isFiniteNumber(lat) || !isFiniteNumber(lon)) continue;
    latlng.push([lat, lon]);
  }

  if (timeSeconds.length === 0 || latlng.length === 0) return null;
  return { latlng, timeSeconds };
}

export function PostActivityPanel({ token, plannedPoints, pace }: PostActivityPanelProps) {
  const inputId = useId();
  const [activityId, setActivityId] = useState('');
  const { state, load, refresh } = useActivityStreams(token, activityId);

  const sample = state.kind === 'ready' ? extractSample(state.data.streams) : null;
  const rows = sample === null ? [] : comparePlanVsReal(plannedPoints, sample, pace);

  return (
    <div className={styles.stack}>
      <div className={styles.inlineFields}>
        <div className={styles.field}>
          <label htmlFor={inputId} className={styles.label}>
            ID de actividad de Strava
          </label>
          <input
            id={inputId}
            type="number"
            inputMode="numeric"
            min={1}
            step={1}
            value={activityId}
            onChange={(event) => setActivityId(event.target.value)}
            className={styles.input}
          />
        </div>
        <button type="button" className={styles.action} onClick={() => void load()}>
          Cargar streams
        </button>
      </div>

      <p className={styles.meta}>Puedes copiar el identificador de la URL de la actividad.</p>

      {state.kind === 'loading' && <p className={styles.meta}>Cargando streams…</p>}

      {state.kind === 'error' && (
        <p role="alert" className={styles.failed}>
          {state.message}
        </p>
      )}

      {state.kind === 'needs-refresh' && (
        <>
          <p role="status" className={styles.meta}>
            {state.detail}
          </p>
          <button type="button" className={styles.action} onClick={() => void refresh()}>
            Re-sincronizar
          </button>
        </>
      )}

      {state.kind === 'ready' && sample === null && (
        <p className={styles.empty}>La actividad no tiene streams de tiempo o GPS</p>
      )}

      {state.kind === 'ready' && sample !== null && rows.length === 0 && (
        <p className={styles.empty}>No hay kilómetros comparables entre el plan y la actividad</p>
      )}

      {rows.length > 0 && (
        <table className={styles.table}>
          <caption className={styles.caption}>Plan vs real · Ritmo plan {formatPace(pace)}</caption>
          <thead>
            <tr>
              <th scope="col">Km</th>
              <th scope="col">Plan</th>
              <th scope="col">Real</th>
              <th scope="col">Diferencia</th>
              <th scope="col">Fatiga</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.km}>
                <td>{row.km}</td>
                <td>{formatDuration(row.plannedSplitMin)}</td>
                <td>{formatDuration(row.realSplitMin)}</td>
                <td>{formatSignedDuration(row.diffMin)}</td>
                <td>{`${row.fatigueIndex.toFixed(1)} %`}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
