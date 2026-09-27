// RouteHeader: top section of the route detail page. Issue 157.
// Pure presentational. Renders name, type chip, file size, and the
// difficulty badge with score. Action buttons (back to /lab, edit)
// are placeholders for now — they'll be wired in #125.

import { Link } from 'react-router-dom';
import { DifficultyBadge } from '../../../ui/DifficultyBadge';
import { Chip } from '../../../ui/Chip';
import type { DifficultyLabel, NormalizedAnalysis, NormalizedInnerTrack } from '../normalize';
import styles from './RouteHeader.module.css';

export interface RouteHeaderProps {
  track: NormalizedInnerTrack;
  analysis: NormalizedAnalysis;
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} kB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function RouteHeader({ track, analysis }: RouteHeaderProps) {
  return (
    <header className={styles.header}>
      <div className={styles.row}>
        <h1 className={styles.title}>{track.name}</h1>
        <div className={styles.actions}>
          <Link to="/lab" className={styles.backLink}>
            ← Laboratorio
          </Link>
        </div>
      </div>
      <div className={styles.meta}>
        <Chip tone="accent">{track.track_type}</Chip>
        <Chip>{formatFileSize(track.file_size_bytes)}</Chip>
        <span className={styles.uploaded}>
          Subido el {track.uploaded_at.toLocaleDateString('es-ES')}
        </span>
      </div>
      <div className={styles.difficulty}>
        <DifficultyBadge
          level={difficultyFromLabel(analysis.difficulty_label)}
          score={analysis.difficulty_score}
        />
      </div>
      <p className={styles.disclaimer}>
        Las métricas derivadas son estimaciones orientativas, no diagnósticos.
      </p>
    </header>
  );
}

function difficultyFromLabel(
  label: DifficultyLabel,
): 'beginner' | 'intermediate' | 'advanced' | 'pro' {
  switch (label) {
    case 'easy':
      return 'beginner';
    case 'moderate':
      return 'intermediate';
    case 'hard':
      return 'advanced';
    case 'extreme':
      return 'pro';
  }
}
