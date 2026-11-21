// Civil-date model for the interactive calendar (#313, T3).
//
// Civil dates are 'YYYY-MM-DD' strings. Every instant→date conversion
// goes through the user's stored IANA zone with Intl (mirroring the
// backend LocalDate helper): we never parse dates in the browser's
// local zone and never construct a local midnight, which some zones
// skip or repeat. Weekday and month math runs on UTC carriers
// (`${key}T00:00:00Z`), which are exact by construction.

const utcCarrier = (key: string) => new Date(`${key}T00:00:00Z`);
const keyOf = (d: Date) => d.toISOString().slice(0, 10);

/** Resolves "today" as the civil date of `now` in `timeZone`. */
export function todayKey(now: Date, timeZone: string): string {
  // en-CA formats as YYYY-MM-DD.
  return new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(now);
}

/** Stored zones come from the API; an invalid one falls back to UTC. */
export function resolveZone(timeZone: string): string {
  try {
    new Intl.DateTimeFormat('en-CA', { timeZone }).format(new Date());
    return timeZone;
  } catch {
    return 'UTC';
  }
}

export function addDays(key: string, days: number): string {
  return keyOf(new Date(utcCarrier(key).getTime() + days * 86_400_000));
}

export function addMonths(key: string, months: number): string {
  const d = utcCarrier(key);
  const target = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + months + 1, 0));
  const lastDay = target.getUTCDate();
  return keyOf(
    new Date(
      Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + months, Math.min(d.getUTCDate(), lastDay)),
    ),
  );
}

/** Monday-based weeks (decision L2): lunes = inicio de semana. */
export function weekStart(key: string): string {
  const offset = (utcCarrier(key).getUTCDay() + 6) % 7;
  return addDays(key, -offset);
}

export function weekDays(anchor: string): string[] {
  const start = weekStart(anchor);
  return Array.from({ length: 7 }, (_, i) => addDays(start, i));
}

/** Fixed six-week month grid starting on the Monday of the week of day 1. */
export function monthDays(anchor: string): string[] {
  const [y, m] = anchor.split('-');
  const start = weekStart(`${y}-${m}-01`);
  return Array.from({ length: 42 }, (_, i) => addDays(start, i));
}

export function rangeOf(days: string[]): { from: string; to: string } {
  return { from: days[0], to: days[days.length - 1] };
}

const monthName = (key: string) =>
  new Intl.DateTimeFormat('es-ES', { month: 'long', timeZone: 'UTC' }).format(utcCarrier(key));
const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

export function monthLabel(anchor: string): string {
  const year = utcCarrier(anchor).getUTCFullYear();
  return `${capitalize(monthName(anchor))} de ${year}`;
}

export function weekLabel(anchor: string): string {
  const days = weekDays(anchor);
  const first = utcCarrier(days[0]);
  const last = utcCarrier(days[6]);
  const d1 = first.getUTCDate();
  const d2 = last.getUTCDate();
  const y2 = last.getUTCFullYear();
  if (first.getUTCMonth() === last.getUTCMonth()) {
    return `${d1} – ${d2} de ${monthName(days[6])} de ${y2}`;
  }
  const y1 = first.getUTCFullYear();
  return `${d1} de ${monthName(days[0])}${y1 !== y2 ? ` de ${y1}` : ''} – ${d2} de ${monthName(days[6])} de ${y2}`;
}

export const dayNumber = (key: string) => Number(key.slice(8, 10));

/** Long Spanish label ('lunes, 23 de noviembre') from a UTC carrier. */
export const dayLabel = (key: string) =>
  new Intl.DateTimeFormat('es-ES', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    timeZone: 'UTC',
  }).format(utcCarrier(key));

export const weekdayHeaders = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom'];
