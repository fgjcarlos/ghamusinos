// RouteDetailContainer: data layer for the route detail page. Issue 157.
// Owns the fetch, the pgtype→plain normalization, and the loading /
// 404 / 500 / no-token state machine. Pure presentational components
// (RouteHeader, RouteMetrics, ClimbList, RiskList) live next to it
// and only see NormalizedTrackDetail — they don't know pgtype exists.

import { useEffect, useState } from 'react';
import { ApiError, type StoredTrackDetail } from '../../lib/api/types';
import { getGpxTrack } from '../../lib/api/gpx';
import {
  normalizeTrackDetail,
  type NormalizedTrackDetail,
} from './normalize';
import styles from './RouteDetailContainer.module.css';

export type RouteDetailStatus =
  | { kind: 'loading' }
  | { kind: 'no-token' }
  | { kind: 'not-found' }
  | { kind: 'error'; message: string }
  | { kind: 'ready'; data: NormalizedTrackDetail };

export interface RouteDetailContainerProps {
  trackId: string;
}

const SKELETON_TIMEOUT_MS = 12_000;

export function RouteDetailContainer({ trackId }: RouteDetailContainerProps) {
  const [status, setStatus] = useState<RouteDetailStatus>({ kind: 'loading' });

  useEffect(() => {
    const token = import.meta.env.VITE_AUTH_TOKEN || '';
    if (!token) {
      setStatus({ kind: 'no-token' });
      return;
    }
    let cancelled = false;
    setStatus({ kind: 'loading' });

    getGpxTrack(token, trackId)
      .then((detail: StoredTrackDetail) => {
        if (cancelled) return;
        setStatus({ kind: 'ready', data: normalizeTrackDetail(detail) });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError) {
          if (err.status === 404) {
            setStatus({ kind: 'not-found' });
            return;
          }
          if (err.status >= 500) {
            setStatus({
              kind: 'error',
              message: `Error ${err.status}: ${err.message}`,
            });
            return;
          }
        }
        setStatus({
          kind: 'error',
          message: err instanceof Error ? err.message : 'Error desconocido',
        });
      });

    return () => {
      cancelled = true;
    };
  }, [trackId]);

  switch (status.kind) {
    case 'loading':
      return <RouteDetailSkeleton data-testid="route-detail-status-loading" />;
    case 'no-token':
      return (
        <section
          className={styles.error}
          role="alert"
          data-testid="route-detail-status-no-token"
        >
          <p>
            No se encontró token de autenticación. Configura{' '}
            <code>VITE_AUTH_TOKEN</code> en tu archivo <code>.env</code>.
          </p>
        </section>
      );
    case 'not-found':
      return (
        <section
          className={styles.error}
          role="alert"
          data-testid="route-detail-status-not-found"
        >
          <p>Esta ruta no existe o no es tuya.</p>
          <p>
            <a href="/lab">← Volver al laboratorio</a>
          </p>
        </section>
      );
    case 'error':
      return (
        <section
          className={styles.error}
          role="alert"
          data-testid="route-detail-status-error"
        >
          <p>{status.message}</p>
        </section>
      );
    case 'ready':
      // Commit 2 will replace this placeholder with the real
      // <RouteDetail data={status.data} /> (Header, Metrics, ClimbList,
      // RiskList, ElevationProfile). For now we surface the data
      // contract to tests via a data-testid.
      return (
        <section data-testid="route-detail-status-ready">
          {/* eslint-disable-next-line react/jsx-pascal-case */}
          <RouteDetailData data={status.data} />
        </section>
      );
  }
}

function RouteDetailSkeleton({ 'data-testid': testId }: { 'data-testid'?: string }) {
  return (
    <section className={styles.skeleton} data-testid={testId} aria-busy="true">
      <div className={styles.skeletonHeader} />
      <div className={styles.skeletonMetrics}>
        <div className={styles.skeletonTile} />
        <div className={styles.skeletonTile} />
        <div className={styles.skeletonTile} />
        <div className={styles.skeletonTile} />
      </div>
      <div className={styles.skeletonProfile} />
    </section>
  );
}

function RouteDetailData({ data }: { data: NormalizedTrackDetail }) {
  // Placeholder visible to the test contract. Commit 2 replaces this
  // with the real <RouteDetail /> composition. The data-testid is the
  // public hook that tests query against.
  return (
    <div data-testid="route-detail-data">
      <span data-testid="route-detail-name">{data.track.name}</span>
      <span data-testid="route-detail-distance-m">{data.analysis.distance_m}</span>
      <span data-testid="route-detail-climbs-count">{data.climbs.length}</span>
      <span data-testid="route-detail-risks-count">{data.risk_zones.length}</span>
    </div>
  );
}

// Avoid an unused-var lint warning while we still have the constant
// referenced in the test fixtures. (Removed once RouteDetail is wired.)
export const _SKELETON_TIMEOUT = SKELETON_TIMEOUT_MS;
