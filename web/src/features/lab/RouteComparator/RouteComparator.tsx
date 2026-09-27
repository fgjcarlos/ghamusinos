// RouteComparator: page-level composition for the route comparator.
// Issue 126. Pure presentational — composes ComparisonMap,
// MetricsDiffTable, RiskZonesPanel and ComparisonElevationProfile
// from the fetched CompareResponse.

import { useMemo } from 'react';
import type { CompareResponse, GpxRiskZone } from '../../../lib/api/types';
import { ComparisonMap, type LngLat } from '../ComparisonMap/ComparisonMap';
import { MetricsDiffTable } from '../MetricsDiffTable/MetricsDiffTable';
import { RiskZonesPanel } from '../RiskZonesPanel/RiskZonesPanel';
import { ComparisonElevationProfile } from '../ComparisonElevationProfile/ComparisonElevationProfile';
import styles from './RouteComparator.module.css';

export interface RouteComparatorProps {
  data: CompareResponse;
}

interface FlatPoint {
  lat: number;
  lng: number;
  ele: number | null;
}

interface RawFlatPoint {
  lat: number;
  lng?: number;
  lon?: number;
  ele?: number | null;
}

function isFlatPoint(p: unknown): p is RawFlatPoint {
  if (typeof p !== 'object' || p === null) return false;
  const o = p as { lat?: unknown; lng?: unknown; lon?: unknown; ele?: unknown };
  const lng = Number.isFinite(o.lng) ? o.lng : o.lon;
  return (
    typeof o.lat === 'number' &&
    Number.isFinite(o.lat) &&
    typeof lng === 'number' &&
    Number.isFinite(lng) &&
    (o.ele === undefined || o.ele === null || typeof o.ele === 'number')
  );
}

function normalizeFlatPoint(point: RawFlatPoint): FlatPoint {
  return {
    lat: point.lat,
    lng: (Number.isFinite(point.lng) ? point.lng : point.lon) as number,
    ele: point.ele ?? null,
  };
}

export function RouteComparator({ data }: RouteComparatorProps) {
  const trackNames = useMemo(() => data.tracks.map((t) => t.track.track.name), [data]);

  const coordsByTrack = useMemo<LngLat[][]>(
    () =>
      data.tracks.map((t) => {
        const list = Array.isArray(t.track.track.points) ? t.track.track.points : [];
        return list
          .filter(isFlatPoint)
          .map(normalizeFlatPoint)
          .map((p) => [p.lng, p.lat] as LngLat);
      }),
    [data],
  );

  const tracksForMap = useMemo(
    () => trackNames.map((name, i) => ({ name, coordinates: coordsByTrack[i] })),
    [trackNames, coordsByTrack],
  );

  const tracksForProfile = useMemo(
    () =>
      data.tracks.map((t) => ({
        name: t.track.track.name,
        points: (Array.isArray(t.track.track.points) ? t.track.track.points : [])
          .filter(isFlatPoint)
          .map(normalizeFlatPoint),
      })),
    [data],
  );

  const flatZones = useMemo<{ trackName: string; zone: GpxRiskZone }[]>(
    () =>
      data.tracks.flatMap((t) =>
        t.risk_zones.map((zone) => ({
          trackName: t.track.track.name,
          zone,
        })),
      ),
    [data],
  );

  return (
    <article
      className={styles.comparator}
      data-testid="route-comparator"
      aria-label="Comparador de rutas"
    >
      <header className={styles.header}>
        <h1 className={styles.title}>Comparador de rutas</h1>
        <p className={styles.meta}>{trackNames.length} tracks comparados</p>
      </header>

      <ComparisonMap tracks={tracksForMap} />

      <MetricsDiffTable metrics={data.diff} trackNames={trackNames} />

      <div className={styles.columns}>
        <RiskZonesPanel zones={flatZones} />
        <ComparisonElevationProfile tracks={tracksForProfile} />
      </div>
    </article>
  );
}
