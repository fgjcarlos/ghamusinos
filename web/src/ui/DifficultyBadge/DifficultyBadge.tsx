// DifficultyBadge: span with Spanish label + numeric score. Issue 154.
// Colour and difficulty are NEVER communicated by color alone:
// the label is always rendered as visible Spanish text.

import styles from './DifficultyBadge.module.css';

export type DifficultyLevel = 'beginner' | 'intermediate' | 'advanced' | 'pro';

const LABELS: Record<DifficultyLevel, string> = {
  beginner: 'Inicial',
  intermediate: 'Intermedio',
  advanced: 'Avanzado',
  pro: 'Pro',
};

const levelClass: Record<DifficultyLevel, string> = {
  beginner: styles.beginner,
  intermediate: styles.intermediate,
  advanced: styles.advanced,
  pro: styles.pro,
};

export interface DifficultyBadgeProps {
  level: DifficultyLevel;
  score: number; // 0..100 inclusive; clamped at render
}

function clampScore(score: number): number {
  if (Number.isNaN(score)) return 0;
  return Math.max(0, Math.min(100, Math.trunc(score)));
}

export function DifficultyBadge({ level, score }: DifficultyBadgeProps) {
  const safe = clampScore(score);
  const text = `${LABELS[level]} ${safe}`;
  return (
    <span className={levelClass[level]} data-testid={`difficulty-${level}`} aria-label={text}>
      {text}
    </span>
  );
}
