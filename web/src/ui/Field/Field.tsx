// Field: wraps a form control with label, optional unit, and error state.
// Issue 154. Presentation-only: no fetch, no router, no useEffect.

import type { ReactNode } from 'react';
import styles from './Field.module.css';

export interface FieldProps {
  label: string;
  unit?: string;
  error?: string | undefined;
  children: ReactNode;
  errorId?: string | undefined;
}

export function Field({ label, unit, error, errorId, children }: FieldProps) {
  return (
    <label className={styles.field}>
      <span className={styles.label} data-testid="field-label">
        {label}
        {unit ? (
          <span className={styles.unit} data-testid="field-unit">
            ({unit})
          </span>
        ) : null}
      </span>
      <span className={styles.control}>{children}</span>
      {error ? (
        <span className={styles.error} id={errorId} role="alert" data-testid="field-error">
          {error}
        </span>
      ) : null}
    </label>
  );
}
