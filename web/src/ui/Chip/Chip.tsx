// Chip: small bordered label for metadata. Issue 154.

import type { ReactNode } from 'react';
import styles from './Chip.module.css';

export type ChipTone = 'neutral' | 'accent';

const toneClass: Record<ChipTone, string> = {
  neutral: styles.neutral,
  accent: styles.accent,
};

export interface ChipProps {
  tone?: ChipTone;
  children: ReactNode;
}

export function Chip({ tone = 'neutral', children }: ChipProps) {
  return <span className={toneClass[tone]}>{children}</span>;
}
