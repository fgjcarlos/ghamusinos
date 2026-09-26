// EmptyState: a centered empty-state container with optional icon,
// message, and one action. Issue 154. Presentation-only.

import type { ReactNode } from 'react';
import styles from './EmptyState.module.css';

export interface EmptyStateProps {
  icon?: ReactNode;
  message: string;
  action?: ReactNode;
}

export function EmptyState({ icon, message, action }: EmptyStateProps) {
  return (
    <div className={styles.empty} role="status">
      {icon ? <div className={styles.icon}>{icon}</div> : null}
      <p className={styles.message}>{message}</p>
      {action ? <div className={styles.action}>{action}</div> : null}
    </div>
  );
}
