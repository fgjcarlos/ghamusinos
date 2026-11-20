import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiError } from '../../lib/api/types';
import type { Workout, WorkoutInput } from '../../lib/api/workout-types';
import { createWorkout, deleteWorkout, listWorkouts, updateWorkout } from '../../lib/api/workouts';
import { Button, EmptyState } from '../../ui';
import { WorkoutForm } from './WorkoutForm';
import { WorkoutsList, WorkoutsSkeleton } from './WorkoutsList';
import { getWorkoutWindow } from './workout-model';
import styles from './Workouts.module.css';

const PAGE_SIZE = 20;
const errorMessage = (error: unknown, fallback: string) =>
  error instanceof ApiError
    ? `${error.message}${error.status >= 500 && error.instance ? ` (ID: ${error.instance})` : ''}`
    : fallback;

export function WorkoutsContainer() {
  const token = import.meta.env.VITE_AUTH_TOKEN || '';
  const [dateWindow] = useState(() => getWorkoutWindow(new Date()));
  const [items, setItems] = useState<Workout[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState('');
  const [listFailed, setListFailed] = useState(false);
  const [form, setForm] = useState<Workout | null | undefined>(undefined);
  const [busy, setBusy] = useState(false);
  const [hasNext, setHasNext] = useState(false);
  const [retryKey, setRetryKey] = useState(0);
  const loadedOffset = useRef(0);
  const lock = useRef(false);
  const requestId = useRef(0);
  const mounted = useRef(false);

  const refresh = useCallback(
    (offset: number, signal?: AbortSignal) => {
      if (!token)
        return Promise.reject(
          new Error('No se encontró token de autenticación. Configura VITE_AUTH_TOKEN.'),
        );
      return listWorkouts(token, dateWindow.from, dateWindow.to, PAGE_SIZE, offset, signal);
    },
    [token, dateWindow],
  );

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      requestId.current += 1;
      lock.current = false;
    };
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    const current = ++requestId.current;
    lock.current = true;
    setLoading(true);
    setError('');
    refresh(0, controller.signal)
      .then((page) => {
        if (controller.signal.aborted || !mounted.current || current !== requestId.current) return;
        setItems(page.data);
        setHasNext(page.has_next);
        loadedOffset.current = page.data.length;
        setListFailed(false);
        setLoading(false);
        lock.current = false;
      })
      .catch((reason) => {
        if (controller.signal.aborted || !mounted.current || current !== requestId.current) return;
        setListFailed(true);
        setError(
          reason instanceof Error && !(reason instanceof ApiError)
            ? reason.message
            : errorMessage(reason, 'Error al cargar entrenos.'),
        );
        setLoading(false);
        lock.current = false;
      });
    return () => {
      controller.abort();
      if (current === requestId.current) {
        requestId.current += 1;
        lock.current = false;
      }
    };
  }, [refresh, retryKey]);

  const reloadFirstPage = async (visible?: Workout) => {
    const current = ++requestId.current;
    try {
      const page = await refresh(0);
      if (!mounted.current || current !== requestId.current) return;
      const byId = new Map(page.data.map((item) => [item.id, item]));
      if (visible) byId.set(visible.id, visible);
      setItems(
        [...byId.values()].sort(
          (a, b) => a.scheduled_on.localeCompare(b.scheduled_on) || a.id.localeCompare(b.id),
        ),
      );
      setHasNext(page.has_next);
      loadedOffset.current = page.data.length;
      setListFailed(false);
    } catch (reason) {
      if (mounted.current && current === requestId.current) setListFailed(true);
      throw reason;
    }
  };
  const save = async (input: WorkoutInput) => {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setError('');
    try {
      if (!token)
        throw new Error('No se encontró token de autenticación. Configura VITE_AUTH_TOKEN.');
      const saved = form?.id
        ? await updateWorkout(token, form.id, input)
        : await createWorkout(token, input);
      setForm(undefined);
      await reloadFirstPage(saved);
    } catch (reason) {
      if (mounted.current)
        setError(
          errorMessage(
            reason,
            reason instanceof Error ? reason.message : 'No se pudo guardar el entreno.',
          ),
        );
    } finally {
      lock.current = false;
      if (mounted.current) setBusy(false);
    }
  };
  const remove = async (workout: Workout) => {
    if (lock.current || !window.confirm(`¿Borrar «${workout.title}»?`)) return;
    lock.current = true;
    setBusy(true);
    setError('');
    try {
      if (!token)
        throw new Error('No se encontró token de autenticación. Configura VITE_AUTH_TOKEN.');
      await deleteWorkout(token, workout.id);
      await reloadFirstPage();
    } catch (reason) {
      if (mounted.current)
        setError(
          errorMessage(
            reason,
            reason instanceof Error ? reason.message : 'No se pudo borrar el entreno.',
          ),
        );
    } finally {
      lock.current = false;
      if (mounted.current) setBusy(false);
    }
  };
  const retryFirstPage = () => {
    if (lock.current) return;
    lock.current = true;
    requestId.current += 1;
    setLoading(true);
    setError('');
    setRetryKey((key) => key + 1);
  };
  const loadMore = async () => {
    if (lock.current || loading || loadingMore || listFailed || !hasNext) return;
    lock.current = true;
    setLoadingMore(true);
    setError('');
    const current = ++requestId.current;
    try {
      const offset = loadedOffset.current;
      const page = await refresh(offset);
      if (!mounted.current || current !== requestId.current) return;
      setItems((current) => {
        const byId = new Map(current.map((item) => [item.id, item]));
        for (const item of page.data) byId.set(item.id, item);
        return [...byId.values()].sort(
          (a, b) => a.scheduled_on.localeCompare(b.scheduled_on) || a.id.localeCompare(b.id),
        );
      });
      loadedOffset.current = offset + page.data.length;
      setHasNext(page.has_next);
    } catch (reason) {
      if (mounted.current && current === requestId.current)
        setError(
          errorMessage(
            reason,
            reason instanceof Error ? reason.message : 'No se pudieron cargar más entrenos.',
          ),
        );
    } finally {
      if (current === requestId.current) {
        lock.current = false;
        if (mounted.current) setLoadingMore(false);
      }
    }
  };

  if (loading && items.length === 0)
    return (
      <section className={styles.page} aria-labelledby="workouts-title">
        <h1 id="workouts-title">Entrenos</h1>
        <WorkoutsSkeleton />
      </section>
    );
  return (
    <section className={styles.page} aria-labelledby="workouts-title">
      <header className={styles.header}>
        <h1 id="workouts-title">Entrenos</h1>
        {form === undefined && (
          <Button disabled={busy || loadingMore || listFailed} onClick={() => setForm(null)}>
            Nuevo entreno
          </Button>
        )}
      </header>
      <p className={styles.range}>
        Próximas sesiones: {dateWindow.from} — {dateWindow.to} (fechas inclusivas)
      </p>
      {error && (
        <div className={styles.error} role="alert">
          {error}
        </div>
      )}
      {listFailed ? (
        <Button variant="secondary" disabled={loading || busy} onClick={retryFirstPage}>
          {loading ? 'Reintentando…' : 'Reintentar'}
        </Button>
      ) : form !== undefined ? (
        <WorkoutForm
          {...(form ? { initial: form } : {})}
          onSave={save}
          onCancel={() => setForm(undefined)}
          busy={busy}
          dateFrom={dateWindow.from}
          dateTo={dateWindow.to}
        />
      ) : items.length === 0 ? (
        <EmptyState
          message="No hay entrenos planificados en este periodo."
          action={
            <Button disabled={busy} onClick={() => setForm(null)}>
              Crear entreno
            </Button>
          }
        />
      ) : (
        <WorkoutsList
          workouts={items}
          onEdit={setForm}
          onDelete={(workout) => void remove(workout)}
          busy={busy || loadingMore}
          hasNext={hasNext}
          loadingMore={loadingMore}
          onLoadMore={() => void loadMore()}
        />
      )}
    </section>
  );
}
