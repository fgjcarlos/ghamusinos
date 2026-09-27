// ComparisonMode: page-level container for the route comparator.
// Issue 126. Fetches up to 3 tracks via the existing compareGpxTracks
// API and hands the response to RouteComparator. JSON export via
// Blob + URL.createObjectURL.

import { useEffect, useState } from 'react';
import { compareGpxTracks } from '../../../lib/api/gpx';
import { ApiError, type CompareResponse } from '../../../lib/api/types';
import { RouteComparator } from '../RouteComparator/RouteComparator';
import styles from './ComparisonMode.module.css';

export type ComparisonStatus =
  | { kind: 'loading' }
  | { kind: 'no-token' }
  | { kind: 'error'; message: string }
  | { kind: 'ready'; data: CompareResponse };

export interface ComparisonModeProps {
  trackIds: string[];
}

const MAX_TRACKS = 3;

export function ComparisonMode({ trackIds }: ComparisonModeProps) {
  const [status, setStatus] = useState<ComparisonStatus>({ kind: 'loading' });

  useEffect(() => {
    const token = import.meta.env.VITE_AUTH_TOKEN || '';
    if (!token) {
      setStatus({ kind: 'no-token' });
      return;
    }
    if (trackIds.length < 2 || trackIds.length > MAX_TRACKS) {
      setStatus({
        kind: 'error',
        message: `Selecciona entre 2 y ${MAX_TRACKS} rutas para comparar.`,
      });
      return;
    }
    let cancelled = false;
    setStatus({ kind: 'loading' });

    compareGpxTracks(token, trackIds)
      .then((data: CompareResponse) => {
        if (cancelled) return;
        setStatus({ kind: 'ready', data });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError) {
          setStatus({ kind: 'error', message: `Error ${err.status}: ${err.message}` });
          return;
        }
        setStatus({
          kind: 'error',
          message: err instanceof Error ? err.message : 'Error desconocido',
        });
      });

    return () => {
      cancelled = true;
    };
  }, [trackIds.join(',')]);

  function exportJSON() {
    if (status.kind !== 'ready') return;
    const payload = {
      exportedAt: new Date().toISOString(),
      trackIds,
      response: status.data,
    };
    const blob = new Blob([JSON.stringify(payload, null, 2)], {
      type: 'application/json',
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `comparison-${Date.now()}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  if (status.kind === 'loading') {
    return (
      <section className={styles.skeleton} data-testid="comparison-status-loading" role="status">
        Cargando comparación…
      </section>
    );
  }

  if (status.kind === 'no-token') {
    return (
      <section className={styles.error} role="alert" data-testid="comparison-status-no-token">
        <p>
          No se encontró token de autenticación. Configura <code>VITE_AUTH_TOKEN</code> en tu
          archivo <code>.env</code>.
        </p>
      </section>
    );
  }

  if (status.kind === 'error') {
    return (
      <section className={styles.error} role="alert" data-testid="comparison-status-error">
        <p>{status.message}</p>
      </section>
    );
  }

  return (
    <section data-testid="comparison-status-ready">
      <button
        type="button"
        className={styles.exportBtn}
        onClick={exportJSON}
        data-testid="comparison-export-json"
      >
        Exportar comparación (JSON)
      </button>
      <RouteComparator data={status.data} />
    </section>
  );
}
