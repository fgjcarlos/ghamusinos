// Format helpers of the reports feature. The number and distance
// formatters are the races ones: both screens show the same numbers.

import type { ReportKind } from '../../lib/api/reports';
import { format } from '../races/race-format';

export const kindLabel: Record<ReportKind, string> = {
  week: 'semana',
  month: 'mes',
  block: 'bloque',
};

export const periodLabel = (start: string, end: string) => `${start} – ${end}`;

/**
 * localDay is the civil day of the user's own clock, as the API expects.
 * `toISOString()` is UTC: west of UTC it names the previous day for most
 * of the evening, so the selector default came out a day early.
 */
export const localDay = (now = new Date()) =>
  `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;

/** GAP in seconds per km as minutes and seconds; null shows no data. */
export function pace(value: number | null) {
  if (value === null) return 'Sin datos';
  const total = Math.round(value);
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')} min/km`;
}

/** The API sends adherence as a 0–1 ratio; null means no planned sessions. */
export const percent = (value: number | null) =>
  value === null ? 'Sin datos' : `${format.format(value * 100)} %`;

export const ef = (value: number | null) => (value === null ? 'Sin datos' : format.format(value));
