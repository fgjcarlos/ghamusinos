// HRZoneBars: 5 horizontal bars showing time-in-zone % for each HR zone.
// Issue 158. Pure presentational.

import styles from './HRZoneBars.module.css';

export interface HRZoneBarsProps {
  /** Five values 0..100. Undefined or empty renders the no-HR state. */
  zones?: number[];
}

export function HRZoneBars({ zones }: HRZoneBarsProps) {
  if (!zones || zones.length === 0) {
    return <span className={styles.empty}>Sin FC</span>;
  }
  const max = Math.max(...zones.slice(0, 5), 1);
  return (
    <div className={styles.bars} role="img" aria-label="Zonas de frecuencia cardíaca">
      {zones.slice(0, 5).map((value, i) => (
        <span
          key={i}
          className={styles.bar}
          style={{ height: `${(value / max) * 100}%` }}
          aria-label={`Zona ${i + 1}: ${value}%`}
        />
      ))}
    </div>
  );
}
