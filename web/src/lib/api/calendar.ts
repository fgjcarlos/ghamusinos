// Calendar activity feed client (#313, T3). The endpoint lives under
// /workouts because it is the workouts feature's bounded read model for
// the calendar: same strict civil-date contract, same bearer auth.

import { ApiError } from './types';

const BASE_URL = '/api/v1/workouts/calendar-activities';

/** One real activity as the calendar sees it (#313, T3): the civil
 * date is already resolved in the user's stored IANA zone and linked
 * marks activities justifying any session of the user. */
export interface CalendarActivity {
  id: string;
  name: string;
  scheduled_on: string;
  linked: boolean;
}

async function json<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let problem: { detail?: string; title?: string } = {};
    try {
      const body: unknown = await response.json();
      if (body && typeof body === 'object' && !Array.isArray(body)) {
        problem = body as typeof problem;
      }
    } catch {
      /* use status fallback */
    }
    throw new ApiError(
      response.status,
      problem.detail ?? problem.title ?? `HTTP ${response.status}`,
    );
  }
  return response.json() as Promise<T>;
}

export async function listCalendarActivities(
  token: string,
  from: string,
  to: string,
  signal?: AbortSignal,
): Promise<{ activities: CalendarActivity[] }> {
  const query = new URLSearchParams({ from, to });
  const init: RequestInit = { method: 'GET', headers: { Authorization: `Bearer ${token}` } };
  if (signal) init.signal = signal;
  return json<{ activities: CalendarActivity[] }>(await fetch(`${BASE_URL}?${query}`, init));
}

export type ComparisonGroup = 'day' | 'week';

type LoadTotals = { duration_seconds: number; distance_m: number; elevation_gain_m: number };

export interface ComparisonPeriod {
  period_start: string;
  planned: LoadTotals;
  actual: LoadTotals;
  compliance: { total: number; completed: number; skipped: number; modified: number };
}

export async function loadCalendarComparison(
  token: string,
  from: string,
  to: string,
  group: ComparisonGroup,
  signal: AbortSignal,
): Promise<{ periods: ComparisonPeriod[] }> {
  const query = new URLSearchParams({ from, to, group });
  return json(
    await fetch(`/api/v1/workouts/compare?${query}`, {
      headers: { Authorization: `Bearer ${token}` },
      signal,
    }),
  );
}
