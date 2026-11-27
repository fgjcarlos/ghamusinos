// Client for the reports API (fase 2.4, #320 · docs/api/reports.md).
// Same shape as lib/api/races.ts: the module owns the bearer token and
// throws ApiError on any non-2xx response.

import { ApiError } from './types';

export type ReportKind = 'week' | 'month' | 'block';

export interface ReportSummary {
  tss: number;
  activities: number;
  distance_m: number;
  elevation_gain_m: number;
  avg_ef: number | null;
  avg_gap_sec_per_km: number | null;
  adherence: number | null;
}

/** One row of GET /reports: the row plus the summary of its payload. */
export interface ReportItem {
  id: string;
  kind: ReportKind;
  period_start: string;
  period_end: string;
  block_id: string | null;
  generated_at: string;
  summary: ReportSummary;
}

export interface ReportPage {
  data: ReportItem[];
  total: number;
  limit: number;
  offset: number;
  has_next: boolean;
}

export interface DailyLoad {
  date: string;
  tss: number;
  activity_count: number;
}

/** Payload version 1 (docs/architecture/report-snapshot.md). */
export interface ReportPayload {
  schema_version: number;
  kind: ReportKind;
  period_start: string;
  period_end: string;
  days: number;
  block: { id: string; name: string } | null;
  race: {
    id: string;
    name: string;
    race_date: string;
    priority: string;
    days_to_race: number;
  } | null;
  load: {
    tss: number;
    daily: DailyLoad[];
    ctl_start: number;
    atl_start: number;
    tsb_start: number;
    ctl_end: number;
    atl_end: number;
    tsb_end: number;
  };
  volume: {
    activities: number;
    distance_m: number;
    elevation_gain_m: number;
    moving_seconds: number;
  };
  performance: {
    avg_ef: number | null;
    ef_samples: number;
    avg_gap_sec_per_km: number | null;
    gap_samples: number;
  };
  plan: {
    planned: number;
    completed: number;
    skipped: number;
    modified: number;
    adherence: number | null;
  };
  previous: {
    period_start: string;
    period_end: string;
    activities: number;
    tss: number;
    distance_m: number;
    elevation_gain_m: number;
    avg_ef: number | null;
    avg_gap_sec_per_km: number | null;
  };
}

export interface ReportDetail {
  id: string;
  generated_at: string;
  payload: ReportPayload;
}

export interface GenerateReportInput {
  kind: ReportKind;
  date?: string;
  block_id?: string;
}

/** The period the generate request named, answered with 202. */
export interface QueuedReport {
  status: 'queued';
  kind: ReportKind;
  period_start: string;
  period_end: string;
}

const base = '/api/v1/reports';

async function request<T>(token: string, path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Authorization', `Bearer ${token}`);
  if (init.body) headers.set('Content-Type', 'application/json');
  const res = await fetch(path, { ...init, headers });
  if (!res.ok) throw new ApiError(res.status, `HTTP ${res.status}`);
  return res.status === 204 ? (undefined as T) : (res.json() as Promise<T>);
}

const options = (signal?: AbortSignal): RequestInit => (signal ? { signal } : {});

export const listReports = (token: string, kind: ReportKind, signal?: AbortSignal) =>
  request<ReportPage>(token, `${base}?kind=${kind}&limit=20&offset=0`, options(signal));

export const getReport = (token: string, id: string, signal?: AbortSignal) =>
  request<ReportDetail>(token, `${base}/${encodeURIComponent(id)}`, options(signal));

export const generateReport = (token: string, input: GenerateReportInput) =>
  request<QueuedReport>(token, `${base}/generate`, {
    method: 'POST',
    body: JSON.stringify(input),
  });
