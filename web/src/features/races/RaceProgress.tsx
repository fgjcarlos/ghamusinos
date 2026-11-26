import { useCallback } from 'react';
import { getRaceProgress } from '../../lib/api/races';
import { Button } from '../../ui';
import { useRaceData } from './useRaceData';
import styles from './Races.module.css';

import { format, km, metres } from './race-format';

export function RaceProgressPanel({ token, id }: { token: string; id: string }) {
  const load = useCallback(
    (signal: AbortSignal) => getRaceProgress(token, id, signal),
    [token, id],
  );
  const { result, retry } = useRaceData(load);
  const data = result.phase === 'ready' ? result.data : null;
  const current = data?.load[data.load.length - 1];
  return (
    <section aria-labelledby="race-progress">
      <h2 id="race-progress">Progreso</h2>
      <p>Métricas orientativas. El volumen registrado no predice el rendimiento en carrera.</p>
      {result.phase === 'loading' && <p role="status">Cargando progreso…</p>}
      {result.phase === 'error' && (
        <div>
          <p role="alert">No se pudo cargar el progreso.</p>
          <Button variant="secondary" onClick={retry}>
            Reintentar progreso
          </Button>
        </div>
      )}
      {data && (
        <>
          <p>
            {data.past
              ? 'La fecha de la carrera ya pasó.'
              : data.days_remaining === 0
                ? 'La carrera es hoy.'
                : `${data.days_remaining === 1 ? 'Falta 1 día' : `Faltan ${data.days_remaining} días`} (${data.weeks_remaining === 1 ? '1 semana completa' : `${data.weeks_remaining} semanas completas`}).`}
          </p>
          {data.sessions.total === 0 && data.activity_count === 0 && !current && (
            <p>No hay sesiones ni actividades para mostrar el progreso.</p>
          )}
          <dl className={styles.stats}>
            <div>
              <dt>Sesiones vinculadas</dt>
              <dd>{data.sessions.total}</dd>
            </div>
            <div>
              <dt>Planificadas</dt>
              <dd>{data.sessions.planned}</dd>
            </div>
            <div>
              <dt>Completadas</dt>
              <dd>{data.sessions.completed}</dd>
            </div>
            <div>
              <dt>Omitidas</dt>
              <dd>{data.sessions.skipped}</dd>
            </div>
            <div>
              <dt>Modificadas</dt>
              <dd>{data.sessions.modified}</dd>
            </div>
          </dl>
          <h3>Últimas cuatro semanas</h3>
          <p>
            {data.activity_count === 0
              ? 'Sin actividades registradas.'
              : `${data.activity_count} actividades registradas.`}
          </p>
          <dl className={styles.stats}>
            <div>
              <dt>Distancia registrada / objetivo</dt>
              <dd>
                {km(data.distance_m)} / {km(data.target_distance_m)}
              </dd>
            </div>
            <div>
              <dt>D+ registrado / objetivo</dt>
              <dd>
                {metres(data.elevation_gain_m)} / {metres(data.target_elevation_gain_m)}
              </dd>
            </div>
          </dl>
          {current ? (
            <>
              <dl className={styles.stats}>
                <div>
                  <dt>CTL actual</dt>
                  <dd>{format.format(current.ctl)}</dd>
                </div>
                <div>
                  <dt>ATL actual</dt>
                  <dd>{format.format(current.atl)}</dd>
                </div>
                <div>
                  <dt>TSB actual</dt>
                  <dd>{format.format(current.tsb)}</dd>
                </div>
              </dl>
              <details>
                <summary>Tendencia diaria de carga</summary>
                <table className={styles.trend} aria-label="Tendencia diaria de carga">
                  <thead>
                    <tr>
                      <th scope="col">Fecha</th>
                      <th scope="col">CTL</th>
                      <th scope="col">ATL</th>
                      <th scope="col">TSB</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.load.map((v) => (
                      <tr key={v.day}>
                        <th scope="row">{v.day}</th>
                        <td>{format.format(v.ctl)}</td>
                        <td>{format.format(v.atl)}</td>
                        <td>{format.format(v.tsb)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </details>
            </>
          ) : (
            <p>Sin carga de entrenamiento calculada.</p>
          )}
        </>
      )}
    </section>
  );
}
