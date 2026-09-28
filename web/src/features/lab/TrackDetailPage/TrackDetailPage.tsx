// TrackDetailPage: top-level page for /rutas/:id (and the legacy
// /lab/:id alias). Issue 125. Wraps MapView on top and the existing
// RouteDetail composition below.

import { useMemo } from 'react';
import { MapView, type LngLat } from '../MapView/MapView';
import { RouteDetail } from '../../gpx/RouteDetail/RouteDetail';
import type { NormalizedTrackDetail } from '../../gpx/normalize';
import styles from './TrackDetailPage.module.css';

export interface TrackDetailPageProps {
  data: NormalizedTrackDetail;
  mapHeight?: number;
}

export function TrackDetailPage({ data, mapHeight = 360 }: TrackDetailPageProps) {
  // The backend points are unknown at this boundary; accept its `lon` wire key
  // or the TS `lng` alias and normalize coordinates for GeoJSON.
  const coordinates = useMemo<LngLat[]>(() => {
    return data.track.points
      .map((p) => {
        if (
          typeof p === 'object' &&
          p !== null &&
          'lat' in p &&
          typeof (p as { lat?: unknown }).lat === 'number' &&
          Number.isFinite((p as { lat: number }).lat)
        ) {
          const point = p as { lat: number; lng?: unknown; lon?: unknown };
          const lng = Number.isFinite(point.lng) ? point.lng : point.lon;
          if (typeof lng !== 'number' || !Number.isFinite(lng)) return null;
          // GeoJSON convention is [lng, lat].
          return [lng, point.lat];
        }
        return null;
      })
      .filter((c): c is LngLat => c !== null);
  }, [data.track.points]);

  return (
    <main className={styles.page} data-testid="track-detail-page">
      <div className={styles.mapSection}>
        {coordinates.length >= 2 ? (
          <MapView coordinates={coordinates} height={mapHeight} />
        ) : (
          // Empty state while the backend hasn't returned points yet.
          <div
            className={styles.mapEmpty}
            data-testid="map-empty"
            role="region"
            aria-label="Mapa no disponible"
          >
            <p>
              El mapa se mostrará aquí cuando los puntos del track estén disponibles en la respuesta
              del backend.
            </p>
          </div>
        )}
      </div>
      <RouteDetail data={data} />
    </main>
  );
}
