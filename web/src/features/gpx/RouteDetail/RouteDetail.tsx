// RouteDetail: composes the route detail page. Issue 157.
// Pure presentational. Lives inside the AppShell via /rutas/:id.
// The data it receives is already normalized (no pgtype).

import { ElevationProfile } from '../ElevationProfile';
import { projectTrack } from '../ElevationProfile/projectTrack';
import { RouteHeader } from '../RouteHeader/RouteHeader';
import { RouteMetrics } from '../RouteMetrics/RouteMetrics';
import { RouteClimbs } from '../RouteClimbs/RouteClimbs';
import { RouteRisks } from '../RouteRisks/RouteRisks';
import type { NormalizedTrackDetail } from '../normalize';
import styles from './RouteDetail.module.css';

export interface RouteDetailProps {
  data: NormalizedTrackDetail;
}

export function RouteDetail({ data }: RouteDetailProps) {
  // The backend doesn't yet return the raw points array in the detail
  // response (TODO in the issue: separate /api/v1/gpx/{id}/points or
  // include them in the existing response). Until then, projectTrack
  // receives an empty points array — the elevation profile renders
  // just axes and ticks. We still pass the climbs and risk_zones so
  // their bands appear on the chart.
  const projected = projectTrack(
    [],
    data.climbs.map((c) => ({
      start_idx: c.start_idx,
      end_idx: c.end_idx,
      gain_m: 0,
    })),
    data.risk_zones.map((r) => ({ start_idx: r.start_idx, end_idx: r.end_idx })),
    { width: 960, height: 200 },
  );

  return (
    <article className={styles.detail} data-testid="route-detail">
      <RouteHeader track={data.track} analysis={data.analysis} />
      <section className={styles.profileSection}>
        <ElevationProfile projected={projected} width={960} height={200} />
      </section>
      <RouteMetrics analysis={data.analysis} />
      <div className={styles.columns}>
        <RouteClimbs climbs={data.climbs} />
        <RouteRisks risks={data.risk_zones} />
      </div>
    </article>
  );
}
