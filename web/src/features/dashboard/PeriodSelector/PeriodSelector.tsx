// PeriodSelector: the four dashboard windows (7d, 30d, 90d, 1y).
// Fases 1.4 (issue #16, DW-001). Pure presentational — the page owns
// the selected period and the resulting from/to window.

import { PERIOD_OPTIONS, type DashboardPeriod } from '../period';
import styles from './PeriodSelector.module.css';

export interface PeriodSelectorProps {
  /** Currently selected period. */
  value: DashboardPeriod;
  /** Called with the new period when the user picks another window. */
  onChange: (period: DashboardPeriod) => void;
}

export function PeriodSelector({ value, onChange }: PeriodSelectorProps) {
  return (
    <div
      className={styles.selector}
      role="group"
      aria-label="Periodo"
      data-testid="period-selector"
    >
      {PERIOD_OPTIONS.map((option) => {
        const selected = option.id === value;
        return (
          <button
            key={option.id}
            type="button"
            className={selected ? styles.active : styles.option}
            aria-pressed={selected}
            title={option.hint}
            data-testid={`period-option-${option.id}`}
            onClick={() => onChange(option.id)}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
