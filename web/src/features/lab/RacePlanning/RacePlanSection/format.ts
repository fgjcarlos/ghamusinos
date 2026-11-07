/**
 * Presentation helpers for the race-planning section.
 *
 * Kept out of the `.tsx` files so the components stay pure JSX and the
 * formatting rules are testable in isolation.
 */

import type { WeatherRole } from '../../../../lib/api/lab';
import type { SolarSegment } from '../model/types';

/** WMO weather codes the lab treats as rain or storms. */
export const RAINY_WEATHER_CODES: ReadonlySet<number> = new Set([
  51, 53, 55, 61, 63, 65, 80, 81, 82, 95, 96, 99,
]);

export function isRainyCode(code: number): boolean {
  return RAINY_WEATHER_CODES.has(code);
}

/** `Xh Ym`, rounding to the nearest minute. */
export function formatDuration(minutes: number): string {
  if (!Number.isFinite(minutes) || minutes <= 0) return '0h 0m';
  const totalMinutes = Math.round(minutes);
  return `${Math.floor(totalMinutes / 60)}h ${totalMinutes % 60}m`;
}

/** `+Xh Ym` / `-Xh Ym`, used for cutoff margins. */
export function formatSignedDuration(minutes: number): string {
  if (!Number.isFinite(minutes)) return '—';
  const sign = minutes < 0 ? '-' : '+';
  return `${sign}${formatDuration(Math.abs(minutes))}`;
}

/** `m:ss /km`, rounding to the nearest second. */
export function formatPace(minPerKm: number): string {
  if (!Number.isFinite(minPerKm) || minPerKm <= 0) return '—';
  const totalSeconds = Math.round(minPerKm * 60);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, '0')} /km`;
}

function pad(value: number): string {
  return String(value).padStart(2, '0');
}

/** `YYYY-MM-DDTHH:mm` for a `datetime-local` input, local time. */
export function toDateTimeLocalValue(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`;
}

/** Default race start: today at 08:00 local time. */
export function defaultRaceDateTimeLocal(now: Date = new Date()): string {
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}T08:00`;
}

/**
 * Converts a `datetime-local` value (interpreted as local time) into an ISO
 * instant. Returns `null` when the value is empty or not a valid date.
 */
export function isoFromLocalDateTime(value: string): string | null {
  if (value.trim() === '') return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date.toISOString();
}

export function weatherRoleLabel(role: WeatherRole): string {
  switch (role) {
    case 'start':
      return 'Salida';
    case 'highest':
      return 'Cima';
    case 'midpoint':
      return 'Intermedio';
    case 'end':
      return 'Meta';
  }
}

/** Maps the backend terrain kind labels to Spanish display text. */
export function surfaceLabel(surface: string): string {
  switch (surface) {
    case 'paved':
      return 'Asfalto';
    case 'gravel':
      return 'Grava';
    case 'trail':
      return 'Sendero';
    case 'technical':
      return 'Técnico';
    case 'unknown':
      return 'Desconocido';
    default:
      return surface;
  }
}

export function solarStatusLabel(status: SolarSegment['status']): string {
  switch (status) {
    case 'sun':
      return 'Sol';
    case 'shade':
      return 'Sombra';
    case 'night':
      return 'Noche';
  }
}
