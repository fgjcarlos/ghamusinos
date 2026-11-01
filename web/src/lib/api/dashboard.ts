/**
 * Dashboard API client — Fase 1.4 (issue #16, PR4 web slice).
 *
 * Mirrors the backend contracts served under /api/v1/dashboard:
 *
 *   - GET  /dashboard/summary        → DashboardSummary      (DA-001)
 *   - GET  /dashboard/load            → TrainingLoadSeries    (DA-002)
 *   - GET  /dashboard/hr-zones        → HRZonesResponse       (DA-003)
 *   - GET  /dashboard/cardiac-drift   → CardiacDriftResponse  (DA-007)
 *   - POST /dashboard/recalc          → RecalcResponse        (DA-004)
 *
 * Every function takes the bearer token as a parameter (no auth layer
 * is installed yet — see the note in ./strava.ts). Failures are raised
 * as ApiError so containers can map them to a message.
 */

import { ApiError } from './types';

const BASE_URL = '/api/v1';

/**
 * makeAuthHeader creates the Authorization header with the bearer token.
 */
function makeAuthHeader(token: string): HeadersInit {
  return {
    Authorization: `Bearer ${token}`,
  };
}

/**
 * handleResponse converts a Response to JSON or throws an ApiError.
 */
async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new ApiError(response.status, `HTTP ${response.status}`);
  }
  return response.json() as Promise<T>;
}

/**
 * Weekly KPIs plus the 7d/30d trends. Trends are `null` when the
 * previous window has no data (the backend refuses to divide by zero).
 */
export interface DashboardSummary {
  weekly_volume_m: number;
  weekly_elevation_m: number;
  weekly_activities_count: number;
  trend_7d_pct: number | null;
  trend_30d_pct: number | null;
}

/**
 * One day of the training-load series.
 */
export interface TrainingLoadPoint {
  /** ISO date (YYYY-MM-DD). */
  day: string;
  ctl: number;
  atl: number;
  tsb: number;
  /** Training Stress Score of the day. */
  tss: number;
}

/**
 * Daily CTL/ATL/TSB series for the requested window.
 */
export interface TrainingLoadSeries {
  from: string;
  to: string;
  series: TrainingLoadPoint[];
  /** Final values of the window; the backend also exposes them. */
  final_ctl?: number;
  final_atl?: number;
  final_tsb?: number;
}

/**
 * The five heart-rate buckets. The backend always returns z1..z5.
 */
export type HRZoneId = 'z1' | 'z2' | 'z3' | 'z4' | 'z5';

export interface HRZoneBucket {
  zone: HRZoneId;
  minutes: number;
}

/**
 * Minutes per HR zone. `degraded` is true when the backend had to
 * approximate the zones because no HR streams were ingested.
 */
export interface HRZonesResponse {
  from: string;
  to: string;
  degraded: boolean;
  zones: HRZoneBucket[];
}

/**
 * Cardiac drift of a single activity (first quartile vs third quartile
 * of the HR samples, in percent).
 */
export interface CardiacDriftActivity {
  activity_id: string;
  drift_pct: number;
  sample_count: number;
}

/**
 * Cardiac drift for the window. `available` is false when no activity
 * in the range carries enough HR samples to compute the drift.
 */
export interface CardiacDriftResponse {
  from: string;
  to: string;
  available: boolean;
  activities: CardiacDriftActivity[];
}

/**
 * Result of enqueuing a full-history recalculation job.
 */
export interface RecalcResponse {
  job_id: string;
  queued_at: string;
  will_recompute_from: string | null;
}

/**
 * fetchDashboardSummary fetches the weekly KPIs and the trends.
 * @param token Bearer token for authorization
 */
export async function fetchDashboardSummary(token: string): Promise<DashboardSummary> {
  const response = await fetch(`${BASE_URL}/dashboard/summary`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return handleResponse<DashboardSummary>(response);
}

/**
 * fetchTrainingLoad fetches the daily CTL/ATL/TSB series for a window.
 * @param token Bearer token for authorization
 * @param from Inclusive start date (YYYY-MM-DD)
 * @param to Inclusive end date (YYYY-MM-DD)
 */
export async function fetchTrainingLoad(
  token: string,
  from: string,
  to: string,
): Promise<TrainingLoadSeries> {
  const params = new URLSearchParams({ from, to });
  const response = await fetch(`${BASE_URL}/dashboard/load?${params}`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return handleResponse<TrainingLoadSeries>(response);
}

/**
 * fetchHRZones fetches the minutes per heart-rate zone for a window.
 * @param token Bearer token for authorization
 * @param from Inclusive start date (YYYY-MM-DD)
 * @param to Inclusive end date (YYYY-MM-DD)
 */
export async function fetchHRZones(
  token: string,
  from: string,
  to: string,
): Promise<HRZonesResponse> {
  const params = new URLSearchParams({ from, to });
  const response = await fetch(`${BASE_URL}/dashboard/hr-zones?${params}`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return handleResponse<HRZonesResponse>(response);
}

/**
 * fetchCardiacDrift fetches the per-activity cardiac drift for a window.
 * @param token Bearer token for authorization
 * @param from Inclusive start date (YYYY-MM-DD)
 * @param to Inclusive end date (YYYY-MM-DD)
 */
export async function fetchCardiacDrift(
  token: string,
  from: string,
  to: string,
): Promise<CardiacDriftResponse> {
  const params = new URLSearchParams({ from, to });
  const response = await fetch(`${BASE_URL}/dashboard/cardiac-drift?${params}`, {
    method: 'GET',
    headers: makeAuthHeader(token),
  });
  return handleResponse<CardiacDriftResponse>(response);
}

/**
 * postRecalc enqueues a full-history recalculation of the training load.
 * @param token Bearer token for authorization
 */
export async function postRecalc(token: string): Promise<RecalcResponse> {
  const response = await fetch(`${BASE_URL}/dashboard/recalc`, {
    method: 'POST',
    headers: { ...makeAuthHeader(token), 'Content-Type': 'application/json' },
  });
  return handleResponse<RecalcResponse>(response);
}
