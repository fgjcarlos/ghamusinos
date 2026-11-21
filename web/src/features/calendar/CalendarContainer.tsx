// Interactive calendar (#313, T3). Week and month views in the user's
// stored IANA zone, native drag & drop for moving sessions and linking
// activities, and real keyboard/touch alternatives for every action
// (date input to move, selectors to link and to set status). Sessions
// in the visible range are fetched through the existing paginated list
// endpoint (every page) and refreshed after each mutation; stale loads
// are dropped by request id.

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type DragEvent,
  type ReactNode,
} from 'react';
import { ApiError } from '../../lib/api/types';
import type { Workout, WorkoutInput, WorkoutStatus } from '../../lib/api/workout-types';
import {
  createWorkout,
  linkWorkoutActivity,
  listWorkouts,
  unlinkWorkoutActivity,
  updateWorkout,
} from '../../lib/api/workouts';
import { listCalendarActivities, type CalendarActivity } from '../../lib/api/calendar';
import { getPreferences } from '../../lib/api/profile';
import { WorkoutForm } from '../workouts/WorkoutForm';
import { EmptyState } from '../../ui';
import {
  addDays,
  addMonths,
  dayLabel,
  dayNumber,
  monthDays,
  monthLabel,
  rangeOf,
  resolveZone,
  todayKey,
  weekDays,
  weekLabel,
  weekdayHeaders,
} from './calendar-model';
import styles from './Calendar.module.css';

const PAGE_SIZE = 100; // backend max limit: fewest round trips per range
const SESSION_MIME = 'application/x-ghamusinos-session';
const ACTIVITY_MIME = 'application/x-ghamusinos-activity';

const statusLabels: Record<WorkoutStatus, string> = {
  planned: 'Planificada',
  completed: 'Completada',
  skipped: 'Omitida',
  modified: 'Modificada',
};

// Las ApiError traen el ProblemDetail del backend; el resto (red,
// offline) no dice nada util al usuario: mejor el mensaje de la accion.
const errorMessage = (reason: unknown, fallback: string) =>
  reason instanceof ApiError ? reason.message : fallback;

type View = 'week' | 'month';

export interface CalendarContainerProps {
  /** Reference clock so tests can pin "today" deterministically. */
  now?: Date;
  /** Slot for the plan-vs-real comparison panel (T4, #312). */
  comparisonPanel?: ReactNode;
}

