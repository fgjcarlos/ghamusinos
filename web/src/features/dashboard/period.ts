// Period selector model for the dashboard (Fases 1.4, DW-001).
//
// Exactly four options — 7d, 30d, 90d, 1y — with 30d as the default.
// Every option maps to a window of days that ends today (UTC).

export type DashboardPeriod = '7d' | '30d' | '90d' | '1y';

export interface PeriodOption {
  id: DashboardPeriod;
  /** Label rendered in the selector. */
  label: string;
  /** Days covered by the window, today included. */
  days: number;
  /** Tooltip / screen-reader hint. */
  hint: string;
}

export const PERIOD_OPTIONS: readonly PeriodOption[] = [
  { id: '7d', label: '7d', days: 7, hint: 'Últimos 7 días' },
  { id: '30d', label: '30d', days: 30, hint: 'Últimos 30 días' },
  { id: '90d', label: '90d', days: 90, hint: 'Últimos 90 días' },
  { id: '1y', label: '1y', days: 365, hint: 'Último año' },
];

export const DEFAULT_PERIOD: DashboardPeriod = '30d';

/**
 * DateRange is the [from, to] window sent to the dashboard endpoints.
 */
export interface DateRange {
  from: string;
  to: string;
}

function toIsoDate(date: Date): string {
  return date.toISOString().slice(0, 10);
}

/**
 * periodRange computes the window of a period, ending today (UTC).
 * The window is inclusive: a 7d period covers today and the six
 * previous days.
 */
export function periodRange(period: DashboardPeriod, today: Date = new Date()): DateRange {
  const option = PERIOD_OPTIONS.find((candidate) => candidate.id === period);
  const days = option ? option.days : 30;
  const to = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), today.getUTCDate()));
  const from = new Date(to);
  from.setUTCDate(from.getUTCDate() - (days - 1));
  return { from: toIsoDate(from), to: toIsoDate(to) };
}
