// TDD contract for the calendar activity feed client (#313, T3):
// GET /api/v1/workouts/calendar-activities with strict civil dates,
// bearer auth and RFC 9457 problem passthrough.

import { afterEach, describe, expect, it, vi } from 'vitest';
import { listCalendarActivities } from './calendar';

const response = (status: number, body?: unknown) =>
  ({
    ok: status >= 200 && status < 300,
    status,
    json: vi.fn(async () => body),
  }) as unknown as Response;

afterEach(() => vi.unstubAllGlobals());

describe('calendar API', () => {
  it('lists activities with inclusive civil-date parameters and bearer auth', async () => {
    const fetch = vi.fn().mockResolvedValue(
      response(200, {
        activities: [{ id: 'a1', name: 'Trail', scheduled_on: '2026-11-21', linked: false }],
      }),
    );
    vi.stubGlobal('fetch', fetch);
    await listCalendarActivities('token', '2026-10-26', '2026-12-06');
    expect(fetch.mock.calls[0]?.[0]).toBe(
      '/api/v1/workouts/calendar-activities?from=2026-10-26&to=2026-12-06',
    );
    expect(fetch.mock.calls[0]?.[1].headers.Authorization).toBe('Bearer token');
  });

  it('forwards the abort signal so stale range changes cancel in flight requests', async () => {
    const fetch = vi.fn().mockResolvedValue(response(200, { activities: [] }));
    vi.stubGlobal('fetch', fetch);
    const controller = new AbortController();
    await listCalendarActivities('token', '2026-10-26', '2026-12-06', controller.signal);
    expect(fetch.mock.calls[0]?.[1].signal).toBe(controller.signal);
  });

  it('exposes the problem detail of a failed feed', async () => {
    const fetch = vi
      .fn()
      .mockResolvedValue(response(400, { detail: 'from must be a valid YYYY-MM-DD date' }));
    vi.stubGlobal('fetch', fetch);
    await expect(listCalendarActivities('token', 'nonsense', '2026-12-06')).rejects.toMatchObject({
      status: 400,
      message: 'from must be a valid YYYY-MM-DD date',
    });
  });
});
