// ProfileBand: vertical highlight rectangle for climbs and risk zones.
// Issue 156. Renders an SVG <rect> positioned and sized by the band
// metadata. The visual treatment (color, opacity) lives in CSS.

import type { ProjectedBand } from './projectTrack';
import styles from './ProfileBand.module.css';

export interface ProfileBandProps {
  band: ProjectedBand;
  /** y coordinate of the band's top edge. Defaults to PADDING.top. */
  y?: number;
  /** Band height. Should equal the plot height. */
  height: number;
  /** Optional human-readable label, exposed as aria-label and <title>. */
  label?: string;
}

const DEFAULT_BAND_TOP = 12;

export function ProfileBand({ band, y = DEFAULT_BAND_TOP, height, label }: ProfileBandProps) {
  const className = band.type === 'climb' ? styles.climb : styles.risk;
  return (
    <rect
      x={band.x}
      y={y}
      width={band.width}
      height={height}
      className={className}
      role="img"
      aria-label={label}
    >
      <title>{label}</title>
    </rect>
  );
}
