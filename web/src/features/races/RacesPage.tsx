import { useCallback, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { listGpxTracks } from '../../lib/api/gpx';
import {
  deleteRace,
  getRace,
  listRaces,
  saveRace,
  setRaceTrack,
  type Race,
  type RaceDetail,
  type RaceInput,
} from '../../lib/api/races';
import { Button, Chip, EmptyState, Field } from '../../ui';
import { RaceForm } from './RaceForm';
import { RaceProgressPanel } from './RaceProgress';
import { countdown, km, metres } from './race-format';
import { useRaceData } from './useRaceData';
import styles from './Races.module.css';

const PAGE_SIZE = 20;
const token = () => import.meta.env.VITE_AUTH_TOKEN || '';
const path = (id: string) => `/carreras/${encodeURIComponent(id)}`;
const statusLabel = {
  planned: 'Planificada',
  completed: 'Completada',
  skipped: 'Omitida',
  modified: 'Modificada',
};

function RaceSummary({ race, showCountdown = false }: { race: Race; showCountdown?: boolean }) {
  return (
    <div className={styles.metadata}>
      <Chip tone={race.priority === 'A' ? 'accent' : 'neutral'}>Prioridad {race.priority}</Chip>
      <time dateTime={race.race_date}>{race.race_date}</time>
      <span>
        {km(race.distance_m)} · D+ {metres(race.elevation_gain_m)}
      </span>
      {showCountdown && <span>{countdown(race.race_date)}</span>}
    </div>
  );
}

export function RacesPage() {
  const auth = token();
  const navigate = useNavigate();
  const [offset, setOffset] = useState(0);
  const load = useCallback(
    (signal: AbortSignal) => listRaces(auth, PAGE_SIZE, offset, signal),
    [auth, offset],
  );
  const { result, retry } = useRaceData(load);
  const [creating, setCreating] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function create(input: RaceInput) {
    setBusy(true);
    setError('');
    try {
      const race = await saveRace(auth, input);
      void navigate(path(race.id));
    } catch {
      setError('No se pudo crear la carrera. Revisa los datos e inténtalo de nuevo.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className={styles.page} aria-labelledby="races-title">
      <header className={styles.header}>
        <h1 id="races-title">Carreras</h1>
        {!creating && <Button onClick={() => setCreating(true)}>Nueva carrera</Button>}
      </header>
      <p>Organiza tus objetivos, vincula rutas y sigue tu preparación.</p>
      {error && (
        <p role="alert" className={styles.error}>
          {error}
        </p>
      )}
      {creating ? (
        <RaceForm
          busy={busy}
          onSave={(v) => void create(v)}
          onCancel={() => {
            setCreating(false);
            setError('');
          }}
        />
      ) : (
        <>
          {result.phase === 'loading' && <p role="status">Cargando carreras…</p>}
          {result.phase === 'error' && (
            <div>
              <p role="alert">No se pudieron cargar las carreras.</p>
              <Button variant="secondary" onClick={retry}>
                Reintentar
              </Button>
            </div>
          )}
          {result.phase === 'ready' && (
            <>
              {result.data.data.length === 0 ? (
                <EmptyState message="No hay carreras en esta página." />
              ) : (
                <ul className={styles.rows}>
                  {result.data.data.map((race) => (
                    <li key={race.id}>
                      <Link to={path(race.id)}>{race.name}</Link>
                      <RaceSummary race={race} showCountdown />
                    </li>
                  ))}
                </ul>
              )}
              {(offset > 0 || result.data.has_next) && (
                <nav className={styles.actions} aria-label="Páginas de carreras">
                  <Button
                    variant="secondary"
                    disabled={!offset}
                    onClick={() => setOffset(offset - PAGE_SIZE)}
                  >
                    Anterior
                  </Button>
                  <Button
                    variant="secondary"
                    disabled={!result.data.has_next}
                    onClick={() => setOffset(offset + PAGE_SIZE)}
                  >
                    Siguiente
                  </Button>
                </nav>
              )}
            </>
          )}
        </>
      )}
    </section>
  );
}

export function RaceDetailRoute() {
  const { id = '' } = useParams<{ id: string }>();
  return <RaceDetailPage key={id} auth={token()} id={id} />;
}

function RaceDetailPage({ auth, id }: { auth: string; id: string }) {
  const navigate = useNavigate();
  const load = useCallback((signal: AbortSignal) => getRace(auth, id, signal), [auth, id]);
  const { result, retry } = useRaceData(load);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const race = result.phase === 'ready' ? result.data : null;

  async function change(action: () => Promise<unknown>, failure: string, onSuccess: () => void) {
    setBusy(true);
    setError('');
    try {
      await action();
      onSuccess();
    } catch {
      setError(failure);
    } finally {
      setBusy(false);
    }
  }

  function edit(input: RaceInput) {
    void change(
      () => saveRace(auth, input, id),
      'No se pudo guardar la carrera.',
      () => {
        setEditing(false);
        retry();
      },
    );
  }

  function remove() {
    if (!window.confirm('¿Borrar esta carrera? Las rutas y sesiones vinculadas se conservarán.'))
      return;
    void change(
      () => deleteRace(auth, id),
      'No se pudo borrar la carrera.',
      () => {
        void navigate('/carreras');
      },
    );
  }

  function link(trackId: string, attached: boolean) {
    void change(
      () => setRaceTrack(auth, id, trackId, attached),
      attached ? 'No se pudo vincular la ruta.' : 'No se pudo desvincular la ruta.',
      retry,
    );
  }

  return (
    <article className={styles.page} aria-labelledby="race-title">
      <Link to="/carreras">Volver a carreras</Link>
      <header className={styles.header}>
        <h1 id="race-title">{race?.name ?? 'Carrera'}</h1>
        {race && !editing && (
          <div className={styles.actions}>
            <Button variant="secondary" disabled={busy} onClick={() => setEditing(true)}>
              Editar carrera
            </Button>
            <Button variant="destructive" disabled={busy} onClick={remove}>
              Borrar carrera
            </Button>
          </div>
        )}
      </header>
      {result.phase === 'loading' && <p role="status">Cargando carrera…</p>}
      {result.phase === 'error' && (
        <div>
          <p role="alert">No se pudo cargar la carrera.</p>
          <Button variant="secondary" onClick={retry}>
            Reintentar
          </Button>
        </div>
      )}
      {error && (
        <p role="alert" className={styles.error}>
          {error}
        </p>
      )}
      {race &&
        (editing ? (
          <RaceForm
            initial={race}
            busy={busy}
            onSave={edit}
            onCancel={() => {
              setEditing(false);
              setError('');
            }}
          />
        ) : (
          <>
            <RaceSummary race={race} />
            {race.notes && <p className={styles.notes}>{race.notes}</p>}
            <section aria-labelledby="race-tracks">
              <h2 id="race-tracks">Rutas GPX vinculadas</h2>
              {race.tracks.length === 0 ? (
                <p>No hay rutas vinculadas.</p>
              ) : (
                <ul className={styles.rows}>
                  {race.tracks.map((track) => (
                    <li key={track.id} className={styles.header}>
                      <Link to={`/rutas/${encodeURIComponent(track.id)}`}>{track.name}</Link>
                      <Button
                        variant="secondary"
                        onClick={() => link(track.id, false)}
                        disabled={busy}
                      >
                        Desvincular<span className={styles.srOnly}> {track.name}</span>
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
              <TrackPicker
                auth={auth}
                tracks={race.tracks}
                busy={busy}
                onAttach={(track) => link(track, true)}
              />
            </section>
            <section aria-labelledby="race-sessions">
              <h2 id="race-sessions">Sesiones vinculadas</h2>
              {race.sessions.length === 0 ? (
                <p>No hay sesiones vinculadas. Puedes elegir esta carrera al editar un entreno.</p>
              ) : (
                <ul className={styles.rows}>
                  {race.sessions.map((session) => (
                    <li className={styles.header} key={session.id}>
                      {session.title}
                      <div className={styles.metadata}>
                        <time dateTime={session.scheduled_on}>{session.scheduled_on}</time>
                        <span>{statusLabel[session.status]}</span>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
              <Link to="/entrenos">Ir a entrenos</Link>
            </section>
            <RaceProgressPanel token={auth} id={id} />
          </>
        ))}
    </article>
  );
}

const trackKey = (id: string) => id.replace(/-/g, '').toLowerCase();

function TrackPicker({
  auth,
  tracks,
  busy,
  onAttach,
}: {
  auth: string;
  tracks: RaceDetail['tracks'];
  busy: boolean;
  onAttach: (id: string) => void;
}) {
  const load = useCallback(
    async (signal: AbortSignal) => {
      const all: RaceDetail['tracks'] = [];
      for (;;) {
        const page = await listGpxTracks(auth, 100, all.length);
        all.push(
          ...page.data.map(({ track }) => ({
            id: typeof track.id === 'string' ? track.id : track.id.bytes,
            name: track.name,
          })),
        );
        if (signal.aborted || !page.has_next || !page.data.length) return all;
      }
    },
    [auth],
  );
  const { result, retry } = useRaceData(load);
  const [chosen, setChosen] = useState('');
  const attached = new Set(tracks.map(({ id }) => trackKey(id)));
  const available =
    result.phase === 'ready' ? result.data.filter(({ id }) => !attached.has(trackKey(id))) : [];
  if (result.phase === 'loading') return <p role="status">Cargando rutas disponibles…</p>;
  if (result.phase === 'error')
    return (
      <div>
        <p role="alert">No se pudieron cargar las rutas disponibles.</p>
        <Button variant="secondary" onClick={retry}>
          Reintentar rutas
        </Button>
      </div>
    );
  if (!available.length)
    return (
      <p>
        No hay más rutas GPX disponibles. <Link to="/rutas">Gestionar rutas</Link>
      </p>
    );
  return (
    <form
      className={styles.actions}
      onSubmit={(event) => {
        event.preventDefault();
        if (chosen && !busy) onAttach(chosen);
      }}
    >
      <Field label="Ruta GPX">
        <select
          required
          value={chosen}
          disabled={busy}
          onChange={(event) => setChosen(event.target.value)}
        >
          <option value="">Elige una ruta</option>
          {available.map(({ id, name }) => (
            <option key={id} value={id}>
              {name}
            </option>
          ))}
        </select>
      </Field>
      <Button type="submit" disabled={busy || !chosen}>
        Vincular ruta
      </Button>
    </form>
  );
}
