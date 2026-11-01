// TDD contract for the dashboard API client (Fase 1.4, DW-001).
// Checks the endpoints, the bearer header, the query parameters and
// the ApiError mapping. No network access: fetch is stubbed.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  fetchCardiacDrift,
  fetchDashboardSummary,
  fetchHRZones,
  fetchTrainingLoad,
  postRecalc,
} from './dashboard';
import { ApiError } from './types';

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as unknown as Response;
}

const SUMMARY = {
  weekly_volume_m: 12000.5,
  weekly_elevation_m: 340.25,
  weekly_activities_count: 4,
  trend_7d_pct: 5.5,
  trend_30d_pct: null,
};

describe('dashboard API client', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal('fetch', vi.fn());
  });

  it('fetches the summary with the bearer token', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse(SUMMARY));
    const data = await fetchDashboardSummary('token-123');
    expect(data).toEqual(SUMMARY);
    const [url, init] = vi.mocked(fetch).mock.calls[0];
    expect(url).toBe('/api/v1/dashboard/summary');
    expect(init?.headers).toMatchObject({ Authorization: 'Bearer token-123' });
  });

  it('fetches the training load with the from/to range', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({ from: '2026-01-01', to: '2026-01-31', series: [] }),
    );
    const data = await fetchTrainingLoad('token-123', '2026-01-01', '2026-01-31');
    expect(data.series).toEqual([]);
    const [url, init] = vi.mocked(fetch).mock.calls[0];
    expect(url).toContain('/api/v1/dashboard/load?');
    expect(url).toContain('from=2026-01-01');
    expect(url).toContain('to=2026-01-31');
    expect(init?.headers).toMatchObject({ Authorization: 'Bearer token-123' });
  });

  it('fetches the HR zones with the from/to range', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({ from: '2026-01-01', to: '2026-01-31', degraded: true, zones: [] }),
    );
    const data = await fetchHRZones('token-123', '2026-01-01', '2026-01-31');
    expect(data.degraded).toBe(true);
    const [url] = vi.mocked(fetch).mock.calls[0];
    expect(url).toContain('/api/v1/dashboard/hr-zones?');
    expect(url).toContain('from=2026-01-01');
    expect(url).toContain('to=2026-01-31');
  });

  it('fetches the cardiac drift with the from/to range', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({ from: '2026-01-01', to: '2026-01-31', available: false, activities: [] }),
    );
    const data = await fetchCardiacDrift('token-123', '2026-01-01', '2026-01-31');
    expect(data.available).toBe(false);
    const [url] = vi.mocked(fetch).mock.calls[0];
    expect(url).toContain('/api/v1/dashboard/cardiac-drift?');
    expect(url).toContain('from=2026-01-01');
    expect(url).toContain('to=2026-01-31');
  });

  it('posts the recalc job', async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse(
        { job_id: '42', queued_at: '2026-01-15T10:00:00Z', will_recompute_from: null },
        202,
      ),
    );
    const data = await postRecalc('token-123');
    expect(data.job_id).toBe('42');
    const [url, init] = vi.mocked(fetch).mock.calls[0];
    expect(url).toBe('/api/v1/dashboard/recalc');
    expect(init?.method).toBe('POST');
    expect(init?.headers).toMatchObject({ Authorization: 'Bearer token-123' });
  });

  it('throws an ApiError with the HTTP status when the request fails', async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse({}, 401));
    await expect(fetchDashboardSummary('token-123')).rejects.toBeInstanceOf(ApiError);
    await expect(fetchDashboardSummary('token-123')).rejects.toMatchObject({ status: 401 });
  });
});
