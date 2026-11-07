// Meteo tab: four-point forecast, loaded on demand for a chosen race instant.

import { useId, useState } from 'react';
import type { TrackWeatherHook } from '../hooks/useTrackWeather';
import { defaultRaceDateTimeLocal, isoFromLocalDateTime, weatherRoleLabel } from './format';
import styles from './RacePlanSection.module.css';

export interface WeatherPanelProps {
  weather: TrackWeatherHook;
}

const INVALID_MESSAGE = 'Introduce una fecha y hora válidas.';

export function WeatherPanel({ weather }: WeatherPanelProps) {
  const inputId = useId();
  const [at, setAt] = useState(() => defaultRaceDateTimeLocal());
  const [validation, setValidation] = useState<string | null>(null);

  function handleLoad() {
    const iso = isoFromLocalDateTime(at);
    if (iso === null) {
      setValidation(INVALID_MESSAGE);
      return;
    }
    setValidation(null);
    void weather.load(iso);
  }

  return (
    <div className={styles.stack}>
      <div className={styles.inlineFields}>
        <div className={styles.field}>
          <label htmlFor={inputId} className={styles.label}>
            Fecha y hora de la previsión
          </label>
          <input
            id={inputId}
            type="datetime-local"
            value={at}
            onChange={(event) => setAt(event.target.value)}
            className={styles.input}
          />
        </div>
        <button type="button" className={styles.action} onClick={handleLoad}>
          Cargar previsión
        </button>
      </div>

      {validation !== null && (
        <p role="alert" className={styles.failed}>
          {validation}
        </p>
      )}

      {weather.loading && <p className={styles.meta}>Cargando previsión…</p>}

      {weather.error !== null && (
        <p role="alert" className={styles.failed}>
          {weather.error}
        </p>
      )}

      {weather.data !== null && (
        <table className={styles.table}>
          <caption className={styles.caption}>Previsión por puntos estratégicos</caption>
          <thead>
            <tr>
              <th scope="col">Punto</th>
              <th scope="col">Km</th>
              <th scope="col">Temperatura</th>
              <th scope="col">Viento</th>
              <th scope="col">Previsión</th>
            </tr>
          </thead>
          <tbody>
            {weather.data.points.map((point) => (
              <tr key={point.role}>
                <td>{weatherRoleLabel(point.role)}</td>
                <td>{point.km.toFixed(1)}</td>
                <td>{point.temperature_c} °C</td>
                <td>{point.wind_kmh} km/h</td>
                <td>{point.available ? point.label : 'No disponible'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
