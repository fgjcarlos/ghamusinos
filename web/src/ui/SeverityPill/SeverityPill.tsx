// SeverityPill: marker for tramo risk. Issue 154.
// Severity is NEVER communicated by color alone: the label is always
// rendered as visible Spanish text.

import styles from './SeverityPill.module.css';

export type SeverityLevel = 'medium' | 'high';

const LABELS: Record<SeverityLevel, string> = {
  medium: 'Riesgo medio',
  high: 'Riesgo alto',
};

const levelClass: Record<SeverityLevel, string> = {
  medium: styles.medium,
  high: styles.high,
};

export interface SeverityPillProps {
  level: SeverityLevel;
}

export function SeverityPill({ level }: SeverityPillProps) {
  return (
    <span
      className={levelClass[level]}
      data-testid={`severity-${level}`}
      aria-label={LABELS[level]}
    >
      {LABELS[level]}
    </span>
  );
}
