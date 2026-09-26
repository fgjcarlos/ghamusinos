// ActivityListContainer: data layer for the activities page. Fetches
// paginated activities from /api/v1/activities, owns the page state,
// and hands data to the presentational ActivityList. Issue 158.

import { useEffect, useState } from 'react';
import { listActivities } from '../../lib/api/strava';
import { ApiError, type Activity } from '../../lib/api/types';
import { ActivityList } from './ActivityList/ActivityList';
import styles from './ActivityListContainer.module.css';

const PAGE_SIZE = 20;

export function ActivityListContainer() {
  const token = import.meta.env.VITE_AUTH_TOKEN || '';
  const [activities, setActivities] = useState<Activity[]>([]);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hasNext, setHasNext] = useState(false);

  useEffect(() => {
    if (!token) {
      setError('No se encontró token de autenticación. Configura VITE_AUTH_TOKEN en .env.');
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    listActivities(token, page, PAGE_SIZE)
      .then((resp) => {
        if (cancelled) return;
        setActivities(resp.data);
        setHasNext(resp.has_next);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError) {
          setError(`Error ${err.status}: ${err.message}`);
        } else {
          setError('Error desconocido al cargar actividades.');
        }
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [page, token]);

  if (loading && activities.length === 0) {
    return (
      <div className={styles.loading} role="status">
        Cargando actividades…
      </div>
    );
  }

  if (error && activities.length === 0) {
    return (
      <div className={styles.error} role="alert">
        {error}
      </div>
    );
  }

  return (
    <ActivityList activities={activities} page={page} hasNext={hasNext} onPageChange={setPage} />
  );
}
