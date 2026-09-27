// ConnectionCard: Strava connection status display for /perfil.
// Issue 159. Pure presentational — receives the connection state
// from the parent container. The DELETE /api/v1/strava/connection
// endpoint doesn't exist yet (the query exists in the db layer but
// no HTTP route), so we only show the Connect button when
// disconnected; the Disconnect button is intentionally omitted
// until that endpoint lands.

import { Link } from 'react-router-dom';
import styles from './ConnectionCard.module.css';

export interface StravaConnectionState {
  connected: boolean;
  athlete_id?: number;
  scopes?: string[];
  last_sync?: string | null;
}

export interface ConnectionCardProps {
  state: StravaConnectionState;
  busy?: boolean | undefined;
}

export function ConnectionCard({ state, busy }: ConnectionCardProps) {
  if (state.connected) {
    return (
      <section
        className={styles.card}
        aria-label="Conexión con Strava"
        data-testid="strava-connection-connected"
      >
        <header className={styles.header}>
          <span className={styles.dotOk} aria-hidden="true" />
          <h2 className={styles.title}>Strava conectado</h2>
        </header>
        <dl className={styles.details}>
          <div className={styles.row}>
            <dt>Athlete ID</dt>
            <dd>{state.athlete_id ?? '—'}</dd>
          </div>
          {state.scopes && state.scopes.length > 0 && (
            <div className={styles.row}>
              <dt>Scopes</dt>
              <dd>{state.scopes.join(', ')}</dd>
            </div>
          )}
          {state.last_sync && (
            <div className={styles.row}>
              <dt>Última sincronización</dt>
              <dd>{new Date(state.last_sync).toLocaleString('es-ES')}</dd>
            </div>
          )}
        </dl>
        {busy && <p className={styles.busy}>Sincronizando…</p>}
      </section>
    );
  }

  return (
    <section
      className={styles.card}
      aria-label="Conexión con Strava"
      data-testid="strava-connection-disconnected"
    >
      <header className={styles.header}>
        <span className={styles.dotOff} aria-hidden="true" />
        <h2 className={styles.title}>Strava no conectado</h2>
      </header>
      <p className={styles.empty}>
        Conecta tu cuenta de Strava para importar actividades, calcular
        zonas de FC y mantener tu biblioteca al día.
      </p>
      <Link
        to="/api/v1/strava/connect"
        className={styles.connect}
        data-testid="strava-connect-link"
      >
        Conectar con Strava
      </Link>
    </section>
  );
}