export function CalendarContainer({ now = new Date(), comparisonPanel }: CalendarContainerProps) {
  const token = import.meta.env.VITE_AUTH_TOKEN || '';
  const [zone, setZone] = useState('');
  const [anchor, setAnchor] = useState<string | null>(null);
  const [view, setView] = useState<View>('month');
  const [sessions, setSessions] = useState<Workout[]>([]);
  const [activities, setActivities] = useState<CalendarActivity[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [actionError, setActionError] = useState('');
  const [pending, setPending] = useState(false);
  const [createDate, setCreateDate] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);
  const requestId = useRef(0);
  const mounted = useRef(false);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      requestId.current += 1;
    };
  }, []);

  // La zona horaria viene de las preferencias del usuario autenticado;
  // una zona invalida o un fallo de preferencias caen a UTC (no es un
  // fallo del calendario).
  useEffect(() => {
    if (!token) {
      setZone('UTC');
      return;
    }
    let alive = true;
    getPreferences(token)
      .then((prefs) => alive && setZone(resolveZone(prefs.timezone)))
      .catch(() => alive && setZone('UTC'));
    return () => {
      alive = false;
    };
  }, [token]);

  useEffect(() => {
    if (zone && anchor === null) setAnchor(todayKey(now, zone));
  }, [zone, anchor, now]);

  const days = useMemo(
    () => (anchor ? (view === 'week' ? weekDays(anchor) : monthDays(anchor)) : []),
    [anchor, view],
  );
  const range = useMemo(() => (days.length ? rangeOf(days) : null), [days]);
  const from = range?.from ?? '';
  const to = range?.to ?? '';

  const performLoad = useCallback(
    async (signal?: AbortSignal) => {
      if (!token || !from || !to) return;
      const current = ++requestId.current;
      setLoading(true);
      setLoadError('');
      try {
        const data: Workout[] = [];
        let offset = 0;
        for (;;) {
          const result = await listWorkouts(token, from, to, PAGE_SIZE, offset, signal);
          data.push(...result.data);
          if (!result.has_next || result.data.length === 0) break;
          offset += result.data.length;
        }
        const feed = await listCalendarActivities(token, from, to, signal);
        if (!mounted.current || current !== requestId.current) return;
        setSessions(data);
        setActivities(feed.activities);
        setLoading(false);
      } catch (reason) {
        if (!mounted.current || current !== requestId.current) return;
        setSessions([]);
        setActivities([]);
        setLoadError(errorMessage(reason, 'Error al cargar el calendario.'));
        setLoading(false);
      }
    },
    [token, from, to],
  );

  useEffect(() => {
    if (!token || !from || !to) return;
    const controller = new AbortController();
    void performLoad(controller.signal);
    return () => controller.abort();
  }, [performLoad, retryKey, token, from, to]);

  const withReload = async (action: () => Promise<unknown>, failure: string) => {
    if (pending) return;
    setPending(true);
    setActionError('');
    try {
      await action();
      setRetryKey((key) => key + 1);
    } catch (reason) {
      if (mounted.current) setActionError(errorMessage(reason, failure));
    } finally {
      if (mounted.current) setPending(false);
    }
  };

  const move = (id: string, date: string) =>
    withReload(
      () => updateWorkout(token, id, { scheduled_on: date }),
      'No se pudo mover la sesión.',
    );
  const changeStatus = (id: string, status: WorkoutStatus) =>
    withReload(() => updateWorkout(token, id, { status }), 'No se pudo cambiar el estado.');
  const link = (sessionId: string, activityId: string) =>
    withReload(
      () => linkWorkoutActivity(token, sessionId, activityId),
      'No se pudo vincular la actividad.',
    );
  const unlink = (sessionId: string) =>
    withReload(
      () => unlinkWorkoutActivity(token, sessionId),
      'No se pudo desvincular la actividad.',
    );
  const create = async (input: WorkoutInput) => {
    if (pending) return;
    setPending(true);
    setActionError('');
    let saved = false;
    try {
      await createWorkout(token, input);
      saved = true;
      setRetryKey((key) => key + 1);
    } catch (reason) {
      if (mounted.current) setActionError(errorMessage(reason, 'No se pudo crear la sesión.'));
    } finally {
      if (mounted.current) setPending(false);
    }
    if (saved && mounted.current) setCreateDate(null);
  };

  const step = (direction: 1 | -1) =>
    setAnchor((current) =>
      current
        ? view === 'week'
          ? addDays(current, 7 * direction)
          : addMonths(current, direction)
        : current,
    );
  const goToday = () => setAnchor(todayKey(now, zone));

  // Drag & drop nativo: sesiones hacia dias (mover) y actividades hacia
  // sesiones (vincular). Los payloads viajan en tipos MIME distintos y
  // el drop de la tarjeta corta la propagacion: soltar una actividad
  // sobre una sesion nunca dispara el drop del dia que la contiene.
  const onSessionDragStart = (event: DragEvent<HTMLLIElement>, id: string) => {
    event.dataTransfer.setData(SESSION_MIME, id);
    event.dataTransfer.effectAllowed = 'move';
  };
  const onActivityDragStart = (event: DragEvent<HTMLLIElement>, id: string) => {
    event.dataTransfer.setData(ACTIVITY_MIME, id);
    event.dataTransfer.effectAllowed = 'link';
  };
  const onDayDragOver = (event: DragEvent<HTMLElement>) => {
    if (event.dataTransfer.types.includes(SESSION_MIME)) event.preventDefault();
  };
  const onDayDrop = (event: DragEvent<HTMLElement>, date: string) => {
    const id = event.dataTransfer.getData(SESSION_MIME);
    if (!id) return;
    event.preventDefault();
    if (sessions.some((s) => s.id === id && s.scheduled_on !== date)) void move(id, date);
  };
  const onCardDragOver = (event: DragEvent<HTMLLIElement>) => {
    if (event.dataTransfer.types.includes(ACTIVITY_MIME)) event.preventDefault();
  };
  const onCardDrop = (event: DragEvent<HTMLLIElement>, sessionId: string) => {
    const id = event.dataTransfer.getData(ACTIVITY_MIME);
    if (!id) return;
    event.stopPropagation();
    event.preventDefault();
    if (unlinked.some((activity) => activity.id === id)) void link(sessionId, id);
  };

  const unlinked = useMemo(() => activities.filter((a) => !a.linked), [activities]);
  const sessionsByDay = useMemo(() => {
    const map = new Map<string, Workout[]>();
    for (const s of sessions) map.set(s.scheduled_on, [...(map.get(s.scheduled_on) ?? []), s]);
    return map;
  }, [sessions]);
  const activitiesByDay = useMemo(() => {
    const map = new Map<string, CalendarActivity[]>();
    for (const a of activities) map.set(a.scheduled_on, [...(map.get(a.scheduled_on) ?? []), a]);
    return map;
  }, [activities]);

  const period = anchor ? (view === 'week' ? weekLabel(anchor) : monthLabel(anchor)) : '';
  const today = zone ? todayKey(now, zone) : '';

  return (
    <section className={styles.page} aria-labelledby="calendar-title">
      <header className={styles.header}>
        <h1 id="calendar-title">Calendario</h1>
        <div className={styles.viewToggle} role="group" aria-label="Vista del calendario">
          <button
            type="button"
            className={styles.toggle}
            aria-pressed={view === 'week'}
            onClick={() => setView('week')}
          >
            Semana
          </button>
          <button
            type="button"
            className={styles.toggle}
            aria-pressed={view === 'month'}
            onClick={() => setView('month')}
          >
            Mes
          </button>
        </div>
      </header>
      <nav className={styles.nav} aria-label="Navegación del calendario">
        <button type="button" className={styles.navBtn} onClick={() => step(-1)} disabled={!anchor}>
          Anterior
        </button>
        <button type="button" className={styles.navBtn} onClick={goToday} disabled={!zone}>
          Hoy
        </button>
        <button type="button" className={styles.navBtn} onClick={() => step(1)} disabled={!anchor}>
          Siguiente
        </button>
      </nav>
      <h2 className={styles.period}>{period}</h2>
      {!token && (
        <div className={styles.error} role="alert">
          No se encontró token de autenticación. Configura <code>VITE_AUTH_TOKEN</code>.
        </div>
      )}
      {actionError && (
        <div className={styles.error} role="alert">
          {actionError}
        </div>
      )}
      {loadError && (
        <div className={styles.error} role="alert">
          {loadError}
          <button type="button" className={styles.navBtn} onClick={() => setRetryKey((k) => k + 1)}>
            Reintentar
          </button>
        </div>
      )}
      {comparisonPanel}
      {createDate && (
        <section className={styles.formPanel} aria-labelledby="new-session-title">
          <h2 id="new-session-title">Nueva sesión</h2>
          <WorkoutForm
            initial={{ scheduled_on: createDate }}
            onSave={create}
            onCancel={() => setCreateDate(null)}
            busy={pending}
          />
        </section>
      )}
      {loading && token && sessions.length === 0 && activities.length === 0 ? (
        <div className={styles.skeleton} role="status" aria-busy="true">
          Cargando calendario…
          <span />
          <span />
        </div>
      ) : (
        <>
          {!loading && !loadError && sessions.length === 0 && activities.length === 0 && (
            <EmptyState message="No hay sesiones ni actividades en este periodo." />
          )}
          <section className={styles.tray} aria-labelledby="tray-title">
            <h2 id="tray-title" className={styles.trayTitle}>
              Actividades sin vincular
            </h2>
            {unlinked.length === 0 ? (
              <p className={styles.trayEmpty}>No hay actividades sin vincular en este periodo.</p>
            ) : (
              <ul className={styles.trayList}>
                {unlinked.map((a) => (
                  <li
                    key={a.id}
                    className={styles.trayItem}
                    draggable={!pending}
                    onDragStart={(e) => onActivityDragStart(e, a.id)}
                  >
                    {a.name} · {a.scheduled_on}
                  </li>
                ))}
              </ul>
            )}
            <p className={styles.hint}>
              Arrastra una actividad sobre una sesión para vincularla, o elígela en el selector de
              cada sesión. Todo también se puede hacer con teclado.
            </p>
          </section>
          <div className={styles.gridWrap}>
            <ol className={styles.weekdays} aria-hidden="true">
              {weekdayHeaders.map((label) => (
                <li key={label}>{label}</li>
              ))}
            </ol>
            <div className={styles.grid}>
              {days.map((date) => (
                <section
                  key={date}
                  className={styles.cell}
                  data-date={date}
                  aria-label={dayLabel(date)}
                  onDragOver={onDayDragOver}
                  onDrop={(e) => onDayDrop(e, date)}
                >
                  <div className={styles.cellHead}>
                    <time
                      dateTime={date}
                      className={date === today ? styles.today : styles.dateNum}
                      {...(date === today ? { 'aria-current': 'date' } : {})}
                    >
                      {dayNumber(date)}
                    </time>
                    <button
                      type="button"
                      className={styles.add}
                      aria-label={`Crear sesión el ${date}`}
                      disabled={pending}
                      onClick={() => setCreateDate(date)}
                    >
                      +
                    </button>
                  </div>
                  <ul className={styles.items}>
                    {(sessionsByDay.get(date) ?? []).map((session) => (
                      <li
                        key={session.id}
                        className={styles.card}
                        data-status={session.status}
                        draggable={!pending}
                        onDragStart={(e) => onSessionDragStart(e, session.id)}
                        onDragOver={onCardDragOver}
                        onDrop={(e) => onCardDrop(e, session.id)}
                      >
                        <p className={styles.cardTitle}>{session.title}</p>
                        <label className={styles.control}>
                          <span>Estado</span>
                          <select
                            value={session.status}
                            disabled={pending}
                            aria-label={`Estado de ${session.title}`}
                            onChange={(e) =>
                              void changeStatus(session.id, e.target.value as WorkoutStatus)
                            }
                          >
                            {Object.entries(statusLabels).map(([value, label]) => (
                              <option key={value} value={value}>
                                {label}
                              </option>
                            ))}
                          </select>
                        </label>
                        <label className={styles.control}>
                          <span>Mover a</span>
                          <input
                            type="date"
                            defaultValue={session.scheduled_on}
                            disabled={pending}
                            aria-label={`Mover ${session.title} a otra fecha`}
                            onChange={(e) => {
                              if (e.target.value) void move(session.id, e.target.value);
                            }}
                          />
                        </label>
                        {session.activity_id ? (
                          <span className={styles.linked}>
                            Vinculada
                            {activities.find((a) => a.id === session.activity_id)
                              ? `: ${activities.find((a) => a.id === session.activity_id)!.name}`
                              : ''}
                            <button
                              type="button"
                              className={styles.unlink}
                              disabled={pending}
                              onClick={() => void unlink(session.id)}
                            >
                              Desvincular {session.title}
                            </button>
                          </span>
                        ) : (
                          <label className={styles.control}>
                            <span>Vincular actividad</span>
                            <select
                              defaultValue=""
                              disabled={pending || unlinked.length === 0}
                              aria-label={`Vincular actividad a ${session.title}`}
                              onChange={(e) => {
                                const value = e.target.value;
                                e.currentTarget.value = '';
                                if (value) void link(session.id, value);
                              }}
                            >
                              <option value="">
                                {unlinked.length === 0
                                  ? 'Sin actividades sin vincular'
                                  : 'Elegir actividad…'}
                              </option>
                              {unlinked.map((a) => (
                                <option key={a.id} value={a.id}>
                                  {a.name} ({a.scheduled_on})
                                </option>
                              ))}
                            </select>
                          </label>
                        )}
                      </li>
                    ))}
                  </ul>
                  <ul className={styles.items}>
                    {(activitiesByDay.get(date) ?? []).map((a) => (
                      <li
                        key={a.id}
                        className={a.linked ? styles.linkedActivity : styles.freeActivity}
                        draggable={!a.linked && !pending}
                        onDragStart={
                          a.linked
                            ? undefined
                            : (e: DragEvent<HTMLLIElement>) => onActivityDragStart(e, a.id)
                        }
                      >
                        {a.name} · {a.linked ? 'vinculada' : 'libre'}
                      </li>
                    ))}
                  </ul>
                </section>
              ))}
            </div>
          </div>
        </>
      )}
    </section>
  );
}
