// ProfileLabel: floating label above a band (e.g. "King climb: +340 m").
// Issue 156. Renders an SVG <text> centered horizontally over the band
// and positioned just above it.

import type { ProjectedBand } from './projectTrack';
import styles from './ProfileLabel.module.css';

export interface ProfileLabelProps {
  band: ProjectedBand;
  /** y coordinate where the label sits (typically top of band). */
  y: number;
  /** Visible text. */
  children: string;
}

export function ProfileLabel({ band, y, children }: ProfileLabelProps) {
  const centerX = band.x + band.width / 2;
  return (
    <text x={centerX} y={y - 4} textAnchor="middle" className={styles.label}>
      {children}
    </text>
  );
}
