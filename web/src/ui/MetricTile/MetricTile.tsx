// MetricTile: a card-shaped label + large value + optional unit.
// Issue 154. Presentation-only.

import styles from './MetricTile.module.css';

export interface MetricTileProps {
  label: string;
  value: string | number;
  unit?: string;
}

export function MetricTile({ label, value, unit }: MetricTileProps) {
  return (
    <div className={styles.tile}>
      <span className={styles.label}>{label}</span>
      <span className={styles.valueRow}>
        <span className={styles.value}>{value}</span>
        {unit ? <span className={styles.unit}>{unit}</span> : null}
      </span>
    </div>
  );
}
