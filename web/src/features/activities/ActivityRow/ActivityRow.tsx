// ActivityRow: one row of the activities grid. Wraps the row in a
// react-router-dom <Link> so the whole row is clickable (the entire
// surface is the navigation target). Issue 158.

import { Link } from 'react-router-dom';
import type { Activity, PgTypeNumeric } from '../../../lib/api/types';
import { HRZoneBars } from '../HRZoneBars/HRZoneBars';
import styles from './ActivityRow.module.css';

function pgNumericToNumber(n: PgTypeNumeric | null): number | null {
  if (!n || !n.valid) return null;
  return Number(BigInt(n.bigint)) * Math.pow(10, -n.exp) * n.sign;
}

function formatDistance(m: number | null): string {
  if (m === null) return '—';
  return `${(m / 1000).toFixed(1)} km`;
}

function formatElevation(m: number | null): string {
  if (m === null) return '—';
  return `${m.toFixed(0)} m`;
}

function formatDuration(totalSeconds: number): string {
  const h = Math.floor(totalSeconds / 3600);
  const m = Math.floor((totalSeconds % 3600) / 60);
  const s = totalSeconds % 60;
  if (h > 0) {
    return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
  }
  return `${m}:${s.toString().padStart(2, '0')}`;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('es-ES', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  });
}

export interface ActivityRowProps {
  activity: Activity;
  /** Five HR-zone percentages. Undefined → HRZoneBars renders "Sin FC". */
  zones?: number[];
}

export function ActivityRow({ activity, zones }: ActivityRowProps) {
  const distanceM = pgNumericToNumber(activity.distance_meters);
  const elevationM = pgNumericToNumber(activity.elevation_gain_m);
  const href = `/actividades/${activity.external_id}`;

  return (
    <Link to={href} className={styles.row} data-testid="activity-row">
      <span className={styles.date}>{formatDate(activity.started_at)}</span>
      <span className={styles.name}>{activity.name}</span>
      <span className={styles.sport}>{activity.sport_type}</span>
      <span className={styles.distance}>{formatDistance(distanceM)}</span>
      <span className={styles.duration}>{formatDuration(activity.moving_seconds)}</span>
      <span className={styles.elevation}>{formatElevation(elevationM)}</span>
      <span className={styles.zones}>
        <HRZoneBars {...(zones !== undefined ? { zones } : {})} />
      </span>
    </Link>
  );
}
