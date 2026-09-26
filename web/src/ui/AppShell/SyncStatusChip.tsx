// SyncStatusChip: indicator of the last Strava sync. Issue 155.
// In this issue, lastSync is passed by prop. In a follow-up (issue 06
// of the foundation backlog), the component will fetch /api/v1/sync/status
// itself. The presentational contract is the same either way.

import styles from './SyncStatusChip.module.css';

/**
 * formatRelativeTime returns a Spanish phrase for the elapsed time
 * between `date` and `now`. Used by SyncStatusChip and exported for
 * direct unit testing.
 *
 * Buckets: < 60 s -> "ahora"; < 60 min -> "hace N min";
 * < 24 h -> "hace N h"; otherwise -> "hace N d".
 */
export function formatRelativeTime(date: Date, now: Date = new Date()): string {
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  if (diffSec < 60) return 'ahora';
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `hace ${diffMin} min`;
  const diffH = Math.floor(diffMin / 60);
  if (diffH < 24) return `hace ${diffH} h`;
  const diffD = Math.floor(diffH / 24);
  return `hace ${diffD} d`;
}

export interface SyncStatusChipProps {
  lastSync: Date | null;
  /**
   * Reference clock for relative-time formatting. Defaults to `new Date()`.
   * Tests pass an explicit value to keep the output deterministic.
   */
  now?: Date;
}

export function SyncStatusChip({ lastSync, now }: SyncStatusChipProps) {
  if (lastSync === null) {
    return (
      <span
        data-testid="sync-status"
        data-state="idle"
        className={styles.idle}
        aria-label="Strava sin sincronizar"
      />
    );
  }
  const text = `Strava · ${formatRelativeTime(lastSync, now)}`;
  return (
    <span data-testid="sync-status" data-state="active" className={styles.active}>
      <span className={styles.dot} aria-hidden="true" />
      <span className={styles.text}>{text}</span>
    </span>
  );
}
